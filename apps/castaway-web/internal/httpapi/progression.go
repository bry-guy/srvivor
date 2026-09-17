package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/conv"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/scoring"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type progressionCommandRequest struct {
	IdempotencyKey string    `json:"idempotency_key"`
	EffectiveAt    time.Time `json:"effective_at"`
}

type progressionFailure struct {
	status  int
	message string
}

func (e *progressionFailure) Error() string {
	return e.message
}

func progressionError(status int, message string) error {
	return &progressionFailure{status: status, message: message}
}

func bindProgressionCommand(c *gin.Context) (progressionCommandRequest, bool) {
	var req progressionCommandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return progressionCommandRequest{}, false
	}
	if strings.TrimSpace(req.IdempotencyKey) == "" {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "idempotency_key is required"})
		return progressionCommandRequest{}, false
	}
	if req.EffectiveAt.IsZero() {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "effective_at is required"})
		return progressionCommandRequest{}, false
	}
	req.IdempotencyKey = strings.TrimSpace(req.IdempotencyKey)
	req.EffectiveAt = req.EffectiveAt.UTC()
	return req, true
}

func progressionTimestamp(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value.UTC(), Valid: true}
}

func progressionPayloadHash(operation string, payload any) (string, []byte, error) {
	encoded, err := json.Marshal(struct {
		Operation string `json:"operation"`
		Payload   any    `json:"payload"`
	}{Operation: operation, Payload: payload})
	if err != nil {
		return "", nil, err
	}
	hash := sha256.Sum256(encoded)
	return hex.EncodeToString(hash[:]), encoded, nil
}

func (s *Server) requireManagedInstanceAdminRequest(c *gin.Context, instanceID uuid.UUID) bool {
	if _, ok := ServicePrincipal(c.Request.Context()); !ok {
		c.JSON(http.StatusUnauthorized, errorResponse{Error: "managed progression requires service authentication"})
		return false
	}
	return s.requireInstanceAdminRequest(c, instanceID)
}

func (s *Server) requireManagedAdminIfNeeded(c *gin.Context, instanceID uuid.UUID) (bool, bool) {
	mode, err := s.queries.GetInstanceProgressionMode(c.Request.Context(), toPGUUID(instanceID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, errorResponse{Error: "instance not found"})
		} else {
			c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		}
		return false, false
	}
	if mode != "managed" {
		return false, true
	}
	if !s.requireManagedInstanceAdminRequest(c, instanceID) {
		return true, false
	}
	return true, true
}

func (s *Server) runManagedSetupWrite(c *gin.Context, instanceID uuid.UUID, status int, rosterKind string, fn func(context.Context, *db.Queries) (any, error)) bool {
	managed, ok := s.requireManagedAdminIfNeeded(c, instanceID)
	if !ok {
		return true
	}
	if !managed {
		return false
	}
	tx, err := s.pool.Begin(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return true
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
	if _, err := qtx.LockInstanceForProgression(c.Request.Context(), toPGUUID(instanceID)); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, errorResponse{Error: "instance not found"})
		} else {
			c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		}
		return true
	}
	actor := discordUserIDFromRequest(c.Request)
	isAdmin, err := qtx.IsInstanceAdmin(c.Request.Context(), db.IsInstanceAdminParams{InstanceID: toPGUUID(instanceID), DiscordUserID: actor})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return true
	}
	if !isAdmin {
		c.JSON(http.StatusForbidden, errorResponse{Error: "forbidden"})
		return true
	}
	if rosterKind == "contestant" {
		draft, err := qtx.LockInstanceDraftProgress(c.Request.Context(), toPGUUID(instanceID))
		if err != nil {
			c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
			return true
		}
		if draft.Status != "pending" {
			c.JSON(http.StatusConflict, errorResponse{Error: "contestant setup is closed after draft opening"})
			return true
		}
	}
	if _, err := qtx.GetLatestManagedEpisodeProgress(c.Request.Context(), toPGUUID(instanceID)); err == nil {
		c.JSON(http.StatusConflict, errorResponse{Error: "managed roster setup is closed after episode progression starts"})
		return true
	} else if !errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return true
	}
	response, err := fn(c.Request.Context(), qtx)
	if err != nil {
		var failure *progressionFailure
		if errors.As(err, &failure) {
			c.JSON(failure.status, errorResponse{Error: failure.message})
		} else {
			c.JSON(statusFromPg(err), errorResponse{Error: err.Error()})
		}
		return true
	}
	if err := tx.Commit(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return true
	}
	c.JSON(status, response)
	return true
}

