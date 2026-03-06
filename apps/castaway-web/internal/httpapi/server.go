package httpapi

import (
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/conv"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/scoring"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func New(pool *pgxpool.Pool) *Server {
	return &Server{pool: pool, queries: db.New(pool)}
}

func (s *Server) Router() *gin.Engine {
	r := gin.Default()

	r.GET("/healthz", s.health)

	r.GET("/instances", s.listInstances)
	r.POST("/instances", s.createInstance)
	r.POST("/instances/import", s.importInstance)
	r.GET("/instances/:instanceID", s.getInstance)
	r.POST("/instances/:instanceID/contestants", s.createContestant)
	r.GET("/instances/:instanceID/contestants", s.listContestants)

	r.POST("/instances/:instanceID/participants", s.createParticipant)
	r.GET("/instances/:instanceID/participants", s.listParticipants)

	r.PUT("/instances/:instanceID/drafts/:participantID", s.replaceDraft)
	r.GET("/instances/:instanceID/drafts/:participantID", s.getDraft)

	r.PUT("/instances/:instanceID/outcomes/:position", s.upsertOutcome)
	r.GET("/instances/:instanceID/outcomes", s.listOutcomes)

	r.GET("/instances/:instanceID/leaderboard", s.leaderboard)

	return r
}

type errorResponse struct {
	Error string `json:"error"`
}

func (s *Server) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

type instanceResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Season    int32  `json:"season"`
	CreatedAt string `json:"created_at"`
}

