package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/conv"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (s *Server) acceptLateDraft(c *gin.Context) {
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
	s.replaceManagedDraft(c, instanceID, participantID, req, true)
}

func progressionRequestForDraft(c *gin.Context, req replaceDraftRequest) (progressionCommandRequest, bool) {
	if strings.TrimSpace(req.IdempotencyKey) == "" {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "idempotency_key is required"})
		return progressionCommandRequest{}, false
	}
	if req.EffectiveAt == nil || req.EffectiveAt.IsZero() {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "effective_at is required"})
		return progressionCommandRequest{}, false
	}
	return progressionCommandRequest{
		IdempotencyKey: strings.TrimSpace(req.IdempotencyKey),
		EffectiveAt:    req.EffectiveAt.UTC(),
	}, true
}

func (s *Server) replaceManagedDraft(c *gin.Context, instanceID, participantID uuid.UUID, req replaceDraftRequest, late bool) {
	command, ok := progressionRequestForDraft(c, req)
	if !ok {
		return
	}
	operation := "draft.replace"
	if late {
		operation = "draft.late"
	}
	payload := struct {
		ParticipantID string                    `json:"participant_id"`
		ContestantIDs []string                  `json:"contestant_ids"`
		Reason        string                    `json:"reason"`
		Command       progressionCommandRequest `json:"command"`
	}{
		ParticipantID: participantID.String(),
		ContestantIDs: req.ContestantIDs,
		Reason:        req.Reason,
		Command:       command,
	}
	s.runManagedCommand(c, instanceID, operation, command, payload, func(ctx context.Context, q *db.Queries) (any, error) {
		draft, err := q.LockInstanceDraftProgress(ctx, toPGUUID(instanceID))
		if err != nil {
			return nil, err
		}
		if late {
			if draft.Status != "closed" {
				return nil, progressionError(http.StatusConflict, "late drafts require a closed draft")
			}
			if draft.ClosedAt.Valid && req.EffectiveAt.Before(draft.ClosedAt.Time) {
				return nil, progressionError(http.StatusConflict, "late draft cannot precede draft closure")
			}
		} else if draft.Status != "open" {
			return nil, progressionError(http.StatusConflict, "draft is not open")
		}
		if draft.OpenedAt.Valid && req.EffectiveAt.Before(draft.OpenedAt.Time) {
			return nil, progressionError(http.StatusConflict, "draft submission cannot precede draft opening")
		}
		latest, latestErr := q.GetLatestInstanceScoreRevision(ctx, toPGUUID(instanceID))
		if latestErr == nil && latest.EffectiveAt.Valid && req.EffectiveAt.Before(latest.EffectiveAt.Time) {
			return nil, progressionError(http.StatusConflict, "draft correction cannot precede the latest publication")
		}
		if latestErr != nil && !errors.Is(latestErr, pgx.ErrNoRows) {
			return nil, latestErr
		}
		if err := replaceDraftInTransaction(ctx, q, instanceID, participantID, req.ContestantIDs); err != nil {
			return nil, err
		}
		response := gin.H{"status": "draft saved"}
		if latestErr == nil {
			snapshot, err := scoreSnapshotFromRevision(latest)
			if err != nil {
				return nil, err
			}
			if _, ok := snapshot.ParticipantNames[participantID.String()]; !ok {
				return nil, progressionError(http.StatusConflict, "participant was not part of the published score; enrollment corrections are not supported")
			}
			if snapshot.Drafts == nil {
				snapshot.Drafts = make(map[string][]scoreSnapshotDraft)
			}
			picks, err := draftSnapshotForParticipant(ctx, q, participantID)
			if err != nil {
				return nil, err
			}
			snapshot.Drafts[participantID.String()] = picks
			reason := strings.TrimSpace(req.Reason)
			if reason == "" {
				reason = "draft correction"
			}
			revision, err := s.publishScoreSnapshot(ctx, q, instanceID, latest.EpisodeNumber, reason, command.EffectiveAt, snapshot)
			if err != nil {
				return nil, err
			}
			response["revision_number"] = revision.RevisionNumber
		}
		return response, nil
	})
}