func (s *Server) rejectManagedActivityOperation(c *gin.Context, activityID uuid.UUID, operation string) bool {
	activity, err := s.queries.GetInstanceActivity(c.Request.Context(), toPGUUID(activityID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, errorResponse{Error: "activity not found"})
		} else {
			c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		}
		return true
	}
	return s.rejectManagedOperation(c, uuid.UUID(activity.InstanceID.Bytes), operation)
}

func (s *Server) rejectManagedOccurrenceOperation(c *gin.Context, occurrenceID uuid.UUID, operation string) bool {
	occurrence, err := s.queries.GetActivityOccurrence(c.Request.Context(), toPGUUID(occurrenceID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, errorResponse{Error: "occurrence not found"})
		} else {
			c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		}
		return true
	}
	return s.rejectManagedActivityOperation(c, uuid.UUID(occurrence.ActivityID.Bytes), operation)
}

func (s *Server) rejectManagedOperation(c *gin.Context, instanceID uuid.UUID, operation string) bool {
	mode, err := s.queries.GetInstanceProgressionMode(c.Request.Context(), toPGUUID(instanceID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, errorResponse{Error: "instance not found"})
		} else {
			c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		}
		return true
	}
	if mode == "managed" {
		c.JSON(http.StatusConflict, errorResponse{Error: operation + " is not supported for managed progression yet"})
		return true
	}
	return false
}

