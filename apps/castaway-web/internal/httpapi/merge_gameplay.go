package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
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

const (
	activityTypeStirThePot            = "stir_the_pot"
	activityTypeIndividualPonyAuction = "individual_pony_auction"
	activityTypeIndividualPony        = "individual_pony"
	activityTypeLoanShark             = "loan_shark"

	occurrenceTypeStirThePotRound = "stir_the_pot_round"
	occurrenceTypeAuctionLot      = "auction_lot"
	occurrenceTypeIndividualPony  = "immunity_result"
	occurrenceTypeLoanIssued      = "loan_issued"
	occurrenceTypeLoanRepayment   = "loan_repayment"

	occurrenceRoleStirThePotContributor = "contributor"
	occurrenceRoleAuctionBidder         = "bidder"
)

type stirThePotRewardTier struct {
	Contributions int32 `json:"contributions"`
	Bonus         int32 `json:"bonus"`
}

type mergeTargetEpisodeMetadata struct {
	EpisodeID     string `json:"episode_id,omitempty"`
	EpisodeNumber int32  `json:"episode_number,omitempty"`
	EpisodeLabel  string `json:"episode_label,omitempty"`
	EpisodeAirsAt string `json:"episode_airs_at,omitempty"`
}

type stirThePotRoundMetadata struct {
	RewardTiers   []stirThePotRewardTier     `json:"reward_tiers,omitempty"`
	TargetEpisode mergeTargetEpisodeMetadata `json:"target_episode,omitempty"`
	ResolvedBy    string                     `json:"resolved_by,omitempty"`
	ResolvedAt    string                     `json:"resolved_at,omitempty"`
}

type stirThePotContributionMetadata struct {
	Contribution int32 `json:"contribution"`
}

type auctionLotMetadata struct {
	ContestantID         string                     `json:"contestant_id"`
	TargetEpisode        mergeTargetEpisodeMetadata `json:"target_episode,omitempty"`
	WinnerParticipantID  string                     `json:"winner_participant_id,omitempty"`
	WinningBidPoints     int32                      `json:"winning_bid_points,omitempty"`
	PricePoints          int32                      `json:"price_points,omitempty"`
	ResolvedAt           string                     `json:"resolved_at,omitempty"`
	ResolutionTiebreaker string                     `json:"resolution_tiebreaker,omitempty"`
}

type auctionBidMetadata struct {
	BidPoints int32 `json:"bid_points"`
}

type individualPonyOccurrenceMetadata struct {
	WinningContestantID string `json:"winning_contestant_id"`
}

type startStirThePotRoundRequest struct {
	Name string `json:"name"`
}

type addStirThePotContributionRequest struct {
	ParticipantID string `json:"participant_id"`
	Points        int32  `json:"points" binding:"required"`
}

type startAuctionLotRequest struct {
	ContestantID string `json:"contestant_id" binding:"required"`
}

type setAuctionBidRequest struct {
	ParticipantID string `json:"participant_id"`
	Points        int32  `json:"points" binding:"required"`
}

type loanSharkRequest struct {
	Points int32 `json:"points" binding:"required"`
}

type recordIndividualPonyImmunityRequest struct {
	ContestantID string     `json:"contestant_id" binding:"required"`
	EffectiveAt  *time.Time `json:"effective_at"`
}

type auctionRankedBid struct {
	participantID   pgtype.UUID
	participantName string
	bidPoints       int32
	createdAt       time.Time
}

func (s *Server) getStirThePotStatus(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok {
		return
	}
	participant, ok := s.requireLinkedParticipant(c, instanceID)
	if !ok {
		return
	}

	now := time.Now().UTC()
	round, _, found, err := s.findOpenStirThePotRound(c.Request.Context(), s.queries, toPGUUID(instanceID), now)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	if !found {
		c.JSON(http.StatusOK, gin.H{
			"open":         false,
			"participant":  participantSummaryToJSON(participant.ID, participant.Name),
			"reward_tiers": defaultStirThePotRewardTiers(),
		})
		return
	}

	contributionPoints, err := s.participantContributionPoints(c.Request.Context(), s.queries, round.ID, participant.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	balance, err := s.currentBonusBalance(c.Request.Context(), s.queries, toPGUUID(instanceID), participant.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	metadata := parseStirThePotRoundMetadata(round.Metadata)
	if len(metadata.RewardTiers) == 0 {
		metadata.RewardTiers = defaultStirThePotRewardTiers()
	}

	c.JSON(http.StatusOK, gin.H{
		"open":                   true,
		"participant":            participantSummaryToJSON(participant.ID, participant.Name),
		"round":                  occurrenceToJSON(round.ID, round.ActivityID, round.OccurrenceType, round.Name, round.EffectiveAt, round.StartsAt, round.EndsAt, round.Status, round.SourceRef, round.Metadata, round.CreatedAt, round.UpdatedAt),
		"my_contribution_points": contributionPoints,
		"bonus_points_available": balance,
		"reward_tiers":           metadata.RewardTiers,
	})
}

func (s *Server) getStirThePotTribeStatus(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok {
		return
	}
	if !s.requireInstanceAdminRequest(c, instanceID) {
		return
	}
	tribeName := strings.TrimSpace(c.Query("name"))
	if tribeName == "" {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "tribe name is required"})
		return
	}
	tribe, found, err := s.resolveTribeByName(c.Request.Context(), s.queries, toPGUUID(instanceID), tribeName)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	if !found {
		c.JSON(http.StatusNotFound, errorResponse{Error: fmt.Sprintf("tribe %q not found", tribeName)})
		return
	}

	now := time.Now().UTC()
	round, _, roundFound, err := s.findOpenStirThePotRound(c.Request.Context(), s.queries, toPGUUID(instanceID), now)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	if !roundFound {
		c.JSON(http.StatusOK, gin.H{
			"open":         false,
			"tribe":        gin.H{"id": pgUUIDString(tribe.ID), "name": tribe.Name, "kind": tribe.Kind},
			"reward_tiers": defaultStirThePotRewardTiers(),
		})
		return
	}

	metadata := parseStirThePotRoundMetadata(round.Metadata)
	if len(metadata.RewardTiers) == 0 {
		metadata.RewardTiers = defaultStirThePotRewardTiers()
	}
	contributionPoints, err := s.groupContributionPoints(c.Request.Context(), s.queries, round.ID, tribe.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"open":                         true,
		"tribe":                        gin.H{"id": pgUUIDString(tribe.ID), "name": tribe.Name, "kind": tribe.Kind},
		"round":                        occurrenceToJSON(round.ID, round.ActivityID, round.OccurrenceType, round.Name, round.EffectiveAt, round.StartsAt, round.EndsAt, round.Status, round.SourceRef, round.Metadata, round.CreatedAt, round.UpdatedAt),
		"contribution_points":          contributionPoints,
		"bonus_points_if_resolved_now": stirThePotBonusForContribution(contributionPoints, metadata.RewardTiers),
		"reward_tiers":                 metadata.RewardTiers,
	})
}

func (s *Server) startStirThePotRound(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok {
		return
	}
	if !s.requireInstanceAdminRequest(c, instanceID) {
		return
	}

	var req startStirThePotRoundRequest
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
			return
		}
	}

	now := time.Now().UTC()
	activity, err := s.ensureSystemActivity(c.Request.Context(), s.queries, toPGUUID(instanceID), activityTypeStirThePot, "Stir the Pot", now)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	if _, _, found, err := s.findOpenStirThePotRound(c.Request.Context(), s.queries, toPGUUID(instanceID), now); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	} else if found {
		c.JSON(http.StatusConflict, errorResponse{Error: "stir the pot is already open"})
		return
	}

	targetEpisode, err := s.nextEpisodeTarget(c.Request.Context(), s.queries, toPGUUID(instanceID), now)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = fmt.Sprintf("Stir the Pot — %s", targetEpisode.EpisodeLabel)
	}
	metadata, err := json.Marshal(stirThePotRoundMetadata{RewardTiers: defaultStirThePotRewardTiers(), TargetEpisode: targetEpisode})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	created, err := s.queries.CreateActivityOccurrence(c.Request.Context(), db.CreateActivityOccurrenceParams{
		ActivityID:     activity.ID,
		OccurrenceType: occurrenceTypeStirThePotRound,
		Name:           name,
		EffectiveAt:    optionalTime(now),
		Status:         "recorded",
		Metadata:       metadata,
	})
	if err != nil {
		c.JSON(statusFromPg(err), errorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"activity": activityToJSON(activity.ID, activity.InstanceID, activity.ActivityType, activity.Name, activity.Status, activity.StartsAt, activity.EndsAt, activity.Metadata, activity.CreatedAt, activity.UpdatedAt),
		"round":    occurrenceToJSON(created.ID, created.ActivityID, created.OccurrenceType, created.Name, created.EffectiveAt, created.StartsAt, created.EndsAt, created.Status, created.SourceRef, created.Metadata, created.CreatedAt, created.UpdatedAt),
	})
}

