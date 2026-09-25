package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/gameplay"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type tribeAssignment struct {
	Name           string   `json:"name"`
	ParticipantIDs []string `json:"participant_ids"`
}

type setTribesRequest struct {
	EffectiveAt time.Time         `json:"effective_at" binding:"required"`
	Tribes      []tribeAssignment `json:"tribes" binding:"required"`
}

type recordTribeChallengeRequest struct {
	Key           string    `json:"key" binding:"required"`
	Kind          string    `json:"kind" binding:"required"`
	WinningTribes []string  `json:"winning_tribes" binding:"required"`
	EffectiveAt   time.Time `json:"effective_at" binding:"required"`
}

// lockLegacyInstanceForAdmin opens a transaction holding the instance lock after checking service auth and
// instance-admin authorization. Callers must roll back the returned transaction.
func (s *Server) lockLegacyInstanceForAdmin(c *gin.Context, instanceID uuid.UUID) (pgx.Tx, *db.Queries, bool) {
	if !s.requireWordleServicePrincipal(c) || !s.requireInstanceAdminRequest(c, instanceID) {
		return nil, nil, false
	}
	tx, err := s.pool.Begin(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return nil, nil, false
	}
	qtx := s.queries.WithTx(tx)
	locked, err := qtx.LockInstanceForProgression(c.Request.Context(), toPGUUID(instanceID))
	if err == nil && locked.ProgressionMode != "legacy" {
		err = progressionError(http.StatusConflict, "only supported for legacy instances")
	}
	if err == nil {
		err = requireWordleInstanceAdmin(c.Request.Context(), qtx, locked.ID, c.Request)
	}
	if err != nil {
		rollbackTx(c, tx)
		writeTribeError(c, err)
		return nil, nil, false
	}
	return tx, qtx, true
}

func (s *Server) getTribes(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok {
		return
	}
	at := wordleClockNow(s)
	if raw := strings.TrimSpace(c.Query("at")); raw != "" {
		parsed, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, errorResponse{Error: "at must be an RFC3339 timestamp"})
			return
		}
		at = parsed.UTC()
	}
	rows, err := s.queries.ListInstanceTribeMembershipsAt(c.Request.Context(), db.ListInstanceTribeMembershipsAtParams{InstanceID: toPGUUID(instanceID), At: wordleTimestamp(at)})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, tribesJSON(at, rows))
}