func (s *Server) runManagedCommand(
	c *gin.Context,
	instanceID uuid.UUID,
	operation string,
	req progressionCommandRequest,
	payload any,
	fn func(context.Context, *db.Queries) (any, error),
) {
	if !s.requireManagedInstanceAdminRequest(c, instanceID) {
		return
	}

	payloadHash, payloadJSON, err := progressionPayloadHash(operation, payload)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
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
	locked, err := qtx.LockInstanceForProgression(c.Request.Context(), toPGUUID(instanceID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, errorResponse{Error: "instance not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	if locked.ProgressionMode != "managed" {
		c.JSON(http.StatusConflict, errorResponse{Error: "instance does not use managed progression"})
		return
	}
	actor := discordUserIDFromRequest(c.Request)
	isAdmin, err := qtx.IsInstanceAdmin(c.Request.Context(), db.IsInstanceAdminParams{InstanceID: toPGUUID(instanceID), DiscordUserID: actor})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	if !isAdmin {
		c.JSON(http.StatusForbidden, errorResponse{Error: "forbidden"})
		return
	}

	existing, err := qtx.GetProgressionCommand(c.Request.Context(), db.GetProgressionCommandParams{
		InstanceID: toPGUUID(instanceID),
		CommandKey: req.IdempotencyKey,
	})
	if err == nil {
		if existing.PayloadHash != payloadHash {
			c.JSON(http.StatusConflict, errorResponse{Error: "idempotency key was already used with a different payload"})
			return
		}
		if err := tx.Commit(c.Request.Context()); err != nil {
			c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
			return
		}
		c.Data(http.StatusOK, "application/json", existing.Response)
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	response, err := fn(c.Request.Context(), qtx)
	if err != nil {
		var failure *progressionFailure
		if errors.As(err, &failure) {
			c.JSON(failure.status, errorResponse{Error: failure.message})
			return
		}
		status := statusFromPg(err)
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
		}
		c.JSON(status, errorResponse{Error: err.Error()})
		return
	}
	responseJSON, err := json.Marshal(response)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	if _, err := qtx.CreateProgressionCommand(c.Request.Context(), db.CreateProgressionCommandParams{
		InstanceID:         toPGUUID(instanceID),
		CommandKey:         req.IdempotencyKey,
		Operation:          operation,
		ActorDiscordUserID: discordUserIDFromRequest(c.Request),
		EffectiveAt:        progressionTimestamp(req.EffectiveAt),
		PayloadHash:        payloadHash,
		Payload:            payloadJSON,
		Response:           responseJSON,
	}); err != nil {
		c.JSON(statusFromPg(err), errorResponse{Error: err.Error()})
		return
	}
	if err := tx.Commit(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json", responseJSON)
}

func (s *Server) openDraft(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok {
		return
	}
	req, ok := bindProgressionCommand(c)
	if !ok {
		return
	}
	s.runManagedCommand(c, instanceID, "draft.open", req, req, func(ctx context.Context, q *db.Queries) (any, error) {
		draft, err := q.LockInstanceDraftProgress(ctx, toPGUUID(instanceID))
		if err != nil {
			return nil, err
		}
		if draft.Status != "pending" {
			return nil, progressionError(http.StatusConflict, "draft is not pending")
		}
		updated, err := q.UpdateInstanceDraftProgress(ctx, db.UpdateInstanceDraftProgressParams{
			InstanceID: toPGUUID(instanceID),
			Status:     "open",
			OpenedAt:   progressionTimestamp(req.EffectiveAt),
			ClosedAt:   pgtype.Timestamptz{},
		})
		if err != nil {
			return nil, err
		}
		return gin.H{"status": updated.Status, "opened_at": req.EffectiveAt.Format(time.RFC3339)}, nil
	})
}

func (s *Server) closeDraft(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok {
		return
	}
	req, ok := bindProgressionCommand(c)
	if !ok {
		return
	}
	s.runManagedCommand(c, instanceID, "draft.close", req, req, func(ctx context.Context, q *db.Queries) (any, error) {
		draft, err := q.LockInstanceDraftProgress(ctx, toPGUUID(instanceID))
		if err != nil {
			return nil, err
		}
		if draft.Status != "open" {
			return nil, progressionError(http.StatusConflict, "draft is not open")
		}
		if draft.OpenedAt.Valid && req.EffectiveAt.Before(draft.OpenedAt.Time) {
			return nil, progressionError(http.StatusConflict, "draft close cannot precede draft open")
		}
		updated, err := q.UpdateInstanceDraftProgress(ctx, db.UpdateInstanceDraftProgressParams{
			InstanceID: toPGUUID(instanceID),
			Status:     "closed",
			OpenedAt:   draft.OpenedAt,
			ClosedAt:   progressionTimestamp(req.EffectiveAt),
		})
		if err != nil {
			return nil, err
		}
		return gin.H{"status": updated.Status, "closed_at": req.EffectiveAt.Format(time.RFC3339)}, nil
	})
}

func parseEpisodeNumber(c *gin.Context) (int32, bool) {
	value, err := strconv.ParseInt(c.Param("episodeNumber"), 10, 32)
	if err != nil || value <= 0 {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "episode number must be a positive integer"})
		return 0, false
	}
	return int32(value), true
}

func (s *Server) startEpisode(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok {
		return
	}
	episodeNumber, ok := parseEpisodeNumber(c)
	if !ok {
		return
	}
	req, ok := bindProgressionCommand(c)
	if !ok {
		return
	}
	s.runManagedCommand(c, instanceID, "episode.start", req, struct {
		EpisodeNumber int32                     `json:"episode_number"`
		Command       progressionCommandRequest `json:"command"`
	}{episodeNumber, req}, func(ctx context.Context, q *db.Queries) (any, error) {
		progress, err := q.LockInstanceEpisodeProgress(ctx, db.LockInstanceEpisodeProgressParams{
			InstanceID:    toPGUUID(instanceID),
			EpisodeNumber: episodeNumber,
		})
		if err != nil {
			return nil, err
		}
		if progress.Status != "locked" {
			return nil, progressionError(http.StatusConflict, "episode is not locked")
		}
		if episodeNumber > 1 {
			previous, err := q.GetPreviousInstanceEpisodeProgress(ctx, db.GetPreviousInstanceEpisodeProgressParams{
				InstanceID:    toPGUUID(instanceID),
				EpisodeNumber: episodeNumber,
			})
			if err != nil {
				return nil, progressionError(http.StatusConflict, "previous episode is not scored")
			}
			if previous.Status != "scored" {
				return nil, progressionError(http.StatusConflict, "previous episode is not scored")
			}
			if previous.ScoredAt.Valid && req.EffectiveAt.Before(previous.ScoredAt.Time) {
				return nil, progressionError(http.StatusConflict, "episode start cannot precede previous episode scoring")
			}
		}
		updated, err := q.UpdateInstanceEpisodeProgress(ctx, db.UpdateInstanceEpisodeProgressParams{
			InstanceID:    toPGUUID(instanceID),
			EpisodeNumber: episodeNumber,
			Status:        "started",
			StartedAt:     progressionTimestamp(req.EffectiveAt),
			CompletedAt:   pgtype.Timestamptz{},
			ScoredAt:      pgtype.Timestamptz{},
		})
		if err != nil {
			return nil, err
		}
		return gin.H{"episode_number": updated.EpisodeNumber, "status": updated.Status, "started_at": req.EffectiveAt.Format(time.RFC3339)}, nil
	})
}

func (s *Server) completeEpisode(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok {
		return
	}
	episodeNumber, ok := parseEpisodeNumber(c)
	if !ok {
		return
	}
	req, ok := bindProgressionCommand(c)
	if !ok {
		return
	}
	s.runManagedCommand(c, instanceID, "episode.complete", req, struct {
		EpisodeNumber int32                     `json:"episode_number"`
		Command       progressionCommandRequest `json:"command"`
	}{episodeNumber, req}, func(ctx context.Context, q *db.Queries) (any, error) {
		progress, err := q.LockInstanceEpisodeProgress(ctx, db.LockInstanceEpisodeProgressParams{
			InstanceID:    toPGUUID(instanceID),
			EpisodeNumber: episodeNumber,
		})
		if err != nil {
			return nil, err
		}
		if progress.Status != "started" {
			return nil, progressionError(http.StatusConflict, "episode is not started")
		}
		if progress.StartedAt.Valid && req.EffectiveAt.Before(progress.StartedAt.Time) {
			return nil, progressionError(http.StatusConflict, "episode completion cannot precede episode start")
		}
		updated, err := q.UpdateInstanceEpisodeProgress(ctx, db.UpdateInstanceEpisodeProgressParams{
			InstanceID:    toPGUUID(instanceID),
			EpisodeNumber: episodeNumber,
			Status:        "completed",
			StartedAt:     progress.StartedAt,
			CompletedAt:   progressionTimestamp(req.EffectiveAt),
			ScoredAt:      pgtype.Timestamptz{},
		})
		if err != nil {
			return nil, err
		}
		return gin.H{"episode_number": updated.EpisodeNumber, "status": updated.Status, "completed_at": req.EffectiveAt.Format(time.RFC3339)}, nil
	})
}

func (s *Server) scoreEpisode(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok {
		return
	}
	episodeNumber, ok := parseEpisodeNumber(c)
	if !ok {
		return
	}
	req, ok := bindProgressionCommand(c)
	if !ok {
		return
	}
	s.runManagedCommand(c, instanceID, "episode.score", req, struct {
		EpisodeNumber int32                     `json:"episode_number"`
		Command       progressionCommandRequest `json:"command"`
	}{episodeNumber, req}, func(ctx context.Context, q *db.Queries) (any, error) {
		progress, err := q.LockInstanceEpisodeProgress(ctx, db.LockInstanceEpisodeProgressParams{
			InstanceID:    toPGUUID(instanceID),
			EpisodeNumber: episodeNumber,
		})
		if err != nil {
			return nil, err
		}
		if progress.Status != "completed" {
			return nil, progressionError(http.StatusConflict, "episode is not completed")
		}
		if progress.CompletedAt.Valid && req.EffectiveAt.Before(progress.CompletedAt.Time) {
			return nil, progressionError(http.StatusConflict, "episode scoring cannot precede episode completion")
		}
		latest, latestErr := q.GetLatestInstanceScoreRevision(ctx, toPGUUID(instanceID))
		if latestErr == nil && latest.EffectiveAt.Valid && req.EffectiveAt.Before(latest.EffectiveAt.Time) {
			return nil, progressionError(http.StatusConflict, "episode scoring cannot precede the latest correction publication")
		}
		if latestErr != nil && !errors.Is(latestErr, pgx.ErrNoRows) {
			return nil, latestErr
		}
		hasFutureOutcome, err := q.HasFutureOutcomeCommand(ctx, db.HasFutureOutcomeCommandParams{
			InstanceID:  toPGUUID(instanceID),
			EffectiveAt: progressionTimestamp(req.EffectiveAt),
		})
		if err != nil {
			return nil, err
		}
		if hasFutureOutcome {
			return nil, progressionError(http.StatusConflict, "episode scoring cannot precede an outcome effective time")
		}
		revision, err := s.publishScoreRevision(ctx, q, instanceID, episodeNumber, "episode score", req.EffectiveAt)
		if err != nil {
			return nil, err
		}
		updated, err := q.UpdateInstanceEpisodeProgress(ctx, db.UpdateInstanceEpisodeProgressParams{
			InstanceID:    toPGUUID(instanceID),
			EpisodeNumber: episodeNumber,
			Status:        "scored",
			StartedAt:     progress.StartedAt,
			CompletedAt:   progress.CompletedAt,
			ScoredAt:      progressionTimestamp(req.EffectiveAt),
		})
		if err != nil {
			return nil, err
		}
		return gin.H{
			"episode_number":  updated.EpisodeNumber,
			"status":          updated.Status,
			"revision_number": revision.RevisionNumber,
			"scored_at":       req.EffectiveAt.Format(time.RFC3339),
		}, nil
	})
}

type scoreSnapshotDraft struct {
	Position     int    `json:"position"`
	ContestantID string `json:"contestant_id"`
}

type scoreInputSnapshot struct {
	TotalPositions   int                             `json:"total_positions"`
	Outcomes         map[string]int                  `json:"outcomes"`
	OutcomeNames     map[string]string               `json:"outcome_names"`
	Drafts           map[string][]scoreSnapshotDraft `json:"drafts"`
	VisibleBonus     map[string]int                  `json:"visible_bonus"`
	ParticipantNames map[string]string               `json:"participant_names"`
	TribeNames       map[string]string               `json:"tribe_names"`
}

type calculatedLeaderboard struct {
	Entries        []scoring.LeaderboardEntry
	ParticipantIDs map[string]pgtype.UUID
	Snapshot       scoreInputSnapshot
}

func tribeNameAt(ctx context.Context, q *db.Queries, participantID pgtype.UUID, at time.Time) (string, error) {
	memberships, err := q.ListActiveParticipantMembershipsAt(ctx, db.ListActiveParticipantMembershipsAtParams{
		ParticipantID: participantID,
		At:            optionalTime(at),
	})
	if err != nil {
		return "", err
	}
	for _, membership := range memberships {
		if strings.EqualFold(strings.TrimSpace(membership.ParticipantGroupKind), "tribe") {
			return membership.ParticipantGroupName, nil
		}
	}
	return "", nil
}

func (s *Server) calculateLeaderboardAt(ctx context.Context, q *db.Queries, instanceID uuid.UUID, at time.Time) (calculatedLeaderboard, error) {
	contestants, err := q.ListContestantsByInstance(ctx, toPGUUID(instanceID))
	if err != nil {
		return calculatedLeaderboard{}, err
	}
	participants, err := q.ListParticipantsByInstance(ctx, toPGUUID(instanceID))
	if err != nil {
		return calculatedLeaderboard{}, err
	}
	draftPicks, err := q.ListDraftPicksForInstance(ctx, toPGUUID(instanceID))
	if err != nil {
		return calculatedLeaderboard{}, err
	}
	outcomes, err := q.ListOutcomePositionsByInstance(ctx, toPGUUID(instanceID))
	if err != nil {
		return calculatedLeaderboard{}, err
	}

	participantNames := make(map[string]string, len(participants))
	participantIDs := make(map[string]pgtype.UUID, len(participants))
	visibleBonus := make(map[string]int, len(participants))
	tribeNames := make(map[string]string, len(participants))
	for _, participant := range participants {
		id := uuid.UUID(participant.ID.Bytes).String()
		participantNames[id] = participant.Name
		participantIDs[id] = participant.ID
		bonus, err := q.GetVisibleBonusTotalByParticipantAsOf(ctx, db.GetVisibleBonusTotalByParticipantAsOfParams{
			InstanceID:    toPGUUID(instanceID),
			ParticipantID: participant.ID,
			AsOf:          progressionTimestamp(at),
		})
		if err != nil {
			return calculatedLeaderboard{}, err
		}
		visibleBonus[id] = int(bonus)
		tribeName, err := tribeNameAt(ctx, q, participant.ID, at)
		if err != nil {
			return calculatedLeaderboard{}, err
		}
		tribeNames[id] = tribeName
	}

	draftsByParticipant := make(map[string][]scoring.DraftPick, len(participants))
	draftSnapshot := make(map[string][]scoreSnapshotDraft, len(participants))
	for _, pick := range draftPicks {
		participantID := uuid.UUID(pick.ParticipantID.Bytes).String()
		contestantID := uuid.UUID(pick.ContestantID.Bytes).String()
		draftsByParticipant[participantID] = append(draftsByParticipant[participantID], scoring.DraftPick{
			Position:     int(pick.Position),
			ContestantID: contestantID,
		})
		draftSnapshot[participantID] = append(draftSnapshot[participantID], scoreSnapshotDraft{
			Position:     int(pick.Position),
			ContestantID: contestantID,
		})
	}
	contestantNames := make(map[string]string, len(contestants))
	for _, contestant := range contestants {
		contestantNames[uuid.UUID(contestant.ID.Bytes).String()] = contestant.Name
	}
	for participantID := range participantIDs {
		if _, ok := draftSnapshot[participantID]; !ok {
			draftSnapshot[participantID] = []scoreSnapshotDraft{}
		}
	}
	finalPositions := make(map[string]int, len(outcomes))
	for _, outcome := range outcomes {
		if outcome.ContestantID.Valid {
			finalPositions[uuid.UUID(outcome.ContestantID.Bytes).String()] = int(outcome.Position)
		}
	}
	entries := scoring.CalculateLeaderboard(len(contestants), participantNames, draftsByParticipant, finalPositions, visibleBonus)
	return calculatedLeaderboard{
		Entries:        entries,
		ParticipantIDs: participantIDs,
		Snapshot: scoreInputSnapshot{
			TotalPositions:   len(contestants),
			Outcomes:         finalPositions,
			OutcomeNames:     contestantNames,
			Drafts:           draftSnapshot,
			VisibleBonus:     visibleBonus,
			ParticipantNames: participantNames,
			TribeNames:       tribeNames,
		},
	}, nil
}

func (s *Server) publishScoreRevision(ctx context.Context, q *db.Queries, instanceID uuid.UUID, episodeNumber int32, reason string, effectiveAt time.Time) (db.CreateInstanceScoreRevisionRow, error) {
	calculated, err := s.calculateLeaderboardAt(ctx, q, instanceID, effectiveAt)
	if err != nil {
		return db.CreateInstanceScoreRevisionRow{}, err
	}
	return s.publishScoreSnapshot(ctx, q, instanceID, episodeNumber, reason, effectiveAt, calculated.Snapshot)
}

func (s *Server) publishScoreSnapshot(ctx context.Context, q *db.Queries, instanceID uuid.UUID, episodeNumber int32, reason string, effectiveAt time.Time, snapshot scoreInputSnapshot) (db.CreateInstanceScoreRevisionRow, error) {
	participants, err := q.ListParticipantsByInstance(ctx, toPGUUID(instanceID))
	if err != nil {
		return db.CreateInstanceScoreRevisionRow{}, err
	}
	if snapshot.TotalPositions <= 0 || snapshot.ParticipantNames == nil || snapshot.Drafts == nil || snapshot.Outcomes == nil || snapshot.OutcomeNames == nil || snapshot.VisibleBonus == nil || snapshot.TribeNames == nil {
		return db.CreateInstanceScoreRevisionRow{}, fmt.Errorf("invalid score publication snapshot")
	}
	participantByID := make(map[string]db.ListParticipantsByInstanceRow, len(participants))
	for _, participant := range participants {
		participantByID[uuid.UUID(participant.ID.Bytes).String()] = participant
	}
	participantNames := make(map[string]string, len(snapshot.ParticipantNames))
	participantIDs := make(map[string]pgtype.UUID, len(snapshot.ParticipantNames))
	for id, name := range snapshot.ParticipantNames {
		if strings.TrimSpace(name) == "" {
			return db.CreateInstanceScoreRevisionRow{}, fmt.Errorf("score publication snapshot has an empty participant name for %s", id)
		}
		parsedID, err := uuid.Parse(id)
		if err != nil {
			return db.CreateInstanceScoreRevisionRow{}, fmt.Errorf("score publication snapshot has invalid participant %s", id)
		}
		if _, ok := participantByID[id]; !ok {
			return db.CreateInstanceScoreRevisionRow{}, fmt.Errorf("score publication snapshot references unknown participant %s", id)
		}
		if _, ok := snapshot.Drafts[id]; !ok {
			return db.CreateInstanceScoreRevisionRow{}, fmt.Errorf("score publication snapshot is missing draft %s", id)
		}
		if _, ok := snapshot.VisibleBonus[id]; !ok {
			return db.CreateInstanceScoreRevisionRow{}, fmt.Errorf("score publication snapshot is missing bonus total %s", id)
		}
		if _, ok := snapshot.TribeNames[id]; !ok {
			return db.CreateInstanceScoreRevisionRow{}, fmt.Errorf("score publication snapshot is missing tribe context %s", id)
		}
		participantIDs[id] = pgtype.UUID{Bytes: parsedID, Valid: true}
		participantNames[id] = name
	}
	for contestantID := range snapshot.Outcomes {
		if _, ok := snapshot.OutcomeNames[contestantID]; !ok {
			return db.CreateInstanceScoreRevisionRow{}, fmt.Errorf("score publication snapshot is missing outcome name %s", contestantID)
		}
	}
	draftsByParticipant := make(map[string][]scoring.DraftPick, len(snapshot.Drafts))
	for participantID, picks := range snapshot.Drafts {
		for _, pick := range picks {
			draftsByParticipant[participantID] = append(draftsByParticipant[participantID], scoring.DraftPick{
				Position:     pick.Position,
				ContestantID: pick.ContestantID,
			})
		}
	}
	entries := scoring.CalculateLeaderboard(snapshot.TotalPositions, participantNames, draftsByParticipant, snapshot.Outcomes, snapshot.VisibleBonus)
	inputSnapshot, err := json.Marshal(snapshot)
	if err != nil {
		return db.CreateInstanceScoreRevisionRow{}, err
	}
	revisionNumber, err := q.NextInstanceScoreRevisionNumber(ctx, toPGUUID(instanceID))
	if err != nil {
		return db.CreateInstanceScoreRevisionRow{}, err
	}
	revision, err := q.CreateInstanceScoreRevision(ctx, db.CreateInstanceScoreRevisionParams{
		InstanceID:     toPGUUID(instanceID),
		RevisionNumber: revisionNumber,
		EpisodeNumber:  episodeNumber,
		Reason:         reason,
		EffectiveAt:    progressionTimestamp(effectiveAt),
		InputSnapshot:  inputSnapshot,
	})
	if err != nil {
		return db.CreateInstanceScoreRevisionRow{}, err
	}
	for _, entry := range entries {
		participantID := participantIDs[entry.ParticipantID]
		score, err := conv.ToInt32(entry.Score)
		if err != nil {
			return db.CreateInstanceScoreRevisionRow{}, err
		}
		draftPoints, err := conv.ToInt32(entry.DraftPoints)
		if err != nil {
			return db.CreateInstanceScoreRevisionRow{}, err
		}
		bonusPoints, err := conv.ToInt32(entry.BonusPoints)
		if err != nil {
			return db.CreateInstanceScoreRevisionRow{}, err
		}
		totalPoints, err := conv.ToInt32(entry.TotalPoints)
		if err != nil {
			return db.CreateInstanceScoreRevisionRow{}, err
		}
		pointsAvailable, err := conv.ToInt32(entry.PointsAvailable)
		if err != nil {
			return db.CreateInstanceScoreRevisionRow{}, err
		}
		if err := q.CreateInstanceScoreRevisionRow(ctx, db.CreateInstanceScoreRevisionRowParams{
			RevisionID:      revision.ID,
			ParticipantName: entry.ParticipantName,
			Score:           score,
			DraftPoints:     draftPoints,
			BonusPoints:     bonusPoints,
			TotalPoints:     totalPoints,
			PointsAvailable: pointsAvailable,
			InstanceID:      toPGUUID(instanceID),
			ParticipantID:   participantID,
		}); err != nil {
			return db.CreateInstanceScoreRevisionRow{}, err
		}
	}
	return revision, nil
}

func scoreSnapshotFromRevision(revision db.GetLatestInstanceScoreRevisionRow) (scoreInputSnapshot, error) {
	var snapshot scoreInputSnapshot
	if err := json.Unmarshal(revision.InputSnapshot, &snapshot); err != nil {
		return scoreInputSnapshot{}, err
	}
	return snapshot, nil
}

func (s *Server) managedLeaderboard(c *gin.Context, instanceID uuid.UUID, participantFilter *uuid.UUID) {
	rows, err := s.queries.ListLatestInstanceScoreRevisionRows(c.Request.Context(), toPGUUID(instanceID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	tribeNames := map[string]string{}
	if len(rows) > 0 {
		var snapshot scoreInputSnapshot
		if err := json.Unmarshal(rows[0].InputSnapshot, &snapshot); err != nil {
			c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
			return
		}
		tribeNames = snapshot.TribeNames
	}
	participants, err := s.queries.ListParticipantsByInstance(c.Request.Context(), toPGUUID(instanceID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	discordIDs := make(map[string]string, len(participants))
	for _, participant := range participants {
		if participant.DiscordUserID.Valid {
			discordIDs[uuid.UUID(participant.ID.Bytes).String()] = participant.DiscordUserID.String
		}
	}
	response := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		participantID := uuid.UUID(row.ParticipantID.Bytes)
		if participantFilter != nil && participantID != *participantFilter {
			continue
		}
		response = append(response, gin.H{
			"participant_id":              participantID.String(),
			"participant_name":            row.ParticipantName,
			"participant_discord_user_id": discordIDs[participantID.String()],
			"current_tribe_name":          tribeNames[participantID.String()],
			"score":                       row.Score,
			"draft_points":                row.DraftPoints,
			"bonus_points":                row.BonusPoints,
			"total_points":                row.TotalPoints,
			"points_available":            row.PointsAvailable,
		})
	}
	c.JSON(http.StatusOK, gin.H{"leaderboard": response})
}