func (s *Server) addStirThePotContribution(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok {
		return
	}
	var req addStirThePotContributionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	if req.Points <= 0 {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "points must be positive"})
		return
	}
	participant, ok := s.resolveRequestedOrLinkedParticipant(c, instanceID, req.ParticipantID)
	if !ok {
		return
	}

	now := time.Now().UTC()
	round, _, found, err := s.findOpenStirThePotRound(c.Request.Context(), s.queries, toPGUUID(instanceID), now)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	if !found {
		c.JSON(http.StatusNotFound, errorResponse{Error: "stir the pot is not open"})
		return
	}

	groupMembership, err := s.currentTribeMembership(c.Request.Context(), s.queries, participant.ID, now)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	balance, err := s.currentBonusBalance(c.Request.Context(), s.queries, toPGUUID(instanceID), participant.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	if balance < req.Points {
		c.JSON(http.StatusBadRequest, errorResponse{Error: fmt.Sprintf("insufficient bonus points: have %d, need %d", balance, req.Points)})
		return
	}

	existingContribution, err := s.participantContributionPoints(c.Request.Context(), s.queries, round.ID, participant.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	newContribution := existingContribution + req.Points
	metadata, err := json.Marshal(stirThePotContributionMetadata{Contribution: newContribution})
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
	if _, err := qtx.UpsertActivityOccurrenceParticipant(c.Request.Context(), db.UpsertActivityOccurrenceParticipantParams{
		ActivityOccurrenceID: round.ID,
		ParticipantID:        participant.ID,
		ParticipantGroupID:   groupMembership.ParticipantGroupID,
		Role:                 occurrenceRoleStirThePotContributor,
		Result:               "",
		Metadata:             metadata,
	}); err != nil {
		c.JSON(statusFromPg(err), errorResponse{Error: err.Error()})
		return
	}
	ledgerMetadata := metadataWithConsumesSecretBalance(metadata, false)
	revealedSecretPoints, err := s.revealSecretPointsOnSpend(c.Request.Context(), qtx, toPGUUID(instanceID), participant.ID, round.ID, groupMembership.ParticipantGroupID, req.Points, now, "Stir the Pot contribution", metadata)
	if err != nil {
		c.JSON(statusFromPg(err), errorResponse{Error: err.Error()})
		return
	}
	if _, err := qtx.CreateBonusPointLedgerEntry(c.Request.Context(), db.CreateBonusPointLedgerEntryParams{
		InstanceID:           toPGUUID(instanceID),
		ParticipantID:        participant.ID,
		ActivityOccurrenceID: round.ID,
		SourceGroupID:        groupMembership.ParticipantGroupID,
		EntryKind:            "spend",
		Points:               -req.Points,
		Visibility:           "secret",
		Reason:               "Stir the Pot contribution",
		EffectiveAt:          optionalTime(now),
		AwardKey:             optionalText(ptrString("stir_the_pot:add:" + uuid.NewString())),
		Metadata:             ledgerMetadata,
	}); err != nil {
		c.JSON(statusFromPg(err), errorResponse{Error: err.Error()})
		return
	}

	if err := tx.Commit(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"participant":            participantSummaryToJSON(participant.ID, participant.Name),
		"round_id":               pgUUIDString(round.ID),
		"group_id":               pgUUIDString(groupMembership.ParticipantGroupID),
		"group_name":             groupMembership.ParticipantGroupName,
		"added_points":           req.Points,
		"my_contribution_points": newContribution,
		"bonus_points_available": balance - req.Points,
		"revealed_secret_points": revealedSecretPoints,
	})
}

