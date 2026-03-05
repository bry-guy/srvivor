package httpapi

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type importSubmission struct {
	ParticipantName string   `json:"participant_name"`
	Rankings        []string `json:"rankings"`
}

type importInstanceRequest struct {
	Season      int32              `json:"season"`
	Name        string             `json:"name"`
	Submissions []importSubmission `json:"submissions"`
}

func (s *Server) importInstance(c *gin.Context) {
	payload, err := s.parseImportPayload(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	if payload.Name == "" {
		payload.Name = fmt.Sprintf("Season %d", payload.Season)
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
	if err := qtx.DeleteInstanceByNameSeason(c.Request.Context(), db.DeleteInstanceByNameSeasonParams{
		Name:   payload.Name,
		Season: payload.Season,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	instance, err := qtx.CreateInstance(c.Request.Context(), db.CreateInstanceParams{
		Name:   payload.Name,
		Season: payload.Season,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	globals, err := qtx.ListContestantsGlobal(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	globalByExact := map[string]string{}
	globalByFirst := map[string]string{}
	for _, contestant := range globals {
		name := strings.TrimSpace(contestant.Name)
		if name == "" {
			continue
		}
		globalByExact[strings.ToLower(name)] = name
		first := strings.ToLower(firstToken(name))
		if _, exists := globalByFirst[first]; !exists {
			globalByFirst[first] = name
		}
	}

	contestantIDByName := map[string]uuid.UUID{}
	for _, submission := range payload.Submissions {
		for _, raw := range submission.Rankings {
			resolved := normalizeContestantName(raw, globalByExact, globalByFirst)
			if resolved == "" {
				continue
			}
			if _, ok := contestantIDByName[resolved]; ok {
				continue
			}
			contestant, createErr := qtx.CreateContestant(c.Request.Context(), db.CreateContestantParams{
				InstanceID: instance.ID,
				Name:       resolved,
			})
			if createErr != nil {
				c.JSON(statusFromPg(createErr), errorResponse{Error: createErr.Error()})
				return
			}
			contestantIDByName[resolved] = uuid.UUID(contestant.ID.Bytes)
		}
	}

	for _, submission := range payload.Submissions {
		participantName := normalizeParticipantName(submission.ParticipantName)
		if participantName == "" {
			continue
		}
		participant, createErr := qtx.CreateParticipant(c.Request.Context(), db.CreateParticipantParams{
			InstanceID: instance.ID,
			Name:       participantName,
		})
		if createErr != nil {
			c.JSON(statusFromPg(createErr), errorResponse{Error: createErr.Error()})
			return
		}

		for index, rawContestant := range submission.Rankings {
			resolvedContestant := normalizeContestantName(rawContestant, globalByExact, globalByFirst)
			contestantID, ok := contestantIDByName[resolvedContestant]
			if !ok {
				continue
			}
			position, convErr := toInt32(index + 1)
			if convErr != nil {
				c.JSON(http.StatusBadRequest, errorResponse{Error: convErr.Error()})
				return
			}
			_, createPickErr := qtx.CreateDraftPick(c.Request.Context(), db.CreateDraftPickParams{
				InstanceID:    instance.ID,
				ParticipantID: participant.ID,
				ContestantID:  toPGUUID(contestantID),
				Position:      position,
			})
			if createPickErr != nil {
				c.JSON(statusFromPg(createPickErr), errorResponse{Error: createPickErr.Error()})
				return
			}
		}
	}

	if err := tx.Commit(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"instance": toInstanceResponse(instance.ID, instance.Name, instance.Season, instance.CreatedAt)})
}

func (s *Server) parseImportPayload(c *gin.Context) (importInstanceRequest, error) {
	contentType := c.ContentType()
	switch contentType {
	case "application/json":
		var payload importInstanceRequest
		if err := c.ShouldBindJSON(&payload); err != nil {
			return importInstanceRequest{}, err
		}
		if payload.Season <= 0 {
			return importInstanceRequest{}, fmt.Errorf("season must be > 0")
		}
		if len(payload.Submissions) == 0 {
			return importInstanceRequest{}, fmt.Errorf("submissions cannot be empty")
		}
		return payload, nil
	case "text/csv":
		seasonRaw := c.Query("season")
		if seasonRaw == "" {
			return importInstanceRequest{}, fmt.Errorf("season query parameter is required for text/csv")
		}
		season, err := strconv.Atoi(seasonRaw)
		if err != nil || season <= 0 {
			return importInstanceRequest{}, fmt.Errorf("invalid season query parameter")
		}
		submissions, err := parseCSVSubmissions(c.Request.Body)
		if err != nil {
			return importInstanceRequest{}, err
		}
		name := c.Query("name")
		if name == "" {
			name = fmt.Sprintf("Season %d", season)
		}
		seasonInt32, convErr := toInt32(season)
		if convErr != nil {
			return importInstanceRequest{}, convErr
		}
		return importInstanceRequest{
			Season:      seasonInt32,
			Name:        name,
			Submissions: submissions,
		}, nil
	default:
		return importInstanceRequest{}, fmt.Errorf("unsupported content type: %s", contentType)
	}
}

func parseCSVSubmissions(reader io.Reader) ([]importSubmission, error) {
	r := csv.NewReader(reader)
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse csv: %w", err)
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("csv requires header and at least one row")
	}

	headers := map[string]int{}
	for index, header := range rows[0] {
		headers[strings.TrimSpace(header)] = index
	}

	participantIdx, ok := headers["Who are you?"]
	if !ok {
		return nil, fmt.Errorf("csv missing 'Who are you?' column")
	}
	rankingsIdx, ok := headers["Rank your survivors, from first to last!"]
	if !ok {
		return nil, fmt.Errorf("csv missing rankings column")
	}

	submissions := make([]importSubmission, 0, len(rows)-1)
	for _, row := range rows[1:] {
		if participantIdx >= len(row) || rankingsIdx >= len(row) {
			continue
		}
		participant := strings.TrimSpace(row[participantIdx])
		rankingText := strings.TrimSpace(row[rankingsIdx])
		if participant == "" || rankingText == "" {
			continue
		}
		rawParts := strings.Split(rankingText, ",")
		rankings := make([]string, 0, len(rawParts))
		for _, part := range rawParts {
			trimmed := strings.TrimSpace(part)
			if trimmed == "" {
				continue
			}
			rankings = append(rankings, trimmed)
		}
		submissions = append(submissions, importSubmission{
			ParticipantName: participant,
			Rankings:        rankings,
		})
	}
	if len(submissions) == 0 {
		return nil, fmt.Errorf("csv contains no valid submissions")
	}
	return submissions, nil
}

func normalizeContestantName(raw string, globalByExact map[string]string, globalByFirst map[string]string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	aliases := map[string]string{
		"rizgod": "Rizo",
	}
	if alias, ok := aliases[strings.ToLower(trimmed)]; ok {
		trimmed = alias
	}

	if exact, ok := globalByExact[strings.ToLower(trimmed)]; ok {
		return exact
	}

	first := firstToken(trimmed)
	if first == "" {
		return ""
	}
	if firstMatch, ok := globalByFirst[strings.ToLower(first)]; ok {
		return firstMatch
	}

	return first
}

func normalizeParticipantName(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	aliases := map[string]string{
		"ken-dog": "Kenny",
		"brain":   "Bryan",
		"mary":    "Marv",
	}
	if alias, ok := aliases[strings.ToLower(trimmed)]; ok {
		trimmed = alias
	}

	if idx := strings.Index(trimmed, "("); idx > 0 {
		trimmed = strings.TrimSpace(trimmed[:idx])
	}

	return firstToken(trimmed)
}

func firstToken(v string) string {
	parts := strings.Fields(strings.TrimSpace(v))
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}