func draftSnapshotForParticipant(ctx context.Context, q *db.Queries, participantID uuid.UUID) ([]scoreSnapshotDraft, error) {
	picks, err := q.ListDraftPicksForParticipant(ctx, toPGUUID(participantID))
	if err != nil {
		return nil, err
	}
	snapshot := make([]scoreSnapshotDraft, 0, len(picks))
	for _, pick := range picks {
		snapshot = append(snapshot, scoreSnapshotDraft{
			Position:     int(pick.Position),
			ContestantID: uuid.UUID(pick.ContestantID.Bytes).String(),
		})
	}
	return snapshot, nil
}

func replaceDraftInTransaction(ctx context.Context, q *db.Queries, instanceID, participantID uuid.UUID, contestantIDs []string) error {
	participant, err := q.GetParticipant(ctx, toPGUUID(participantID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return progressionError(http.StatusNotFound, "participant not found")
		}
		return err
	}
	if participant.InstanceID != toPGUUID(instanceID) {
		return progressionError(http.StatusBadRequest, "participant does not belong to this instance")
	}
	contestants, err := q.ListContestantsByInstance(ctx, toPGUUID(instanceID))
	if err != nil {
		return err
	}
	if len(contestantIDs) != len(contestants) {
		return progressionError(http.StatusBadRequest, "contestant_ids must include every contestant exactly once")
	}
	allowed := make(map[uuid.UUID]struct{}, len(contestants))
	for _, contestant := range contestants {
		allowed[uuid.UUID(contestant.ID.Bytes)] = struct{}{}
	}
	seen := make(map[uuid.UUID]struct{}, len(contestantIDs))
	parsed := make([]uuid.UUID, 0, len(contestantIDs))
	for _, rawID := range contestantIDs {
		contestantID, err := uuid.Parse(rawID)
		if err != nil {
			return progressionError(http.StatusBadRequest, "invalid contestant id: "+rawID)
		}
		if _, ok := allowed[contestantID]; !ok {
			return progressionError(http.StatusBadRequest, "contestant does not belong to this instance: "+rawID)
		}
		if _, ok := seen[contestantID]; ok {
			return progressionError(http.StatusBadRequest, "duplicate contestant id: "+rawID)
		}
		seen[contestantID] = struct{}{}
		parsed = append(parsed, contestantID)
	}
	if err := q.DeleteDraftPicksForParticipant(ctx, toPGUUID(participantID)); err != nil {
		return err
	}
	for index, contestantID := range parsed {
		position, err := conv.ToInt32(index + 1)
		if err != nil {
			return err
		}
		if _, err := q.CreateDraftPick(ctx, db.CreateDraftPickParams{
			InstanceID:    toPGUUID(instanceID),
			ParticipantID: toPGUUID(participantID),
			ContestantID:  toPGUUID(contestantID),
			Position:      position,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) upsertManagedOutcome(c *gin.Context, instanceID uuid.UUID, position int32, req upsertOutcomeRequest) {
	if strings.TrimSpace(req.IdempotencyKey) == "" {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "idempotency_key is required"})
		return
	}
	if req.EffectiveAt == nil || req.EffectiveAt.IsZero() {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "effective_at is required"})
		return
	}
	command := progressionCommandRequest{IdempotencyKey: strings.TrimSpace(req.IdempotencyKey), EffectiveAt: req.EffectiveAt.UTC()}
	payload := struct {
		Position int32                `json:"position"`
		Request  upsertOutcomeRequest `json:"request"`
	}{Position: position, Request: req}
	s.runManagedCommand(c, instanceID, "outcome.upsert", command, payload, func(ctx context.Context, q *db.Queries) (any, error) {
		progress, err := q.GetLatestManagedEpisodeProgress(ctx, toPGUUID(instanceID))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, progressionError(http.StatusConflict, "an episode must be started before outcomes can be recorded")
			}
			return nil, err
		}
		if progress.Status == "completed" && progress.CompletedAt.Valid && command.EffectiveAt.Before(progress.CompletedAt.Time) {
			return nil, progressionError(http.StatusConflict, "outcome correction cannot precede episode completion")
		}
		if progress.Status == "started" && progress.StartedAt.Valid && command.EffectiveAt.Before(progress.StartedAt.Time) {
			return nil, progressionError(http.StatusConflict, "outcome cannot precede episode start")
		}
		contestantParam := pgtype.UUID{Valid: false}
		if strings.TrimSpace(req.ContestantID) != "" {
			contestantID, err := uuid.Parse(req.ContestantID)
			if err != nil {
				return nil, progressionError(http.StatusBadRequest, "invalid contestant_id")
			}
			exists, err := q.InstanceHasContestant(ctx, db.InstanceHasContestantParams{
				InstanceID:   toPGUUID(instanceID),
				ContestantID: toPGUUID(contestantID),
			})
			if err != nil {
				return nil, err
			}
			if !exists {
				return nil, progressionError(http.StatusBadRequest, "contestant does not belong to this instance")
			}
			contestantParam = toPGUUID(contestantID)
		}
		outcome, err := q.UpsertOutcomePosition(ctx, db.UpsertOutcomePositionParams{
			InstanceID:   toPGUUID(instanceID),
			Position:     position,
			ContestantID: contestantParam,
		})
		if err != nil {
			return nil, err
		}
		response := gin.H{"position": outcome.Position}
		if outcome.ContestantID.Valid {
			response["contestant_id"] = uuid.UUID(outcome.ContestantID.Bytes).String()
		} else {
			response["contestant_id"] = nil
		}
		if progress.Status == "scored" {
			latest, err := q.GetLatestInstanceScoreRevision(ctx, toPGUUID(instanceID))
			if err != nil {
				return nil, err
			}
			if latest.EffectiveAt.Valid && command.EffectiveAt.Before(latest.EffectiveAt.Time) {
				return nil, progressionError(http.StatusConflict, "outcome correction cannot precede the latest publication")
			}
			snapshot, err := scoreSnapshotFromRevision(latest)
			if err != nil {
				return nil, err
			}
			if snapshot.Outcomes == nil {
				snapshot.Outcomes = make(map[string]int)
			}
			if snapshot.OutcomeNames == nil {
				snapshot.OutcomeNames = make(map[string]string)
			}
			for contestantID, existingPosition := range snapshot.Outcomes {
				if existingPosition == int(position) || (outcome.ContestantID.Valid && contestantID == uuid.UUID(outcome.ContestantID.Bytes).String()) {
					delete(snapshot.Outcomes, contestantID)
					delete(snapshot.OutcomeNames, contestantID)
				}
			}
			if outcome.ContestantID.Valid {
				contestantID := uuid.UUID(outcome.ContestantID.Bytes).String()
				contestants, err := q.ListContestantsByInstance(ctx, toPGUUID(instanceID))
				if err != nil {
					return nil, err
				}
				for _, contestant := range contestants {
					if uuid.UUID(contestant.ID.Bytes).String() == contestantID {
						snapshot.OutcomeNames[contestantID] = contestant.Name
						break
					}
				}
				snapshot.Outcomes[contestantID] = int(position)
			}
			reason := strings.TrimSpace(req.Reason)
			if reason == "" {
				reason = "outcome correction"
			}
			revision, err := s.publishScoreSnapshot(ctx, q, instanceID, progress.EpisodeNumber, reason, command.EffectiveAt, snapshot)
			if err != nil {
				return nil, err
			}
			response["revision_number"] = revision.RevisionNumber
		}
		return gin.H{"outcome": response}, nil
	})
}