func (s *Server) getAuctionStatus(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok {
		return
	}
	participant, ok := s.requireLinkedParticipant(c, instanceID)
	if !ok {
		return
	}

	now := time.Now().UTC()
	activity, openLots, err := s.listOpenAuctionLots(c.Request.Context(), s.queries, toPGUUID(instanceID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	balance, err := s.currentBonusBalance(c.Request.Context(), s.queries, toPGUUID(instanceID), participant.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	ownerships, err := s.queries.ListActiveParticipantPonyOwnershipsByOwnerAt(c.Request.Context(), db.ListActiveParticipantPonyOwnershipsByOwnerAtParams{
		InstanceID:         toPGUUID(instanceID),
		OwnerParticipantID: participant.ID,
		At:                 optionalTime(now),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	lotsJSON := make([]gin.H, 0, len(openLots))
	for _, lot := range openLots {
		myBidPoints, err := s.participantBidPoints(c.Request.Context(), s.queries, lot.Occurrence.ID, participant.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
			return
		}
		lotsJSON = append(lotsJSON, gin.H{
			"lot":             occurrenceToJSON(lot.Occurrence.ID, lot.Occurrence.ActivityID, lot.Occurrence.OccurrenceType, lot.Occurrence.Name, lot.Occurrence.EffectiveAt, lot.Occurrence.StartsAt, lot.Occurrence.EndsAt, lot.Occurrence.Status, lot.Occurrence.SourceRef, lot.Occurrence.Metadata, lot.Occurrence.CreatedAt, lot.Occurrence.UpdatedAt),
			"contestant_id":   lot.ContestantID.String(),
			"contestant_name": lot.ContestantName,
			"my_bid_points":   myBidPoints,
		})
	}

	poniesJSON := make([]gin.H, 0, len(ownerships))
	for _, ownership := range ownerships {
		poniesJSON = append(poniesJSON, gin.H{
			"id":              pgUUIDString(ownership.ID),
			"contestant_id":   pgUUIDString(ownership.ContestantID),
			"contestant_name": ownership.ContestantName,
			"acquired_at":     formatTimestamp(ownership.AcquiredAt),
		})
	}

	loanStatus, err := s.loanStatusPayload(c.Request.Context(), toPGUUID(instanceID), participant.ID, now)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	response := gin.H{
		"participant":            participantSummaryToJSON(participant.ID, participant.Name),
		"bonus_points_available": balance,
		"open_lots":              lotsJSON,
		"ponies":                 poniesJSON,
		"loan":                   loanStatus,
	}
	if activity != nil {
		response["activity"] = activityToJSON(activity.ID, activity.InstanceID, activity.ActivityType, activity.Name, activity.Status, activity.StartsAt, activity.EndsAt, activity.Metadata, activity.CreatedAt, activity.UpdatedAt)
		response["open"] = true
	} else {
		response["open"] = false
	}

	c.JSON(http.StatusOK, response)
}

func (s *Server) startAuctionLot(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok {
		return
	}
	if !s.requireInstanceAdminRequest(c, instanceID) {
		return
	}

	var req startAuctionLotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	contestantID, err := uuid.Parse(req.ContestantID)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid contestant_id"})
		return
	}
	contestant, err := s.requireContestant(c.Request.Context(), toPGUUID(instanceID), contestantID)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		c.JSON(status, errorResponse{Error: err.Error()})
		return
	}

	now := time.Now().UTC()
	activity, err := s.ensureSystemActivity(c.Request.Context(), s.queries, toPGUUID(instanceID), activityTypeIndividualPonyAuction, "Individual Pony Auction", now)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	if _, found, err := s.findOpenAuctionLotByContestant(c.Request.Context(), s.queries, activity.ID, contestantID); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	} else if found {
		c.JSON(http.StatusConflict, errorResponse{Error: "auction lot is already open for this contestant"})
		return
	}
	targetEpisode, err := s.nextEpisodeTarget(c.Request.Context(), s.queries, toPGUUID(instanceID), now)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	metadata, err := json.Marshal(auctionLotMetadata{ContestantID: contestantID.String(), TargetEpisode: targetEpisode})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	created, err := s.queries.CreateActivityOccurrence(c.Request.Context(), db.CreateActivityOccurrenceParams{
		ActivityID:     activity.ID,
		OccurrenceType: occurrenceTypeAuctionLot,
		Name:           fmt.Sprintf("%s Auction Lot — %s", contestant.Name, targetEpisode.EpisodeLabel),
		EffectiveAt:    optionalTime(now),
		Status:         "recorded",
		Metadata:       metadata,
	})
	if err != nil {
		c.JSON(statusFromPg(err), errorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"activity":     activityToJSON(activity.ID, activity.InstanceID, activity.ActivityType, activity.Name, activity.Status, activity.StartsAt, activity.EndsAt, activity.Metadata, activity.CreatedAt, activity.UpdatedAt),
		"lot":          occurrenceToJSON(created.ID, created.ActivityID, created.OccurrenceType, created.Name, created.EffectiveAt, created.StartsAt, created.EndsAt, created.Status, created.SourceRef, created.Metadata, created.CreatedAt, created.UpdatedAt),
		"contestant":   gin.H{"id": contestant.ID.String(), "name": contestant.Name},
		"bidding_open": true,
	})
}

func (s *Server) setAuctionBid(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok {
		return
	}
	contestantID, ok := parseUUIDPath(c, "contestantID")
	if !ok {
		return
	}
	var req setAuctionBidRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	if req.Points <= 0 {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "points must be positive"})
		return
	}
	participant, ok := s.resolveRequestedOrLinkedParticipant(c, instanceID, req.ParticipantID)
	if !ok {
		return
	}
	contestant, err := s.requireContestant(c.Request.Context(), toPGUUID(instanceID), contestantID)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		c.JSON(status, errorResponse{Error: err.Error()})
		return
	}

	_, openLots, err := s.listOpenAuctionLots(c.Request.Context(), s.queries, toPGUUID(instanceID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	var lot auctionLotView
	found := false
	for _, candidate := range openLots {
		if candidate.ContestantID == contestantID {
			lot = candidate
			found = true
			break
		}
	}
	if !found {
		c.JSON(http.StatusNotFound, errorResponse{Error: "auction lot is not open for this contestant"})
		return
	}

	oldBid, err := s.participantBidPoints(c.Request.Context(), s.queries, lot.Occurrence.ID, participant.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	delta := req.Points - oldBid
	if delta == 0 {
		balance, balErr := s.currentBonusBalance(c.Request.Context(), s.queries, toPGUUID(instanceID), participant.ID)
		if balErr != nil {
			c.JSON(http.StatusInternalServerError, errorResponse{Error: balErr.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"participant":            participantSummaryToJSON(participant.ID, participant.Name),
			"contestant":             gin.H{"id": contestant.ID.String(), "name": contestant.Name},
			"lot_id":                 pgUUIDString(lot.Occurrence.ID),
			"my_bid_points":          req.Points,
			"bonus_points_available": balance,
			"revealed_secret_points": 0,
		})
		return
	}
	balance, err := s.currentBonusBalance(c.Request.Context(), s.queries, toPGUUID(instanceID), participant.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	if delta > 0 && balance < delta {
		c.JSON(http.StatusBadRequest, errorResponse{Error: fmt.Sprintf("insufficient bonus points: have %d, need %d", balance, delta)})
		return
	}

	now := time.Now().UTC()
	metadata, err := json.Marshal(auctionBidMetadata{BidPoints: req.Points})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	ledgerMetadata := metadataWithConsumesSecretBalance(metadata, false)

	tx, err := s.pool.Begin(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	defer rollbackTx(c, tx)

	qtx := s.queries.WithTx(tx)
	if _, err := qtx.UpsertActivityOccurrenceParticipant(c.Request.Context(), db.UpsertActivityOccurrenceParticipantParams{
		ActivityOccurrenceID: lot.Occurrence.ID,
		ParticipantID:        participant.ID,
		Role:                 occurrenceRoleAuctionBidder,
		Result:               "",
		Metadata:             metadata,
	}); err != nil {
		c.JSON(statusFromPg(err), errorResponse{Error: err.Error()})
		return
	}
	revealedSecretPoints := int32(0)
	if delta > 0 {
		revealedSecretPoints, err = s.revealSecretPointsOnSpend(c.Request.Context(), qtx, toPGUUID(instanceID), participant.ID, lot.Occurrence.ID, pgtype.UUID{}, delta, now, fmt.Sprintf("auction bid on %s", contestant.Name), metadata)
		if err != nil {
			c.JSON(statusFromPg(err), errorResponse{Error: err.Error()})
			return
		}
	}
	if delta != 0 {
		entryKind := "spend"
		points := -delta
		reason := fmt.Sprintf("Bid on %s set to %d", contestant.Name, req.Points)
		if delta < 0 {
			entryKind = "correction"
			points = -delta
			reason = fmt.Sprintf("Bid on %s reduced to %d", contestant.Name, req.Points)
		}
		if _, err := qtx.CreateBonusPointLedgerEntry(c.Request.Context(), db.CreateBonusPointLedgerEntryParams{
			InstanceID:           toPGUUID(instanceID),
			ParticipantID:        participant.ID,
			ActivityOccurrenceID: lot.Occurrence.ID,
			EntryKind:            entryKind,
			Points:               points,
			Visibility:           "secret",
			Reason:               reason,
			EffectiveAt:          optionalTime(now),
			AwardKey:             optionalText(ptrString("auction:bid:" + uuid.NewString())),
			Metadata:             ledgerMetadata,
		}); err != nil {
			c.JSON(statusFromPg(err), errorResponse{Error: err.Error()})
			return
		}
	}
	if err := tx.Commit(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	updatedBalance := balance - delta
	c.JSON(http.StatusOK, gin.H{
		"participant":            participantSummaryToJSON(participant.ID, participant.Name),
		"contestant":             gin.H{"id": contestant.ID.String(), "name": contestant.Name},
		"lot_id":                 pgUUIDString(lot.Occurrence.ID),
		"my_bid_points":          req.Points,
		"previous_bid_points":    oldBid,
		"bonus_points_available": updatedBalance,
		"revealed_secret_points": revealedSecretPoints,
	})
}

func (s *Server) stopAuctionLot(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok {
		return
	}
	contestantID, ok := parseUUIDPath(c, "contestantID")
	if !ok {
		return
	}
	if !s.requireInstanceAdminRequest(c, instanceID) {
		return
	}

	contestant, err := s.requireContestant(c.Request.Context(), toPGUUID(instanceID), contestantID)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		c.JSON(status, errorResponse{Error: err.Error()})
		return
	}
	activity, err := s.primarySystemActivity(c.Request.Context(), s.queries, toPGUUID(instanceID), activityTypeIndividualPonyAuction)
	if err != nil {
		c.JSON(http.StatusNotFound, errorResponse{Error: "auction activity not found"})
		return
	}
	lot, found, err := s.findOpenAuctionLotByContestant(c.Request.Context(), s.queries, activity.ID, contestantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	if !found {
		c.JSON(http.StatusNotFound, errorResponse{Error: "auction lot is not open for this contestant"})
		return
	}

	bids, err := s.rankAuctionBids(c.Request.Context(), s.queries, lot.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	now := time.Now().UTC()

	tx, err := s.pool.Begin(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	defer rollbackTx(c, tx)
	qtx := s.queries.WithTx(tx)

	var winner *auctionRankedBid
	winningBidPoints := int32(0)
	pricePoints := int32(0)
	if len(bids) > 0 {
		winner = &bids[0]
		winningBidPoints = bids[0].bidPoints
		if len(bids) > 1 {
			pricePoints = bids[1].bidPoints
		}
	}

	for index, bid := range bids {
		refund := bid.bidPoints
		reason := fmt.Sprintf("Refunded bid on %s", contestant.Name)
		if index == 0 {
			refund = bid.bidPoints - pricePoints
			reason = fmt.Sprintf("Auction settled for %s at %d points", contestant.Name, pricePoints)
		}
		if refund <= 0 {
			continue
		}
		if _, err := qtx.CreateBonusPointLedgerEntry(c.Request.Context(), db.CreateBonusPointLedgerEntryParams{
			InstanceID:           toPGUUID(instanceID),
			ParticipantID:        bid.participantID,
			ActivityOccurrenceID: lot.ID,
			EntryKind:            "correction",
			Points:               refund,
			Visibility:           "secret",
			Reason:               reason,
			EffectiveAt:          optionalTime(now),
			AwardKey:             optionalText(ptrString("auction:refund:" + uuid.NewString())),
			Metadata:             metadataWithConsumesSecretBalance(lot.Metadata, false),
		}); err != nil {
			c.JSON(statusFromPg(err), errorResponse{Error: err.Error()})
			return
		}
	}

	if winner != nil {
		if _, err := qtx.CreateParticipantPonyOwnership(c.Request.Context(), db.CreateParticipantPonyOwnershipParams{
			InstanceID:                 toPGUUID(instanceID),
			OwnerParticipantID:         winner.participantID,
			ContestantID:               toPGUUID(contestantID),
			SourceActivityOccurrenceID: lot.ID,
			AcquiredAt:                 optionalTime(now),
			ReleasedAt:                 pgtype.Timestamptz{},
			Status:                     "active",
			Metadata:                   lot.Metadata,
		}); err != nil {
			c.JSON(statusFromPg(err), errorResponse{Error: err.Error()})
			return
		}
	}

	updatedMetadata := auctionLotMetadata{ContestantID: contestantID.String(), WinningBidPoints: winningBidPoints, PricePoints: pricePoints, ResolvedAt: now.Format(time.RFC3339), ResolutionTiebreaker: "highest bid, then earliest submitted bid, then participant name"}
	if err := json.Unmarshal(nonEmptyMetadata(lot.Metadata), &updatedMetadata); err == nil {
		updatedMetadata.ContestantID = contestantID.String()
		updatedMetadata.WinningBidPoints = winningBidPoints
		updatedMetadata.PricePoints = pricePoints
		updatedMetadata.ResolvedAt = now.Format(time.RFC3339)
		updatedMetadata.ResolutionTiebreaker = "highest bid, then earliest submitted bid, then participant name"
	}
	if winner != nil {
		updatedMetadata.WinnerParticipantID = pgUUIDString(winner.participantID)
	}
	metadata, err := json.Marshal(updatedMetadata)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	if _, err := qtx.UpdateActivityOccurrenceStatusAndMetadata(c.Request.Context(), db.UpdateActivityOccurrenceStatusAndMetadataParams{
		ID:       lot.ID,
		Status:   "resolved",
		EndsAt:   optionalTime(now),
		Metadata: metadata,
	}); err != nil {
		c.JSON(statusFromPg(err), errorResponse{Error: err.Error()})
		return
	}

	if err := tx.Commit(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	response := gin.H{
		"contestant":         gin.H{"id": contestant.ID.String(), "name": contestant.Name},
		"lot_id":             pgUUIDString(lot.ID),
		"winning_bid_points": winningBidPoints,
		"price_points":       pricePoints,
	}
	if winner != nil {
		response["winner"] = gin.H{"participant_id": pgUUIDString(winner.participantID), "participant_name": winner.participantName}
	} else {
		response["winner"] = nil
	}
	c.JSON(http.StatusOK, response)
}

func (s *Server) getMyPonies(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok {
		return
	}
	participant, ok := s.requireLinkedParticipant(c, instanceID)
	if !ok {
		return
	}
	now := time.Now().UTC()
	ownerships, err := s.queries.ListActiveParticipantPonyOwnershipsByOwnerAt(c.Request.Context(), db.ListActiveParticipantPonyOwnershipsByOwnerAtParams{
		InstanceID:         toPGUUID(instanceID),
		OwnerParticipantID: participant.ID,
		At:                 optionalTime(now),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	ponies := make([]gin.H, 0, len(ownerships))
	for _, ownership := range ownerships {
		ponies = append(ponies, gin.H{
			"id":              pgUUIDString(ownership.ID),
			"contestant_id":   pgUUIDString(ownership.ContestantID),
			"contestant_name": ownership.ContestantName,
			"acquired_at":     formatTimestamp(ownership.AcquiredAt),
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"participant": participantSummaryToJSON(participant.ID, participant.Name),
		"ponies":      ponies,
	})
}

func (s *Server) getLoanSharkStatus(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok {
		return
	}
	participant, ok := s.requireLinkedParticipant(c, instanceID)
	if !ok {
		return
	}
	status, err := s.loanStatusPayload(c.Request.Context(), toPGUUID(instanceID), participant.ID, time.Now().UTC())
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"participant": participantSummaryToJSON(participant.ID, participant.Name),
		"loan":        status,
	})
}

func (s *Server) borrowFromLoanShark(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok {
		return
	}
	participant, ok := s.requireLinkedParticipant(c, instanceID)
	if !ok {
		return
	}

	var req loanSharkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	if req.Points <= 0 {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "points must be positive"})
		return
	}

	now := time.Now().UTC()
	auctionActivity, err := s.primarySystemActivity(c.Request.Context(), s.queries, toPGUUID(instanceID), activityTypeIndividualPonyAuction)
	if err != nil {
		c.JSON(http.StatusConflict, errorResponse{Error: "individual pony auction is not active"})
		return
	}
	_ = auctionActivity
	loanActivity, err := s.ensureSystemActivity(c.Request.Context(), s.queries, toPGUUID(instanceID), activityTypeLoanShark, "Loan Shark", now)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	maxPrincipal, interestPoints, err := s.loanTermsForParticipant(c.Request.Context(), toPGUUID(instanceID), participant.ID, now)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	activeLoan, hasActiveLoan, err := s.activeLoan(c.Request.Context(), s.queries, toPGUUID(instanceID), participant.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	currentPrincipal := int32(0)
	currentInterest := interestPoints
	currentPrincipalRepaid := int32(0)
	currentInterestRepaid := int32(0)
	dueAt := optionalTime(s.loanDueAt(c.Request.Context(), toPGUUID(instanceID), now))
	loanID := pgtype.UUID{}
	metadata := []byte("{}")
	status := "active"
	grantedAt := optionalTime(now)
	settledAt := pgtype.Timestamptz{}
	if hasActiveLoan {
		loanID = activeLoan.ID
		currentPrincipal = activeLoan.PrincipalPoints
		currentInterest = activeLoan.InterestPoints
		currentPrincipalRepaid = activeLoan.PrincipalRepaidPoints
		currentInterestRepaid = activeLoan.InterestRepaidPoints
		dueAt = activeLoan.DueAt
		metadata = activeLoan.Metadata
		status = activeLoan.Status
		grantedAt = activeLoan.GrantedAt
		settledAt = activeLoan.SettledAt
	}
	if currentPrincipal+req.Points > maxPrincipal {
		c.JSON(http.StatusBadRequest, errorResponse{Error: fmt.Sprintf("borrow limit exceeded: max %d, current %d, requested %d", maxPrincipal, currentPrincipal, req.Points)})
		return
	}

	tx, err := s.pool.Begin(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	defer rollbackTx(c, tx)
	qtx := s.queries.WithTx(tx)

	occurrence, err := qtx.CreateActivityOccurrence(c.Request.Context(), db.CreateActivityOccurrenceParams{
		ActivityID:     loanActivity.ID,
		OccurrenceType: occurrenceTypeLoanIssued,
		Name:           fmt.Sprintf("Loan Shark loan — %s (+%d)", participant.Name, req.Points),
		EffectiveAt:    optionalTime(now),
		Status:         "resolved",
		Metadata:       []byte("{}"),
	})
	if err != nil {
		c.JSON(statusFromPg(err), errorResponse{Error: err.Error()})
		return
	}
	if _, err := qtx.CreateBonusPointLedgerEntry(c.Request.Context(), db.CreateBonusPointLedgerEntryParams{
		InstanceID:           toPGUUID(instanceID),
		ParticipantID:        participant.ID,
		ActivityOccurrenceID: occurrence.ID,
		EntryKind:            "award",
		Points:               req.Points,
		Visibility:           "secret",
		Reason:               fmt.Sprintf("Loan Shark loan for %d points", req.Points),
		EffectiveAt:          optionalTime(now),
		AwardKey:             optionalText(ptrString("loan_shark:borrow:" + uuid.NewString())),
		Metadata:             []byte("{}"),
	}); err != nil {
		c.JSON(statusFromPg(err), errorResponse{Error: err.Error()})
		return
	}

	if hasActiveLoan {
		if _, err := qtx.UpdateParticipantLoan(c.Request.Context(), db.UpdateParticipantLoanParams{
			ID:                    loanID,
			Status:                status,
			PrincipalPoints:       currentPrincipal + req.Points,
			InterestPoints:        currentInterest,
			PrincipalRepaidPoints: currentPrincipalRepaid,
			InterestRepaidPoints:  currentInterestRepaid,
			DueAt:                 dueAt,
			SettledAt:             settledAt,
			Metadata:              metadata,
		}); err != nil {
			c.JSON(statusFromPg(err), errorResponse{Error: err.Error()})
			return
		}
	} else {
		if _, err := qtx.CreateParticipantLoan(c.Request.Context(), db.CreateParticipantLoanParams{
			InstanceID:            toPGUUID(instanceID),
			ParticipantID:         participant.ID,
			ActivityID:            loanActivity.ID,
			Status:                "active",
			PrincipalPoints:       req.Points,
			InterestPoints:        interestPoints,
			PrincipalRepaidPoints: 0,
			InterestRepaidPoints:  0,
			GrantedAt:             grantedAt,
			DueAt:                 dueAt,
			SettledAt:             pgtype.Timestamptz{},
			Metadata:              []byte("{}"),
		}); err != nil {
			c.JSON(statusFromPg(err), errorResponse{Error: err.Error()})
			return
		}
	}

	if err := tx.Commit(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	statusPayload, err := s.loanStatusPayload(c.Request.Context(), toPGUUID(instanceID), participant.ID, now)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"participant": participantSummaryToJSON(participant.ID, participant.Name), "loan": statusPayload})
}

func (s *Server) repayLoanShark(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok {
		return
	}
	participant, ok := s.requireLinkedParticipant(c, instanceID)
	if !ok {
		return
	}

	var req loanSharkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	if req.Points <= 0 {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "points must be positive"})
		return
	}

	now := time.Now().UTC()
	activeLoan, hasActiveLoan, err := s.activeLoan(c.Request.Context(), s.queries, toPGUUID(instanceID), participant.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	if !hasActiveLoan {
		c.JSON(http.StatusNotFound, errorResponse{Error: "no active loan found"})
		return
	}
	balance, err := s.currentBonusBalance(c.Request.Context(), s.queries, toPGUUID(instanceID), participant.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	if balance < req.Points {
		c.JSON(http.StatusBadRequest, errorResponse{Error: fmt.Sprintf("insufficient bonus points: have %d, need %d", balance, req.Points)})
		return
	}

	interestOutstanding := activeLoan.InterestPoints - activeLoan.InterestRepaidPoints
	principalOutstanding := activeLoan.PrincipalPoints - activeLoan.PrincipalRepaidPoints
	totalOutstanding := interestOutstanding + principalOutstanding
	if totalOutstanding <= 0 {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "loan is already fully repaid"})
		return
	}
	if req.Points > totalOutstanding {
		c.JSON(http.StatusBadRequest, errorResponse{Error: fmt.Sprintf("repayment exceeds outstanding balance of %d", totalOutstanding)})
		return
	}
	interestApplied := req.Points
	if interestApplied > interestOutstanding {
		interestApplied = interestOutstanding
	}
	principalApplied := req.Points - interestApplied
	newInterestRepaid := activeLoan.InterestRepaidPoints + interestApplied
	newPrincipalRepaid := activeLoan.PrincipalRepaidPoints + principalApplied
	newStatus := "active"
	settledAt := pgtype.Timestamptz{}
	if newInterestRepaid >= activeLoan.InterestPoints && newPrincipalRepaid >= activeLoan.PrincipalPoints {
		newStatus = "repaid"
		settledAt = optionalTime(now)
	}

	loanActivity, err := s.ensureSystemActivity(c.Request.Context(), s.queries, toPGUUID(instanceID), activityTypeLoanShark, "Loan Shark", now)
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
	occurrence, err := qtx.CreateActivityOccurrence(c.Request.Context(), db.CreateActivityOccurrenceParams{
		ActivityID:     loanActivity.ID,
		OccurrenceType: occurrenceTypeLoanRepayment,
		Name:           fmt.Sprintf("Loan Shark repayment — %s (-%d)", participant.Name, req.Points),
		EffectiveAt:    optionalTime(now),
		Status:         "resolved",
		Metadata:       []byte("{}"),
	})
	if err != nil {
		c.JSON(statusFromPg(err), errorResponse{Error: err.Error()})
		return
	}
	ledgerMetadata := metadataWithConsumesSecretBalance([]byte("{}"), false)
	revealedSecretPoints, err := s.revealSecretPointsOnSpend(c.Request.Context(), qtx, toPGUUID(instanceID), participant.ID, occurrence.ID, pgtype.UUID{}, req.Points, now, fmt.Sprintf("Loan Shark repayment of %d points", req.Points), []byte("{}"))
	if err != nil {
		c.JSON(statusFromPg(err), errorResponse{Error: err.Error()})
		return
	}
	if _, err := qtx.CreateBonusPointLedgerEntry(c.Request.Context(), db.CreateBonusPointLedgerEntryParams{
		InstanceID:           toPGUUID(instanceID),
		ParticipantID:        participant.ID,
		ActivityOccurrenceID: occurrence.ID,
		EntryKind:            "spend",
		Points:               -req.Points,
		Visibility:           "secret",
		Reason:               fmt.Sprintf("Loan Shark repayment of %d points", req.Points),
		EffectiveAt:          optionalTime(now),
		AwardKey:             optionalText(ptrString("loan_shark:repay:" + uuid.NewString())),
		Metadata:             ledgerMetadata,
	}); err != nil {
		c.JSON(statusFromPg(err), errorResponse{Error: err.Error()})
		return
	}
	if _, err := qtx.UpdateParticipantLoan(c.Request.Context(), db.UpdateParticipantLoanParams{
		ID:                    activeLoan.ID,
		Status:                newStatus,
		PrincipalPoints:       activeLoan.PrincipalPoints,
		InterestPoints:        activeLoan.InterestPoints,
		PrincipalRepaidPoints: newPrincipalRepaid,
		InterestRepaidPoints:  newInterestRepaid,
		DueAt:                 activeLoan.DueAt,
		SettledAt:             settledAt,
		Metadata:              activeLoan.Metadata,
	}); err != nil {
		c.JSON(statusFromPg(err), errorResponse{Error: err.Error()})
		return
	}
	if err := tx.Commit(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	statusPayload, err := s.loanStatusPayload(c.Request.Context(), toPGUUID(instanceID), participant.ID, now)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"participant": participantSummaryToJSON(participant.ID, participant.Name), "loan": statusPayload, "revealed_secret_points": revealedSecretPoints})
}

func (s *Server) recordIndividualPonyImmunity(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok {
		return
	}
	if !s.requireInstanceAdminRequest(c, instanceID) {
		return
	}

	var req recordIndividualPonyImmunityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	contestantID, err := uuid.Parse(req.ContestantID)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid contestant_id"})
		return
	}
	contestant, err := s.requireContestant(c.Request.Context(), toPGUUID(instanceID), contestantID)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		c.JSON(status, errorResponse{Error: err.Error()})
		return
	}
	effectiveAt := time.Now().UTC()
	if req.EffectiveAt != nil {
		effectiveAt = req.EffectiveAt.UTC()
	}

	tx, err := s.pool.Begin(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	defer rollbackTx(c, tx)
	qtx := s.queries.WithTx(tx)
	activity, err := s.ensureSystemActivity(c.Request.Context(), qtx, toPGUUID(instanceID), activityTypeIndividualPony, "Individual Pony", effectiveAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	metadata, err := json.Marshal(individualPonyOccurrenceMetadata{WinningContestantID: contestantID.String()})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	occurrence, err := qtx.CreateActivityOccurrence(c.Request.Context(), db.CreateActivityOccurrenceParams{
		ActivityID:     activity.ID,
		OccurrenceType: occurrenceTypeIndividualPony,
		Name:           fmt.Sprintf("%s Individual Immunity", contestant.Name),
		EffectiveAt:    optionalTime(effectiveAt),
		Status:         "recorded",
		Metadata:       metadata,
	})
	if err != nil {
		c.JSON(statusFromPg(err), errorResponse{Error: err.Error()})
		return
	}
	createdEntries, err := gameplay.NewService(qtx).ResolveActivityOccurrence(c.Request.Context(), occurrence.ID)
	if err != nil {
		c.JSON(statusFromPg(err), errorResponse{Error: err.Error()})
		return
	}
	if _, err := qtx.UpdateActivityOccurrenceStatusAndMetadata(c.Request.Context(), db.UpdateActivityOccurrenceStatusAndMetadataParams{
		ID:       occurrence.ID,
		Status:   "resolved",
		EndsAt:   optionalTime(effectiveAt),
		Metadata: metadata,
	}); err != nil {
		c.JSON(statusFromPg(err), errorResponse{Error: err.Error()})
		return
	}
	if err := tx.Commit(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	entriesJSON := make([]gin.H, 0, len(createdEntries))
	for _, entry := range createdEntries {
		entriesJSON = append(entriesJSON, gin.H{
			"id":                     pgUUIDString(entry.ID),
			"instance_id":            pgUUIDString(entry.InstanceID),
			"participant_id":         pgUUIDString(entry.ParticipantID),
			"activity_occurrence_id": pgUUIDString(entry.ActivityOccurrenceID),
			"source_group_id":        pgUUIDPointer(entry.SourceGroupID),
			"entry_kind":             entry.EntryKind,
			"points":                 entry.Points,
			"visibility":             entry.Visibility,
			"reason":                 entry.Reason,
			"effective_at":           formatTimestamp(entry.EffectiveAt),
			"award_key":              pgTextPointer(entry.AwardKey),
			"metadata":               json.RawMessage(entry.Metadata),
			"created_at":             formatTimestamp(entry.CreatedAt),
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"contestant":      gin.H{"id": contestant.ID.String(), "name": contestant.Name},
		"occurrence_id":   pgUUIDString(occurrence.ID),
		"created_count":   len(entriesJSON),
		"created_entries": entriesJSON,
	})
}

func (s *Server) requireLinkedParticipant(c *gin.Context, instanceID uuid.UUID) (db.GetParticipantByDiscordUserIDRow, bool) {
	discordUserID := discordUserIDFromRequest(c.Request)
	if strings.TrimSpace(discordUserID) == "" {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "missing discord user id"})
		return db.GetParticipantByDiscordUserIDRow{}, false
	}
	participant, err := s.queries.GetParticipantByDiscordUserID(c.Request.Context(), db.GetParticipantByDiscordUserIDParams{
		InstanceID:    toPGUUID(instanceID),
		DiscordUserID: pgtype.Text{String: discordUserID, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, errorResponse{Error: "participant not linked"})
			return db.GetParticipantByDiscordUserIDRow{}, false
		}
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return db.GetParticipantByDiscordUserIDRow{}, false
	}
	return participant, true
}

func linkedParticipantRowToParticipant(row db.GetParticipantByDiscordUserIDRow) db.GetParticipantRow {
	return db.GetParticipantRow(row)
}

func (s *Server) resolveRequestedOrLinkedParticipant(c *gin.Context, instanceID uuid.UUID, requestedParticipantID string) (db.GetParticipantRow, bool) {
	if strings.TrimSpace(requestedParticipantID) == "" {
		participant, ok := s.requireLinkedParticipant(c, instanceID)
		if !ok {
			return db.GetParticipantRow{}, false
		}
		return linkedParticipantRowToParticipant(participant), true
	}
	if !s.requireInstanceAdminRequest(c, instanceID) {
		return db.GetParticipantRow{}, false
	}
	participantID, err := uuid.Parse(strings.TrimSpace(requestedParticipantID))
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid participant_id"})
		return db.GetParticipantRow{}, false
	}
	participant, err := s.queries.GetParticipant(c.Request.Context(), toPGUUID(participantID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, errorResponse{Error: "participant not found"})
			return db.GetParticipantRow{}, false
		}
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return db.GetParticipantRow{}, false
	}
	if participant.InstanceID != toPGUUID(instanceID) {
		c.JSON(http.StatusNotFound, errorResponse{Error: "participant not found"})
		return db.GetParticipantRow{}, false
	}
	return participant, true
}

func (s *Server) requireInstanceAdminRequest(c *gin.Context, instanceID uuid.UUID) bool {
	discordUserID := discordUserIDFromRequest(c.Request)
	if strings.TrimSpace(discordUserID) == "" {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "missing discord user id"})
		return false
	}
	isAdmin, err := s.isInstanceAdmin(c.Request.Context(), toPGUUID(instanceID), discordUserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return false
	}
	if !isAdmin {
		c.JSON(http.StatusForbidden, errorResponse{Error: "forbidden"})
		return false
	}
	return true
}

func (s *Server) resolveTribeByName(ctx context.Context, q *db.Queries, instanceID pgtype.UUID, raw string) (db.ListParticipantGroupsByInstanceRow, bool, error) {
	groups, err := q.ListParticipantGroupsByInstance(ctx, instanceID)
	if err != nil {
		return db.ListParticipantGroupsByInstanceRow{}, false, err
	}
	query := strings.TrimSpace(raw)
	exactMatches := make([]db.ListParticipantGroupsByInstanceRow, 0, 1)
	containsMatches := make([]db.ListParticipantGroupsByInstanceRow, 0, 1)
	for _, group := range groups {
		if !strings.EqualFold(strings.TrimSpace(group.Kind), "tribe") {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(group.Name), query) {
			exactMatches = append(exactMatches, group)
			continue
		}
		if matchesContainsFold(group.Name, query) {
			containsMatches = append(containsMatches, group)
		}
	}
	if len(exactMatches) == 1 {
		return exactMatches[0], true, nil
	}
	if len(exactMatches) > 1 {
		return db.ListParticipantGroupsByInstanceRow{}, false, fmt.Errorf("tribe %q is ambiguous", raw)
	}
	if len(containsMatches) == 1 {
		return containsMatches[0], true, nil
	}
	if len(containsMatches) > 1 {
		return db.ListParticipantGroupsByInstanceRow{}, false, fmt.Errorf("tribe %q is ambiguous", raw)
	}
	return db.ListParticipantGroupsByInstanceRow{}, false, nil
}

func (s *Server) nextEpisodeTarget(ctx context.Context, q *db.Queries, instanceID pgtype.UUID, now time.Time) (mergeTargetEpisodeMetadata, error) {
	episodes, err := q.ListInstanceEpisodes(ctx, instanceID)
	if err != nil {
		return mergeTargetEpisodeMetadata{}, fmt.Errorf("list instance episodes: %w", err)
	}
	if len(episodes) == 0 {
		return mergeTargetEpisodeMetadata{}, fmt.Errorf("instance has no configured episodes")
	}
	sort.Slice(episodes, func(i, j int) bool {
		if episodes[i].AirsAt.Time.Equal(episodes[j].AirsAt.Time) {
			return episodes[i].EpisodeNumber < episodes[j].EpisodeNumber
		}
		return episodes[i].AirsAt.Time.Before(episodes[j].AirsAt.Time)
	})
	for _, episode := range episodes {
		if !episode.AirsAt.Time.After(now) {
			continue
		}
		return mergeTargetEpisodeMetadata{
			EpisodeID:     pgUUIDString(episode.ID),
			EpisodeNumber: episode.EpisodeNumber,
			EpisodeLabel:  episode.Label,
			EpisodeAirsAt: episode.AirsAt.Time.Format(time.RFC3339),
		}, nil
	}
	return mergeTargetEpisodeMetadata{}, fmt.Errorf("no next episode is configured after the current episode")
}

func (s *Server) ensureSystemActivity(ctx context.Context, q *db.Queries, instanceID pgtype.UUID, activityType, name string, startsAt time.Time) (db.ListInstanceActivitiesByTypeRow, error) {
	activities, err := q.ListInstanceActivitiesByType(ctx, db.ListInstanceActivitiesByTypeParams{InstanceID: instanceID, ActivityType: activityType})
	if err != nil {
		return db.ListInstanceActivitiesByTypeRow{}, err
	}
	for _, activity := range activities {
		if strings.EqualFold(strings.TrimSpace(activity.Name), strings.TrimSpace(name)) {
			return activity, nil
		}
	}
	created, err := q.CreateInstanceActivity(ctx, db.CreateInstanceActivityParams{
		InstanceID:   instanceID,
		ActivityType: activityType,
		Name:         name,
		Status:       "active",
		StartsAt:     optionalTime(startsAt),
		Metadata:     []byte("{}"),
	})
	if err != nil {
		return db.ListInstanceActivitiesByTypeRow{}, err
	}
	return db.ListInstanceActivitiesByTypeRow(created), nil
}

func (s *Server) primarySystemActivity(ctx context.Context, q *db.Queries, instanceID pgtype.UUID, activityType string) (*db.ListInstanceActivitiesByTypeRow, error) {
	activities, err := q.ListInstanceActivitiesByType(ctx, db.ListInstanceActivitiesByTypeParams{InstanceID: instanceID, ActivityType: activityType})
	if err != nil {
		return nil, err
	}
	if len(activities) == 0 {
		return nil, pgx.ErrNoRows
	}
	activity := activities[len(activities)-1]
	return &activity, nil
}

type auctionLotView struct {
	Occurrence     db.ListActivityOccurrencesByActivityAndStatusRow
	ContestantID   uuid.UUID
	ContestantName string
	Metadata       []byte
}

func (s *Server) listOpenAuctionLots(ctx context.Context, q *db.Queries, instanceID pgtype.UUID) (*db.ListInstanceActivitiesByTypeRow, []auctionLotView, error) {
	activity, err := s.primarySystemActivity(ctx, q, instanceID, activityTypeIndividualPonyAuction)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	occurrences, err := q.ListActivityOccurrencesByActivityAndStatus(ctx, db.ListActivityOccurrencesByActivityAndStatusParams{ActivityID: activity.ID, Status: "recorded"})
	if err != nil {
		return nil, nil, err
	}
	contestants, err := q.ListContestantsByInstance(ctx, instanceID)
	if err != nil {
		return nil, nil, err
	}
	contestantNames := make(map[string]string, len(contestants))
	for _, contestant := range contestants {
		contestantNames[pgUUIDString(contestant.ID)] = contestant.Name
	}
	lots := make([]auctionLotView, 0, len(occurrences))
	for _, occurrence := range occurrences {
		if occurrence.OccurrenceType != occurrenceTypeAuctionLot {
			continue
		}
		var metadata auctionLotMetadata
		if err := json.Unmarshal(nonEmptyMetadata(occurrence.Metadata), &metadata); err != nil {
			continue
		}
		contestantID, err := uuid.Parse(strings.TrimSpace(metadata.ContestantID))
		if err != nil {
			continue
		}
		contestantName := contestantNames[contestantID.String()]
		lots = append(lots, auctionLotView{Occurrence: occurrence, ContestantID: contestantID, ContestantName: contestantName, Metadata: occurrence.Metadata})
	}
	sort.Slice(lots, func(i, j int) bool {
		return strings.ToLower(lots[i].ContestantName) < strings.ToLower(lots[j].ContestantName)
	})
	return activity, lots, nil
}

func (s *Server) findOpenAuctionLotByContestant(ctx context.Context, q *db.Queries, activityID pgtype.UUID, contestantID uuid.UUID) (db.ListActivityOccurrencesByActivityAndStatusRow, bool, error) {
	occurrences, err := q.ListActivityOccurrencesByActivityAndStatus(ctx, db.ListActivityOccurrencesByActivityAndStatusParams{ActivityID: activityID, Status: "recorded"})
	if err != nil {
		return db.ListActivityOccurrencesByActivityAndStatusRow{}, false, err
	}
	for _, occurrence := range occurrences {
		if occurrence.OccurrenceType != occurrenceTypeAuctionLot {
			continue
		}
		var metadata auctionLotMetadata
		if err := json.Unmarshal(nonEmptyMetadata(occurrence.Metadata), &metadata); err != nil {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(metadata.ContestantID), contestantID.String()) {
			return occurrence, true, nil
		}
	}
	return db.ListActivityOccurrencesByActivityAndStatusRow{}, false, nil
}

func (s *Server) findOpenStirThePotRound(ctx context.Context, q *db.Queries, instanceID pgtype.UUID, at time.Time) (db.ListActivityOccurrencesByActivityAndStatusRow, db.ListInstanceActivitiesByTypeRow, bool, error) {
	activity, err := s.primarySystemActivity(ctx, q, instanceID, activityTypeStirThePot)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.ListActivityOccurrencesByActivityAndStatusRow{}, db.ListInstanceActivitiesByTypeRow{}, false, nil
		}
		return db.ListActivityOccurrencesByActivityAndStatusRow{}, db.ListInstanceActivitiesByTypeRow{}, false, err
	}
	occurrences, err := q.ListActivityOccurrencesByActivityAndStatus(ctx, db.ListActivityOccurrencesByActivityAndStatusParams{ActivityID: activity.ID, Status: "recorded"})
	if err != nil {
		return db.ListActivityOccurrencesByActivityAndStatusRow{}, db.ListInstanceActivitiesByTypeRow{}, false, err
	}
	for index := len(occurrences) - 1; index >= 0; index-- {
		occurrence := occurrences[index]
		if occurrence.OccurrenceType != occurrenceTypeStirThePotRound {
			continue
		}
		if occurrence.EffectiveAt.Time.After(at) {
			continue
		}
		return occurrence, *activity, true, nil
	}
	return db.ListActivityOccurrencesByActivityAndStatusRow{}, db.ListInstanceActivitiesByTypeRow{}, false, nil
}

