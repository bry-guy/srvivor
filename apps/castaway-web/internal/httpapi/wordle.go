package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/gameplay"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	wordleActivityType    = "tribe_wordle"
	wordleParticipantRole = "participant"
	wordleRoundKeyLimit   = 64
)

type createWordleRoundRequest struct {
	RoundKey string    `json:"round_key" binding:"required"`
	Name     string    `json:"name" binding:"required"`
	OpensAt  time.Time `json:"opens_at" binding:"required"`
	CutoffAt time.Time `json:"cutoff_at" binding:"required"`
}

type putWordleParticipantRequest struct {
	ParticipantGroupID string `json:"participant_group_id" binding:"required"`
	GuessCount         int32  `json:"guess_count" binding:"required"`
}

type wordleRoundData struct {
	ID                 pgtype.UUID
	InstanceID         pgtype.UUID
	ActivityID         pgtype.UUID
	ActivityType       string
	OccurrenceType     string
	OccurrenceStatus   string
	EffectiveAt        pgtype.Timestamptz
	Name               string
	RoundKey           string
	OpensAt            pgtype.Timestamptz
	CutoffAt           pgtype.Timestamptz
	ClosedAt           pgtype.Timestamptz
	ResolutionResponse []byte
	CreatedAt          pgtype.Timestamptz
	UpdatedAt          pgtype.Timestamptz
}

func (s *Server) requireWordleServicePrincipal(c *gin.Context) bool {
	if _, ok := ServicePrincipal(c.Request.Context()); !ok {
		c.JSON(http.StatusUnauthorized, errorResponse{Error: "Wordle lifecycle requires service authentication"})
		return false
	}
	return true
}

func (s *Server) rejectWordleRoundOperation(c *gin.Context, roundID uuid.UUID, operation string) bool {
	_, err := s.queries.GetWordleRound(c.Request.Context(), toPGUUID(roundID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false
		}
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return true
	}
	c.JSON(http.StatusConflict, errorResponse{Error: operation + " is not supported for a lifecycle-owned Wordle round; use the Wordle round endpoint"})
	return true
}

