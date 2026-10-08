package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// raiseAdminAlert queues a DM to the season's admins once per key; raising it again changes nothing.
func (s *Server) raiseAdminAlert(c *gin.Context) {
	if !requireAdminService(c) {
		return
	}
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok {
		return
	}
	key := strings.TrimSpace(c.Param("alertKey"))
	var req struct {
		Body string `json:"body"`
	}
	if c.ShouldBindJSON(&req) != nil || key == "" || len(key) > 120 || strings.TrimSpace(req.Body) == "" || utf8.RuneCountInString(req.Body) > 1900 {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "an alert key (at most 120 characters) and a nonblank body (at most 1900 characters) are required"})
		return
	}
	ctx := c.Request.Context()
	if err := requireWordleInstanceAdmin(ctx, s.queries, toPGUUID(instanceID), c.Request); err != nil {
		writeAnnouncementError(c, err)
		return
	}
	tag, err := s.pool.Exec(ctx, `INSERT INTO admin_alerts (instance_id, alert_key, body)
		SELECT id, $2, $3 FROM instances WHERE public_id = $1 ON CONFLICT (instance_id, alert_key) DO NOTHING`,
		toPGUUID(instanceID), key, req.Body)
	if err != nil {
		writeAnnouncementError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"created": tag.RowsAffected() == 1})
}

// claimAdminAlert hands the bot the next undelivered alert for a season bound in its guilds, with that
// season's admins. The claim is a 10-minute lease: until delivery is confirmed it's offered again.
func (s *Server) claimAdminAlert(c *gin.Context) {
	if !requireAdminService(c) {
		return
	}
	var req struct {
		GuildIDs []string `json:"guild_ids"`
	}
	if c.ShouldBindJSON(&req) != nil || len(req.GuildIDs) == 0 {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "guild_ids are required"})
		return
	}
	var id uuid.UUID
	var instanceID uuid.UUID
	var body string
	err := s.pool.QueryRow(c.Request.Context(), `
		UPDATE admin_alerts a SET claimed_at = $1 FROM instances i
		WHERE i.id = a.instance_id AND a.id = (SELECT al.id FROM admin_alerts al
			WHERE al.delivered_at IS NULL AND (al.claimed_at IS NULL OR al.claimed_at < $1::timestamptz - interval '10 minutes')
				AND EXISTS (SELECT 1 FROM discord_channel_bindings b WHERE b.instance_id = al.instance_id AND b.guild_id = ANY($2::text[]))
			ORDER BY al.created_at LIMIT 1 FOR UPDATE SKIP LOCKED)
		RETURNING a.id, i.public_id, a.body`, s.now(), req.GuildIDs).Scan(&id, &instanceID, &body)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusOK, gin.H{"alert": nil})
		return
	}
	if err != nil {
		writeAnnouncementError(c, err)
		return
	}
	admins, err := s.instanceAdminIDs(c, toPGUUID(instanceID))
	if err != nil {
		writeAnnouncementError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"alert": gin.H{"id": id, "body": body}, "admin_discord_user_ids": admins})
}

// confirmAdminAlert records that every admin got the alert.
func (s *Server) confirmAdminAlert(c *gin.Context) {
	if !requireAdminService(c) {
		return
	}
	id, err := uuid.Parse(c.Param("alertID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid alert id"})
		return
	}
	if _, err := s.pool.Exec(c.Request.Context(), `UPDATE admin_alerts SET delivered_at = $2 WHERE id = $1 AND delivered_at IS NULL`, id, s.now()); err != nil {
		writeAnnouncementError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// listScheduledGames lists a season's scheduled (episode) games of every type, without answers, so
// automation can check each week has its game.
func (s *Server) listScheduledGames(c *gin.Context) {
	if !requireAdminService(c) {
		return
	}
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok {
		return
	}
	ctx := c.Request.Context()
	if err := requireWordleInstanceAdmin(ctx, s.queries, toPGUUID(instanceID), c.Request); err != nil {
		writeAnnouncementError(c, err)
		return
	}
	rows, err := s.pool.Query(ctx, `
		SELECT g.type, g.public_id, g.episode_number, g.opens_at, g.cutoff_at, g.scored FROM (
			SELECT 'castawordle' AS type, w.public_id, w.episode_number, w.opens_at, w.cutoff_at, w.instance_id,
				COALESCE(o.status = 'resolved', w.wordle_round_id IS NULL) AS scored
				FROM castawordle_games w LEFT JOIN activity_occurrences o ON o.public_id = w.wordle_round_id WHERE w.episode_number IS NOT NULL
			UNION ALL SELECT 'press_the_button', public_id, episode_number, opens_at, cutoff_at, instance_id, resolved_at IS NOT NULL FROM button_games WHERE episode_number IS NOT NULL
			UNION ALL SELECT 'spell_it_out', public_id, episode_number, opens_at, cutoff_at, instance_id, resolved_at IS NOT NULL FROM scramble_games WHERE episode_number IS NOT NULL
		) g JOIN instances i ON i.id = g.instance_id WHERE i.public_id = $1 ORDER BY g.episode_number, g.type`, toPGUUID(instanceID))
	if err != nil {
		writeAnnouncementError(c, err)
		return
	}
	defer rows.Close()
	games := []gin.H{}
	for rows.Next() {
		var kind string
		var id uuid.UUID
		var episode int32
		var opens, cutoff time.Time
		var scored bool
		if err := rows.Scan(&kind, &id, &episode, &opens, &cutoff, &scored); err != nil {
			writeAnnouncementError(c, err)
			return
		}
		games = append(games, gin.H{"type": kind, "id": id, "episode_number": episode, "opens_at": opens.UTC(), "cutoff_at": cutoff.UTC(), "scored": scored})
	}
	if err := rows.Err(); err != nil {
		writeAnnouncementError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"games": games})
}