func (s *Server) participantContributionPoints(ctx context.Context, q *db.Queries, occurrenceID, participantID pgtype.UUID) (int32, error) {
	row, err := q.GetActivityOccurrenceParticipant(ctx, db.GetActivityOccurrenceParticipantParams{ActivityOccurrenceID: occurrenceID, ParticipantID: participantID, Role: occurrenceRoleStirThePotContributor})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	metadata, err := parseStirThePotContributionMetadata(row.Metadata)
	if err != nil {
		return 0, err
	}
	return metadata.Contribution, nil
}

func (s *Server) groupContributionPoints(ctx context.Context, q *db.Queries, occurrenceID, groupID pgtype.UUID) (int32, error) {
	participants, err := q.ListActivityOccurrenceParticipants(ctx, occurrenceID)
	if err != nil {
		return 0, err
	}
	groupIDString := pgUUIDString(groupID)
	var total int32
	for _, participant := range participants {
		if participant.Role != occurrenceRoleStirThePotContributor || !participant.ParticipantGroupID.Valid || pgUUIDString(participant.ParticipantGroupID) != groupIDString {
			continue
		}
		metadata, err := parseStirThePotContributionMetadata(participant.Metadata)
		if err != nil {
			return 0, err
		}
		total += metadata.Contribution
	}
	return total, nil
}

