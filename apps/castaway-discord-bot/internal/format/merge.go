package format

import (
	"fmt"
	"strings"

	"github.com/bry-guy/srvivor/apps/castaway-discord-bot/internal/castaway"
	"github.com/bry-guy/srvivor/apps/castaway-discord-bot/internal/messages"
)

func StirThePotStatus(instance castaway.Instance, status castaway.StirThePotStatus) string {
	return TrimMessage(messages.Render("stir-the-pot-status.md", map[string]any{
		"Season": instance.Season, "Open": status.Open, "Round": status.Round.Name,
		"Mine": status.MyContributionPoints, "Bonus": status.BonusPointsAvailable,
		"Tiers": strings.Join(stirThePotRewardTierLabels(status.RewardTiers, false), ", "),
	}))
}

func StirThePotTribeStatus(instance castaway.Instance, status castaway.StirThePotTribeStatus) string {
	data := map[string]any{
		"Season": instance.Season, "Tribe": status.Tribe.Name, "Open": status.Open, "Round": status.Round.Name,
		"Contribution": status.ContributionPoints, "BonusIfNow": status.BonusPointsIfResolvedNow,
		"HasNext": false, "NextLabel": "", "NextNeeded": 0,
		"Tiers": strings.Join(stirThePotRewardTierLabels(status.RewardTiers, false), ", "),
	}
	if nextTier, pointsNeeded, ok := nextStirThePotTier(status.ContributionPoints, status.RewardTiers, false); ok {
		data["HasNext"] = true
		data["NextLabel"] = stirThePotRewardTierLabelForRound(nextTier, status.RewardTiers, false)
		data["NextNeeded"] = pointsNeeded
	}
	return TrimMessage(messages.Render("stir-the-pot-tribe-status.md", data))
}

func StirThePotContributionResult(instance castaway.Instance, result castaway.StirThePotContributionResult, self bool) string {
	return TrimMessage(messages.Fragment("stir-the-pot-contribution.md", map[string]any{
		"Season": instance.Season, "Self": self, "Added": result.AddedPoints, "Name": result.Participant.Name,
		"Mine": result.MyContributionPoints, "Bonus": result.BonusPointsAvailable,
	}))
}

func StirThePotStartResult(instance castaway.Instance, result castaway.StirThePotStartResult) string {
	return TrimMessage(messages.Fragment("stir-the-pot-start.md", map[string]any{
		"Season": instance.Season, "Round": result.Round.Name, "Activity": result.Activity.Name,
	}))
}

func StirThePotCloseResult(instance castaway.Instance, result castaway.StirThePotCloseResult, announcedCount int) string {
	return TrimMessage(messages.Fragment("stir-the-pot-close.md", map[string]any{
		"Season": instance.Season, "Round": result.Round.Name, "Tribes": len(result.Tribes), "Announced": max(announcedCount, 0),
	}))
}

func StirThePotCloseAnnouncement(tribeResult castaway.StirThePotClosedTribeResult) string {
	return TrimMessage(messages.Fragment("stir-the-pot-close-announcement.md", map[string]any{
		"Tribe": tribeResult.Tribe.Name, "Contribution": tribeResult.ContributionPoints,
		"Bonus": tribeResult.BonusPointsEarned, "Total": tribeResult.TotalPotentialPonyPoints,
	}))
}

func AuctionStatus(instance castaway.Instance, status castaway.AuctionStatus) string {
	return TrimMessage(messages.Render("auction-status.md", map[string]any{
		"Season": instance.Season, "Bonus": status.BonusPointsAvailable, "LoanDue": status.Loan.TotalDuePoints,
		"NoLots": !status.Open || len(status.OpenLots) == 0, "Lots": status.OpenLots, "Ponies": status.Ponies,
	}))
}

func AuctionBidResult(instance castaway.Instance, result castaway.AuctionBidResult, self bool) string {
	return TrimMessage(messages.Fragment("auction-bid.md", map[string]any{
		"Season": instance.Season, "Self": self, "Contestant": result.Contestant.Name,
		"Participant": result.Participant.Name, "Bid": result.MyBidPoints,
		"Previous": result.PreviousBidPoints, "Bonus": result.BonusPointsAvailable,
	}))
}

func AuctionLotStartResult(instance castaway.Instance, result castaway.AuctionLotStartResult) string {
	return TrimMessage(messages.Fragment("auction-lot-start.md", map[string]any{
		"Season": instance.Season, "Contestant": result.Contestant.Name, "Lot": result.Lot.Name,
	}))
}