func (s *Server) createWordleRound(c *gin.Context) {
	if !s.requireWordleServicePrincipal(c) {
		return
	}
	activityID, ok := parseUUIDPath(c, "activityID")
	if !ok {
		return
	}

	var req createWordleRoundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	roundKey := strings.TrimSpace(req.RoundKey)
	name := strings.TrimSpace(req.Name)
	if roundKey == "" || utf8.RuneCountInString(roundKey) > wordleRoundKeyLimit {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "round_key must be between 1 and 64 characters"})
		return
	}
	if name == "" {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "name cannot be empty"})
		return
	}
	if req.OpensAt.IsZero() || req.CutoffAt.IsZero() {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "opens_at and cutoff_at are required"})
		return
	}
	opensAt := req.OpensAt.UTC().Truncate(time.Microsecond)
	cutoffAt := req.CutoffAt.UTC().Truncate(time.Microsecond)
	if !cutoffAt.After(opensAt) {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "cutoff_at must be after opens_at"})
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
	if activity.ActivityType != wordleActivityType {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "activity must have type tribe_wordle"})
		return
	}
	instanceID := uuid.UUID(activity.InstanceID.Bytes)
	if !s.requireInstanceAdminRequest(c, instanceID) {
		return
	}

	tx, err := s.pool.Begin(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	defer rollbackTx(c, tx)
	qtx := s.queries.WithTx(tx)

	lockedInstance, err := qtx.LockInstanceForProgression(c.Request.Context(), toPGUUID(instanceID))
	if err != nil {
		writeWordleError(c, err)
		return
	}
	if lockedInstance.ProgressionMode != "legacy" {
		c.JSON(http.StatusConflict, errorResponse{Error: "Wordle lifecycle is only supported for legacy instances"})
		return
	}
	if err := requireWordleInstanceAdmin(c.Request.Context(), qtx, lockedInstance.ID, c.Request); err != nil {
		writeWordleError(c, err)
		return
	}

	activity, err = qtx.GetInstanceActivity(c.Request.Context(), toPGUUID(activityID))
	if err != nil {
		writeWordleError(c, err)
		return
	}
	if !sameWordleUUID(activity.InstanceID, lockedInstance.ID) || activity.ActivityType != wordleActivityType {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "activity is not a tribe_wordle activity in this instance"})
		return
	}

	existing, err := qtx.GetWordleRoundByActivityAndKey(c.Request.Context(), db.GetWordleRoundByActivityAndKeyParams{
		ActivityID: toPGUUID(activityID),
		RoundKey:   roundKey,
	})
	if err == nil {
		if existing.Name != name || !existing.OpensAt.Time.Equal(opensAt) || !existing.CutoffAt.Time.Equal(cutoffAt) {
			c.JSON(http.StatusConflict, errorResponse{Error: "round_key already exists with different round details"})
			return
		}
		if err := tx.Commit(c.Request.Context()); err != nil {
			c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"round": wordleRoundJSON(wordleRoundFromActivityKey(existing))})
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		writeWordleError(c, err)
		return
	}

	occurrence, err := qtx.CreateActivityOccurrence(c.Request.Context(), db.CreateActivityOccurrenceParams{
		ActivityID:     toPGUUID(activityID),
		OccurrenceType: wordleActivityType,
		Name:           name,
		EffectiveAt:    wordleTimestamp(cutoffAt),
		Status:         "recorded",
		Metadata:       []byte(`{}`),
	})
	if err != nil {
		writeWordleError(c, err)
		return
	}
	if _, err := qtx.CreateWordleRound(c.Request.Context(), db.CreateWordleRoundParams{
		ActivityID:           toPGUUID(activityID),
		ActivityOccurrenceID: occurrence.ID,
		RoundKey:             roundKey,
		OpensAt:              wordleTimestamp(opensAt),
		CutoffAt:             wordleTimestamp(cutoffAt),
	}); err != nil {
		writeWordleError(c, err)
		return
	}
	round, err := qtx.GetWordleRound(c.Request.Context(), occurrence.ID)
	if err != nil {
		writeWordleError(c, err)
		return
	}
	if err := tx.Commit(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"round": wordleRoundJSON(wordleRoundFromGet(round))})
}

func (s *Server) getWordleRound(c *gin.Context) {
	if !s.requireWordleServicePrincipal(c) {
		return
	}
	roundID, ok := parseUUIDPath(c, "roundID")
	if !ok {
		return
	}
	tx, qtx, round, ok := s.lockWordleRound(c, roundID)
	if !ok {
		return
	}
	defer rollbackTx(c, tx)
	participants, err := qtx.ListActivityOccurrenceParticipants(c.Request.Context(), toPGUUID(roundID))
	if err != nil {
		writeWordleError(c, err)
		return
	}
	response := make([]gin.H, 0, len(participants))
	for _, participant := range participants {
		response = append(response, gin.H{
			"id":                     participant.ID,
			"participant_id":         pgUUIDString(participant.ParticipantID),
			"participant_name":       participant.ParticipantName,
			"participant_group_id":   pgUUIDPointer(participant.ParticipantGroupID),
			"participant_group_name": pgTextPointer(participant.ParticipantGroupName),
			"role":                   participant.Role,
			"result":                 participant.Result,
			"metadata":               json.RawMessage(participant.Metadata),
			"created_at":             formatTimestamp(participant.CreatedAt),
		})
	}
	if err := tx.Commit(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"round":        wordleRoundJSON(wordleRoundFromLock(round)),
		"participants": response,
	})
}