func (s *Server) participantBidPoints(ctx context.Context, q *db.Queries, occurrenceID, participantID pgtype.UUID) (int32, error) {
	row, err := q.GetActivityOccurrenceParticipant(ctx, db.GetActivityOccurrenceParticipantParams{ActivityOccurrenceID: occurrenceID, ParticipantID: participantID, Role: occurrenceRoleAuctionBidder})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	var metadata auctionBidMetadata
	if err := json.Unmarshal(nonEmptyMetadata(row.Metadata), &metadata); err != nil {
		return 0, err
	}
	return metadata.BidPoints, nil
}

func (s *Server) currentBonusBalance(ctx context.Context, q *db.Queries, instanceID, participantID pgtype.UUID) (int32, error) {
	visible, err := q.GetVisibleBonusTotalByParticipant(ctx, db.GetVisibleBonusTotalByParticipantParams{InstanceID: instanceID, ParticipantID: participantID})
	if err != nil {
		return 0, err
	}
	secret, err := q.GetSecretBonusTotalByParticipant(ctx, db.GetSecretBonusTotalByParticipantParams{InstanceID: instanceID, ParticipantID: participantID})
	if err != nil {
		return 0, err
	}
	return visible + secret, nil
}

func (s *Server) revealSecretPointsOnSpend(ctx context.Context, q *db.Queries, instanceID, participantID, activityOccurrenceID, sourceGroupID pgtype.UUID, spendPoints int32, now time.Time, reason string, metadata []byte) (int32, error) {
	if spendPoints <= 0 {
		return 0, nil
	}
	availableSecret, err := q.GetAvailableSecretBalanceByParticipant(ctx, db.GetAvailableSecretBalanceByParticipantParams{
		InstanceID:    instanceID,
		ParticipantID: participantID,
	})
	if err != nil {
		return 0, err
	}
	revealedPoints := spendPoints
	if revealedPoints > availableSecret {
		revealedPoints = availableSecret
	}
	if revealedPoints <= 0 {
		return 0, nil
	}
	revealMetadata, err := json.Marshal(map[string]any{
		"points": revealedPoints,
		"reason": reason,
	})
	if err != nil {
		return 0, err
	}
	if len(metadata) > 0 && string(metadata) != "{}" {
		revealMetadata = metadata
	}
	if _, err := q.CreateBonusPointLedgerEntry(ctx, db.CreateBonusPointLedgerEntryParams{
		InstanceID:           instanceID,
		ParticipantID:        participantID,
		ActivityOccurrenceID: activityOccurrenceID,
		SourceGroupID:        sourceGroupID,
		EntryKind:            "conversion",
		Points:               -revealedPoints,
		Visibility:           "secret",
		Reason:               fmt.Sprintf("Revealed %d secret bonus point(s) for %s", revealedPoints, reason),
		EffectiveAt:          optionalTime(now),
		AwardKey:             optionalText(ptrString("secret:reveal:debit:" + uuid.NewString())),
		Metadata:             revealMetadata,
	}); err != nil {
		return 0, err
	}
	if _, err := q.CreateBonusPointLedgerEntry(ctx, db.CreateBonusPointLedgerEntryParams{
		InstanceID:           instanceID,
		ParticipantID:        participantID,
		ActivityOccurrenceID: activityOccurrenceID,
		SourceGroupID:        sourceGroupID,
		EntryKind:            "reveal",
		Points:               revealedPoints,
		Visibility:           "revealed",
		Reason:               fmt.Sprintf("Revealed %d secret bonus point(s) for %s", revealedPoints, reason),
		EffectiveAt:          optionalTime(now),
		AwardKey:             optionalText(ptrString("secret:reveal:credit:" + uuid.NewString())),
		Metadata:             revealMetadata,
	}); err != nil {
		return 0, err
	}
	return revealedPoints, nil
}

