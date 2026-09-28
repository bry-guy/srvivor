package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"strings"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Draft submissions are opt-in per instance: an admin opens them with the starting tribes and the channel
// to announce in. Each player's first saved draft then becomes one resolved occurrence recording their
// submission order and tribe; the first two while open earn +2/+1, and a pinged announcement is queued.
const draftSubmissionActivityType = "draft_submission"

var draftSubmissionBonus = map[int]int32{1: 2, 2: 1}

type draftSubmissionConfig struct {
	Tribes    []string `json:"tribes"`
	GuildID   string   `json:"guild_id"`
	ChannelID string   `json:"channel_id"`
	ThreadID  string   `json:"thread_id,omitempty"` // watched by the bot for draft posts
}

type draftSubmissionResult struct {
	Order    int    `json:"order"`
	Tribe    string `json:"tribe,omitempty"`
	Points   int32  `json:"bonus_points"`
	Eligible bool   `json:"bonus_eligible"`
}

// Approved copy. %[1]s is the player (mention or bold name), %[2]s the tribe emoji, %[3]s the tribe.
const (
	draftFirstCopy  = "🔥 %[1]s — first to lock in a draft. That's the kind of hunger that wins this game. You've earned **+2 bonus points**... and a buff. Reach in. It's %[2]s **%[3]s**."
	draftSecondCopy = "%[1]s, right behind them. Second draft in — that's worth **+1 bonus point**. Not first... but you're in the fight. Grab your buff: %[2]s **%[3]s**."
	draftLastCopy   = "%[1]s... last draft in. I gotta be honest — I was starting to wonder if you'd make it to the island at all. But you're here. Grab your buff: %[2]s **%[3]s**. Try to keep up."
)

var draftRestCopy = []string{
	"%[1]s Your draft is in. And for that, you've earned something every castaway wants: a %[2]s **%[3]s** buff. Wear it well.",
	"%[1]s You made it into the game. Take this %[2]s **%[3]s** buff. Your new family is waiting.",
	"%[1]s Draft received. Welcome to the Open Era. Reach in, grab a buff... it's %[2]s **%[3]s**.",
}

var tribeColorEmoji = map[string]string{"savu": "🟣", "toka": "🟡"}

func draftCopy(template, player, tribe string) string {
	emoji := tribeColorEmoji[strings.ToLower(tribe)]
	if emoji == "" {
		emoji = "🏝️"
	}
	return fmt.Sprintf(template, player, emoji, tribe)
}

// pickDraftTribe is a shuffled bag: a random tribe among those with the fewest members, so tribes never
// differ by more than one and the next buff can't be predicted.
func pickDraftTribe(tribes []string, counts map[string]int, roll func(int) int) string {
	var smallest []string
	for _, tribe := range tribes {
		switch {
		case len(smallest) == 0 || counts[tribe] < counts[smallest[0]]:
			smallest = []string{tribe}
		case counts[tribe] == counts[smallest[0]]:
			smallest = append(smallest, tribe)
		}
	}
	return smallest[roll(len(smallest))]
}

func (s *Server) openDraftSubmissions(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok {
		return
	}
	var req draftSubmissionConfig
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Tribes) < 2 || !validAnnouncementID(req.GuildID) || !validAnnouncementID(req.ChannelID) {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "tribes (at least two), guild_id and channel_id are required"})
		return
	}
	seen := map[string]bool{}
	for i, tribe := range req.Tribes {
		req.Tribes[i] = strings.TrimSpace(tribe)
		if req.Tribes[i] == "" || seen[strings.ToLower(req.Tribes[i])] {
			c.JSON(http.StatusBadRequest, errorResponse{Error: "tribe names must be non-empty and unique"})
			return
		}
		seen[strings.ToLower(req.Tribes[i])] = true
	}
	tx, qtx, ok := s.lockLegacyInstanceForAdmin(c, instanceID)
	if !ok {
		return
	}
	defer rollbackTx(c, tx)
	ctx := c.Request.Context()
	binding, err := qtx.GetDiscordChannelBinding(ctx, db.GetDiscordChannelBindingParams{GuildID: req.GuildID, ChannelID: req.ChannelID})
	if err != nil || binding.InstanceID != toPGUUID(instanceID) {
		c.JSON(http.StatusConflict, errorResponse{Error: "channel is not bound to this instance"})
		return
	}
	existing, err := qtx.ListInstanceActivitiesByType(ctx, db.ListInstanceActivitiesByTypeParams{InstanceID: toPGUUID(instanceID), ActivityType: draftSubmissionActivityType})
	if err != nil {
		writeTribeError(c, err)
		return
	}
	if len(existing) > 0 {
		c.JSON(http.StatusConflict, errorResponse{Error: "draft submissions were already opened for this instance"})
		return
	}
	metadata, err := json.Marshal(req)
	if err != nil {
		writeTribeError(c, err)
		return
	}
	if _, err := qtx.CreateInstanceActivity(ctx, db.CreateInstanceActivityParams{InstanceID: toPGUUID(instanceID), ActivityType: draftSubmissionActivityType, Name: "Draft submissions", Status: "active", StartsAt: wordleTimestamp(s.now()), Metadata: metadata}); err != nil {
		writeTribeError(c, err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		writeTribeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"status": "open", "tribes": req.Tribes})
}

