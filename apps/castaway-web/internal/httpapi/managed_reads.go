package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"

	"github.com/jackc/pgx/v5"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (s *Server) managedOutcomes(c *gin.Context, instanceID uuid.UUID) {
	revision, err := s.queries.GetLatestInstanceScoreRevision(c.Request.Context(), toPGUUID(instanceID))
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusOK, gin.H{"outcomes": []gin.H{}})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	var snapshot scoreInputSnapshot
	if err := json.Unmarshal(revision.InputSnapshot, &snapshot); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	type publishedOutcome struct {
		position     int
		contestantID string
		name         string
	}
	outcomes := make([]publishedOutcome, 0, len(snapshot.Outcomes))
	for contestantID, position := range snapshot.Outcomes {
		outcomes = append(outcomes, publishedOutcome{
			position:     position,
			contestantID: contestantID,
			name:         snapshot.OutcomeNames[contestantID],
		})
	}
	sort.Slice(outcomes, func(i, j int) bool {
		return outcomes[i].position < outcomes[j].position
	})
	response := make([]gin.H, 0, len(outcomes))
	for _, outcome := range outcomes {
		response = append(response, gin.H{
			"position":        outcome.position,
			"contestant_id":   outcome.contestantID,
			"contestant_name": outcome.name,
		})
	}
	c.JSON(http.StatusOK, gin.H{"outcomes": response})
}
