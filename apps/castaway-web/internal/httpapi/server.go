package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/conv"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/gameplay"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/scoring"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	pool                    *pgxpool.Pool
	queries                 *db.Queries
	serviceAuth             ServiceAuthConfig
	serviceAuthBearerTokens map[string]struct{}
}

type Option func(*Server)

func WithServiceAuth(cfg ServiceAuthConfig) Option {
	return func(s *Server) {
		normalized := normalizeServiceAuthConfig(cfg)
		s.serviceAuth = normalized
		tokens := make(map[string]struct{}, len(normalized.BearerTokens))
		for _, token := range normalized.BearerTokens {
			tokens[token] = struct{}{}
		}
		s.serviceAuthBearerTokens = tokens
	}
}

func New(pool *pgxpool.Pool, options ...Option) *Server {
	server := &Server{
		pool:                    pool,
		queries:                 db.New(pool),
		serviceAuth:             normalizeServiceAuthConfig(ServiceAuthConfig{}),
		serviceAuthBearerTokens: make(map[string]struct{}),
	}
	for _, option := range options {
		option(server)
	}
	return server
}

func (s *Server) Router() *gin.Engine {
	r := gin.Default()

	r.GET("/healthz", s.health)

	protected := r.Group("/")
	protected.Use(s.requireServiceAuth())
	protected.GET("/instances", s.listInstances)
	protected.POST("/instances", s.createInstance)
	protected.POST("/instances/import", s.importInstance)
	protected.GET("/instances/:instanceID", s.getInstance)
	protected.POST("/instances/:instanceID/contestants", s.createContestant)
	protected.GET("/instances/:instanceID/contestants", s.listContestants)

	protected.POST("/instances/:instanceID/participants", s.createParticipant)
	protected.GET("/instances/:instanceID/participants", s.listParticipants)
	protected.GET("/instances/:instanceID/participants/me", s.getLinkedParticipant)
	protected.PUT("/instances/:instanceID/participants/:participantID/discord-link", s.linkParticipantDiscordUser)
	protected.DELETE("/instances/:instanceID/participants/:participantID/discord-link", s.unlinkParticipantDiscordUser)
	protected.GET("/instances/:instanceID/participants/:participantID/bonus-ledger", s.bonusLedger)

	protected.PUT("/instances/:instanceID/drafts/:participantID", s.replaceDraft)
	protected.GET("/instances/:instanceID/drafts/:participantID", s.getDraft)

	protected.PUT("/instances/:instanceID/outcomes/:position", s.upsertOutcome)
	protected.GET("/instances/:instanceID/outcomes", s.listOutcomes)

	protected.GET("/instances/:instanceID/leaderboard", s.leaderboard)
	protected.GET("/instances/:instanceID/activities", s.listActivities)
	protected.POST("/instances/:instanceID/activities", s.createActivity)
	protected.GET("/activities/:activityID", s.getActivity)
	protected.GET("/activities/:activityID/occurrences", s.listOccurrences)
	protected.POST("/activities/:activityID/occurrences", s.createOccurrence)
	protected.GET("/occurrences/:occurrenceID", s.getOccurrence)
	protected.POST("/occurrences/:occurrenceID/participants", s.createOccurrenceParticipant)
	protected.POST("/occurrences/:occurrenceID/groups", s.createOccurrenceGroup)
	protected.POST("/occurrences/:occurrenceID/resolve", s.resolveOccurrence)
	protected.GET("/instances/:instanceID/participants/:participantID/activity-history", s.participantActivityHistory)

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

	if err := gameplay.NewService(qtx).CopyInstanceSchedule(c.Request.Context(), createdInstance.ID, createdInstance.Season); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
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
		participants = append(participants, participantSummaryToJSON(participant.ID, participant.Name))
	}

	c.JSON(http.StatusOK, gin.H{"participants": participants})
}

func (s *Server) getLinkedParticipant(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok {
		return
	}
	discordUserID := discordUserIDFromRequest(c.Request)
	if discordUserID == "" {
		c.JSON(http.StatusNotFound, errorResponse{Error: "participant not linked"})
		return
	}

	participant, err := s.queries.GetParticipantByDiscordUserID(c.Request.Context(), db.GetParticipantByDiscordUserIDParams{
		InstanceID:    toPGUUID(instanceID),
		DiscordUserID: pgtype.Text{String: discordUserID, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, errorResponse{Error: "participant not linked"})
			return
		}
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"participant": participantSummaryToJSON(participant.ID, participant.Name)})
}

type linkParticipantDiscordUserRequest struct {
	DiscordUserID string `json:"discord_user_id"`
}