func toInstanceResponse(id pgtype.UUID, name string, season int32, createdAt pgtype.Timestamptz) instanceResponse {
	return instanceResponse{
		ID:        uuid.UUID(id.Bytes).String(),
		Name:      name,
		Season:    season,
		CreatedAt: createdAt.Time.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}

func (s *Server) listInstances(c *gin.Context) {
	seasonFilter, ok := parseOptionalSeasonQuery(c)
	if !ok {
		return
	}
	nameFilter := strings.TrimSpace(c.Query("name"))

	instances, err := s.queries.ListInstances(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	response := make([]instanceResponse, 0, len(instances))
	for _, instance := range instances {
		if seasonFilter != nil && instance.Season != *seasonFilter {
			continue
		}
		if !matchesContainsFold(instance.Name, nameFilter) {
			continue
		}
		response = append(response, toInstanceResponse(instance.ID, instance.Name, instance.Season, instance.CreatedAt))
	}

	c.JSON(http.StatusOK, gin.H{"instances": response})
}

type createInstanceRequest struct {
	Name        string   `json:"name" binding:"required"`
	Season      int32    `json:"season" binding:"required"`
	Contestants []string `json:"contestants"`
}

func (s *Server) createInstance(c *gin.Context) {
	var req createInstanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	tx, err := s.pool.Begin(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	defer func() {
		rollbackErr := tx.Rollback(c.Request.Context())
		if rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			if ginErr := c.Error(rollbackErr); ginErr != nil {
				ginErr.Type = gin.ErrorTypePrivate
			}
		}
	}()

	qtx := s.queries.WithTx(tx)
	createdInstance, err := qtx.CreateInstance(c.Request.Context(), db.CreateInstanceParams{
		Name:   req.Name,
		Season: req.Season,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	for _, contestantName := range req.Contestants {
		if contestantName == "" {
			continue
		}
		_, err := qtx.CreateContestant(c.Request.Context(), db.CreateContestantParams{
			InstanceID: createdInstance.ID,
			Name:       contestantName,
		})
		if err != nil {
			c.JSON(statusFromPg(err), errorResponse{Error: err.Error()})
			return
		}
	}

	if err := tx.Commit(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"instance": toInstanceResponse(createdInstance.ID, createdInstance.Name, createdInstance.Season, createdInstance.CreatedAt)})
}

func (s *Server) getInstance(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok {
		return
	}

	instance, err := s.queries.GetInstance(c.Request.Context(), toPGUUID(instanceID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, errorResponse{Error: "instance not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	contestants, err := s.queries.ListContestantsByInstance(c.Request.Context(), toPGUUID(instanceID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	participantRows, err := s.queries.ListParticipantsByInstance(c.Request.Context(), toPGUUID(instanceID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	contestantResponse := make([]gin.H, 0, len(contestants))
	for _, contestant := range contestants {
		contestantResponse = append(contestantResponse, gin.H{
			"id":   uuid.UUID(contestant.ID.Bytes).String(),
			"name": contestant.Name,
		})
	}

	participantResponse := make([]gin.H, 0, len(participantRows))
	for _, participant := range participantRows {
		participantResponse = append(participantResponse, gin.H{
			"id":   uuid.UUID(participant.ID.Bytes).String(),
			"name": participant.Name,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"instance":     toInstanceResponse(instance.ID, instance.Name, instance.Season, instance.CreatedAt),
		"contestants":  contestantResponse,
		"participants": participantResponse,
	})
}

type createContestantRequest struct {
	Name string `json:"name" binding:"required"`
}

func (s *Server) createContestant(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok {
		return
	}

	var req createContestantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	contestant, err := s.queries.CreateContestant(c.Request.Context(), db.CreateContestantParams{
		InstanceID: toPGUUID(instanceID),
		Name:       req.Name,
	})
	if err != nil {
		c.JSON(statusFromPg(err), errorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"contestant": gin.H{
		"id":   uuid.UUID(contestant.ID.Bytes).String(),
		"name": contestant.Name,
	}})
}

func (s *Server) listContestants(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok {
		return
	}

	contestantRows, err := s.queries.ListContestantsByInstance(c.Request.Context(), toPGUUID(instanceID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	contestants := make([]gin.H, 0, len(contestantRows))
	for _, contestant := range contestantRows {
		contestants = append(contestants, gin.H{
			"id":   uuid.UUID(contestant.ID.Bytes).String(),
			"name": contestant.Name,
		})
	}

	c.JSON(http.StatusOK, gin.H{"contestants": contestants})
}

type createParticipantRequest struct {
	Name string `json:"name" binding:"required"`
}

func (s *Server) createParticipant(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok {
		return
	}

	var req createParticipantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	participant, err := s.queries.CreateParticipant(c.Request.Context(), db.CreateParticipantParams{
		InstanceID: toPGUUID(instanceID),
		Name:       req.Name,
	})
	if err != nil {
		c.JSON(statusFromPg(err), errorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"participant": gin.H{
		"id":   uuid.UUID(participant.ID.Bytes).String(),
		"name": participant.Name,
	}})
}

func (s *Server) listParticipants(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok {
		return
	}
	nameFilter := strings.TrimSpace(c.Query("name"))

	participantRows, err := s.queries.ListParticipantsByInstance(c.Request.Context(), toPGUUID(instanceID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	participants := make([]gin.H, 0, len(participantRows))
	for _, participant := range participantRows {
		if !matchesContainsFold(participant.Name, nameFilter) {
			continue
		}
		participants = append(participants, gin.H{
			"id":   uuid.UUID(participant.ID.Bytes).String(),
			"name": participant.Name,
		})
	}

	c.JSON(http.StatusOK, gin.H{"participants": participants})
}

type replaceDraftRequest struct {
	ContestantIDs []string `json:"contestant_ids" binding:"required"`
}

func (s *Server) replaceDraft(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok {
		return
	}
	participantID, ok := parseUUIDPath(c, "participantID")
	if !ok {
		return
	}

	var req replaceDraftRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	if len(req.ContestantIDs) == 0 {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "contestant_ids cannot be empty"})
		return
	}

	participant, err := s.queries.GetParticipant(c.Request.Context(), toPGUUID(participantID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, errorResponse{Error: "participant not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	if uuid.UUID(participant.InstanceID.Bytes) != instanceID {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "participant does not belong to this instance"})
		return
	}

	contestants, err := s.queries.ListContestantsByInstance(c.Request.Context(), toPGUUID(instanceID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	if len(req.ContestantIDs) != len(contestants) {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "contestant_ids must include every contestant exactly once"})
		return
	}

	allowedContestants := make(map[uuid.UUID]struct{}, len(contestants))
	for _, contestant := range contestants {
		allowedContestants[uuid.UUID(contestant.ID.Bytes)] = struct{}{}
	}

	seen := make(map[uuid.UUID]struct{}, len(req.ContestantIDs))
	contestantUUIDs := make([]uuid.UUID, 0, len(req.ContestantIDs))
	for _, rawID := range req.ContestantIDs {
		contestantID, err := uuid.Parse(rawID)
		if err != nil {
			c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid contestant id: " + rawID})
			return
		}
		if _, ok := allowedContestants[contestantID]; !ok {
			c.JSON(http.StatusBadRequest, errorResponse{Error: "contestant does not belong to this instance: " + rawID})
			return
		}
		if _, exists := seen[contestantID]; exists {
			c.JSON(http.StatusBadRequest, errorResponse{Error: "duplicate contestant id: " + rawID})
			return
		}
		seen[contestantID] = struct{}{}
		contestantUUIDs = append(contestantUUIDs, contestantID)
	}

	tx, err := s.pool.Begin(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	defer func() {
		rollbackErr := tx.Rollback(c.Request.Context())
		if rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			if ginErr := c.Error(rollbackErr); ginErr != nil {
				ginErr.Type = gin.ErrorTypePrivate
			}
		}
	}()

	qtx := s.queries.WithTx(tx)
	if err := qtx.DeleteDraftPicksForParticipant(c.Request.Context(), toPGUUID(participantID)); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	for i, contestantID := range contestantUUIDs {
		position, err := conv.ToInt32(i + 1)
		if err != nil {
			c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
			return
		}
		_, err = qtx.CreateDraftPick(c.Request.Context(), db.CreateDraftPickParams{
			InstanceID:    toPGUUID(instanceID),
			ParticipantID: toPGUUID(participantID),
			ContestantID:  toPGUUID(contestantID),
			Position:      position,
		})
		if err != nil {
			c.JSON(statusFromPg(err), errorResponse{Error: err.Error()})
			return
		}
	}

	if err := tx.Commit(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "draft saved"})
}

func (s *Server) getDraft(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok {
		return
	}
	participantID, ok := parseUUIDPath(c, "participantID")
	if !ok {
		return
	}

	participant, err := s.queries.GetParticipant(c.Request.Context(), toPGUUID(participantID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, errorResponse{Error: "participant not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	if uuid.UUID(participant.InstanceID.Bytes) != instanceID {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "participant does not belong to this instance"})
		return
	}

	picks, err := s.queries.ListDraftPicksForParticipant(c.Request.Context(), toPGUUID(participantID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	contestants, err := s.queries.ListContestantsByInstance(c.Request.Context(), toPGUUID(instanceID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	contestantByID := make(map[uuid.UUID]string, len(contestants))
	for _, contestant := range contestants {
		contestantByID[uuid.UUID(contestant.ID.Bytes)] = contestant.Name
	}

	responsePicks := make([]gin.H, 0, len(picks))
	for _, pick := range picks {
		contestantID := uuid.UUID(pick.ContestantID.Bytes)
		responsePicks = append(responsePicks, gin.H{
			"position":        pick.Position,
			"contestant_id":   contestantID.String(),
			"contestant_name": contestantByID[contestantID],
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"participant": gin.H{
			"id":   participantID.String(),
			"name": participant.Name,
		},
		"picks": responsePicks,
	})
}

type upsertOutcomeRequest struct {
	ContestantID string `json:"contestant_id"`
}

func (s *Server) upsertOutcome(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok {
		return
	}

	position, err := strconv.Atoi(c.Param("position"))
	if err != nil || position <= 0 {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "position must be a positive integer"})
		return
	}

	var req upsertOutcomeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	contestantParam := pgtype.UUID{Valid: false}
	if req.ContestantID != "" {
		contestantID, err := uuid.Parse(req.ContestantID)
		if err != nil {
			c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid contestant_id"})
			return
		}

		exists, err := s.queries.InstanceHasContestant(c.Request.Context(), db.InstanceHasContestantParams{
			InstanceID:   toPGUUID(instanceID),
			ContestantID: toPGUUID(contestantID),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
			return
		}
		if !exists {
			c.JSON(http.StatusBadRequest, errorResponse{Error: "contestant does not belong to this instance"})
			return
		}
		contestantParam = toPGUUID(contestantID)
	}

	positionInt32, err := conv.ToInt32(position)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	outcome, err := s.queries.UpsertOutcomePosition(c.Request.Context(), db.UpsertOutcomePositionParams{
		InstanceID:   toPGUUID(instanceID),
		Position:     positionInt32,
		ContestantID: contestantParam,
	})
	if err != nil {
		c.JSON(statusFromPg(err), errorResponse{Error: err.Error()})
		return
	}

	response := gin.H{
		"position": outcome.Position,
	}
	if outcome.ContestantID.Valid {
		response["contestant_id"] = uuid.UUID(outcome.ContestantID.Bytes).String()
	} else {
		response["contestant_id"] = nil
	}
	c.JSON(http.StatusOK, gin.H{"outcome": response})
}

func (s *Server) listOutcomes(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok {
		return
	}

	outcomes, err := s.queries.ListOutcomePositionsByInstance(c.Request.Context(), toPGUUID(instanceID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	contestants, err := s.queries.ListContestantsByInstance(c.Request.Context(), toPGUUID(instanceID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	contestantByID := make(map[uuid.UUID]string, len(contestants))
	for _, contestant := range contestants {
		contestantByID[uuid.UUID(contestant.ID.Bytes)] = contestant.Name
	}

	response := make([]gin.H, 0, len(outcomes))
	for _, outcome := range outcomes {
		row := gin.H{"position": outcome.Position}
		if outcome.ContestantID.Valid {
			contestantID := uuid.UUID(outcome.ContestantID.Bytes)
			row["contestant_id"] = contestantID.String()
			row["contestant_name"] = contestantByID[contestantID]
		} else {
			row["contestant_id"] = nil
			row["contestant_name"] = nil
		}
		response = append(response, row)
	}

	c.JSON(http.StatusOK, gin.H{"outcomes": response})
}

func (s *Server) leaderboard(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok {
		return
	}

	participantFilter, ok := parseOptionalParticipantIDQuery(c)
	if !ok {
		return
	}

	contestants, err := s.queries.ListContestantsByInstance(c.Request.Context(), toPGUUID(instanceID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	participants, err := s.queries.ListParticipantsByInstance(c.Request.Context(), toPGUUID(instanceID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	draftPicks, err := s.queries.ListDraftPicksForInstance(c.Request.Context(), toPGUUID(instanceID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	outcomes, err := s.queries.ListOutcomePositionsByInstance(c.Request.Context(), toPGUUID(instanceID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	participantNames := make(map[string]string, len(participants))
	for _, participant := range participants {
		participantNames[uuid.UUID(participant.ID.Bytes).String()] = participant.Name
	}

	draftsByParticipant := make(map[string][]scoring.DraftPick, len(participants))
	for _, pick := range draftPicks {
		participantID := uuid.UUID(pick.ParticipantID.Bytes).String()
		draftsByParticipant[participantID] = append(draftsByParticipant[participantID], scoring.DraftPick{
			Position:     int(pick.Position),
			ContestantID: uuid.UUID(pick.ContestantID.Bytes).String(),
		})
	}
	for participantID := range draftsByParticipant {
		sort.Slice(draftsByParticipant[participantID], func(i, j int) bool {
			return draftsByParticipant[participantID][i].Position < draftsByParticipant[participantID][j].Position
		})
	}

	finalPositions := map[string]int{}
	for _, outcome := range outcomes {
		if !outcome.ContestantID.Valid {
			continue
		}
		finalPositions[uuid.UUID(outcome.ContestantID.Bytes).String()] = int(outcome.Position)
	}

	leaderboard := scoring.CalculateLeaderboard(len(contestants), participantNames, draftsByParticipant, finalPositions)

	response := make([]gin.H, 0, len(leaderboard))
	for _, row := range leaderboard {
		if participantFilter != nil && row.ParticipantID != participantFilter.String() {
			continue
		}
		response = append(response, gin.H{
			"participant_id":   row.ParticipantID,
			"participant_name": row.ParticipantName,
			"score":            row.Score,
			"points_available": row.PointsAvailable,
		})
	}

	c.JSON(http.StatusOK, gin.H{"leaderboard": response})
}

func parseOptionalSeasonQuery(c *gin.Context) (*int32, bool) {
	raw := strings.TrimSpace(c.Query("season"))
	if raw == "" {
		return nil, true
	}

	season, err := strconv.Atoi(raw)
	if err != nil || season <= 0 {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "season must be a positive integer"})
		return nil, false
	}

	seasonInt32, err := conv.ToInt32(season)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return nil, false
	}
	return &seasonInt32, true
}

func parseOptionalParticipantIDQuery(c *gin.Context) (*uuid.UUID, bool) {
	raw := strings.TrimSpace(c.Query("participant_id"))
	if raw == "" {
		return nil, true
	}

	participantID, err := uuid.Parse(raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid participant_id"})
		return nil, false
	}
	return &participantID, true
}

func matchesContainsFold(candidate, filter string) bool {
	trimmedFilter := strings.TrimSpace(filter)
	if trimmedFilter == "" {
		return true
	}
	return strings.Contains(strings.ToLower(candidate), strings.ToLower(trimmedFilter))
}

func parseUUIDPath(c *gin.Context, key string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(key))
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid " + key})
		return uuid.Nil, false
	}
	return id, true
}

func toPGUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: [16]byte(id), Valid: true}
}

func statusFromPg(err error) int {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return http.StatusConflict
		case "23503":
			return http.StatusBadRequest
		case "23514":
			return http.StatusBadRequest
		}
	}
	return http.StatusInternalServerError
}