func metadataWithConsumesSecretBalance(raw []byte, consumes bool) []byte {
	payload := map[string]any{}
	if len(raw) > 0 && string(raw) != "{}" {
		if err := json.Unmarshal(raw, &payload); err != nil {
			payload = map[string]any{}
		}
	}
	payload["consumes_secret_balance"] = consumes
	encoded, err := json.Marshal(payload)
	if err != nil {
		return raw
	}
	return encoded
}

func (s *Server) currentTribeMembership(ctx context.Context, q *db.Queries, participantID pgtype.UUID, at time.Time) (db.ListActiveParticipantMembershipsAtRow, error) {
	memberships, err := q.ListActiveParticipantMembershipsAt(ctx, db.ListActiveParticipantMembershipsAtParams{ParticipantID: participantID, At: optionalTime(at)})
	if err != nil {
		return db.ListActiveParticipantMembershipsAtRow{}, err
	}
	tribes := make([]db.ListActiveParticipantMembershipsAtRow, 0, len(memberships))
	for _, membership := range memberships {
		if strings.EqualFold(strings.TrimSpace(membership.ParticipantGroupKind), "tribe") {
			tribes = append(tribes, membership)
		}
	}
	if len(tribes) == 0 {
		return db.ListActiveParticipantMembershipsAtRow{}, fmt.Errorf("participant does not currently belong to a tribe")
	}
	if len(tribes) > 1 {
		return db.ListActiveParticipantMembershipsAtRow{}, fmt.Errorf("participant belongs to multiple active tribes")
	}
	return tribes[0], nil
}