func (s *Server) linkParticipantDiscordUser(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok {
		return
	}
	participantID, ok := parseUUIDPath(c, "participantID")
	if !ok {
		return
	}
	callerDiscordUserID := discordUserIDFromRequest(c.Request)
	if callerDiscordUserID == "" {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "missing discord user id"})
		return
	}

	isAdmin, err := s.isInstanceAdmin(c.Request.Context(), toPGUUID(instanceID), callerDiscordUserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	if !isAdmin {
		c.JSON(http.StatusForbidden, errorResponse{Error: "forbidden"})
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
	if participant.InstanceID != toPGUUID(instanceID) {
		c.JSON(http.StatusNotFound, errorResponse{Error: "participant not found"})
		return
	}

	targetDiscordUserID := callerDiscordUserID
	if queryTarget := strings.TrimSpace(c.Query("discord_user_id")); queryTarget != "" {
		targetDiscordUserID = queryTarget
	}
	if c.Request.ContentLength != 0 {
		var req linkParticipantDiscordUserRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
			return
		}
		if strings.TrimSpace(req.DiscordUserID) != "" {
			targetDiscordUserID = strings.TrimSpace(req.DiscordUserID)
		}
	}

	updated, err := s.queries.SetParticipantDiscordUserID(c.Request.Context(), db.SetParticipantDiscordUserIDParams{
		ID:            toPGUUID(participantID),
		DiscordUserID: pgtype.Text{String: targetDiscordUserID, Valid: true},
	})
	if err != nil {
		c.JSON(statusFromPg(err), errorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"participant": participantSummaryToJSON(updated.ID, updated.Name)})
}

