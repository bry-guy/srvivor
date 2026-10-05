package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Approval gate: an announcement saved with an approval time stays a draft until an instance admin replies
// "yes" to the bot's DM; then it sends at that time. Approval is only a gate: once the time passes it can't be
// approved, and approving names the exact revision (text, channel, time, pings) the admin was shown. Editing
// the text holds it again and re-asks, even after approval.

// approvalRevision fingerprints what the admin reviewed.
const approvalRevision = `md5(a.body || '|' || a.channel_id || '|' || a.approval_send_at::text || '|' || a.notify_users::text)`

// requestAnnouncementApproval makes an unsent announcement wait for approval to send at send_at.
func (s *Server) requestAnnouncementApproval(c *gin.Context) {
	var req struct {
		SendAt time.Time `json:"send_at" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || !req.SendAt.After(s.now()) {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "a future send_at is required"})
		return
	}
	s.changeUnsentAnnouncement(c, `UPDATE announcements SET status = 'draft', scheduled_at = NULL, approval_gated = true, approval_send_at = $3, approval_claimed_at = NULL, approval_notified_at = NULL`,
		"", "", req.SendAt.UTC())
}

// instanceAdminIDs lists an instance's admins' Discord IDs.
func (s *Server) instanceAdminIDs(c *gin.Context, instanceID pgtype.UUID) ([]string, error) {
	rows, err := s.pool.Query(c.Request.Context(), `SELECT a.discord_user_id FROM instance_admins a JOIN instances i ON i.id = a.instance_id
		WHERE i.public_id = $1 ORDER BY a.discord_user_id`, instanceID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// claimAnnouncementApproval leases one announcement whose approval DM hasn't been delivered (until the bot
// confirms delivery), with the admins to DM.
func (s *Server) claimAnnouncementApproval(c *gin.Context) {
	if !requireAdminService(c) {
		return
	}
	var req struct {
		GuildIDs []string `json:"guild_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.GuildIDs) == 0 {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "guild_ids are required"})
		return
	}
	// Body, time and revision come from the same row version, so the DM shows exactly what "yes" approves.
	// The claim is a 10-minute lease: until the bot confirms delivery it's offered again.
	var a announcementRecord
	var sendAt time.Time
	var revision string
	var id, instanceID pgtype.UUID
	err := s.pool.QueryRow(c.Request.Context(), `
		UPDATE announcements a SET approval_claimed_at = $1 FROM instances i
		WHERE i.id = a.instance_id AND a.id = (SELECT id FROM announcements WHERE status = 'draft' AND approval_send_at > $1
			AND approval_notified_at IS NULL AND (approval_claimed_at IS NULL OR approval_claimed_at < $1 - interval '10 minutes')
			AND guild_id = ANY($2::text[]) ORDER BY approval_send_at LIMIT 1 FOR UPDATE SKIP LOCKED)
		RETURNING a.id, i.public_id, a.channel_id, a.body, a.notify_users, a.approval_send_at, `+approvalRevision,
		s.now(), req.GuildIDs).Scan(&id, &instanceID, &a.ChannelID, &a.Body, &a.NotifyUsers, &sendAt, &revision)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusOK, gin.H{"announcement": nil})
		return
	}
	if err != nil {
		writeAnnouncementError(c, err)
		return
	}
	a.ID, a.InstanceID = uuid.UUID(id.Bytes), uuid.UUID(instanceID.Bytes)
	admins, err := s.instanceAdminIDs(c, toPGUUID(a.InstanceID))
	if err != nil {
		writeAnnouncementError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"announcement": a, "send_at": sendAt.UTC(), "revision": revision, "admin_discord_user_ids": admins})
}

// approveAnnouncement schedules an approval-gated announcement on an instance admin's say-so.
func (s *Server) approveAnnouncement(c *gin.Context) {
	if !requireAdminService(c) {
		return
	}
	id, err := uuid.Parse(c.Param("announcementID"))
	var req struct {
		AdminID  string `json:"admin_discord_user_id" binding:"required"`
		Revision string `json:"revision" binding:"required"`
	}
	if err != nil || c.ShouldBindJSON(&req) != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "announcement id, admin_discord_user_id and revision are required"})
		return
	}
	var at time.Time
	err = s.pool.QueryRow(c.Request.Context(), `
		UPDATE announcements a SET status = 'pending', scheduled_at = a.approval_send_at, due_at = a.approval_send_at
		WHERE a.id = $1 AND a.status = 'draft' AND a.approval_send_at > $3 AND `+approvalRevision+` = $4
			AND EXISTS (SELECT 1 FROM instance_admins ad WHERE ad.instance_id = a.instance_id AND ad.discord_user_id = $2)
		RETURNING a.due_at`, id, req.AdminID, s.now(), req.Revision).Scan(&at)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusConflict, errorResponse{Error: "can't approve: it was already approved, changed since this DM (check for a newer one), its time has passed (reschedule it), or you aren't an admin of this season"})
		return
	}
	if err != nil {
		writeAnnouncementError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"scheduled_at": at.UTC()})
}

// confirmApprovalDelivery records that every admin got the approval DM for this revision.
func (s *Server) confirmApprovalDelivery(c *gin.Context) {
	if !requireAdminService(c) {
		return
	}
	id, err := uuid.Parse(c.Param("announcementID"))
	var req struct {
		Revision string `json:"revision" binding:"required"`
	}
	if err != nil || c.ShouldBindJSON(&req) != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "announcement id and revision are required"})
		return
	}
	if _, err := s.pool.Exec(c.Request.Context(), `UPDATE announcements a SET approval_notified_at = $3
		WHERE a.id = $1 AND a.status = 'draft' AND `+approvalRevision+` = $2`, id, req.Revision, s.now()); err != nil {
		writeAnnouncementError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
