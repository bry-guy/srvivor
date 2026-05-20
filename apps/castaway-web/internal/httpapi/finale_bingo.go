package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	activityTypeFinaleBingo            = "finale_bingo"
	occurrenceTypeFinaleBingoScores    = "finale_bingo_scores"
	occurrenceTypeFinaleBingoLoanShark = "finale_bingo_loan_shark"
)

type finaleBingoScoreInput struct {
	ParticipantID string `json:"participant_id" binding:"required"`
	BoxPoints     int32  `json:"box_points"`
	BingoCount    int32  `json:"bingo_count"`
	Notes         string `json:"notes"`
}

type finaleBingoLoanSharkInput struct {
	SharkParticipantID  string `json:"shark_participant_id" binding:"required"`
	TargetParticipantID string `json:"target_participant_id" binding:"required"`
}

type recordFinaleBingoLoanSharksRequest struct {
	Name        string                      `json:"name"`
	Assignments []finaleBingoLoanSharkInput `json:"assignments" binding:"required"`
	EffectiveAt *time.Time                  `json:"effective_at"`
}

type recordFinaleBingoScoresRequest struct {
	Name        string                      `json:"name"`
	Scores      []finaleBingoScoreInput     `json:"scores" binding:"required"`
	LoanSharks  []finaleBingoLoanSharkInput `json:"loan_sharks"`
	EffectiveAt *time.Time                  `json:"effective_at"`
}

type finaleBingoLoanSharkMetadata struct {
	Assignments []finaleBingoLoanSharkInput `json:"assignments"`
	RecordedBy  string                      `json:"recorded_by,omitempty"`
	RecordedAt  string                      `json:"recorded_at,omitempty"`
}

type finaleBingoScoresMetadata struct {
	Scores     []finaleBingoScoreInput     `json:"scores"`
	LoanSharks []finaleBingoLoanSharkInput `json:"loan_sharks,omitempty"`
	RecordedBy string                      `json:"recorded_by,omitempty"`
	RecordedAt string                      `json:"recorded_at,omitempty"`
}

type finaleBingoCalculatedScore struct {
	ParticipantID     string `json:"participant_id"`
	ParticipantName   string `json:"participant_name"`
	BoxPoints         int32  `json:"box_points"`
	BingoCount        int32  `json:"bingo_count"`
	BingoPoints       int32  `json:"bingo_points"`
	BasePoints        int32  `json:"base_points"`
	LoanSharkTargetID string `json:"loan_shark_target_id,omitempty"`
	LoanSharkTarget   string `json:"loan_shark_target,omitempty"`
	LoanSharkBingos   int32  `json:"loan_shark_bingos,omitempty"`
	LoanSharkPoints   int32  `json:"loan_shark_points,omitempty"`
	TotalPoints       int32  `json:"total_points"`
	Notes             string `json:"notes,omitempty"`
}

func (s *Server) recordFinaleBingoLoanSharks(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok {
		return
	}
	if !s.requireInstanceAdminRequest(c, instanceID) {
		return
	}
	var req recordFinaleBingoLoanSharksRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	now := time.Now().UTC()
	effectiveAt := now
	if req.EffectiveAt != nil {
		effectiveAt = req.EffectiveAt.UTC()
	}
	participantsByID, _, ok := s.finaleBingoParticipants(c, instanceID)
	if !ok {
		return
	}
	assignments, ok := validateFinaleBingoLoanSharks(c, req.Assignments, participantsByID)
	if !ok {
		return
	}
	metadata, err := json.Marshal(finaleBingoLoanSharkMetadata{Assignments: assignments, RecordedBy: discordUserIDFromRequest(c.Request), RecordedAt: now.Format(time.RFC3339)})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	tx, err := s.pool.Begin(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	defer rollbackTx(c, tx)
	qtx := s.queries.WithTx(tx)
	activity, err := s.ensureSystemActivity(c.Request.Context(), qtx, toPGUUID(instanceID), activityTypeFinaleBingo, "Finale Bingo", effectiveAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "Finale Bingo Loan Sharks"
	}
	occurrence, err := qtx.CreateActivityOccurrence(c.Request.Context(), db.CreateActivityOccurrenceParams{
		ActivityID:     activity.ID,
		OccurrenceType: occurrenceTypeFinaleBingoLoanShark,
		Name:           name,
		EffectiveAt:    optionalTime(effectiveAt),
		StartsAt:       optionalTime(effectiveAt),
		EndsAt:         optionalTime(effectiveAt.Add(time.Second)),
		Status:         "resolved",
		Metadata:       metadata,
	})
	if err != nil {
		c.JSON(statusFromPg(err), errorResponse{Error: err.Error()})
		return
	}
	if err := tx.Commit(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"occurrence_id": pgUUIDString(occurrence.ID), "assignments": assignments})
}