func (s *Server) unlinkParticipantDiscordUser(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok {
		return
	}
	participantID, ok := parseUUIDPath(c, "participantID")
	if !ok {
		return
	}
	callerDiscordUserID := discordUserIDFromRequest(c.Request)
	if callerDiscordUserID == "" {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "missing discord user id"})
		return
	}

	isAdmin, err := s.isInstanceAdmin(c.Request.Context(), toPGUUID(instanceID), callerDiscordUserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	if !isAdmin {
		c.JSON(http.StatusForbidden, errorResponse{Error: "forbidden"})
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
	if participant.InstanceID != toPGUUID(instanceID) {
		c.JSON(http.StatusNotFound, errorResponse{Error: "participant not found"})
		return
	}

	updated, err := s.queries.ClearParticipantDiscordUserID(c.Request.Context(), toPGUUID(participantID))
	if err != nil {
		c.JSON(statusFromPg(err), errorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"participant": participantSummaryToJSON(updated.ID, updated.Name)})
}

func participantSummaryToJSON(id pgtype.UUID, name string) gin.H {
	return gin.H{
		"id":   pgUUIDString(id),
		"name": name,
	}
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

	visibleBonusByParticipant := make(map[string]int, len(participants))
	gameplayService := gameplay.NewService(s.queries)
	for _, participant := range participants {
		participantID := uuid.UUID(participant.ID.Bytes).String()
		bonusPoints, err := gameplayService.VisibleBonusTotalByParticipant(c.Request.Context(), toPGUUID(instanceID), participant.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
			return
		}
		visibleBonusByParticipant[participantID] = int(bonusPoints)
	}

	leaderboard := scoring.CalculateLeaderboard(len(contestants), participantNames, draftsByParticipant, finalPositions, visibleBonusByParticipant)

	response := make([]gin.H, 0, len(leaderboard))
	for _, row := range leaderboard {
		if participantFilter != nil && row.ParticipantID != participantFilter.String() {
			continue
		}
		response = append(response, gin.H{
			"participant_id":   row.ParticipantID,
			"participant_name": row.ParticipantName,
			"score":            row.Score,
			"draft_points":     row.DraftPoints,
			"bonus_points":     row.BonusPoints,
			"total_points":     row.TotalPoints,
			"points_available": row.PointsAvailable,
		})
	}

	c.JSON(http.StatusOK, gin.H{"leaderboard": response})
}

func (s *Server) bonusLedger(c *gin.Context) {
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
	if participant.InstanceID != toPGUUID(instanceID) {
		c.JSON(http.StatusNotFound, errorResponse{Error: "participant not found"})
		return
	}

	canViewSecret, err := s.canViewSecretParticipantData(c.Request.Context(), toPGUUID(instanceID), discordUserIDFromRequest(c.Request), participant)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	bonusPoints := int32(0)
	var ledger []gin.H
	if canViewSecret {
		visibleBonusPoints, visibleErr := s.queries.GetVisibleBonusTotalByParticipant(c.Request.Context(), db.GetVisibleBonusTotalByParticipantParams{
			InstanceID:    toPGUUID(instanceID),
			ParticipantID: toPGUUID(participantID),
		})
		if visibleErr != nil {
			c.JSON(http.StatusInternalServerError, errorResponse{Error: visibleErr.Error()})
			return
		}
		secretBonusPoints, secretErr := s.queries.GetSecretBonusTotalByParticipant(c.Request.Context(), db.GetSecretBonusTotalByParticipantParams{
			InstanceID:    toPGUUID(instanceID),
			ParticipantID: toPGUUID(participantID),
		})
		if secretErr != nil {
			c.JSON(http.StatusInternalServerError, errorResponse{Error: secretErr.Error()})
			return
		}
		bonusPoints = visibleBonusPoints + secretBonusPoints

		ledgerRows, listErr := s.queries.ListAllBonusPointLedgerEntriesForParticipant(c.Request.Context(), db.ListAllBonusPointLedgerEntriesForParticipantParams{
			InstanceID:    toPGUUID(instanceID),
			ParticipantID: toPGUUID(participantID),
		})
		if listErr != nil {
			c.JSON(http.StatusInternalServerError, errorResponse{Error: listErr.Error()})
			return
		}
		ledger = make([]gin.H, 0, len(ledgerRows))
		for _, row := range ledgerRows {
			ledger = append(ledger, gin.H{
				"id":                     pgUUIDString(row.ID),
				"activity_id":            pgUUIDString(row.ActivityID),
				"activity_type":          row.ActivityType,
				"activity_name":          row.ActivityName,
				"activity_occurrence_id": pgUUIDString(row.ActivityOccurrenceID),
				"occurrence_type":        row.OccurrenceType,
				"occurrence_name":        row.OccurrenceName,
				"source_group_id":        pgUUIDPointer(row.SourceGroupID),
				"source_group_name":      pgTextPointer(row.SourceGroupName),
				"entry_kind":             row.EntryKind,
				"points":                 row.Points,
				"visibility":             row.Visibility,
				"reason":                 row.Reason,
				"effective_at":           formatTimestamp(row.EffectiveAt),
				"award_key":              pgTextPointer(row.AwardKey),
				"created_at":             formatTimestamp(row.CreatedAt),
			})
		}
	} else {
		bonusPoints, err = s.queries.GetVisibleBonusTotalByParticipant(c.Request.Context(), db.GetVisibleBonusTotalByParticipantParams{
			InstanceID:    toPGUUID(instanceID),
			ParticipantID: toPGUUID(participantID),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
			return
		}

		ledgerRows, listErr := s.queries.ListVisibleBonusPointLedgerEntriesForParticipant(c.Request.Context(), db.ListVisibleBonusPointLedgerEntriesForParticipantParams{
			InstanceID:    toPGUUID(instanceID),
			ParticipantID: toPGUUID(participantID),
		})
		if listErr != nil {
			c.JSON(http.StatusInternalServerError, errorResponse{Error: listErr.Error()})
			return
		}
		ledger = make([]gin.H, 0, len(ledgerRows))
		for _, row := range ledgerRows {
			ledger = append(ledger, gin.H{
				"id":                     pgUUIDString(row.ID),
				"activity_id":            pgUUIDString(row.ActivityID),
				"activity_type":          row.ActivityType,
				"activity_name":          row.ActivityName,
				"activity_occurrence_id": pgUUIDString(row.ActivityOccurrenceID),
				"occurrence_type":        row.OccurrenceType,
				"occurrence_name":        row.OccurrenceName,
				"source_group_id":        pgUUIDPointer(row.SourceGroupID),
				"source_group_name":      pgTextPointer(row.SourceGroupName),
				"entry_kind":             row.EntryKind,
				"points":                 row.Points,
				"visibility":             row.Visibility,
				"reason":                 row.Reason,
				"effective_at":           formatTimestamp(row.EffectiveAt),
				"award_key":              pgTextPointer(row.AwardKey),
				"created_at":             formatTimestamp(row.CreatedAt),
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"participant": gin.H{
			"id":   pgUUIDString(participant.ID),
			"name": participant.Name,
		},
		"bonus_points": bonusPoints,
		"ledger":       ledger,
	})
}

func (s *Server) participantActivityHistory(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok {
		return
	}
	participantID, ok := parseUUIDPath(c, "participantID")
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

	participant, err := s.queries.GetParticipant(c.Request.Context(), toPGUUID(participantID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, errorResponse{Error: "participant not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	if participant.InstanceID != toPGUUID(instanceID) {
		c.JSON(http.StatusNotFound, errorResponse{Error: "participant not found"})
		return
	}

	involvementRows, err := s.queries.ListParticipantOccurrenceInvolvementByInstance(c.Request.Context(), db.ListParticipantOccurrenceInvolvementByInstanceParams{
		InstanceID:    toPGUUID(instanceID),
		ParticipantID: toPGUUID(participantID),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	canViewSecret, err := s.canViewSecretParticipantData(c.Request.Context(), toPGUUID(instanceID), discordUserIDFromRequest(c.Request), participant)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	var ledgerRows []db.ListAllBonusPointLedgerEntriesForParticipantRow
	if canViewSecret {
		allLedgerRows, listErr := s.queries.ListAllBonusPointLedgerEntriesForParticipant(c.Request.Context(), db.ListAllBonusPointLedgerEntriesForParticipantParams{
			InstanceID:    toPGUUID(instanceID),
			ParticipantID: toPGUUID(participantID),
		})
		if listErr != nil {
			c.JSON(http.StatusInternalServerError, errorResponse{Error: listErr.Error()})
			return
		}
		ledgerRows = allLedgerRows
	} else {
		visibleLedgerRows, listErr := s.queries.ListVisibleBonusPointLedgerEntriesForParticipant(c.Request.Context(), db.ListVisibleBonusPointLedgerEntriesForParticipantParams{
			InstanceID:    toPGUUID(instanceID),
			ParticipantID: toPGUUID(participantID),
		})
		if listErr != nil {
			c.JSON(http.StatusInternalServerError, errorResponse{Error: listErr.Error()})
			return
		}
		ledgerRows = make([]db.ListAllBonusPointLedgerEntriesForParticipantRow, 0, len(visibleLedgerRows))
		for _, row := range visibleLedgerRows {
			ledgerRows = append(ledgerRows, db.ListAllBonusPointLedgerEntriesForParticipantRow{
				ID:                   row.ID,
				InstanceID:           row.InstanceID,
				ParticipantID:        row.ParticipantID,
				ActivityOccurrenceID: row.ActivityOccurrenceID,
				OccurrenceType:       row.OccurrenceType,
				OccurrenceName:       row.OccurrenceName,
				ActivityID:           row.ActivityID,
				ActivityType:         row.ActivityType,
				ActivityName:         row.ActivityName,
				SourceGroupID:        row.SourceGroupID,
				SourceGroupName:      row.SourceGroupName,
				EntryKind:            row.EntryKind,
				Points:               row.Points,
				Visibility:           row.Visibility,
				Reason:               row.Reason,
				EffectiveAt:          row.EffectiveAt,
				AwardKey:             row.AwardKey,
				CreatedAt:            row.CreatedAt,
			})
		}
	}

	type historyOccurrence struct {
		Occurrence  gin.H   `json:"occurrence"`
		Involvement any     `json:"involvement,omitempty"`
		Ledger      []gin.H `json:"ledger"`
	}
	type historyActivity struct {
		Activity    gin.H               `json:"activity"`
		Occurrences []historyOccurrence `json:"occurrences"`
	}

	activities := make([]historyActivity, 0)
	activityIndexes := map[string]int{}
	occurrenceIndexes := map[string]struct{ activityIndex, occurrenceIndex int }{}
	activityJSONCache := map[string]gin.H{}

	ensureActivity := func(activityID string, activity gin.H) int {
		if index, exists := activityIndexes[activityID]; exists {
			return index
		}
		activityJSONCache[activityID] = activity
		activities = append(activities, historyActivity{Activity: activity, Occurrences: []historyOccurrence{}})
		index := len(activities) - 1
		activityIndexes[activityID] = index
		return index
	}

	for _, row := range involvementRows {
		activityID := pgUUIDString(row.ActivityID)
		activityIndex := ensureActivity(activityID, activityToJSON(row.ActivityID, instance.ID, row.ActivityType, row.ActivityName, row.ActivityStatus, row.ActivityStartsAt, row.ActivityEndsAt, row.ActivityMetadata, row.ActivityCreatedAt, row.ActivityUpdatedAt))
		occurrenceID := pgUUIDString(row.OccurrenceID)
		if _, exists := occurrenceIndexes[occurrenceID]; exists {
			continue
		}
		activities[activityIndex].Occurrences = append(activities[activityIndex].Occurrences, historyOccurrence{
			Occurrence:  occurrenceToJSON(row.OccurrenceID, row.ActivityID, row.OccurrenceType, row.OccurrenceName, row.EffectiveAt, row.StartsAt, row.EndsAt, row.OccurrenceStatus, row.SourceRef, row.OccurrenceMetadata, row.OccurrenceCreatedAt, row.OccurrenceUpdatedAt),
			Involvement: occurrenceHistoryInvolvementToJSON(row),
			Ledger:      []gin.H{},
		})
		occurrenceIndexes[occurrenceID] = struct{ activityIndex, occurrenceIndex int }{activityIndex: activityIndex, occurrenceIndex: len(activities[activityIndex].Occurrences) - 1}
	}

	for _, row := range ledgerRows {
		activityID := pgUUIDString(row.ActivityID)
		activityJSON, exists := activityJSONCache[activityID]
		if !exists {
			activityRow, getErr := s.queries.GetInstanceActivity(c.Request.Context(), row.ActivityID)
			if getErr != nil {
				c.JSON(http.StatusInternalServerError, errorResponse{Error: getErr.Error()})
				return
			}
			activityJSON = activityToJSON(activityRow.ID, activityRow.InstanceID, activityRow.ActivityType, activityRow.Name, activityRow.Status, activityRow.StartsAt, activityRow.EndsAt, activityRow.Metadata, activityRow.CreatedAt, activityRow.UpdatedAt)
		}
		activityIndex := ensureActivity(activityID, activityJSON)
		occurrenceID := pgUUIDString(row.ActivityOccurrenceID)
		indexPair, exists := occurrenceIndexes[occurrenceID]
		if !exists {
			activities[activityIndex].Occurrences = append(activities[activityIndex].Occurrences, historyOccurrence{
				Occurrence: gin.H{
					"id":              occurrenceID,
					"activity_id":     activityID,
					"occurrence_type": row.OccurrenceType,
					"name":            row.OccurrenceName,
					"effective_at":    formatTimestamp(row.EffectiveAt),
				},
				Ledger: []gin.H{},
			})
			indexPair = struct{ activityIndex, occurrenceIndex int }{activityIndex: activityIndex, occurrenceIndex: len(activities[activityIndex].Occurrences) - 1}
			occurrenceIndexes[occurrenceID] = indexPair
		}
		activities[indexPair.activityIndex].Occurrences[indexPair.occurrenceIndex].Ledger = append(
			activities[indexPair.activityIndex].Occurrences[indexPair.occurrenceIndex].Ledger,
			gin.H{
				"id":                     pgUUIDString(row.ID),
				"instance_id":            pgUUIDString(row.InstanceID),
				"participant_id":         pgUUIDString(row.ParticipantID),
				"participant_name":       participant.Name,
				"activity_id":            pgUUIDString(row.ActivityID),
				"activity_type":          row.ActivityType,
				"activity_name":          row.ActivityName,
				"activity_occurrence_id": pgUUIDString(row.ActivityOccurrenceID),
				"occurrence_type":        row.OccurrenceType,
				"occurrence_name":        row.OccurrenceName,
				"source_group_id":        pgUUIDPointer(row.SourceGroupID),
				"source_group_name":      pgTextPointer(row.SourceGroupName),
				"entry_kind":             row.EntryKind,
				"points":                 row.Points,
				"visibility":             row.Visibility,
				"reason":                 row.Reason,
				"effective_at":           formatTimestamp(row.EffectiveAt),
				"award_key":              pgTextPointer(row.AwardKey),
				"created_at":             formatTimestamp(row.CreatedAt),
				"metadata":               json.RawMessage(row.Metadata),
			},
		)
	}

	c.JSON(http.StatusOK, gin.H{
		"participant": gin.H{
			"id":   pgUUIDString(participant.ID),
			"name": participant.Name,
		},
		"instance":   toInstanceResponse(instance.ID, instance.Name, instance.Season, instance.CreatedAt),
		"activities": activities,
	})
}

type createActivityRequest struct {
	ActivityType string           `json:"activity_type" binding:"required"`
	Name         string           `json:"name" binding:"required"`
	Status       string           `json:"status" binding:"required"`
	StartsAt     time.Time        `json:"starts_at" binding:"required"`
	EndsAt       *time.Time       `json:"ends_at"`
	Metadata     *json.RawMessage `json:"metadata"`
}

func (s *Server) listActivities(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok {
		return
	}

	activities, err := s.queries.ListInstanceActivitiesByInstance(c.Request.Context(), toPGUUID(instanceID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	response := make([]gin.H, 0, len(activities))
	for _, activity := range activities {
		response = append(response, activityToJSON(activity.ID, activity.InstanceID, activity.ActivityType, activity.Name, activity.Status, activity.StartsAt, activity.EndsAt, activity.Metadata, activity.CreatedAt, activity.UpdatedAt))
	}
	c.JSON(http.StatusOK, gin.H{"activities": response})
}

func (s *Server) getActivity(c *gin.Context) {
	activityID, ok := parseUUIDPath(c, "activityID")
	if !ok {
		return
	}

	activity, err := s.queries.GetInstanceActivity(c.Request.Context(), toPGUUID(activityID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, errorResponse{Error: "activity not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	groupAssignments, err := s.queries.ListActivityGroupAssignments(c.Request.Context(), toPGUUID(activityID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	participantAssignments, err := s.queries.ListActivityParticipantAssignments(c.Request.Context(), toPGUUID(activityID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	groupResponse := make([]gin.H, 0, len(groupAssignments))
	for _, assignment := range groupAssignments {
		groupResponse = append(groupResponse, activityGroupAssignmentToJSON(assignment))
	}

	participantResponse := make([]gin.H, 0, len(participantAssignments))
	for _, assignment := range participantAssignments {
		participantResponse = append(participantResponse, activityParticipantAssignmentToJSON(assignment))
	}

	c.JSON(http.StatusOK, gin.H{
		"activity":                activityToJSON(activity.ID, activity.InstanceID, activity.ActivityType, activity.Name, activity.Status, activity.StartsAt, activity.EndsAt, activity.Metadata, activity.CreatedAt, activity.UpdatedAt),
		"group_assignments":       groupResponse,
		"participant_assignments": participantResponse,
	})
}

func (s *Server) createActivity(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok {
		return
	}

	var req createActivityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	activity, err := s.queries.CreateInstanceActivity(c.Request.Context(), db.CreateInstanceActivityParams{
		InstanceID:   toPGUUID(instanceID),
		ActivityType: req.ActivityType,
		Name:         req.Name,
		Status:       req.Status,
		StartsAt:     optionalTime(req.StartsAt),
		EndsAt:       optionalTimePtr(req.EndsAt),
		Metadata:     defaultJSONB(req.Metadata),
	})
	if err != nil {
		c.JSON(statusFromPg(err), errorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"activity": activityToJSON(activity.ID, activity.InstanceID, activity.ActivityType, activity.Name, activity.Status, activity.StartsAt, activity.EndsAt, activity.Metadata, activity.CreatedAt, activity.UpdatedAt)})
}

type createOccurrenceRequest struct {
	OccurrenceType string           `json:"occurrence_type" binding:"required"`
	Name           string           `json:"name" binding:"required"`
	EffectiveAt    time.Time        `json:"effective_at" binding:"required"`
	StartsAt       *time.Time       `json:"starts_at"`
	EndsAt         *time.Time       `json:"ends_at"`
	Status         string           `json:"status" binding:"required"`
	SourceRef      *string          `json:"source_ref"`
	Metadata       *json.RawMessage `json:"metadata"`
}

func (s *Server) listOccurrences(c *gin.Context) {
	activityID, ok := parseUUIDPath(c, "activityID")
	if !ok {
		return
	}

	occurrences, err := s.queries.ListActivityOccurrencesByActivity(c.Request.Context(), toPGUUID(activityID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	response := make([]gin.H, 0, len(occurrences))
	for _, occurrence := range occurrences {
		response = append(response, occurrenceToJSON(occurrence.ID, occurrence.ActivityID, occurrence.OccurrenceType, occurrence.Name, occurrence.EffectiveAt, occurrence.StartsAt, occurrence.EndsAt, occurrence.Status, occurrence.SourceRef, occurrence.Metadata, occurrence.CreatedAt, occurrence.UpdatedAt))
	}
	c.JSON(http.StatusOK, gin.H{"occurrences": response})
}

func (s *Server) getOccurrence(c *gin.Context) {
	occurrenceID, ok := parseUUIDPath(c, "occurrenceID")
	if !ok {
		return
	}

	occurrence, err := s.queries.GetActivityOccurrence(c.Request.Context(), toPGUUID(occurrenceID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, errorResponse{Error: "occurrence not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	activity, err := s.queries.GetInstanceActivity(c.Request.Context(), occurrence.ActivityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	participants, err := s.queries.ListActivityOccurrenceParticipants(c.Request.Context(), toPGUUID(occurrenceID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	groups, err := s.queries.ListActivityOccurrenceGroups(c.Request.Context(), toPGUUID(occurrenceID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	ledgerRows, err := s.queries.ListVisibleBonusPointLedgerEntriesByOccurrence(c.Request.Context(), toPGUUID(occurrenceID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	participantResponse := make([]gin.H, 0, len(participants))
	for _, row := range participants {
		participantResponse = append(participantResponse, occurrenceParticipantToJSON(row))
	}

	groupResponse := make([]gin.H, 0, len(groups))
	for _, row := range groups {
		groupResponse = append(groupResponse, occurrenceGroupToJSON(row))
	}

	ledgerResponse := make([]gin.H, 0, len(ledgerRows))
	for _, row := range ledgerRows {
		ledgerResponse = append(ledgerResponse, visibleOccurrenceLedgerToJSON(row))
	}

	c.JSON(http.StatusOK, gin.H{
		"activity":     activityToJSON(activity.ID, activity.InstanceID, activity.ActivityType, activity.Name, activity.Status, activity.StartsAt, activity.EndsAt, activity.Metadata, activity.CreatedAt, activity.UpdatedAt),
		"occurrence":   occurrenceToJSON(occurrence.ID, occurrence.ActivityID, occurrence.OccurrenceType, occurrence.Name, occurrence.EffectiveAt, occurrence.StartsAt, occurrence.EndsAt, occurrence.Status, occurrence.SourceRef, occurrence.Metadata, occurrence.CreatedAt, occurrence.UpdatedAt),
		"participants": participantResponse,
		"groups":       groupResponse,
		"ledger":       ledgerResponse,
	})
}

func (s *Server) createOccurrence(c *gin.Context) {
	activityID, ok := parseUUIDPath(c, "activityID")
	if !ok {
		return
	}

	var req createOccurrenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	occurrence, err := s.queries.CreateActivityOccurrence(c.Request.Context(), db.CreateActivityOccurrenceParams{
		ActivityID:     toPGUUID(activityID),
		OccurrenceType: req.OccurrenceType,
		Name:           req.Name,
		EffectiveAt:    optionalTime(req.EffectiveAt),
		StartsAt:       optionalTimePtr(req.StartsAt),
		EndsAt:         optionalTimePtr(req.EndsAt),
		Status:         req.Status,
		SourceRef:      optionalText(req.SourceRef),
		Metadata:       defaultJSONB(req.Metadata),
	})
	if err != nil {
		c.JSON(statusFromPg(err), errorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"occurrence": occurrenceToJSON(occurrence.ID, occurrence.ActivityID, occurrence.OccurrenceType, occurrence.Name, occurrence.EffectiveAt, occurrence.StartsAt, occurrence.EndsAt, occurrence.Status, occurrence.SourceRef, occurrence.Metadata, occurrence.CreatedAt, occurrence.UpdatedAt)})
}

type createOccurrenceParticipantRequest struct {
	ParticipantID      string           `json:"participant_id" binding:"required"`
	ParticipantGroupID *string          `json:"participant_group_id"`
	Role               string           `json:"role" binding:"required"`
	Result             *string          `json:"result"`
	Metadata           *json.RawMessage `json:"metadata"`
}

func (s *Server) createOccurrenceParticipant(c *gin.Context) {
	occurrenceID, ok := parseUUIDPath(c, "occurrenceID")
	if !ok {
		return
	}

	var req createOccurrenceParticipantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	participantID, err := uuid.Parse(req.ParticipantID)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid participant_id"})
		return
	}

	participantGroupID := pgtype.UUID{}
	if req.ParticipantGroupID != nil {
		parsed, err := uuid.Parse(*req.ParticipantGroupID)
		if err != nil {
			c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid participant_group_id"})
			return
		}
		participantGroupID = toPGUUID(parsed)
	}

	result := ""
	if req.Result != nil {
		result = *req.Result
	}

	created, err := s.queries.CreateActivityOccurrenceParticipant(c.Request.Context(), db.CreateActivityOccurrenceParticipantParams{
		ActivityOccurrenceID: toPGUUID(occurrenceID),
		ParticipantID:        toPGUUID(participantID),
		ParticipantGroupID:   participantGroupID,
		Role:                 req.Role,
		Result:               result,
		Metadata:             defaultJSONB(req.Metadata),
	})
	if err != nil {
		c.JSON(statusFromPg(err), errorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"participant_result": gin.H{
		"id":                     created.ID,
		"activity_occurrence_id": pgUUIDString(created.ActivityOccurrenceID),
		"participant_id":         pgUUIDString(created.ParticipantID),
		"participant_group_id":   pgUUIDPointer(created.ParticipantGroupID),
		"role":                   created.Role,
		"result":                 created.Result,
		"metadata":               json.RawMessage(created.Metadata),
		"created_at":             formatTimestamp(created.CreatedAt),
	}})
}

type createOccurrenceGroupRequest struct {
	ParticipantGroupID string           `json:"participant_group_id" binding:"required"`
	Role               string           `json:"role" binding:"required"`
	Result             *string          `json:"result"`
	Metadata           *json.RawMessage `json:"metadata"`
}

func (s *Server) createOccurrenceGroup(c *gin.Context) {
	occurrenceID, ok := parseUUIDPath(c, "occurrenceID")
	if !ok {
		return
	}

	var req createOccurrenceGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	participantGroupID, err := uuid.Parse(req.ParticipantGroupID)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid participant_group_id"})
		return
	}

	result := ""
	if req.Result != nil {
		result = *req.Result
	}

	created, err := s.queries.CreateActivityOccurrenceGroup(c.Request.Context(), db.CreateActivityOccurrenceGroupParams{
		ActivityOccurrenceID: toPGUUID(occurrenceID),
		ParticipantGroupID:   toPGUUID(participantGroupID),
		Role:                 req.Role,
		Result:               result,
		Metadata:             defaultJSONB(req.Metadata),
	})
	if err != nil {
		c.JSON(statusFromPg(err), errorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"group_result": gin.H{
		"id":                     created.ID,
		"activity_occurrence_id": pgUUIDString(created.ActivityOccurrenceID),
		"participant_group_id":   pgUUIDString(created.ParticipantGroupID),
		"role":                   created.Role,
		"result":                 created.Result,
		"metadata":               json.RawMessage(created.Metadata),
		"created_at":             formatTimestamp(created.CreatedAt),
	}})
}

func (s *Server) resolveOccurrence(c *gin.Context) {
	occurrenceID, ok := parseUUIDPath(c, "occurrenceID")
	if !ok {
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

	createdEntries, err := gameplay.NewService(s.queries.WithTx(tx)).ResolveActivityOccurrence(c.Request.Context(), toPGUUID(occurrenceID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, errorResponse{Error: "occurrence not found"})
			return
		}
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, errorResponse{Error: err.Error()})
			return
		}
		c.JSON(statusFromPg(err), errorResponse{Error: err.Error()})
		return
	}

	if err := tx.Commit(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	response := make([]gin.H, 0, len(createdEntries))
	for _, entry := range createdEntries {
		response = append(response, gin.H{
			"id":                     pgUUIDString(entry.ID),
			"instance_id":            pgUUIDString(entry.InstanceID),
			"participant_id":         pgUUIDString(entry.ParticipantID),
			"activity_occurrence_id": pgUUIDString(entry.ActivityOccurrenceID),
			"source_group_id":        pgUUIDPointer(entry.SourceGroupID),
			"entry_kind":             entry.EntryKind,
			"points":                 entry.Points,
			"visibility":             entry.Visibility,
			"reason":                 entry.Reason,
			"effective_at":           formatTimestamp(entry.EffectiveAt),
			"award_key":              pgTextPointer(entry.AwardKey),
			"metadata":               json.RawMessage(entry.Metadata),
			"created_at":             formatTimestamp(entry.CreatedAt),
		})
	}

	c.JSON(http.StatusOK, gin.H{"created_entries": response, "created_count": len(response)})
}

func activityToJSON(id, instanceID pgtype.UUID, activityType, name, status string, startsAt, endsAt pgtype.Timestamptz, metadata []byte, createdAt, updatedAt pgtype.Timestamptz) gin.H {
	return gin.H{
		"id":            pgUUIDString(id),
		"instance_id":   pgUUIDString(instanceID),
		"activity_type": activityType,
		"name":          name,
		"status":        status,
		"starts_at":     formatTimestamp(startsAt),
		"ends_at":       formatNullableTimestamp(endsAt),
		"metadata":      json.RawMessage(metadata),
		"created_at":    formatTimestamp(createdAt),
		"updated_at":    formatTimestamp(updatedAt),
	}
}

func activityGroupAssignmentToJSON(row db.ListActivityGroupAssignmentsRow) gin.H {
	return gin.H{
		"id":                     row.ID,
		"activity_id":            pgUUIDString(row.ActivityID),
		"participant_group_id":   pgUUIDString(row.ParticipantGroupID),
		"participant_group_name": row.ParticipantGroupName,
		"role":                   row.Role,
		"starts_at":              formatTimestamp(row.StartsAt),
		"ends_at":                formatNullableTimestamp(row.EndsAt),
		"configuration":          json.RawMessage(row.Configuration),
		"created_at":             formatTimestamp(row.CreatedAt),
	}
}

func activityParticipantAssignmentToJSON(row db.ListActivityParticipantAssignmentsRow) gin.H {
	return gin.H{
		"id":                     row.ID,
		"activity_id":            pgUUIDString(row.ActivityID),
		"participant_id":         pgUUIDString(row.ParticipantID),
		"participant_name":       row.ParticipantName,
		"participant_group_id":   pgUUIDPointer(row.ParticipantGroupID),
		"participant_group_name": pgTextPointer(row.ParticipantGroupName),
		"role":                   row.Role,
		"starts_at":              formatTimestamp(row.StartsAt),
		"ends_at":                formatNullableTimestamp(row.EndsAt),
		"configuration":          json.RawMessage(row.Configuration),
		"created_at":             formatTimestamp(row.CreatedAt),
	}
}

func occurrenceToJSON(id, activityID pgtype.UUID, occurrenceType, name string, effectiveAt, startsAt, endsAt pgtype.Timestamptz, status string, sourceRef pgtype.Text, metadata []byte, createdAt, updatedAt pgtype.Timestamptz) gin.H {
	return gin.H{
		"id":              pgUUIDString(id),
		"activity_id":     pgUUIDString(activityID),
		"occurrence_type": occurrenceType,
		"name":            name,
		"effective_at":    formatTimestamp(effectiveAt),
		"starts_at":       formatNullableTimestamp(startsAt),
		"ends_at":         formatNullableTimestamp(endsAt),
		"status":          status,
		"source_ref":      pgTextPointer(sourceRef),
		"metadata":        json.RawMessage(metadata),
		"created_at":      formatTimestamp(createdAt),
		"updated_at":      formatTimestamp(updatedAt),
	}
}

func occurrenceParticipantToJSON(row db.ListActivityOccurrenceParticipantsRow) gin.H {
	return gin.H{
		"id":                     row.ID,
		"activity_occurrence_id": pgUUIDString(row.ActivityOccurrenceID),
		"participant_id":         pgUUIDString(row.ParticipantID),
		"participant_name":       row.ParticipantName,
		"participant_group_id":   pgUUIDPointer(row.ParticipantGroupID),
		"participant_group_name": pgTextPointer(row.ParticipantGroupName),
		"role":                   row.Role,
		"result":                 row.Result,
		"metadata":               json.RawMessage(row.Metadata),
		"created_at":             formatTimestamp(row.CreatedAt),
	}
}

func occurrenceGroupToJSON(row db.ListActivityOccurrenceGroupsRow) gin.H {
	return gin.H{
		"id":                     row.ID,
		"activity_occurrence_id": pgUUIDString(row.ActivityOccurrenceID),
		"participant_group_id":   pgUUIDString(row.ParticipantGroupID),
		"participant_group_name": row.ParticipantGroupName,
		"role":                   row.Role,
		"result":                 row.Result,
		"metadata":               json.RawMessage(row.Metadata),
		"created_at":             formatTimestamp(row.CreatedAt),
	}
}

func visibleOccurrenceLedgerToJSON(row db.ListVisibleBonusPointLedgerEntriesByOccurrenceRow) gin.H {
	return gin.H{
		"id":                     pgUUIDString(row.ID),
		"instance_id":            pgUUIDString(row.InstanceID),
		"participant_id":         pgUUIDString(row.ParticipantID),
		"participant_name":       row.ParticipantName,
		"activity_occurrence_id": pgUUIDString(row.ActivityOccurrenceID),
		"occurrence_type":        row.OccurrenceType,
		"occurrence_name":        row.OccurrenceName,
		"activity_id":            pgUUIDString(row.ActivityID),
		"activity_type":          row.ActivityType,
		"activity_name":          row.ActivityName,
		"source_group_id":        pgUUIDPointer(row.SourceGroupID),
		"source_group_name":      pgTextPointer(row.SourceGroupName),
		"entry_kind":             row.EntryKind,
		"points":                 row.Points,
		"visibility":             row.Visibility,
		"reason":                 row.Reason,
		"effective_at":           formatTimestamp(row.EffectiveAt),
		"award_key":              pgTextPointer(row.AwardKey),
		"metadata":               json.RawMessage(row.Metadata),
		"created_at":             formatTimestamp(row.CreatedAt),
	}
}

func occurrenceHistoryInvolvementToJSON(row db.ListParticipantOccurrenceInvolvementByInstanceRow) gin.H {
	return gin.H{
		"id":                     row.OccurrenceParticipantResultID,
		"activity_occurrence_id": pgUUIDString(row.OccurrenceID),
		"participant_id":         pgUUIDString(row.ParticipantID),
		"participant_group_id":   pgUUIDPointer(row.ParticipantGroupID),
		"participant_group_name": pgTextPointer(row.ParticipantGroupName),
		"role":                   row.Role,
		"result":                 row.Result,
		"metadata":               json.RawMessage(row.ParticipantMetadata),
		"created_at":             formatTimestamp(row.ParticipantCreatedAt),
	}
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

func formatTimestamp(value pgtype.Timestamptz) string {
	return value.Time.UTC().Format("2006-01-02T15:04:05Z07:00")
}

func formatNullableTimestamp(value pgtype.Timestamptz) any {
	if !value.Valid {
		return nil
	}
	return formatTimestamp(value)
}

func defaultJSONB(raw *json.RawMessage) []byte {
	if raw == nil {
		return []byte("{}")
	}
	return []byte(*raw)
}

func optionalTime(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}

func optionalTimePtr(value *time.Time) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}
	return optionalTime(*value)
}

func pgUUIDString(value pgtype.UUID) string {
	return uuid.UUID(value.Bytes).String()
}

func pgUUIDPointer(value pgtype.UUID) *string {
	if !value.Valid {
		return nil
	}
	formatted := pgUUIDString(value)
	return &formatted
}

func pgTextPointer(value pgtype.Text) *string {
	if !value.Valid {
		return nil
	}
	formatted := value.String
	return &formatted
}

func optionalText(value *string) pgtype.Text {
	if value == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *value, Valid: true}
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