func (s *Server) putWordleParticipant(c *gin.Context) {
	roundID, ok := parseUUIDPath(c, "roundID")
	if !ok {
		return
	}
	participantID, ok := parseUUIDPath(c, "participantID")
	if !ok {
		return
	}
	var req putWordleParticipantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	if req.GuessCount <= 0 {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "guess_count must be positive"})
		return
	}
	groupID, err := uuid.Parse(strings.TrimSpace(req.ParticipantGroupID))
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid participant_group_id"})
		return
	}

	tx, qtx, round, ok := s.lockWordleRound(c, roundID)
	if !ok {
		return
	}
	defer rollbackTx(c, tx)
	now := wordleClockNow(s)
	if now.Before(round.OpensAt.Time) || !now.Before(round.CutoffAt.Time) {
		c.JSON(http.StatusConflict, errorResponse{Error: "Wordle submissions are closed outside the round window"})
		return
	}
	if round.ClosedAt.Valid || len(round.ResolutionResponse) > 0 || round.OccurrenceStatus != "recorded" {
		c.JSON(http.StatusConflict, errorResponse{Error: "Wordle round is closed"})
		return
	}

	participant, err := qtx.GetParticipant(c.Request.Context(), toPGUUID(participantID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusBadRequest, errorResponse{Error: "participant does not belong to this instance"})
			return
		}
		writeWordleError(c, err)
		return
	}
	if !sameWordleUUID(participant.InstanceID, round.InstanceID) {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "participant does not belong to this instance"})
		return
	}
	group, err := qtx.GetParticipantGroup(c.Request.Context(), toPGUUID(groupID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusBadRequest, errorResponse{Error: "participant group does not belong to this instance"})
			return
		}
		writeWordleError(c, err)
		return
	}
	if !sameWordleUUID(group.InstanceID, round.InstanceID) {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "participant group does not belong to this instance"})
		return
	}
	if !strings.EqualFold(strings.TrimSpace(group.Kind), "tribe") {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "participant group must be a tribe"})
		return
	}
	members, err := qtx.ListActiveParticipantGroupMembershipsAt(c.Request.Context(), db.ListActiveParticipantGroupMembershipsAtParams{
		ParticipantGroupID: toPGUUID(groupID),
		At:                 round.CutoffAt,
	})
	if err != nil {
		writeWordleError(c, err)
		return
	}
	member := false
	for _, candidate := range members {
		if sameWordleUUID(candidate.ParticipantID, toPGUUID(participantID)) {
			member = true
			break
		}
	}
	if !member {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "participant is not an active member of the tribe at cutoff"})
		return
	}
	metadata, err := json.Marshal(map[string]int32{"guess_count": req.GuessCount})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	created, err := qtx.UpsertActivityOccurrenceParticipant(c.Request.Context(), db.UpsertActivityOccurrenceParticipantParams{
		ActivityOccurrenceID: toPGUUID(roundID),
		ParticipantID:        toPGUUID(participantID),
		ParticipantGroupID:   toPGUUID(groupID),
		Role:                 wordleParticipantRole,
		Metadata:             metadata,
	})
	if err != nil {
		writeWordleError(c, err)
		return
	}
	if err := tx.Commit(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"participant_result": gin.H{
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

func (s *Server) closeWordleRound(c *gin.Context) {
	roundID, ok := parseUUIDPath(c, "roundID")
	if !ok {
		return
	}
	tx, qtx, round, ok := s.lockWordleRound(c, roundID)
	if !ok {
		return
	}
	defer rollbackTx(c, tx)
	if !round.ClosedAt.Valid {
		now := wordleClockNow(s)
		closed, err := qtx.UpdateWordleRoundClosedAt(c.Request.Context(), db.UpdateWordleRoundClosedAtParams{
			ID:       toPGUUID(roundID),
			ClosedAt: wordleTimestamp(now),
		})
		if err != nil {
			writeWordleError(c, err)
			return
		}
		round.ClosedAt = closed.ClosedAt
		round.UpdatedAt = closed.UpdatedAt
	}
	if err := tx.Commit(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"round": wordleRoundJSON(wordleRoundFromLock(round))})
}

func (s *Server) resolveWordleRound(c *gin.Context) {
	roundID, ok := parseUUIDPath(c, "roundID")
	if !ok {
		return
	}
	tx, qtx, round, ok := s.lockWordleRound(c, roundID)
	if !ok {
		return
	}
	defer rollbackTx(c, tx)
	if len(round.ResolutionResponse) > 0 {
		payload := append([]byte(nil), round.ResolutionResponse...)
		if err := tx.Commit(c.Request.Context()); err != nil {
			c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
			return
		}
		c.Data(http.StatusOK, "application/json; charset=utf-8", payload)
		return
	}
	now := wordleClockNow(s)
	if now.Before(round.CutoffAt.Time) {
		c.JSON(http.StatusConflict, errorResponse{Error: "Wordle round cannot resolve before cutoff"})
		return
	}
	if !round.ClosedAt.Valid {
		c.JSON(http.StatusConflict, errorResponse{Error: "Wordle round must be closed before resolution"})
		return
	}
	if round.OccurrenceStatus != "recorded" {
		c.JSON(http.StatusConflict, errorResponse{Error: "Wordle round is not resolvable"})
		return
	}

	createdEntries, err := gameplay.NewService(qtx).ResolveActivityOccurrence(c.Request.Context(), toPGUUID(roundID))
	if err != nil {
		writeWordleError(c, err)
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
	payload, err := json.Marshal(gin.H{"created_entries": response, "created_count": len(response)})
	if err != nil {
		writeWordleError(c, err)
		return
	}
	stored, err := qtx.UpdateWordleRoundResolution(c.Request.Context(), db.UpdateWordleRoundResolutionParams{
		ID:                 toPGUUID(roundID),
		ResolutionResponse: payload,
	})
	if err != nil {
		writeWordleError(c, err)
		return
	}
	payload = append([]byte(nil), stored.ResolutionResponse...)
	if err := tx.Commit(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", payload)
}

func (s *Server) lockWordleRound(c *gin.Context, roundID uuid.UUID) (pgx.Tx, *db.Queries, db.LockWordleRoundRow, bool) {
	if !s.requireWordleServicePrincipal(c) {
		return nil, nil, db.LockWordleRoundRow{}, false
	}
	round, err := s.queries.GetWordleRound(c.Request.Context(), toPGUUID(roundID))
	if err != nil {
		writeWordleError(c, err)
		return nil, nil, db.LockWordleRoundRow{}, false
	}
	if !s.requireInstanceAdminRequest(c, uuid.UUID(round.InstanceID.Bytes)) {
		return nil, nil, db.LockWordleRoundRow{}, false
	}
	tx, err := s.pool.Begin(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return nil, nil, db.LockWordleRoundRow{}, false
	}
	qtx := s.queries.WithTx(tx)
	lockedInstance, err := qtx.LockInstanceForProgression(c.Request.Context(), round.InstanceID)
	if err != nil {
		rollbackTx(c, tx)
		writeWordleError(c, err)
		return nil, nil, db.LockWordleRoundRow{}, false
	}
	if lockedInstance.ProgressionMode != "legacy" {
		rollbackTx(c, tx)
		c.JSON(http.StatusConflict, errorResponse{Error: "Wordle lifecycle is only supported for legacy instances"})
		return nil, nil, db.LockWordleRoundRow{}, false
	}
	if err := requireWordleInstanceAdmin(c.Request.Context(), qtx, lockedInstance.ID, c.Request); err != nil {
		rollbackTx(c, tx)
		writeWordleError(c, err)
		return nil, nil, db.LockWordleRoundRow{}, false
	}
	lockedRound, err := qtx.LockWordleRound(c.Request.Context(), toPGUUID(roundID))
	if err != nil {
		rollbackTx(c, tx)
		writeWordleError(c, err)
		return nil, nil, db.LockWordleRoundRow{}, false
	}
	if lockedRound.ActivityType != wordleActivityType || !sameWordleUUID(lockedRound.InstanceID, lockedInstance.ID) {
		rollbackTx(c, tx)
		c.JSON(http.StatusConflict, errorResponse{Error: "round is not a tribe_wordle lifecycle round"})
		return nil, nil, db.LockWordleRoundRow{}, false
	}
	return tx, qtx, lockedRound, true
}

func requireWordleInstanceAdmin(ctx context.Context, q *db.Queries, instanceID pgtype.UUID, request *http.Request) error {
	actor := strings.TrimSpace(discordUserIDFromRequest(request))
	if actor == "" {
		return progressionError(http.StatusBadRequest, "missing discord user id")
	}
	isAdmin, err := q.IsInstanceAdmin(ctx, db.IsInstanceAdminParams{InstanceID: instanceID, DiscordUserID: actor})
	if err != nil {
		return err
	}
	if !isAdmin {
		return progressionError(http.StatusForbidden, "forbidden")
	}
	return nil
}

func writeWordleError(c *gin.Context, err error) {
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, errorResponse{Error: "Wordle round not found"})
		return
	}
	var failure *progressionFailure
	if errors.As(err, &failure) {
		c.JSON(failure.status, errorResponse{Error: failure.message})
		return
	}
	c.JSON(statusFromPg(err), errorResponse{Error: err.Error()})
}

func sameWordleUUID(left, right pgtype.UUID) bool {
	return left.Valid && right.Valid && left.Bytes == right.Bytes
}

func wordleRoundFromGet(row db.GetWordleRoundRow) wordleRoundData {
	return wordleRoundData{
		ID: row.ID, InstanceID: row.InstanceID, ActivityID: row.ActivityID, ActivityType: row.ActivityType,
		OccurrenceType: row.OccurrenceType, OccurrenceStatus: row.OccurrenceStatus, EffectiveAt: row.EffectiveAt,
		Name: row.Name, RoundKey: row.RoundKey, OpensAt: row.OpensAt, CutoffAt: row.CutoffAt, ClosedAt: row.ClosedAt,
		ResolutionResponse: row.ResolutionResponse, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func wordleRoundFromActivityKey(row db.GetWordleRoundByActivityAndKeyRow) wordleRoundData {
	return wordleRoundData{
		ID: row.ID, InstanceID: row.InstanceID, ActivityID: row.ActivityID, ActivityType: row.ActivityType,
		OccurrenceType: row.OccurrenceType, OccurrenceStatus: row.OccurrenceStatus, EffectiveAt: row.EffectiveAt,
		Name: row.Name, RoundKey: row.RoundKey, OpensAt: row.OpensAt, CutoffAt: row.CutoffAt, ClosedAt: row.ClosedAt,
		ResolutionResponse: row.ResolutionResponse, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func wordleRoundFromLock(row db.LockWordleRoundRow) wordleRoundData {
	return wordleRoundData{
		ID: row.ID, InstanceID: row.InstanceID, ActivityID: row.ActivityID, ActivityType: row.ActivityType,
		OccurrenceType: row.OccurrenceType, OccurrenceStatus: row.OccurrenceStatus, EffectiveAt: row.EffectiveAt,
		Name: row.Name, RoundKey: row.RoundKey, OpensAt: row.OpensAt, CutoffAt: row.CutoffAt, ClosedAt: row.ClosedAt,
		ResolutionResponse: row.ResolutionResponse, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func wordleClockNow(s *Server) time.Time {
	return s.now().UTC().Truncate(time.Microsecond)
}

func wordleTimestamp(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value.UTC().Truncate(time.Microsecond), Valid: true}
}

func wordleFormatTimestamp(value pgtype.Timestamptz) string {
	return value.Time.UTC().Format(time.RFC3339Nano)
}

func wordleFormatNullableTimestamp(value pgtype.Timestamptz) any {
	if !value.Valid {
		return nil
	}
	return wordleFormatTimestamp(value)
}

func wordleRoundJSON(round wordleRoundData) gin.H {
	var resolution any
	if len(round.ResolutionResponse) > 0 {
		resolution = json.RawMessage(round.ResolutionResponse)
	}
	return gin.H{
		"id":              pgUUIDString(round.ID),
		"instance_id":     pgUUIDString(round.InstanceID),
		"activity_id":     pgUUIDString(round.ActivityID),
		"activity_type":   round.ActivityType,
		"occurrence_type": round.OccurrenceType,
		"status":          round.OccurrenceStatus,
		"name":            round.Name,
		"round_key":       round.RoundKey,
		"effective_at":    wordleFormatTimestamp(round.EffectiveAt),
		"opens_at":        wordleFormatTimestamp(round.OpensAt),
		"cutoff_at":       wordleFormatTimestamp(round.CutoffAt),
		"closed_at":       wordleFormatNullableTimestamp(round.ClosedAt),
		"resolved":        len(round.ResolutionResponse) > 0,
		"resolution":      resolution,
		"created_at":      wordleFormatTimestamp(round.CreatedAt),
		"updated_at":      wordleFormatTimestamp(round.UpdatedAt),
	}
}