// setTribes replaces the whole tribe arrangement from effective_at onward, which covers starting tribes,
// swaps, merges, and splits. Participants left out are no longer on a tribe.
func (s *Server) setTribes(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok {
		return
	}
	var req setTribesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	at := req.EffectiveAt.UTC().Truncate(time.Microsecond)
	want := map[string][]string{}
	names := map[string]string{}
	seen := map[string]bool{}
	for _, tribe := range req.Tribes {
		name := strings.TrimSpace(tribe.Name)
		key := strings.ToLower(name)
		if name == "" || names[key] != "" {
			c.JSON(http.StatusBadRequest, errorResponse{Error: "tribe names must be non-empty and unique"})
			return
		}
		if len(tribe.ParticipantIDs) == 0 {
			c.JSON(http.StatusBadRequest, errorResponse{Error: "tribe " + name + " has no participants"})
			return
		}
		names[key] = name
		for _, raw := range tribe.ParticipantIDs {
			id, err := uuid.Parse(strings.TrimSpace(raw))
			if err != nil || seen[id.String()] {
				c.JSON(http.StatusBadRequest, errorResponse{Error: "participant ids must be valid and appear once"})
				return
			}
			seen[id.String()] = true
			want[key] = append(want[key], id.String())
		}
		slices.Sort(want[key])
	}

	tx, qtx, ok := s.lockLegacyInstanceForAdmin(c, instanceID)
	if !ok {
		return
	}
	defer rollbackTx(c, tx)
	ctx := c.Request.Context()

	current, err := qtx.ListInstanceTribeMembershipsAt(ctx, db.ListInstanceTribeMembershipsAtParams{InstanceID: toPGUUID(instanceID), At: wordleTimestamp(at)})
	if err != nil {
		writeTribeError(c, err)
		return
	}
	have := map[string][]string{}
	for _, row := range current {
		key := strings.ToLower(row.ParticipantGroupName)
		have[key] = append(have[key], pgUUIDString(row.ParticipantID))
	}
	unchanged := len(have) == len(want)
	for key, ids := range want {
		slices.Sort(have[key])
		unchanged = unchanged && slices.Equal(have[key], ids)
	}
	if unchanged {
		if err := tx.Commit(ctx); err != nil {
			writeTribeError(c, err)
			return
		}
		c.JSON(http.StatusOK, tribesJSON(at, current))
		return
	}

	later, err := qtx.CountInstanceTribeMembershipsStartingAtOrAfter(ctx, db.CountInstanceTribeMembershipsStartingAtOrAfterParams{InstanceID: toPGUUID(instanceID), At: wordleTimestamp(at)})
	if err != nil {
		writeTribeError(c, err)
		return
	}
	if later > 0 {
		c.JSON(http.StatusConflict, errorResponse{Error: "tribes were already changed at or after effective_at; changes must be chronological"})
		return
	}
	if err := qtx.EndInstanceTribeMembershipsAt(ctx, db.EndInstanceTribeMembershipsAtParams{InstanceID: toPGUUID(instanceID), At: wordleTimestamp(at)}); err != nil {
		writeTribeError(c, err)
		return
	}
	groups, err := qtx.ListParticipantGroupsByInstance(ctx, toPGUUID(instanceID))
	if err != nil {
		writeTribeError(c, err)
		return
	}
	groupIDs := map[string]pgtype.UUID{}
	for _, group := range groups {
		if group.Kind == "tribe" {
			groupIDs[strings.ToLower(group.Name)] = group.ID
		}
	}
	for key, ids := range want {
		groupID, exists := groupIDs[key]
		if !exists {
			group, err := qtx.CreateParticipantGroup(ctx, db.CreateParticipantGroupParams{InstanceID: toPGUUID(instanceID), Name: names[key], Kind: "tribe", Metadata: []byte(`{}`)})
			if err != nil {
				writeTribeError(c, err)
				return
			}
			groupID = group.ID
		}
		for _, id := range ids {
			participantID := toPGUUID(uuid.MustParse(id))
			if _, err := qtx.CreateParticipantGroupMembershipPeriod(ctx, db.CreateParticipantGroupMembershipPeriodParams{
				ParticipantGroupID: groupID,
				ParticipantID:      participantID,
				Role:               "member",
				StartsAt:           wordleTimestamp(at),
				Metadata:           []byte(`{}`),
			}); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					err = progressionError(http.StatusBadRequest, "participant "+id+" does not belong to this instance")
				}
				writeTribeError(c, err)
				return
			}
		}
	}
	updated, err := qtx.ListInstanceTribeMembershipsAt(ctx, db.ListInstanceTribeMembershipsAtParams{InstanceID: toPGUUID(instanceID), At: wordleTimestamp(at)})
	if err != nil {
		writeTribeError(c, err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		writeTribeError(c, err)
		return
	}
	c.JSON(http.StatusOK, tribesJSON(at, updated))
}