func (s *Server) previewFinaleBingoScores(c *gin.Context) {
	s.handleFinaleBingoScores(c, false)
}

func (s *Server) recordFinaleBingoScores(c *gin.Context) {
	s.handleFinaleBingoScores(c, true)
}

func (s *Server) handleFinaleBingoScores(c *gin.Context, write bool) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok {
		return
	}
	if !s.requireInstanceAdminRequest(c, instanceID) {
		return
	}
	var req recordFinaleBingoScoresRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	now := time.Now().UTC()
	effectiveAt := now
	if req.EffectiveAt != nil {
		effectiveAt = req.EffectiveAt.UTC()
	}
	participantsByID, participantOrder, ok := s.finaleBingoParticipants(c, instanceID)
	if !ok {
		return
	}
	loanSharks := req.LoanSharks
	if len(loanSharks) == 0 {
		loaded, err := s.latestFinaleBingoLoanSharks(c.Request.Context(), toPGUUID(instanceID), participantsByID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
			return
		}
		loanSharks = loaded
	}
	loanSharks, ok = validateFinaleBingoLoanSharks(c, loanSharks, participantsByID)
	if !ok {
		return
	}
	calculated, ok := calculateFinaleBingoScores(c, req.Scores, loanSharks, participantsByID, participantOrder)
	if !ok {
		return
	}
	if !write {
		c.JSON(http.StatusOK, gin.H{"scores": calculated, "loan_sharks": loanSharks})
		return
	}
	metadata, err := json.Marshal(finaleBingoScoresMetadata{Scores: req.Scores, LoanSharks: loanSharks, RecordedBy: discordUserIDFromRequest(c.Request), RecordedAt: now.Format(time.RFC3339)})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	tx, err := s.pool.Begin(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	defer rollbackTx(c, tx)
	qtx := s.queries.WithTx(tx)
	activity, err := s.ensureSystemActivity(c.Request.Context(), qtx, toPGUUID(instanceID), activityTypeFinaleBingo, "Finale Bingo", effectiveAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "Finale Bingo Scores"
	}
	occurrence, err := qtx.CreateActivityOccurrence(c.Request.Context(), db.CreateActivityOccurrenceParams{
		ActivityID:     activity.ID,
		OccurrenceType: occurrenceTypeFinaleBingoScores,
		Name:           name,
		EffectiveAt:    optionalTime(effectiveAt),
		StartsAt:       optionalTime(effectiveAt),
		EndsAt:         optionalTime(effectiveAt.Add(time.Second)),
		Status:         "resolved",
		Metadata:       metadata,
	})
	if err != nil {
		c.JSON(statusFromPg(err), errorResponse{Error: err.Error()})
		return
	}
	created := make([]db.CreateBonusPointLedgerEntryRow, 0, len(calculated)*2)
	for _, score := range calculated {
		participantID := toPGUUID(uuid.MustParse(score.ParticipantID))
		if score.BasePoints > 0 {
			entry, err := qtx.CreateBonusPointLedgerEntry(c.Request.Context(), db.CreateBonusPointLedgerEntryParams{
				InstanceID:           toPGUUID(instanceID),
				ParticipantID:        participantID,
				ActivityOccurrenceID: occurrence.ID,
				EntryKind:            "award",
				Points:               score.BasePoints,
				Visibility:           "public",
				Reason:               fmt.Sprintf("Finale Bingo: %d boxes + %d bingos", score.BoxPoints, score.BingoCount),
				EffectiveAt:          optionalTime(effectiveAt),
				AwardKey:             optionalText(ptrString("finale_bingo:base")),
				Metadata:             []byte("{}"),
			})
			if err != nil {
				c.JSON(statusFromPg(err), errorResponse{Error: err.Error()})
				return
			}
			created = append(created, entry)
		}
		if score.LoanSharkPoints > 0 {
			entry, err := qtx.CreateBonusPointLedgerEntry(c.Request.Context(), db.CreateBonusPointLedgerEntryParams{
				InstanceID:           toPGUUID(instanceID),
				ParticipantID:        participantID,
				ActivityOccurrenceID: occurrence.ID,
				EntryKind:            "award",
				Points:               score.LoanSharkPoints,
				Visibility:           "public",
				Reason:               fmt.Sprintf("Finale Bingo Loan Shark: copied %d bingo(s) from %s", score.LoanSharkBingos, score.LoanSharkTarget),
				EffectiveAt:          optionalTime(effectiveAt),
				AwardKey:             optionalText(ptrString("finale_bingo:loan_shark:" + score.LoanSharkTargetID)),
				Metadata:             []byte("{}"),
			})
			if err != nil {
				c.JSON(statusFromPg(err), errorResponse{Error: err.Error()})
				return
			}
			created = append(created, entry)
		}
	}
	if err := tx.Commit(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	entries := make([]gin.H, 0, len(created))
	for _, entry := range created {
		entries = append(entries, gin.H{"id": pgUUIDString(entry.ID), "participant_id": pgUUIDString(entry.ParticipantID), "points": entry.Points, "reason": entry.Reason})
	}
	c.JSON(http.StatusOK, gin.H{"occurrence_id": pgUUIDString(occurrence.ID), "scores": calculated, "loan_sharks": loanSharks, "created_entries": entries})
}

func (s *Server) finaleBingoParticipants(c *gin.Context, instanceID uuid.UUID) (map[string]db.ListParticipantsByInstanceRow, []string, bool) {
	participants, err := s.queries.ListParticipantsByInstance(c.Request.Context(), toPGUUID(instanceID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return nil, nil, false
	}
	byID := make(map[string]db.ListParticipantsByInstanceRow, len(participants))
	order := make([]string, 0, len(participants))
	for _, participant := range participants {
		id := pgUUIDString(participant.ID)
		byID[id] = participant
		order = append(order, id)
	}
	sort.Slice(order, func(i, j int) bool { return byID[order[i]].Name < byID[order[j]].Name })
	return byID, order, true
}

func validateFinaleBingoLoanSharks(c *gin.Context, raw []finaleBingoLoanSharkInput, participants map[string]db.ListParticipantsByInstanceRow) ([]finaleBingoLoanSharkInput, bool) {
	assignments := make([]finaleBingoLoanSharkInput, 0, len(raw))
	seen := map[string]bool{}
	for _, assignment := range raw {
		sharkID, err := uuid.Parse(strings.TrimSpace(assignment.SharkParticipantID))
		if err != nil {
			c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid shark_participant_id"})
			return nil, false
		}
		targetID, err := uuid.Parse(strings.TrimSpace(assignment.TargetParticipantID))
		if err != nil {
			c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid target_participant_id"})
			return nil, false
		}
		shark := sharkID.String()
		target := targetID.String()
		if _, ok := participants[shark]; !ok {
			c.JSON(http.StatusBadRequest, errorResponse{Error: "shark_participant_id does not belong to this instance"})
			return nil, false
		}
		if _, ok := participants[target]; !ok {
			c.JSON(http.StatusBadRequest, errorResponse{Error: "target_participant_id does not belong to this instance"})
			return nil, false
		}
		if shark == target {
			c.JSON(http.StatusBadRequest, errorResponse{Error: "loan shark target must be a different participant"})
			return nil, false
		}
		if seen[shark] {
			c.JSON(http.StatusBadRequest, errorResponse{Error: "duplicate loan shark assignment for participant " + shark})
			return nil, false
		}
		seen[shark] = true
		assignments = append(assignments, finaleBingoLoanSharkInput{SharkParticipantID: shark, TargetParticipantID: target})
	}
	return assignments, true
}

func calculateFinaleBingoScores(c *gin.Context, rawScores []finaleBingoScoreInput, loanSharks []finaleBingoLoanSharkInput, participants map[string]db.ListParticipantsByInstanceRow, participantOrder []string) ([]finaleBingoCalculatedScore, bool) {
	scoresByParticipant := map[string]finaleBingoCalculatedScore{}
	for _, input := range rawScores {
		participantUUID, err := uuid.Parse(strings.TrimSpace(input.ParticipantID))
		if err != nil {
			c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid participant_id"})
			return nil, false
		}
		participantID := participantUUID.String()
		participant, ok := participants[participantID]
		if !ok {
			c.JSON(http.StatusBadRequest, errorResponse{Error: "participant_id does not belong to this instance"})
			return nil, false
		}
		if _, exists := scoresByParticipant[participantID]; exists {
			c.JSON(http.StatusBadRequest, errorResponse{Error: "duplicate score for participant " + participantID})
			return nil, false
		}
		if input.BoxPoints < 0 || input.BoxPoints > 17 {
			c.JSON(http.StatusBadRequest, errorResponse{Error: "box_points must be between 0 and 17"})
			return nil, false
		}
		if input.BingoCount < 0 || input.BingoCount > 10 {
			c.JSON(http.StatusBadRequest, errorResponse{Error: "bingo_count must be between 0 and 10"})
			return nil, false
		}
		bingoPoints := input.BingoCount * 3
		basePoints := input.BoxPoints + bingoPoints
		scoresByParticipant[participantID] = finaleBingoCalculatedScore{ParticipantID: participantID, ParticipantName: participant.Name, BoxPoints: input.BoxPoints, BingoCount: input.BingoCount, BingoPoints: bingoPoints, BasePoints: basePoints, TotalPoints: basePoints, Notes: input.Notes}
	}
	for _, assignment := range loanSharks {
		shark := assignment.SharkParticipantID
		target := assignment.TargetParticipantID
		sharkScore, ok := scoresByParticipant[shark]
		if !ok {
			c.JSON(http.StatusBadRequest, errorResponse{Error: "loan shark participant is missing a score: " + shark})
			return nil, false
		}
		targetScore, ok := scoresByParticipant[target]
		if !ok {
			c.JSON(http.StatusBadRequest, errorResponse{Error: "loan shark target is missing a score: " + target})
			return nil, false
		}
		sharkScore.LoanSharkTargetID = target
		sharkScore.LoanSharkTarget = targetScore.ParticipantName
		sharkScore.LoanSharkBingos = targetScore.BingoCount
		sharkScore.LoanSharkPoints = targetScore.BingoPoints
		sharkScore.TotalPoints += targetScore.BingoPoints
		scoresByParticipant[shark] = sharkScore
	}
	calculated := make([]finaleBingoCalculatedScore, 0, len(scoresByParticipant))
	for _, participantID := range participantOrder {
		if score, ok := scoresByParticipant[participantID]; ok {
			calculated = append(calculated, score)
		}
	}
	return calculated, true
}

func (s *Server) latestFinaleBingoLoanSharks(ctx context.Context, instanceID pgtype.UUID, participants map[string]db.ListParticipantsByInstanceRow) ([]finaleBingoLoanSharkInput, error) {
	activities, err := s.queries.ListInstanceActivitiesByType(ctx, db.ListInstanceActivitiesByTypeParams{InstanceID: instanceID, ActivityType: activityTypeFinaleBingo})
	if err != nil {
		return nil, err
	}
	for activityIndex := len(activities) - 1; activityIndex >= 0; activityIndex-- {
		activity := activities[activityIndex]
		occurrences, err := s.queries.ListActivityOccurrencesByActivity(ctx, activity.ID)
		if err != nil {
			return nil, err
		}
		for i := len(occurrences) - 1; i >= 0; i-- {
			occurrence := occurrences[i]
			if occurrence.OccurrenceType != occurrenceTypeFinaleBingoLoanShark || occurrence.Status != "resolved" {
				continue
			}
			var metadata finaleBingoLoanSharkMetadata
			if err := json.Unmarshal(occurrence.Metadata, &metadata); err != nil {
				return nil, err
			}
			assignments, ok := sanitizeFinaleBingoLoanSharks(metadata.Assignments, participants)
			if !ok {
				return nil, fmt.Errorf("stored finale bingo loan shark assignment is invalid")
			}
			return assignments, nil
		}
	}
	return nil, nil
}

func sanitizeFinaleBingoLoanSharks(raw []finaleBingoLoanSharkInput, participants map[string]db.ListParticipantsByInstanceRow) ([]finaleBingoLoanSharkInput, bool) {
	assignments := make([]finaleBingoLoanSharkInput, 0, len(raw))
	seen := map[string]bool{}
	for _, assignment := range raw {
		sharkID, err := uuid.Parse(strings.TrimSpace(assignment.SharkParticipantID))
		if err != nil {
			return nil, false
		}
		targetID, err := uuid.Parse(strings.TrimSpace(assignment.TargetParticipantID))
		if err != nil {
			return nil, false
		}
		shark := sharkID.String()
		target := targetID.String()
		if shark == target || seen[shark] {
			return nil, false
		}
		if _, ok := participants[shark]; !ok {
			return nil, false
		}
		if _, ok := participants[target]; !ok {
			return nil, false
		}
		seen[shark] = true
		assignments = append(assignments, finaleBingoLoanSharkInput{SharkParticipantID: shark, TargetParticipantID: target})
	}
	return assignments, true
}