// closeDraftSubmissions ends bonus eligibility and ribs the last player to submit so far.
func (s *Server) closeDraftSubmissions(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok {
		return
	}
	tx, qtx, ok := s.lockLegacyInstanceForAdmin(c, instanceID)
	if !ok {
		return
	}
	defer rollbackTx(c, tx)
	ctx := c.Request.Context()
	activity, config, err := draftSubmissionActivity(ctx, qtx, toPGUUID(instanceID))
	if err == nil && activity == nil {
		err = progressionError(http.StatusNotFound, "draft submissions were never opened for this instance")
	}
	if err != nil {
		writeTribeError(c, err)
		return
	}
	if activity.Status != "active" {
		c.JSON(http.StatusOK, gin.H{"status": "closed"})
		return
	}
	var participantID pgtype.UUID
	var result draftSubmissionResult
	var raw []byte
	err = tx.QueryRow(ctx, `SELECT ao.source_ref::uuid, ao.metadata FROM activity_occurrences ao JOIN instance_activities ia ON ia.id = ao.activity_id
		WHERE ia.public_id = $1 AND ao.status = 'resolved' ORDER BY (ao.metadata->>'order')::int DESC LIMIT 1`, activity.ID).Scan(&participantID, &raw)
	last := gin.H(nil)
	if err == nil {
		if err = json.Unmarshal(raw, &result); err == nil && result.Order > 2 {
			if err = s.queueDraftAnnouncement(ctx, tx, instanceID, config, participantID, "draft-last", draftLastCopy, result.Tribe); err == nil {
				last = gin.H{"participant_id": pgUUIDString(participantID), "order": result.Order}
			}
		}
	} else if errors.Is(err, pgx.ErrNoRows) {
		err = nil
	}
	if err == nil {
		_, err = tx.Exec(ctx, `UPDATE instance_activities SET status = 'completed', ends_at = $2, updated_at = NOW() WHERE public_id = $1`, activity.ID, s.now())
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		writeTribeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "closed", "last": last})
}

func draftSubmissionActivity(ctx context.Context, q *db.Queries, instanceID pgtype.UUID) (*db.ListInstanceActivitiesByTypeRow, draftSubmissionConfig, error) {
	var config draftSubmissionConfig
	rows, err := q.ListInstanceActivitiesByType(ctx, db.ListInstanceActivitiesByTypeParams{InstanceID: instanceID, ActivityType: draftSubmissionActivityType})
	if err != nil || len(rows) == 0 {
		return nil, config, err
	}
	return &rows[0], config, json.Unmarshal(rows[0].Metadata, &config)
}

// recordDraftSubmission runs inside the draft-save transaction with the instance locked. A player's first
// draft post claims their submission order (a pending claim if it had problems); the claim resolves, with
// bonus, tribe, and post, when a complete draft is saved. Later saves only change picks.
func (s *Server) recordDraftSubmission(ctx context.Context, tx pgx.Tx, q *db.Queries, instanceID uuid.UUID, participantID pgtype.UUID) (*draftSubmissionResult, error) {
	activity, config, err := draftSubmissionActivity(ctx, q, toPGUUID(instanceID))
	if err != nil || activity == nil {
		return nil, err
	}
	claim, result, err := s.claimDraftOrder(ctx, q, activity, participantID)
	if err != nil || claim == nil || claim.Status == "resolved" {
		return nil, err
	}
	at := s.now()
	current, err := q.ListInstanceTribeMembershipsAt(ctx, db.ListInstanceTribeMembershipsAtParams{InstanceID: toPGUUID(instanceID), At: wordleTimestamp(at)})
	if err != nil {
		return nil, err
	}
	counts := map[string]int{}
	for _, row := range current {
		counts[row.ParticipantGroupName]++
		if row.ParticipantID == participantID {
			return nil, progressionError(http.StatusConflict, "player is already on tribe "+row.ParticipantGroupName)
		}
	}
	result.Tribe = pickDraftTribe(config.Tribes, counts, rand.IntN)
	if result.Eligible {
		result.Points = draftSubmissionBonus[result.Order]
	}
	metadata, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	if _, err := q.UpdateActivityOccurrenceStatusAndMetadata(ctx, db.UpdateActivityOccurrenceStatusAndMetadataParams{ID: claim.ID, Status: "resolved", Metadata: metadata}); err != nil {
		return nil, err
	}
	groupID, err := draftTribeGroup(ctx, q, instanceID, result.Tribe)
	if err != nil {
		return nil, err
	}
	if _, err := q.CreateParticipantGroupMembershipPeriod(ctx, db.CreateParticipantGroupMembershipPeriodParams{ParticipantGroupID: groupID, ParticipantID: participantID, Role: "member", StartsAt: wordleTimestamp(at), Metadata: []byte(`{}`)}); err != nil {
		return nil, err
	}
	if result.Points > 0 {
		if _, err := q.CreateBonusPointLedgerEntry(ctx, db.CreateBonusPointLedgerEntryParams{
			InstanceID: toPGUUID(instanceID), ParticipantID: participantID, ActivityOccurrenceID: claim.ID,
			EntryKind: "award", Points: result.Points, Visibility: "public", Reason: fmt.Sprintf("Draft submitted #%d", result.Order),
			EffectiveAt: wordleTimestamp(at), AwardKey: pgtype.Text{String: "draft-submission", Valid: true}, Metadata: []byte(`{}`),
		}); err != nil {
			return nil, err
		}
	}
	template := draftRestCopy[(result.Order-3+len(draftRestCopy)*8)%len(draftRestCopy)]
	switch {
	case result.Points == 2:
		template = draftFirstCopy
	case result.Points == 1:
		template = draftSecondCopy
	}
	return &result, s.queueDraftAnnouncement(ctx, tx, instanceID, config, participantID, "draft-submission", template, result.Tribe)
}