func (s *Server) requireContestant(ctx context.Context, instanceID pgtype.UUID, contestantID uuid.UUID) (db.GetContestantRow, error) {
	exists, err := s.queries.InstanceHasContestant(ctx, db.InstanceHasContestantParams{InstanceID: instanceID, ContestantID: toPGUUID(contestantID)})
	if err != nil {
		return db.GetContestantRow{}, err
	}
	if !exists {
		return db.GetContestantRow{}, fmt.Errorf("contestant not found in this instance")
	}
	return s.queries.GetContestant(ctx, toPGUUID(contestantID))
}

func (s *Server) rankAuctionBids(ctx context.Context, q *db.Queries, occurrenceID pgtype.UUID) ([]auctionRankedBid, error) {
	participants, err := q.ListActivityOccurrenceParticipants(ctx, occurrenceID)
	if err != nil {
		return nil, err
	}
	ranked := make([]auctionRankedBid, 0, len(participants))
	for _, participant := range participants {
		if participant.Role != occurrenceRoleAuctionBidder {
			continue
		}
		var metadata auctionBidMetadata
		if err := json.Unmarshal(nonEmptyMetadata(participant.Metadata), &metadata); err != nil {
			return nil, err
		}
		if metadata.BidPoints <= 0 {
			continue
		}
		ranked = append(ranked, auctionRankedBid{participantID: participant.ParticipantID, participantName: participant.ParticipantName, bidPoints: metadata.BidPoints, createdAt: participant.CreatedAt.Time})
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].bidPoints != ranked[j].bidPoints {
			return ranked[i].bidPoints > ranked[j].bidPoints
		}
		if !ranked[i].createdAt.Equal(ranked[j].createdAt) {
			return ranked[i].createdAt.Before(ranked[j].createdAt)
		}
		return strings.ToLower(ranked[i].participantName) < strings.ToLower(ranked[j].participantName)
	})
	return ranked, nil
}

