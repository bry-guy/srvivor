package format

import (
	"fmt"
	"strings"

	"github.com/bry-guy/srvivor/apps/castaway-discord-bot/internal/castaway"
)

func StirThePotStatus(instance castaway.Instance, status castaway.StirThePotStatus) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("**Season %d: Stir the Pot**\n", instance.Season))
	if !status.Open {
		builder.WriteString("Stir the Pot is not currently open.")
		return TrimMessage(builder.String())
	}
	builder.WriteString(fmt.Sprintf("- Round: %s\n", status.Round.Name))
	builder.WriteString(fmt.Sprintf("- Your contribution: %d\n", status.MyContributionPoints))
	builder.WriteString(fmt.Sprintf("- Bonus points available: %d\n", status.BonusPointsAvailable))
	if len(status.RewardTiers) > 0 {
		builder.WriteString("- Reward tiers: ")
		parts := make([]string, 0, len(status.RewardTiers))
		for _, tier := range status.RewardTiers {
			parts = append(parts, fmt.Sprintf("%d→+%d", tier.Contributions, tier.Bonus))
		}
		builder.WriteString(strings.Join(parts, ", "))
	}
	return TrimMessage(strings.TrimSpace(builder.String()))
}

func StirThePotContributionResult(instance castaway.Instance, result castaway.StirThePotContributionResult) string {
	return TrimMessage(strings.Join([]string{
		fmt.Sprintf("**Season %d: Stir the Pot**", instance.Season),
		fmt.Sprintf("Added %d points to the pot.", result.AddedPoints),
		fmt.Sprintf("- Your contribution: %d", result.MyContributionPoints),
		fmt.Sprintf("- Bonus points available: %d", result.BonusPointsAvailable),
	}, "\n"))
}

func StirThePotStartResult(instance castaway.Instance, result castaway.StirThePotStartResult) string {
	return TrimMessage(strings.Join([]string{
		fmt.Sprintf("**Season %d: Stir the Pot**", instance.Season),
		fmt.Sprintf("Opened %s.", result.Round.Name),
		fmt.Sprintf("- Activity: %s", result.Activity.Name),
	}, "\n"))
}

func AuctionStatus(instance castaway.Instance, status castaway.AuctionStatus) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("**Season %d: Individual Pony Auction**\n", instance.Season))
	builder.WriteString(fmt.Sprintf("- Bonus points available: %d\n", status.BonusPointsAvailable))
	builder.WriteString(fmt.Sprintf("- Loan due: %d\n", status.Loan.TotalDuePoints))
	if !status.Open || len(status.OpenLots) == 0 {
		builder.WriteString("\nNo open auction lots.")
	} else {
		builder.WriteString("\n**Open lots**\n")
		for _, lot := range status.OpenLots {
			builder.WriteString(fmt.Sprintf("- %s", lot.ContestantName))
			if lot.MyBidPoints > 0 {
				builder.WriteString(fmt.Sprintf(" — your bid: %d", lot.MyBidPoints))
			}
			builder.WriteString("\n")
		}
	}
	if len(status.Ponies) > 0 {
		builder.WriteString("\n**Your ponies**\n")
		for _, pony := range status.Ponies {
			builder.WriteString(fmt.Sprintf("- %s\n", pony.ContestantName))
		}
	}
	return TrimMessage(strings.TrimSpace(builder.String()))
}

func AuctionBidResult(instance castaway.Instance, result castaway.AuctionBidResult) string {
	return TrimMessage(strings.Join([]string{
		fmt.Sprintf("**Season %d: Bid submitted**", instance.Season),
		fmt.Sprintf("%s — your bid is now %d.", result.Contestant.Name, result.MyBidPoints),
		fmt.Sprintf("- Previous bid: %d", result.PreviousBidPoints),
		fmt.Sprintf("- Bonus points available: %d", result.BonusPointsAvailable),
	}, "\n"))
}