// recordTribeChallenge awards every member of each winning tribe (+2 immunity, +1 reward). The key makes
// retries safe: repeating a request returns the original awards, and a different payload conflicts.
func (s *Server) recordTribeChallenge(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok {
		return
	}
	var req recordTribeChallengeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	key := strings.TrimSpace(req.Key)
	kind := strings.ToLower(strings.TrimSpace(req.Kind))
	if _, ok := gameplay.TribeChallengePoints[kind]; !ok {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "kind must be immunity or reward"})
		return
	}
	if key == "" || len(key) > 64 || len(req.WinningTribes) == 0 {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "key (1-64 characters) and winning_tribes are required"})
		return
	}
	at := req.EffectiveAt.UTC().Truncate(time.Microsecond)

	tx, qtx, ok := s.lockLegacyInstanceForAdmin(c, instanceID)
	if !ok {
		return
	}
	defer rollbackTx(c, tx)
	ctx := c.Request.Context()

	current, err := qtx.ListInstanceTribeMembershipsAt(ctx, db.ListInstanceTribeMembershipsAtParams{InstanceID: toPGUUID(instanceID), At: wordleTimestamp(at)})
	if err != nil {
		writeTribeError(c, err)
		return
	}
	tribeIDs := map[string]string{}
	for _, row := range current {
		tribeIDs[strings.ToLower(row.ParticipantGroupName)] = pgUUIDString(row.ParticipantGroupID)
	}
	winningIDs := make([]string, 0, len(req.WinningTribes))
	for _, tribe := range req.WinningTribes {
		id, ok := tribeIDs[strings.ToLower(strings.TrimSpace(tribe))]
		if !ok || slices.Contains(winningIDs, id) {
			c.JSON(http.StatusBadRequest, errorResponse{Error: "tribe " + tribe + " has no members at effective_at, or is listed twice"})
			return
		}
		winningIDs = append(winningIDs, id)
	}
	sort.Strings(winningIDs)
	metadata, err := json.Marshal(map[string][]string{"winning_tribe_ids": winningIDs})
	if err != nil {
		writeTribeError(c, err)
		return
	}

	activities, err := qtx.ListInstanceActivitiesByType(ctx, db.ListInstanceActivitiesByTypeParams{InstanceID: toPGUUID(instanceID), ActivityType: gameplay.TribeChallengeActivityType})
	if err != nil {
		writeTribeError(c, err)
		return
	}
	var activityID pgtype.UUID
	if len(activities) > 0 {
		activityID = activities[0].ID
	} else {
		activity, err := qtx.CreateInstanceActivity(ctx, db.CreateInstanceActivityParams{InstanceID: toPGUUID(instanceID), ActivityType: gameplay.TribeChallengeActivityType, Name: "Tribe challenges", Status: "active", StartsAt: wordleTimestamp(at), Metadata: []byte(`{}`)})
		if err != nil {
			writeTribeError(c, err)
			return
		}
		activityID = activity.ID
	}

	existing, err := qtx.GetActivityOccurrenceBySourceRef(ctx, db.GetActivityOccurrenceBySourceRefParams{ActivityID: activityID, SourceRef: pgtype.Text{String: key, Valid: true}})
	if err == nil {
		var stored map[string][]string
		if err := json.Unmarshal(existing.Metadata, &stored); err != nil {
			writeTribeError(c, err)
			return
		}
		if existing.OccurrenceType != kind || !existing.EffectiveAt.Time.Equal(at) || !slices.Equal(stored["winning_tribe_ids"], winningIDs) {
			c.JSON(http.StatusConflict, errorResponse{Error: "key already recorded with a different result"})
			return
		}
		entries, err := qtx.ListVisibleBonusPointLedgerEntriesByOccurrence(ctx, existing.ID)
		if err != nil {
			writeTribeError(c, err)
			return
		}
		if err := tx.Commit(ctx); err != nil {
			writeTribeError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"occurrence_id": pgUUIDString(existing.ID), "awarded_count": len(entries)})
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		writeTribeError(c, err)
		return
	}

	occurrence, err := qtx.CreateActivityOccurrence(ctx, db.CreateActivityOccurrenceParams{
		ActivityID:     activityID,
		OccurrenceType: kind,
		Name:           strings.Join(req.WinningTribes, ", ") + " won " + kind,
		EffectiveAt:    wordleTimestamp(at),
		Status:         "recorded",
		SourceRef:      pgtype.Text{String: key, Valid: true},
		Metadata:       metadata,
	})
	if err != nil {
		writeTribeError(c, err)
		return
	}
	created, err := gameplay.NewService(qtx).ResolveActivityOccurrence(ctx, occurrence.ID)
	if err != nil {
		writeTribeError(c, err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		writeTribeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"occurrence_id": pgUUIDString(occurrence.ID), "awarded_count": len(created)})
}

func tribesJSON(at time.Time, rows []db.ListInstanceTribeMembershipsAtRow) gin.H {
	tribes := []gin.H{}
	members := [][]gin.H{}
	index := map[string]int{}
	for _, row := range rows {
		id := pgUUIDString(row.ParticipantGroupID)
		i, ok := index[id]
		if !ok {
			i = len(tribes)
			index[id] = i
			tribes = append(tribes, gin.H{"id": id, "name": row.ParticipantGroupName})
			members = append(members, []gin.H{})
		}
		members[i] = append(members[i], gin.H{"participant_id": pgUUIDString(row.ParticipantID), "name": row.ParticipantName})
	}
	for i := range tribes {
		tribes[i]["members"] = members[i]
	}
	return gin.H{"at": at.Format(time.RFC3339Nano), "tribes": tribes}
}

func writeTribeError(c *gin.Context, err error) {
	var failure *progressionFailure
	if errors.As(err, &failure) {
		c.JSON(failure.status, errorResponse{Error: failure.message})
		return
	}
	c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
}