func (s *Server) activeLoan(ctx context.Context, q *db.Queries, instanceID, participantID pgtype.UUID) (db.GetActiveParticipantLoanByParticipantRow, bool, error) {
	loan, err := q.GetActiveParticipantLoanByParticipant(ctx, db.GetActiveParticipantLoanByParticipantParams{InstanceID: instanceID, ParticipantID: participantID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.GetActiveParticipantLoanByParticipantRow{}, false, nil
		}
		return db.GetActiveParticipantLoanByParticipantRow{}, false, err
	}
	return loan, true, nil
}

func (s *Server) loanTermsForParticipant(ctx context.Context, instanceID, participantID pgtype.UUID, at time.Time) (int32, int32, error) {
	maxPrincipal := int32(3)
	interestPoints := int32(1)
	advantages, err := s.queries.ListActiveAdvantagesByTypeForParticipant(ctx, db.ListActiveAdvantagesByTypeForParticipantParams{InstanceID: instanceID, ParticipantID: participantID, AdvantageType: activityTypeLoanShark, At: optionalTime(at)})
	if err != nil {
		return 0, 0, err
	}
	if len(advantages) > 0 {
		maxPrincipal = 4
		interestPoints = 0
	}
	return maxPrincipal, interestPoints, nil
}

func (s *Server) loanDueAt(ctx context.Context, instanceID pgtype.UUID, fallback time.Time) time.Time {
	episodes, err := s.queries.ListInstanceEpisodes(ctx, instanceID)
	if err != nil || len(episodes) == 0 {
		return fallback.AddDate(0, 1, 0)
	}
	latest := episodes[len(episodes)-1].AirsAt.Time
	for _, episode := range episodes {
		if episode.AirsAt.Time.After(latest) {
			latest = episode.AirsAt.Time
		}
	}
	return latest
}

func (s *Server) loanStatusPayload(ctx context.Context, instanceID, participantID pgtype.UUID, at time.Time) (gin.H, error) {
	maxPrincipal, defaultInterest, err := s.loanTermsForParticipant(ctx, instanceID, participantID, at)
	if err != nil {
		return nil, err
	}
	loan, hasActiveLoan, err := s.activeLoan(ctx, s.queries, instanceID, participantID)
	if err != nil {
		return nil, err
	}
	balance, err := s.currentBonusBalance(ctx, s.queries, instanceID, participantID)
	if err != nil {
		return nil, err
	}
	response := gin.H{
		"has_active_loan":        hasActiveLoan,
		"max_principal_points":   maxPrincipal,
		"interest_points":        defaultInterest,
		"bonus_points_available": balance,
	}
	if !hasActiveLoan {
		response["principal_points"] = 0
		response["principal_repaid_points"] = 0
		response["interest_repaid_points"] = 0
		response["total_due_points"] = 0
		response["remaining_borrow_points"] = maxPrincipal
		return response, nil
	}
	principalOutstanding := loan.PrincipalPoints - loan.PrincipalRepaidPoints
	interestOutstanding := loan.InterestPoints - loan.InterestRepaidPoints
	response["loan_id"] = pgUUIDString(loan.ID)
	response["status"] = loan.Status
	response["principal_points"] = loan.PrincipalPoints
	response["interest_points"] = loan.InterestPoints
	response["principal_repaid_points"] = loan.PrincipalRepaidPoints
	response["interest_repaid_points"] = loan.InterestRepaidPoints
	response["principal_outstanding_points"] = principalOutstanding
	response["interest_outstanding_points"] = interestOutstanding
	response["total_due_points"] = principalOutstanding + interestOutstanding
	response["remaining_borrow_points"] = maxPrincipal - loan.PrincipalPoints
	response["granted_at"] = formatTimestamp(loan.GrantedAt)
	response["due_at"] = formatTimestamp(loan.DueAt)
	response["activity_id"] = pgUUIDPointer(loan.ActivityID)
	return response, nil
}

func rollbackTx(c *gin.Context, tx pgx.Tx) {
	if tx == nil {
		return
	}
	rollbackErr := tx.Rollback(c.Request.Context())
	if rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
		if ginErr := c.Error(rollbackErr); ginErr != nil {
			ginErr.Type = gin.ErrorTypePrivate
		}
	}
}

func defaultStirThePotRewardTiers() []stirThePotRewardTier {
	return []stirThePotRewardTier{{Contributions: 2, Bonus: 1}, {Contributions: 5, Bonus: 2}, {Contributions: 8, Bonus: 3}, {Contributions: 11, Bonus: 4}}
}

func stirThePotBonusForContribution(total int32, tiers []stirThePotRewardTier) int32 {
	var best int32
	for _, tier := range tiers {
		if total >= tier.Contributions && tier.Bonus > best {
			best = tier.Bonus
		}
	}
	return best
}

func parseStirThePotRoundMetadata(raw []byte) stirThePotRoundMetadata {
	var metadata stirThePotRoundMetadata
	if err := json.Unmarshal(nonEmptyMetadata(raw), &metadata); err != nil {
		return stirThePotRoundMetadata{}
	}
	return metadata
}

func parseStirThePotContributionMetadata(raw []byte) (stirThePotContributionMetadata, error) {
	var metadata stirThePotContributionMetadata
	if err := json.Unmarshal(nonEmptyMetadata(raw), &metadata); err != nil {
		return stirThePotContributionMetadata{}, err
	}
	return metadata, nil
}

func nonEmptyMetadata(raw []byte) []byte {
	if len(raw) == 0 {
		return []byte("{}")
	}
	return raw
}

func ptrString(value string) *string {
	return &value
}
