package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
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
	_, err := s.pool.Exec(ctx, `INSERT INTO admin_alerts (instance_id, alert_key, body)
		SELECT sc.instance_id, 'automation-stale-' || to_char(sc.heartbeat_at AT TIME ZONE 'UTC', 'YYYYMMDD"T"HH24MISS'),
			'⚠️ **Season automation hasn''t run successfully since ' || to_char(sc.heartbeat_at AT TIME ZONE 'America/New_York', 'Mon FMDD FMHH12:MIam') ||
			' ET.** Scheduled imports, game checks and scores drafts are paused until it recovers. Check the probst-season-check CronJob logs.'
		FROM season_configs sc WHERE sc.heartbeat_at < $1::timestamptz - interval '30 minutes'
		ON CONFLICT (instance_id, alert_key) DO NOTHING`, s.now())
	return err
}