func AuctionLotStartResult(instance castaway.Instance, result castaway.AuctionLotStartResult) string {
	return TrimMessage(strings.Join([]string{
		fmt.Sprintf("**Season %d: Auction lot opened**", instance.Season),
		fmt.Sprintf("%s is now open for bidding.", result.Contestant.Name),
		fmt.Sprintf("- Lot: %s", result.Lot.Name),
	}, "\n"))
}

func AuctionLotStopResult(instance castaway.Instance, result castaway.AuctionLotStopResult) string {
	lines := []string{
		fmt.Sprintf("**Season %d: Auction lot resolved**", instance.Season),
		fmt.Sprintf("%s closed at %d points.", result.Contestant.Name, result.PricePoints),
		fmt.Sprintf("- Winning bid: %d", result.WinningBidPoints),
	}
	if result.Winner != nil {
		lines = append(lines, fmt.Sprintf("- Winner: %s", result.Winner.ParticipantName))
	} else {
		lines = append(lines, "- Winner: none")
	}
	return TrimMessage(strings.Join(lines, "\n"))
}

func BidList(instance castaway.Instance, status castaway.AuctionStatus) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("**Season %d: Your bids**\n", instance.Season))
	count := 0
	for _, lot := range status.OpenLots {
		if lot.MyBidPoints <= 0 {
			continue
		}
		count++
		builder.WriteString(fmt.Sprintf("- %s — %d\n", lot.ContestantName, lot.MyBidPoints))
	}
	if count == 0 {
		builder.WriteString("No active bids.")
	}
	return TrimMessage(strings.TrimSpace(builder.String()))
}

func PonyList(instance castaway.Instance, list castaway.PonyList) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("**Season %d: Your ponies**\n", instance.Season))
	if len(list.Ponies) == 0 {
		builder.WriteString("You do not currently own any individual ponies.")
		return TrimMessage(builder.String())
	}
	for _, pony := range list.Ponies {
		builder.WriteString(fmt.Sprintf("- %s\n", pony.ContestantName))
	}
	return TrimMessage(strings.TrimSpace(builder.String()))
}

func LoanStatus(instance castaway.Instance, response castaway.LoanStatusResponse) string {
	loan := response.Loan
	lines := []string{
		fmt.Sprintf("**Season %d: Loan Shark**", instance.Season),
		fmt.Sprintf("- Principal: %d", loan.PrincipalPoints),
		fmt.Sprintf("- Interest: %d", loan.InterestPoints),
		fmt.Sprintf("- Total due: %d", loan.TotalDuePoints),
		fmt.Sprintf("- Remaining borrow: %d", loan.RemainingBorrowPoints),
		fmt.Sprintf("- Bonus points available: %d", loan.BonusPointsAvailable),
	}
	if strings.TrimSpace(loan.DueAt) != "" {
		lines = append(lines, fmt.Sprintf("- Due at: %s", formatTimeLong(loan.DueAt)))
	}
	return TrimMessage(strings.Join(lines, "\n"))
}

func LoanActionResult(instance castaway.Instance, response castaway.LoanStatusResponse, verb string, points int) string {
	return TrimMessage(strings.Join([]string{
		fmt.Sprintf("**Season %d: Loan Shark**", instance.Season),
		fmt.Sprintf("%s %d points.", verb, points),
		fmt.Sprintf("- Total due: %d", response.Loan.TotalDuePoints),
		fmt.Sprintf("- Remaining borrow: %d", response.Loan.RemainingBorrowPoints),
		fmt.Sprintf("- Bonus points available: %d", response.Loan.BonusPointsAvailable),
	}, "\n"))
}

func IndividualPonyImmunityResult(instance castaway.Instance, result castaway.IndividualPonyImmunityResult) string {
	return TrimMessage(strings.Join([]string{
		fmt.Sprintf("**Season %d: Individual Pony immunity**", instance.Season),
		fmt.Sprintf("Recorded immunity for %s.", result.Contestant.Name),
		fmt.Sprintf("- Created bonus entries: %d", result.CreatedCount),
	}, "\n"))
}
