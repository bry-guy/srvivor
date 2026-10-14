package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

// seasonConfig is an instance's synced season file and scores template. Revision is a content hash, so an
// update names the revision it replaces and loses (409) if someone synced in between.
type seasonConfig struct {
	SeasonYAML     string    `json:"season_yaml"`
	ScoresTemplate string    `json:"scores_template"`
	Revision       string    `json:"revision"`
	UpdatedAt      time.Time `json:"updated_at"`
	UpdatedBy      string    `json:"updated_by"`
}

func seasonConfigRevision(yaml, tmpl string) string {
	h := sha256.Sum256([]byte(yaml + "\x00" + tmpl))
	return hex.EncodeToString(h[:8])
}

func (s *Server) getSeasonConfig(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok || !s.requireInstanceAdminRequest(c, instanceID) {
		return
	}
	var cfg seasonConfig
	err := s.pool.QueryRow(c.Request.Context(), `SELECT sc.season_yaml, sc.scores_template, sc.revision, sc.updated_at, sc.updated_by
		FROM season_configs sc JOIN instances i ON i.id = sc.instance_id WHERE i.public_id = $1`, toPGUUID(instanceID)).
		Scan(&cfg.SeasonYAML, &cfg.ScoresTemplate, &cfg.Revision, &cfg.UpdatedAt, &cfg.UpdatedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, errorResponse{Error: "no season config synced for this instance"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "load season config"})
		return
	}
	c.JSON(http.StatusOK, cfg)
}

// putSeasonConfig replaces the config if base_revision is still current ("" when none exists yet).
func (s *Server) putSeasonConfig(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok || !s.requireInstanceAdminRequest(c, instanceID) {
		return
	}
	var req struct {
		SeasonYAML     string `json:"season_yaml" binding:"required"`
		ScoresTemplate string `json:"scores_template" binding:"required"`
		BaseRevision   string `json:"base_revision"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "season_yaml and scores_template are required"})
		return
	}
	rev := seasonConfigRevision(req.SeasonYAML, req.ScoresTemplate)
	tag, err := s.pool.Exec(c.Request.Context(), `
		INSERT INTO season_configs (instance_id, season_yaml, scores_template, revision, updated_at, updated_by)
		SELECT i.id, $2, $3, $4, $5, $6 FROM instances i WHERE i.public_id = $1
			AND COALESCE((SELECT revision FROM season_configs WHERE instance_id = i.id), '') = $7
		ON CONFLICT (instance_id) DO UPDATE SET season_yaml = EXCLUDED.season_yaml, scores_template = EXCLUDED.scores_template,
			revision = EXCLUDED.revision, updated_at = EXCLUDED.updated_at, updated_by = EXCLUDED.updated_by
			WHERE season_configs.revision = $7`,
		toPGUUID(instanceID), req.SeasonYAML, req.ScoresTemplate, rev, s.now(), c.GetHeader(discordUserIDHeader), req.BaseRevision)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "save season config"})
		return
	}
	if tag.RowsAffected() == 0 {
		c.JSON(http.StatusConflict, errorResponse{Error: "the season config changed since base_revision; re-run sync"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"revision": rev})
}

// automationHeartbeat records a successful scheduled run for the instance's synced season.
func (s *Server) automationHeartbeat(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok || !s.requireInstanceAdminRequest(c, instanceID) {
		return
	}
	tag, err := s.pool.Exec(c.Request.Context(), `UPDATE season_configs sc SET heartbeat_at = $2 FROM instances i
		WHERE i.id = sc.instance_id AND i.public_id = $1`, toPGUUID(instanceID), s.now())
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "record heartbeat"})
		return
	}
	if tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, errorResponse{Error: "no season config synced for this instance"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// raiseStaleAutomationAlerts queues one admin DM per stall (keyed by the last heartbeat) when a season's
// scheduled job hasn't succeeded for 30 minutes. The bot's alert polling drives it, so a dead job can't hide.
func (s *Server) raiseStaleAutomationAlerts(ctx context.Context) error {
	rows, err := s.pool.Query(ctx, `SELECT instance_id, heartbeat_at FROM season_configs WHERE heartbeat_at < $1::timestamptz - interval '30 minutes'`, s.now())
	if err != nil {
		return err
	}
	type stale struct {
		instance int64
		at       time.Time
	}
	found, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (stale, error) {
		var x stale
		return x, r.Scan(&x.instance, &x.at)
	})
	if err != nil {
		return err
	}
	for _, x := range found {
		body := message("automation-stale.md", map[string]string{"Since": x.at.In(easternTZ()).Format("Jan 2 3:04pm")})
		if _, err := s.pool.Exec(ctx, `INSERT INTO admin_alerts (instance_id, alert_key, body) VALUES ($1, $2, $3) ON CONFLICT (instance_id, alert_key) DO NOTHING`,
			x.instance, "automation-stale-"+x.at.UTC().Format("20060102T150405"), body); err != nil {
			return err
		}
	}
	return nil
}

// putEpisodes makes the season's episode schedule match a season file (`probst season sync --fix-episodes`):
// listed episodes are created or moved; unlisted ones are left alone. Admin only; Probst shows the diff first.
func (s *Server) putEpisodes(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok || !s.requireInstanceAdminRequest(c, instanceID) {
		return
	}
	var req struct {
		Episodes []createInstanceEpisodeRequest `json:"episodes" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Episodes) == 0 {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "episodes are required"})
		return
	}
	for _, e := range req.Episodes {
		if e.EpisodeNumber < 0 || e.AirsAt.IsZero() {
			c.JSON(http.StatusBadRequest, errorResponse{Error: "each episode needs episode_number >= 0 and airs_at"})
			return
		}
	}
	ctx := c.Request.Context()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		writeAnnouncementError(c, err)
		return
	}
	defer rollbackTx(c, tx)
	var id int64
	if err := tx.QueryRow(ctx, `SELECT id FROM instances WHERE public_id = $1 FOR UPDATE`, toPGUUID(instanceID)).Scan(&id); err != nil {
		writeAnnouncementError(c, err)
		return
	}
	// Park moved episodes out of the way first: airs_at is unique per season, so a shift can collide midway.
	for _, e := range req.Episodes {
		if _, err := tx.Exec(ctx, `UPDATE instance_episodes SET airs_at = airs_at + interval '100 years' WHERE instance_id = $1 AND episode_number = $2 AND airs_at <> $3`,
			id, e.EpisodeNumber, e.AirsAt.UTC()); err != nil {
			writeAnnouncementError(c, err)
			return
		}
	}
	changed := 0
	for _, e := range req.Episodes {
		label := e.Label
		if label == "" {
			label = fmt.Sprintf("Episode %d", e.EpisodeNumber)
		}
		tag, err := tx.Exec(ctx, `INSERT INTO instance_episodes (instance_id, episode_number, label, airs_at) VALUES ($1, $2, $3, $4)
			ON CONFLICT (instance_id, episode_number) DO UPDATE SET airs_at = EXCLUDED.airs_at, updated_at = NOW()
			WHERE instance_episodes.airs_at <> EXCLUDED.airs_at`, id, e.EpisodeNumber, label, e.AirsAt.UTC())
		if err != nil {
			writeAnnouncementError(c, err)
			return
		}
		changed += int(tag.RowsAffected())
	}
	if err := tx.Commit(ctx); err != nil {
		writeAnnouncementError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"changed": changed})
}
