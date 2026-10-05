package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/draftparse"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// watchDraftThread sets the Discord thread the bot reads draft posts from.
func (s *Server) watchDraftThread(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok {
		return
	}
	var req struct {
		ThreadID string `json:"thread_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || !validAnnouncementID(req.ThreadID) {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "thread_id must be a Discord ID"})
		return
	}
	tx, qtx, ok := s.lockLegacyInstanceForAdmin(c, instanceID)
	if !ok {
		return
	}
	defer rollbackTx(c, tx)
	ctx := c.Request.Context()
	activity, _, err := draftSubmissionActivity(ctx, qtx, toPGUUID(instanceID))
	if err == nil && activity == nil {
		err = progressionError(http.StatusConflict, "open draft submissions first")
	}
	if err == nil {
		_, err = tx.Exec(ctx, `UPDATE instance_activities SET metadata = metadata || jsonb_build_object('thread_id', $2::text), updated_at = NOW() WHERE public_id = $1`, activity.ID, req.ThreadID)
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		writeTribeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"thread_id": req.ThreadID})
}

// listDraftThreads tells the bot which threads to watch.
func (s *Server) listDraftThreads(c *gin.Context) {
	if !requireAdminService(c) {
		return
	}
	rows, err := s.pool.Query(c.Request.Context(), `SELECT i.public_id, ia.metadata->>'thread_id' FROM instance_activities ia JOIN instances i ON i.id = ia.instance_id
		WHERE ia.activity_type = $1 AND ia.metadata ? 'thread_id' ORDER BY ia.id`, draftSubmissionActivityType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	defer rows.Close()
	threads := []gin.H{}
	for rows.Next() {
		var instanceID pgtype.UUID
		var threadID string
		if err := rows.Scan(&instanceID, &threadID); err != nil {
			c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
			return
		}
		threads = append(threads, gin.H{"instance_id": pgUUIDString(instanceID), "thread_id": threadID})
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"threads": threads})
}

type draftThreadMessage struct {
	MessageID string `json:"message_id"`
	Version   string `json:"version"` // edited timestamp, or created timestamp when never edited
	AuthorID  string `json:"author_discord_user_id"`
	Content   string `json:"content"`
	// FixedBy is the admin who corrected this draft by DM; the corrected post is saved as the author's draft.
	FixedBy string `json:"fixed_by_discord_user_id"`
	// OverwritePicks lets an admin's fix replace a saved draft, but only the one they were shown (its
	// fingerprint from the "conflict" response), never a newer one.
	OverwritePicks string `json:"overwrite_picks"`
}

// receiveDraftThreadMessage handles one post in a watched thread. Chat is ignored. A complete draft from a
// linked player is saved (first one claims their order, bonus, tribe, and post). Anything else is returned
// as a problem for the bot to DM the admin; a problem draft still claims the player's submission order.
func (s *Server) receiveDraftThreadMessage(c *gin.Context) {
	if !requireAdminService(c) {
		return
	}
	threadID := c.Param("threadID")
	var req draftThreadMessage
	if err := c.ShouldBindJSON(&req); err != nil || !validAnnouncementID(threadID) || !validAnnouncementID(req.MessageID) || !validAnnouncementID(req.AuthorID) || req.Version == "" {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "message_id, version, and author_discord_user_id are required"})
		return
	}
	ctx := c.Request.Context()
	var instanceID pgtype.UUID
	err := s.pool.QueryRow(ctx, `SELECT i.public_id FROM instance_activities ia JOIN instances i ON i.id = ia.instance_id
		WHERE ia.activity_type = $1 AND ia.metadata->>'thread_id' = $2`, draftSubmissionActivityType, threadID).Scan(&instanceID)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, errorResponse{Error: "thread is not watched"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	admins, err := s.instanceAdminIDs(c, instanceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	if req.FixedBy != "" && !slices.Contains(admins, req.FixedBy) {
		c.JSON(http.StatusForbidden, errorResponse{Error: "only an admin of this season can fix drafts"})
		return
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	defer rollbackTx(c, tx)
	q := s.queries.WithTx(tx)
	if _, err := q.LockInstanceForProgression(ctx, instanceID); err != nil {
		writeTribeError(c, err)
		return
	}
	respond := func(status string, body gin.H) {
		if _, err := tx.Exec(ctx, `INSERT INTO draft_thread_messages (instance_id, message_id, version, status) SELECT id, $2, $3, $4 FROM instances WHERE public_id = $1`, instanceID, req.MessageID, req.Version, status); err != nil {
			writeTribeError(c, err)
			return
		}
		if err := tx.Commit(ctx); err != nil {
			writeTribeError(c, err)
			return
		}
		body["status"] = status
		body["instance_id"] = pgUUIDString(instanceID)
		if status == "problem" {
			body["admin_discord_user_ids"] = admins
		}
		c.JSON(http.StatusOK, body)
	}
	if req.FixedBy != "" {
		// A fix never silently replaces a saved draft: the admin is shown a fingerprint of the saved picks and
		// must confirm overwriting exactly those.
		var saved string
		if err := tx.QueryRow(ctx, `SELECT COALESCE(md5(string_agg(d.contestant_id::text, ',' ORDER BY d.position)), '')
			FROM draft_picks d JOIN participants p ON p.id = d.participant_id JOIN instances i ON i.id = p.instance_id
			WHERE i.public_id = $1 AND p.discord_user_id = $2`, instanceID, req.AuthorID).Scan(&saved); err != nil {
			writeTribeError(c, err)
			return
		}
		if saved != "" && saved != req.OverwritePicks {
			c.JSON(http.StatusOK, gin.H{"status": "conflict", "instance_id": pgUUIDString(instanceID), "saved_picks": saved})
			return
		}
	}
	var seen string
	if err := tx.QueryRow(ctx, `SELECT status FROM draft_thread_messages WHERE message_id = $1 AND version = $2`, req.MessageID, req.Version).Scan(&seen); err == nil {
		c.JSON(http.StatusOK, gin.H{"status": "duplicate", "instance_id": pgUUIDString(instanceID)})
		return
	} else if !errors.Is(err, pgx.ErrNoRows) {
		writeTribeError(c, err)
		return
	}

	rows, err := q.ListContestantsByInstance(ctx, instanceID)
	if err != nil {
		writeTribeError(c, err)
		return
	}
	contestants := make([]draftparse.Contestant, len(rows))
	for i, row := range rows {
		contestants[i] = draftparse.Contestant{ID: pgUUIDString(row.ID), Name: row.Name}
	}
	roster := draftparse.NewRoster(contestants)
	draft := draftparse.Parse(req.Content, roster)
	if !draftparse.IsCandidate(draft, roster) {
		respond("ignored", gin.H{})
		return
	}
	participant, err := q.GetParticipantByDiscordUserID(ctx, db.GetParticipantByDiscordUserIDParams{InstanceID: instanceID, DiscordUserID: pgtype.Text{String: req.AuthorID, Valid: true}})
	if errors.Is(err, pgx.ErrNoRows) {
		respond("problem", gin.H{"player": "<@" + req.AuthorID + ">", "problems": []string{"the author isn't linked to a player in this season"}})
		return
	}
	if err != nil {
		writeTribeError(c, err)
		return
	}
	if !draft.Ready() {
		activity, _, err := draftSubmissionActivity(ctx, q, instanceID)
		if err == nil {
			_, _, err = s.claimDraftOrder(ctx, q, activity, participant.ID)
		}
		if err != nil {
			writeTribeError(c, err)
			return
		}
		respond("problem", gin.H{"player": participant.Name, "participant_id": pgUUIDString(participant.ID), "problems": draft.Problems})
		return
	}
	ids := make([]uuid.UUID, len(draft.Order))
	for i, pick := range draft.Order {
		ids[i] = uuid.MustParse(pick.ID)
	}
	changed, err := saveDraftPicks(ctx, q, uuid.UUID(instanceID.Bytes), participant.ID, ids)
	if err != nil {
		writeTribeError(c, err)
		return
	}
	submission, err := s.recordDraftSubmission(ctx, tx, q, uuid.UUID(instanceID.Bytes), participant.ID)
	if err != nil {
		writeTribeError(c, err)
		return
	}
	status := "unchanged"
	if changed || submission != nil {
		status = "saved"
	}
	respond(status, gin.H{"player": participant.Name, "participant_id": pgUUIDString(participant.ID), "submission": submission})
}

// rejectDraftSubmission drops a player's pending (never completed) claim, so they lose their submission
// order and their next draft post starts a new claim. Completed submissions can't be rejected.
func (s *Server) rejectDraftSubmission(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok {
		return
	}
	participantID, ok := parseUUIDPath(c, "participantID")
	if !ok {
		return
	}
	tx, qtx, ok := s.lockLegacyInstanceForAdmin(c, instanceID)
	if !ok {
		return
	}
	defer rollbackTx(c, tx)
	ctx := c.Request.Context()
	activity, _, err := draftSubmissionActivity(ctx, qtx, toPGUUID(instanceID))
	if err == nil && activity == nil {
		err = progressionError(http.StatusNotFound, "draft submissions were never opened")
	}
	var claim db.GetActivityOccurrenceBySourceRefRow
	if err == nil {
		claim, err = qtx.GetActivityOccurrenceBySourceRef(ctx, db.GetActivityOccurrenceBySourceRefParams{ActivityID: activity.ID, SourceRef: pgtype.Text{String: participantID.String(), Valid: true}})
		if errors.Is(err, pgx.ErrNoRows) {
			err = progressionError(http.StatusNotFound, "player has no pending draft submission")
		}
	}
	if err == nil && claim.Status != "recorded" {
		err = progressionError(http.StatusConflict, "player's draft submission is already complete")
	}
	var result draftSubmissionResult
	if err == nil {
		err = json.Unmarshal(claim.Metadata, &result)
	}
	if err == nil {
		_, err = tx.Exec(ctx, `DELETE FROM activity_occurrences WHERE public_id = $1`, claim.ID)
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		writeTribeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "rejected", "order": result.Order})
}