func AuctionLotStopResult(instance castaway.Instance, result castaway.AuctionLotStopResult) string {
	winner := ""
	if result.Winner != nil {
		winner = result.Winner.ParticipantName
	}
	return TrimMessage(messages.Fragment("auction-lot-stop.md", map[string]any{
		"Season": instance.Season, "Contestant": result.Contestant.Name, "Price": result.PricePoints,
		"Winning": result.WinningBidPoints, "HasWinner": result.Winner != nil, "Winner": winner,
	}))
}

func BidList(instance castaway.Instance, status castaway.AuctionStatus) string {
	var bids []castaway.AuctionLotStatus
	for _, lot := range status.OpenLots {
		if lot.MyBidPoints > 0 {
			bids = append(bids, lot)
		}
	}
	return TrimMessage(messages.Render("bid-list.md", map[string]any{"Season": instance.Season, "Bids": bids}))
}

func PonyList(instance castaway.Instance, list castaway.PonyList) string {
	return TrimMessage(messages.Render("ponies.md", map[string]any{"Season": instance.Season, "Ponies": list.Ponies}))
}

func LoanStatus(instance castaway.Instance, response castaway.LoanStatusResponse) string {
	loan := response.Loan
	dueAt := ""
	if strings.TrimSpace(loan.DueAt) != "" {
		dueAt = formatTimeLong(loan.DueAt)
	}
	return TrimMessage(messages.Fragment("loan-status.md", map[string]any{
		"Season": instance.Season, "Principal": loan.PrincipalPoints, "Interest": loan.InterestPoints,
		"TotalDue": loan.TotalDuePoints, "Remaining": loan.RemainingBorrowPoints,
		"Bonus": loan.BonusPointsAvailable, "DueAt": dueAt,
	}))
}

// LoanActionResult takes the verb as rendered text (the loan-borrowed or loan-repaid template).
func LoanActionResult(instance castaway.Instance, response castaway.LoanStatusResponse, verb string, points int) string {
	return TrimMessage(messages.Fragment("loan-action.md", map[string]any{
		"Season": instance.Season, "Verb": verb, "Points": points, "TotalDue": response.Loan.TotalDuePoints,
		"Remaining": response.Loan.RemainingBorrowPoints, "Bonus": response.Loan.BonusPointsAvailable,
	}))
}

func IndividualPonyImmunityResult(instance castaway.Instance, result castaway.IndividualPonyImmunityResult) string {
	return TrimMessage(messages.Fragment("pony-immunity.md", map[string]any{
		"Season": instance.Season, "Name": result.Contestant.Name, "Created": result.CreatedCount,
	}))
}

func nextStirThePotTier(currentContribution int, tiers []castaway.StirThePotRewardTier, revealFinalTier bool) (castaway.StirThePotRewardTier, int, bool) {
	for _, tier := range tiers {
		if !revealFinalTier && isHiddenStirThePotTier(tier, tiers) {
			continue
		}
		if currentContribution < int(tier.Contributions) {
			return tier, int(tier.Contributions) - currentContribution, true
		}
	}
	return castaway.StirThePotRewardTier{}, 0, false
}

func stirThePotRewardTierLabels(tiers []castaway.StirThePotRewardTier, revealFinalTier bool) []string {
	parts := make([]string, 0, len(tiers))
	for _, tier := range tiers {
		parts = append(parts, stirThePotRewardTierLabelForRound(tier, tiers, revealFinalTier))
	}
	return parts
}

func stirThePotRewardTierLabelForRound(tier castaway.StirThePotRewardTier, tiers []castaway.StirThePotRewardTier, revealFinalTier bool) string {
	if !revealFinalTier && isHiddenStirThePotTier(tier, tiers) {
		return fmt.Sprintf("?→+%d", tier.Bonus)
	}
	return fmt.Sprintf("%d→+%d", tier.Contributions, tier.Bonus)
}

func isHiddenStirThePotTier(tier castaway.StirThePotRewardTier, tiers []castaway.StirThePotRewardTier) bool {
	if len(tiers) == 0 {
		return false
	}
	return tier.Bonus >= 4
}

func SecretRevealAnnouncement(participant castaway.Participant, revealedSecretPoints int) string {
	name := strings.TrimSpace(participant.Name)
	discordUserID := strings.TrimSpace(participant.DiscordUserID)
	if (name == "" && discordUserID == "") || revealedSecretPoints <= 0 {
		return ""
	}
	return messages.Fragment("secret-reveal.md", map[string]any{"ID": discordUserID, "Name": name, "Count": revealedSecretPoints})
}