type draftClaim struct {
	ID     pgtype.UUID
	Status string
}

// claimDraftOrder returns the player's submission claim, creating a pending one with the next order if
// they have none: one past the highest existing order, so a rejected player goes to the back of the line
// and nobody else moves up.
func (s *Server) claimDraftOrder(ctx context.Context, q *db.Queries, activity *db.ListInstanceActivitiesByTypeRow, participantID pgtype.UUID) (*draftClaim, draftSubmissionResult, error) {
	var result draftSubmissionResult
	existing, err := q.GetActivityOccurrenceBySourceRef(ctx, db.GetActivityOccurrenceBySourceRefParams{ActivityID: activity.ID, SourceRef: pgtype.Text{String: pgUUIDString(participantID), Valid: true}})
	if err == nil {
		return &draftClaim{ID: existing.ID, Status: existing.Status}, result, json.Unmarshal(existing.Metadata, &result)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, result, err
	}
	occurrences, err := q.ListActivityOccurrencesByActivity(ctx, activity.ID)
	if err != nil {
		return nil, result, err
	}
	for _, o := range occurrences {
		var prior draftSubmissionResult
		if json.Unmarshal(o.Metadata, &prior) == nil && prior.Order > result.Order {
			result.Order = prior.Order
		}
	}
	result.Order++
	result.Eligible = activity.Status == "active"
	metadata, err := json.Marshal(result)
	if err != nil {
		return nil, result, err
	}
	created, err := q.CreateActivityOccurrence(ctx, db.CreateActivityOccurrenceParams{
		ActivityID: activity.ID, OccurrenceType: "submission", Name: fmt.Sprintf("Draft #%d", result.Order),
		EffectiveAt: wordleTimestamp(s.now()), Status: "recorded", SourceRef: pgtype.Text{String: pgUUIDString(participantID), Valid: true}, Metadata: metadata,
	})
	if err != nil {
		return nil, result, err
	}
	return &draftClaim{ID: created.ID, Status: created.Status}, result, nil
}

func draftTribeGroup(ctx context.Context, q *db.Queries, instanceID uuid.UUID, tribe string) (pgtype.UUID, error) {
	groups, err := q.ListParticipantGroupsByInstance(ctx, toPGUUID(instanceID))
	if err != nil {
		return pgtype.UUID{}, err
	}
	for _, group := range groups {
		if group.Kind == "tribe" && strings.EqualFold(group.Name, tribe) {
			return group.ID, nil
		}
	}
	group, err := q.CreateParticipantGroup(ctx, db.CreateParticipantGroupParams{InstanceID: toPGUUID(instanceID), Name: tribe, Kind: "tribe", Metadata: []byte(`{}`)})
	return group.ID, err
}

// queueDraftAnnouncement queues a pinged post for the player; keyed by player so it's queued at most once.
func (s *Server) queueDraftAnnouncement(ctx context.Context, tx pgx.Tx, instanceID uuid.UUID, config draftSubmissionConfig, participantID pgtype.UUID, prefix, template, tribe string) error {
	var name string
	var discordID pgtype.Text
	if err := tx.QueryRow(ctx, `SELECT name, discord_user_id FROM participants WHERE public_id = $1`, participantID).Scan(&name, &discordID); err != nil {
		return err
	}
	player := "**" + name + "**"
	if discordID.Valid && discordID.String != "" {
		player = "<@" + discordID.String + ">"
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO announcements (instance_id, guild_id, channel_id, request_key, body, due_at, status, notify_users)
		SELECT id, $2, $3, $4, $5, $6, 'pending', true FROM instances WHERE public_id = $1
		ON CONFLICT (instance_id, request_key) DO NOTHING`,
		toPGUUID(instanceID), config.GuildID, config.ChannelID, prefix+"-"+pgUUIDString(participantID), draftCopy(template, player, tribe), s.now())
	return err
}
