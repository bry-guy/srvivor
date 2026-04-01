package discord

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/bry-guy/srvivor/apps/castaway-discord-bot/internal/castaway"
	"github.com/bry-guy/srvivor/apps/castaway-discord-bot/internal/state"
	"github.com/bwmarrin/discordgo"
)

type testCastawayAPI struct {
	instances                        []castaway.Instance
	contestantsByInstance            map[string][]castaway.Contestant
	participantsByInstance           map[string][]castaway.Participant
	linkedParticipantByInstance      map[string]map[string]castaway.Participant
	leaderboardByInstance            map[string][]castaway.LeaderboardRow
	bonusLedgerByParticipant         map[string]castaway.ParticipantBonusLedger
	draftsByInstance                 map[string]map[string]castaway.Draft
	activitiesByInstance             map[string][]castaway.Activity
	activityDetails                  map[string]castaway.ActivityDetail
	occurrencesByActivity            map[string][]castaway.Occurrence
	occurrenceDetails                map[string]castaway.OccurrenceDetail
	historyByParticipant             map[string]castaway.ParticipantActivityHistory
	stirThePotStatusByInstance       map[string]castaway.StirThePotStatus
	stirThePotTribeStatusByInstance  map[string]map[string]castaway.StirThePotTribeStatus
	stirThePotContributionByInstance map[string]castaway.StirThePotContributionResult
	stirThePotStartByInstance        map[string]castaway.StirThePotStartResult
	auctionStatusByInstance          map[string]castaway.AuctionStatus
	auctionLotStartByContestant      map[string]castaway.AuctionLotStartResult
	auctionLotStopByContestant       map[string]castaway.AuctionLotStopResult
	auctionBidByContestant           map[string]castaway.AuctionBidResult
	ponyListByInstance               map[string]castaway.PonyList
	loanStatusByInstance             map[string]castaway.LoanStatusResponse
	loanBorrowByInstance             map[string]castaway.LoanStatusResponse
	loanRepayByInstance              map[string]castaway.LoanStatusResponse
	individualPonyByContestant       map[string]castaway.IndividualPonyImmunityResult
}

func TestScoreCommandRegression_UsesLeaderboardStyleOutput(t *testing.T) {
	bot, store := newTestBot(t, testCastawayAPI{
		instances:              []castaway.Instance{{ID: "instance-49", Name: "Historical Season 49", Season: 49}},
		participantsByInstance: map[string][]castaway.Participant{"instance-49": {{ID: "participant-bryan", Name: "Bryan"}}},
		leaderboardByInstance: map[string][]castaway.LeaderboardRow{"instance-49": {
			{ParticipantID: "participant-bryan", ParticipantName: "@prettybry", CurrentTribeName: "Leafy Green", Score: 81, DraftPoints: 76, BonusPoints: 5, TotalPoints: 81, PointsAvailable: -198},
		}},
	})

	if err := store.SetUserDefault("guild-1", "user-1", "instance-49"); err != nil {
		t.Fatalf("set user default: %v", err)
	}

	message, err := bot.executeCommand(context.Background(), testInteraction("guild-1", "user-1", 0), commandSpec{name: "score", options: []*discordgo.ApplicationCommandInteractionDataOption{stringOption("participant", "Bryan")}})
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}

	expected := "**Season 49: Score**\n1. :leafy_green: @prettybry: 81 (76+5)"
	if message != expected {
		t.Fatalf("unexpected score message:\nexpected: %q\nactual:   %q", expected, message)
	}
}

func TestScoreCommandRegression_HidesSecretBonusBreakdownForLinkedSelf(t *testing.T) {
	bot, store := newTestBot(t, testCastawayAPI{
		instances:                   []castaway.Instance{{ID: "instance-50", Name: "Historical Season 50", Season: 50}},
		participantsByInstance:      map[string][]castaway.Participant{"instance-50": {{ID: "participant-bryan", Name: "Bryan"}}},
		linkedParticipantByInstance: map[string]map[string]castaway.Participant{"instance-50": {"user-1": {ID: "participant-bryan", Name: "Bryan"}}},
		leaderboardByInstance: map[string][]castaway.LeaderboardRow{"instance-50": {
			{ParticipantID: "participant-bryan", ParticipantName: "@prettybry", CurrentTribeName: "Lotus", Score: 78, DraftPoints: 76, BonusPoints: 2, TotalPoints: 78, PointsAvailable: -198},
		}},
		bonusLedgerByParticipant: map[string]castaway.ParticipantBonusLedger{"participant-bryan": {
			Participant: castaway.Participant{ID: "participant-bryan", Name: "Bryan"},
			BonusPoints: 5,
			Ledger:      []castaway.BonusLedgerEntry{{ParticipantName: "Bryan", Points: 2, Visibility: "public"}, {ParticipantName: "Bryan", Points: 3, Visibility: "secret"}},
		}},
	})
	if err := store.SetUserDefault("guild-1", "user-1", "instance-50"); err != nil {
		t.Fatalf("set user default: %v", err)
	}

	message, err := bot.executeCommand(context.Background(), testInteraction("guild-1", "user-1", 0), commandSpec{name: "score", options: []*discordgo.ApplicationCommandInteractionDataOption{stringOption("participant", "Bryan")}})
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}
	if message != "**Season 50: Score**\n1. :lotus: @prettybry: 78 (76+2)" {
		t.Fatalf("unexpected score message: %q", message)
	}
}

func TestScoresCommandRegression_ResolvesSingleSeasonInstance(t *testing.T) {
	bot, _ := newTestBot(t, testCastawayAPI{
		instances: []castaway.Instance{{ID: "instance-50", Name: "Historical Season 50", Season: 50}},
		leaderboardByInstance: map[string][]castaway.LeaderboardRow{"instance-50": {
			{ParticipantID: "participant-keeling", ParticipantName: "@keeling", CurrentTribeName: "Lotus", Score: 6, DraftPoints: 5, BonusPoints: 1, TotalPoints: 6, PointsAvailable: 294},
			{ParticipantID: "participant-adam", ParticipantName: "@adam", CurrentTribeName: "Tangerine", Score: 5, DraftPoints: 5, BonusPoints: 0, TotalPoints: 5, PointsAvailable: 292},
			{ParticipantID: "participant-amanda", ParticipantName: "@amanda", CurrentTribeName: "Leafy Green", Score: 3, DraftPoints: 2, BonusPoints: 1, TotalPoints: 3, PointsAvailable: 281},
		}},
	})

	message, err := bot.executeCommand(context.Background(), testInteraction("guild-1", "user-1", 0), commandSpec{name: "scores", options: []*discordgo.ApplicationCommandInteractionDataOption{intOption("season", 50)}})
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}

	expected := strings.Join([]string{"**Season 50: Leaderboard**", "1. :lotus: @keeling: 6 (5+1)", "2. :tangerine: @adam: 5 (5+0)", "3. :leafy_green: @amanda: 3 (2+1)"}, "\n")
	if message != expected {
		t.Fatalf("unexpected leaderboard message:\nexpected: %q\nactual:   %q", expected, message)
	}
}

func TestDraftCommandRegression_UsesGuildDefault(t *testing.T) {
	bot, store := newTestBot(t, testCastawayAPI{
		instances:              []castaway.Instance{{ID: "instance-50", Name: "Historical Season 50", Season: 50}},
		participantsByInstance: map[string][]castaway.Participant{"instance-50": {{ID: "participant-bryan", Name: "Bryan"}}},
		draftsByInstance:       map[string]map[string]castaway.Draft{"instance-50": {"participant-bryan": {Participant: castaway.Participant{ID: "participant-bryan", Name: "Bryan"}, Picks: []castaway.DraftPick{{Position: 1, ContestantID: "emily", ContestantName: "Emily"}, {Position: 2, ContestantID: "christian", ContestantName: "Christian"}, {Position: 3, ContestantID: "q", ContestantName: "Q"}}}}},
	})

	if err := store.SetGuildDefault("guild-1", "instance-50"); err != nil {
		t.Fatalf("set guild default: %v", err)
	}

	message, err := bot.executeCommand(context.Background(), testInteraction("guild-1", "user-1", 0), commandSpec{name: "draft", options: []*discordgo.ApplicationCommandInteractionDataOption{stringOption("participant", "Bryan")}})
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}

	expected := strings.Join([]string{"**Season 50: Bryan Draft**", "1. Emily", "2. Christian", "3. Q"}, "\n")
	if message != expected {
		t.Fatalf("unexpected draft message:\nexpected: %q\nactual:   %q", expected, message)
	}
}

func TestDraftCommandRegression_DefaultsToLinkedParticipant(t *testing.T) {
	bot, store := newTestBot(t, testCastawayAPI{
		instances:                   []castaway.Instance{{ID: "instance-50", Name: "Historical Season 50", Season: 50}},
		participantsByInstance:      map[string][]castaway.Participant{"instance-50": {{ID: "participant-bryan", Name: "Bryan"}}},
		linkedParticipantByInstance: map[string]map[string]castaway.Participant{"instance-50": {"user-1": {ID: "participant-bryan", Name: "Bryan"}}},
		draftsByInstance:            map[string]map[string]castaway.Draft{"instance-50": {"participant-bryan": {Participant: castaway.Participant{ID: "participant-bryan", Name: "Bryan"}, Picks: []castaway.DraftPick{{Position: 1, ContestantID: "emily", ContestantName: "Emily"}}}}},
	})

	if err := store.SetGuildDefault("guild-1", "instance-50"); err != nil {
		t.Fatalf("set guild default: %v", err)
	}

	message, err := bot.executeCommand(context.Background(), testInteraction("guild-1", "user-1", 0), commandSpec{name: "draft"})
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}

	expected := strings.Join([]string{"**Season 50: Bryan Draft**", "1. Emily"}, "\n")
	if message != expected {
		t.Fatalf("unexpected draft message:\nexpected: %q\nactual:   %q", expected, message)
	}
}

func TestInstancesCommandRegression_ListsInstances(t *testing.T) {
	bot, _ := newTestBot(t, testCastawayAPI{instances: []castaway.Instance{{ID: "instance-50", Name: "Historical Season 50", Season: 50}, {ID: "instance-49", Name: "Historical Season 49", Season: 49}}})

	message, err := bot.executeCommand(context.Background(), testInteraction("guild-1", "user-1", 0), commandSpec{name: "instances"})
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}
	for _, fragment := range []string{"**Instances**", "Season 50 — Historical Season 50", "Season 49 — Historical Season 49"} {
		if !strings.Contains(message, fragment) {
			t.Fatalf("expected fragment %q in %q", fragment, message)
		}
	}
}

func TestInstanceCommandRegression_UserDefaultLifecycle(t *testing.T) {
	bot, _ := newTestBot(t, testCastawayAPI{instances: []castaway.Instance{{ID: "instance-50", Name: "Historical Season 50", Season: 50}}})
	interaction := testInteraction("guild-1", "user-1", 0)

	setMessage, err := bot.executeCommand(context.Background(), interaction, commandSpec{group: "instance", name: "set", options: []*discordgo.ApplicationCommandInteractionDataOption{stringOption("instance", "Historical Season 50"), intOption("season", 50)}})
	if err != nil {
		t.Fatalf("set default: %v", err)
	}
	if setMessage != "Saved your default instance: Season 50 — Historical Season 50" {
		t.Fatalf("unexpected set message: %q", setMessage)
	}

	showMessage, err := bot.executeCommand(context.Background(), interaction, commandSpec{group: "instance", name: "show"})
	if err != nil {
		t.Fatalf("show defaults: %v", err)
	}
	showExpected := strings.Join([]string{"**Saved instance defaults**", "- You: Season 50 — Historical Season 50", "- Guild: not set"}, "\n")
	if showMessage != showExpected {
		t.Fatalf("unexpected show message:\nexpected: %q\nactual:   %q", showExpected, showMessage)
	}

	clearMessage, err := bot.executeCommand(context.Background(), interaction, commandSpec{group: "instance", name: "unset"})
	if err != nil {
		t.Fatalf("clear defaults: %v", err)
	}
	if clearMessage != "Cleared your default instance." {
		t.Fatalf("unexpected clear message: %q", clearMessage)
	}
}

func TestInstanceCommandRegression_GuildScopeRequiresManageServer(t *testing.T) {
	bot, _ := newTestBot(t, testCastawayAPI{instances: []castaway.Instance{{ID: "instance-50", Name: "Historical Season 50", Season: 50}}})
	_, err := bot.executeCommand(context.Background(), testInteraction("guild-1", "user-1", 0), commandSpec{group: "instance", name: "set", options: []*discordgo.ApplicationCommandInteractionDataOption{stringOption("instance", "Historical Season 50"), intOption("season", 50), stringOption("scope", "guild")}})
	if err == nil {
		t.Fatal("expected permission error")
	}
	if err.Error() != "guild scope requires Manage Server permission" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveInstanceRegression_ClearsStaleUserDefaultBeforeGuildFallback(t *testing.T) {
	bot, store := newTestBot(t, testCastawayAPI{instances: []castaway.Instance{{ID: "instance-49", Name: "Historical Season 49", Season: 49}}})
	if err := store.SetUserDefault("guild-1", "user-1", "stale-instance"); err != nil {
		t.Fatalf("set stale user default: %v", err)
	}
	if err := store.SetGuildDefault("guild-1", "instance-49"); err != nil {
		t.Fatalf("set guild default: %v", err)
	}

	instance, err := bot.resolveInstance(context.Background(), testInteraction("guild-1", "user-1", 0), "", nil)
	if err != nil {
		t.Fatalf("resolve instance: %v", err)
	}
	if instance.ID != "instance-49" {
		t.Fatalf("unexpected instance: %#v", instance)
	}
}

func TestPotStatusCommandRegression_ShowsOpenRound(t *testing.T) {
	bot, store := newTestBot(t, testCastawayAPI{
		instances:                   []castaway.Instance{{ID: "instance-50", Name: "Historical Season 50", Season: 50}},
		linkedParticipantByInstance: map[string]map[string]castaway.Participant{"instance-50": {"user-1": {ID: "participant-1", Name: "Bryan"}}},
		stirThePotStatusByInstance: map[string]castaway.StirThePotStatus{"instance-50": {
			Open:                 true,
			Participant:          castaway.Participant{ID: "participant-1", Name: "Bryan"},
			Round:                castaway.Occurrence{ID: "pot-1", Name: "Stir the Pot — Episode 5"},
			MyContributionPoints: 3,
			BonusPointsAvailable: 7,
			RewardTiers:          []castaway.StirThePotRewardTier{{Contributions: 2, Bonus: 1}, {Contributions: 5, Bonus: 2}},
		}},
	})
	if err := store.SetUserDefault("guild-1", "user-1", "instance-50"); err != nil {
		t.Fatalf("set user default: %v", err)
	}

	message, err := bot.executeCommand(context.Background(), testInteraction("guild-1", "user-1", 0), commandSpec{group: "pot", name: "status"})
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}
	for _, fragment := range []string{"**Season 50: Stir the Pot**", "- Round: Stir the Pot — Episode 5", "- Your contribution: 3", "- Bonus points available: 7"} {
		if !strings.Contains(message, fragment) {
			t.Fatalf("expected fragment %q in %q", fragment, message)
		}
	}
}

func TestPotShowCommandRegression_ShowsCurrentTribeTotal(t *testing.T) {
	bot, store := newTestBot(t, testCastawayAPI{
		instances: []castaway.Instance{{ID: "instance-50", Name: "Historical Season 50", Season: 50}},
		stirThePotTribeStatusByInstance: map[string]map[string]castaway.StirThePotTribeStatus{"instance-50": {
			"lotus": {
				Open:                     true,
				Tribe:                    castaway.ParticipantGroup{ID: "tribe-lotus", Name: "Lotus", Kind: "tribe"},
				Round:                    castaway.Occurrence{ID: "pot-1", Name: "Stir the Pot — Episode 6"},
				ContributionPoints:       5,
				BonusPointsIfResolvedNow: 2,
				RewardTiers:              []castaway.StirThePotRewardTier{{Contributions: 2, Bonus: 1}, {Contributions: 5, Bonus: 2}, {Contributions: 8, Bonus: 3}},
			},
		}},
	})
	if err := store.SetUserDefault("guild-1", "admin-1", "instance-50"); err != nil {
		t.Fatalf("set user default: %v", err)
	}

	message, err := bot.executeCommand(context.Background(), testInteraction("guild-1", "admin-1", 0), commandSpec{group: "pot", name: "show", options: []*discordgo.ApplicationCommandInteractionDataOption{stringOption("tribe", "Lotus")}})
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}
	for _, fragment := range []string{"**Season 50: Stir the Pot**", "- Tribe: Lotus", "- Round: Stir the Pot — Episode 6", "- Current contribution: 5", "- Bonus if resolved now: +2", "- Next tier: 8→+3 (3 more)"} {
		if !strings.Contains(message, fragment) {
			t.Fatalf("expected fragment %q in %q", fragment, message)
		}
	}
}

func TestPotAddCommandRegression_AllowsAdminToContributeForNamedParticipant(t *testing.T) {
	bot, store := newTestBot(t, testCastawayAPI{
		instances:              []castaway.Instance{{ID: "instance-50", Name: "Historical Season 50", Season: 50}},
		participantsByInstance: map[string][]castaway.Participant{"instance-50": {{ID: "participant-keith", Name: "Keith"}}},
		stirThePotContributionByInstance: map[string]castaway.StirThePotContributionResult{"instance-50": {
			Participant:          castaway.Participant{ID: "participant-keith", Name: "Keith"},
			AddedPoints:          1,
			MyContributionPoints: 1,
			BonusPointsAvailable: 3,
		}},
	})
	if err := store.SetUserDefault("guild-1", "admin-1", "instance-50"); err != nil {
		t.Fatalf("set user default: %v", err)
	}

	message, err := bot.executeCommand(context.Background(), testInteraction("guild-1", "admin-1", 0), commandSpec{group: "pot", name: "add", options: []*discordgo.ApplicationCommandInteractionDataOption{intOption("points", 1), stringOption("player", "Keith")}})
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}
	for _, fragment := range []string{"**Season 50: Stir the Pot**", "Recorded 1 Stir the Pot points for Keith.", "- Keith contribution: 1", "- Bonus points available: 3"} {
		if !strings.Contains(message, fragment) {
			t.Fatalf("expected fragment %q in %q", fragment, message)
		}
	}
}

func TestPotAddCommandRegression_RejectsUnlinkedCaller(t *testing.T) {
	bot, store := newTestBot(t, testCastawayAPI{
		instances: []castaway.Instance{{ID: "instance-50", Name: "Historical Season 50", Season: 50}},
	})
	if err := store.SetUserDefault("guild-1", "user-2", "instance-50"); err != nil {
		t.Fatalf("set user default: %v", err)
	}

	_, err := bot.executeCommand(context.Background(), testInteraction("guild-1", "user-2", 0), commandSpec{group: "pot", name: "add", options: []*discordgo.ApplicationCommandInteractionDataOption{intOption("points", 1)}})
	if err == nil {
		t.Fatal("expected unlinked error")
	}
	if err.Error() != "you are not linked to a Castaway player for this season; ask a Castaway admin to run /castaway link first" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBidCommandRegression_SetsBlindBidAgainstContestant(t *testing.T) {
	bot, store := newTestBot(t, testCastawayAPI{
		instances:                   []castaway.Instance{{ID: "instance-50", Name: "Historical Season 50", Season: 50}},
		contestantsByInstance:       map[string][]castaway.Contestant{"instance-50": {{ID: "c-joe", Name: "Joe"}}},
		linkedParticipantByInstance: map[string]map[string]castaway.Participant{"instance-50": {"user-1": {ID: "participant-1", Name: "Bryan"}}},
		auctionBidByContestant: map[string]castaway.AuctionBidResult{"c-joe": {
			Contestant:           castaway.Contestant{ID: "c-joe", Name: "Joe"},
			LotID:                "lot-1",
			MyBidPoints:          4,
			PreviousBidPoints:    1,
			BonusPointsAvailable: 6,
		}},
	})
	if err := store.SetUserDefault("guild-1", "user-1", "instance-50"); err != nil {
		t.Fatalf("set user default: %v", err)
	}

	message, err := bot.executeCommand(context.Background(), testInteraction("guild-1", "user-1", 0), commandSpec{name: "bid", options: []*discordgo.ApplicationCommandInteractionDataOption{stringOption("survivor", "Joe"), intOption("points", 4)}})
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}
	for _, fragment := range []string{"**Season 50: Bid submitted**", "Joe — your bid is now 4.", "- Bonus points available: 6"} {
		if !strings.Contains(message, fragment) {
			t.Fatalf("expected fragment %q in %q", fragment, message)
		}
	}
}

func TestBidCommandRegression_AllowsAdminToBidForNamedParticipant(t *testing.T) {
	bot, store := newTestBot(t, testCastawayAPI{
		instances:             []castaway.Instance{{ID: "instance-50", Name: "Historical Season 50", Season: 50}},
		contestantsByInstance: map[string][]castaway.Contestant{"instance-50": {{ID: "c-joe", Name: "Joe"}}},
		participantsByInstance: map[string][]castaway.Participant{"instance-50": {
			{ID: "participant-keith", Name: "Keith"},
		}},
		auctionBidByContestant: map[string]castaway.AuctionBidResult{"c-joe": {
			Participant:          castaway.Participant{ID: "participant-keith", Name: "Keith"},
			Contestant:           castaway.Contestant{ID: "c-joe", Name: "Joe"},
			LotID:                "lot-1",
			MyBidPoints:          3,
			PreviousBidPoints:    0,
			BonusPointsAvailable: 1,
		}},
	})
	if err := store.SetUserDefault("guild-1", "admin-1", "instance-50"); err != nil {
		t.Fatalf("set user default: %v", err)
	}

	message, err := bot.executeCommand(context.Background(), testInteraction("guild-1", "admin-1", 0), commandSpec{name: "bid", options: []*discordgo.ApplicationCommandInteractionDataOption{stringOption("survivor", "Joe"), intOption("points", 3), stringOption("player", "Keith")}})
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}
	for _, fragment := range []string{"**Season 50: Bid submitted**", "Joe — Keith's bid is now 3.", "- Bonus points available: 1"} {
		if !strings.Contains(message, fragment) {
			t.Fatalf("expected fragment %q in %q", fragment, message)
		}
	}
}

func TestBidCommandRegression_RejectsUnlinkedCaller(t *testing.T) {
	bot, store := newTestBot(t, testCastawayAPI{
		instances:             []castaway.Instance{{ID: "instance-50", Name: "Historical Season 50", Season: 50}},
		contestantsByInstance: map[string][]castaway.Contestant{"instance-50": {{ID: "c-joe", Name: "Joe"}}},
	})
	if err := store.SetUserDefault("guild-1", "user-2", "instance-50"); err != nil {
		t.Fatalf("set user default: %v", err)
	}

	_, err := bot.executeCommand(context.Background(), testInteraction("guild-1", "user-2", 0), commandSpec{name: "bid", options: []*discordgo.ApplicationCommandInteractionDataOption{stringOption("survivor", "Joe"), intOption("points", 3)}})
	if err == nil {
		t.Fatal("expected unlinked error")
	}
	if err.Error() != "you are not linked to a Castaway player for this season; ask a Castaway admin to run /castaway link first" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAuctionStartCommandRegression_OpensLotForContestant(t *testing.T) {
	bot, store := newTestBot(t, testCastawayAPI{
		instances:             []castaway.Instance{{ID: "instance-50", Name: "Historical Season 50", Season: 50}},
		contestantsByInstance: map[string][]castaway.Contestant{"instance-50": {{ID: "c-angelina", Name: "Angelina"}}},
		auctionLotStartByContestant: map[string]castaway.AuctionLotStartResult{"c-angelina": {
			Activity:    castaway.Activity{ID: "a-auction", Name: "Individual Pony Auction"},
			Lot:         castaway.Occurrence{ID: "lot-1", Name: "Angelina Auction Lot"},
			Contestant:  castaway.Contestant{ID: "c-angelina", Name: "Angelina"},
			BiddingOpen: true,
		}},
	})
	if err := store.SetUserDefault("guild-1", "user-1", "instance-50"); err != nil {
		t.Fatalf("set user default: %v", err)
	}

	message, err := bot.executeCommand(context.Background(), testInteraction("guild-1", "admin-1", 0), commandSpec{group: "auction", name: "start", options: []*discordgo.ApplicationCommandInteractionDataOption{stringOption("survivor", "Angelina")}})
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}
	for _, fragment := range []string{"**Season 50: Auction lot opened**", "Angelina is now open for bidding.", "- Lot: Angelina Auction Lot"} {
		if !strings.Contains(message, fragment) {
			t.Fatalf("expected fragment %q in %q", fragment, message)
		}
	}
}

func TestLoanRequestCommandRegression_BorrowsPoints(t *testing.T) {
	bot, store := newTestBot(t, testCastawayAPI{
		instances:                   []castaway.Instance{{ID: "instance-50", Name: "Historical Season 50", Season: 50}},
		linkedParticipantByInstance: map[string]map[string]castaway.Participant{"instance-50": {"user-1": {ID: "participant-1", Name: "Bryan"}}},
		loanBorrowByInstance: map[string]castaway.LoanStatusResponse{"instance-50": {
			Participant: castaway.Participant{ID: "participant-1", Name: "Bryan"},
			Loan:        castaway.LoanStatus{PrincipalPoints: 2, InterestPoints: 1, TotalDuePoints: 3, RemainingBorrowPoints: 1, BonusPointsAvailable: 8},
		}},
	})
	if err := store.SetUserDefault("guild-1", "user-1", "instance-50"); err != nil {
		t.Fatalf("set user default: %v", err)
	}

	message, err := bot.executeCommand(context.Background(), testInteraction("guild-1", "user-1", 0), commandSpec{group: "loan", name: "request", options: []*discordgo.ApplicationCommandInteractionDataOption{intOption("points", 2)}})
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}
	for _, fragment := range []string{"**Season 50: Loan Shark**", "Borrowed 2 points.", "- Total due: 3", "- Remaining borrow: 1"} {
		if !strings.Contains(message, fragment) {
			t.Fatalf("expected fragment %q in %q", fragment, message)
		}
	}
}

func TestAuctionAwardCommandRegression_RecordsIndividualImmunity(t *testing.T) {
	bot, store := newTestBot(t, testCastawayAPI{
		instances:             []castaway.Instance{{ID: "instance-50", Name: "Historical Season 50", Season: 50}},
		contestantsByInstance: map[string][]castaway.Contestant{"instance-50": {{ID: "c-mike", Name: "Mike"}}},
		individualPonyByContestant: map[string]castaway.IndividualPonyImmunityResult{"c-mike": {
			Contestant:   castaway.Contestant{ID: "c-mike", Name: "Mike"},
			OccurrenceID: "occ-pony-1",
			CreatedCount: 2,
		}},
	})
	if err := store.SetUserDefault("guild-1", "user-1", "instance-50"); err != nil {
		t.Fatalf("set user default: %v", err)
	}

	message, err := bot.executeCommand(context.Background(), testInteraction("guild-1", "admin-1", 0), commandSpec{group: "auction", name: "award", options: []*discordgo.ApplicationCommandInteractionDataOption{stringOption("survivor", "Mike")}})
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}
	for _, fragment := range []string{"**Season 50: Individual Pony immunity**", "Recorded immunity for Mike.", "- Created bonus entries: 2"} {
		if !strings.Contains(message, fragment) {
			t.Fatalf("expected fragment %q in %q", fragment, message)
		}
	}
}

func TestActivitiesCommandRegression_ListsActivitiesForInstance(t *testing.T) {
	bot, store := newTestBot(t, testCastawayAPI{instances: []castaway.Instance{{ID: "instance-50", Name: "Historical Season 50", Season: 50}}, activitiesByInstance: map[string][]castaway.Activity{"instance-50": {{ID: "act-1", InstanceID: "instance-50", ActivityType: "tribal_pony", Name: "Tribal Pony", Status: "active"}, {ID: "act-2", InstanceID: "instance-50", ActivityType: "journey", Name: "Journey 1", Status: "completed"}}}})
	if err := store.SetUserDefault("guild-1", "user-1", "instance-50"); err != nil {
		t.Fatalf("set user default: %v", err)
	}

	message, err := bot.executeCommand(context.Background(), testInteraction("guild-1", "user-1", 0), commandSpec{name: "activities"})
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}

	expected := strings.Join([]string{"**Season 50: Activities**", "- **Tribal Pony** (tribal_pony) — active", "- **Journey 1** (journey) — completed"}, "\n")
	if message != expected {
		t.Fatalf("unexpected activities message:\nexpected: %q\nactual:   %q", expected, message)
	}
}

func TestActivityCommandRegression_ShowsDetailedActivity(t *testing.T) {
	bot, store := newTestBot(t, testCastawayAPI{
		instances:            []castaway.Instance{{ID: "instance-50", Name: "Historical Season 50", Season: 50}},
		activitiesByInstance: map[string][]castaway.Activity{"instance-50": {{ID: "act-1", InstanceID: "instance-50", ActivityType: "journey", Name: "Journey 1", Status: "completed"}}},
		activityDetails: map[string]castaway.ActivityDetail{"act-1": {
			Activity:               castaway.Activity{ID: "act-1", InstanceID: "instance-50", ActivityType: "journey", Name: "Journey 1", Status: "completed", StartsAt: "2026-03-12T00:00:00Z"},
			GroupAssignments:       []castaway.ActivityGroupAssignment{{ParticipantGroupName: "Leaf", Role: "tribe"}},
			ParticipantAssignments: []castaway.ActivityParticipantAssignment{{ParticipantName: "Mooney", ParticipantGroupName: "Leaf", Role: "delegate"}},
		}},
		occurrencesByActivity: map[string][]castaway.Occurrence{"act-1": {{ID: "occ-1", ActivityID: "act-1", OccurrenceType: "attendance", Name: "Journey 1 Attendance", EffectiveAt: "2026-03-12T00:00:00Z", Status: "resolved"}}},
	})
	if err := store.SetUserDefault("guild-1", "user-1", "instance-50"); err != nil {
		t.Fatalf("set user default: %v", err)
	}

	message, err := bot.executeCommand(context.Background(), testInteraction("guild-1", "user-1", 0), commandSpec{name: "activity", options: []*discordgo.ApplicationCommandInteractionDataOption{stringOption("activity", "Journey 1")}})
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}
	for _, fragment := range []string{"**Season 50: Journey 1**", "- Type: journey", "**Assignments**", "Mooney — role=delegate, group=Leaf", "**Occurrences**", "Journey 1 Attendance"} {
		if !strings.Contains(message, fragment) {
			t.Fatalf("expected fragment %q in %q", fragment, message)
		}
	}
}

func TestOccurrencesCommandRegression_ListsOccurrencesWithImpactSummary(t *testing.T) {
	bot, store := newTestBot(t, testCastawayAPI{
		instances:             []castaway.Instance{{ID: "instance-50", Name: "Historical Season 50", Season: 50}},
		activitiesByInstance:  map[string][]castaway.Activity{"instance-50": {{ID: "act-1", InstanceID: "instance-50", ActivityType: "tribal_pony", Name: "Tribal Pony", Status: "active"}}},
		occurrencesByActivity: map[string][]castaway.Occurrence{"act-1": {{ID: "occ-1", ActivityID: "act-1", OccurrenceType: "immunity_result", Name: "Episode 1 Immunity", EffectiveAt: "2026-03-05T01:00:00Z", Status: "resolved"}}},
		occurrenceDetails:     map[string]castaway.OccurrenceDetail{"occ-1": {Occurrence: castaway.Occurrence{ID: "occ-1", ActivityID: "act-1", OccurrenceType: "immunity_result", Name: "Episode 1 Immunity", EffectiveAt: "2026-03-05T01:00:00Z", Status: "resolved"}, Ledger: []castaway.BonusLedgerEntry{{ParticipantName: "Amanda", Points: 1, Visibility: "public"}, {ParticipantName: "Bryan", Points: 1, Visibility: "public"}}}},
	})
	if err := store.SetUserDefault("guild-1", "user-1", "instance-50"); err != nil {
		t.Fatalf("set user default: %v", err)
	}

	message, err := bot.executeCommand(context.Background(), testInteraction("guild-1", "user-1", 0), commandSpec{name: "occurrences", options: []*discordgo.ApplicationCommandInteractionDataOption{stringOption("activity", "Tribal Pony")}})
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}
	for _, fragment := range []string{"Episode 1 Immunity", "- Status: resolved", "- Impact: Amanda — +1 public", "- Impact: Bryan — +1 public"} {
		if !strings.Contains(message, fragment) {
			t.Fatalf("expected fragment %q in %q", fragment, message)
		}
	}
}

func TestOccurrenceCommandRegression_ShowsDetailedOccurrence(t *testing.T) {
	bot, store := newTestBot(t, testCastawayAPI{
		instances:             []castaway.Instance{{ID: "instance-50", Name: "Historical Season 50", Season: 50}},
		activitiesByInstance:  map[string][]castaway.Activity{"instance-50": {{ID: "act-1", InstanceID: "instance-50", ActivityType: "journey", Name: "Journey 1", Status: "completed"}}},
		occurrencesByActivity: map[string][]castaway.Occurrence{"act-1": {{ID: "occ-1", ActivityID: "act-1", OccurrenceType: "journey_resolution", Name: "Journey 1 Tribal Diplomacy", EffectiveAt: "2026-03-14T01:00:00Z", Status: "resolved"}}},
		occurrenceDetails: map[string]castaway.OccurrenceDetail{"occ-1": {
			Occurrence:   castaway.Occurrence{ID: "occ-1", ActivityID: "act-1", OccurrenceType: "journey_resolution", Name: "Journey 1 Tribal Diplomacy", EffectiveAt: "2026-03-14T01:00:00Z", Status: "resolved"},
			Participants: []castaway.OccurrenceParticipant{{ParticipantName: "Adam", ParticipantGroupName: "Tangerine", Role: "delegate", Result: "STEAL"}, {ParticipantName: "Katie", ParticipantGroupName: "Lotus", Role: "delegate", Result: "SHARE"}},
			Ledger:       []castaway.BonusLedgerEntry{{ParticipantName: "Katie", Points: 1, Visibility: "public"}},
		}},
	})
	if err := store.SetUserDefault("guild-1", "user-1", "instance-50"); err != nil {
		t.Fatalf("set user default: %v", err)
	}

	message, err := bot.executeCommand(context.Background(), testInteraction("guild-1", "user-1", 0), commandSpec{name: "occurrence", options: []*discordgo.ApplicationCommandInteractionDataOption{stringOption("activity", "Journey 1"), stringOption("occurrence", "Journey 1 Tribal Diplomacy")}})
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}
	for _, fragment := range []string{"**Journey 1 Tribal Diplomacy**", "- Activity: Journey 1", "- Type: journey_resolution", "**Recorded**", "Adam — role=delegate, result=STEAL, group=Tangerine", "**Impact**"} {
		if !strings.Contains(message, fragment) {
			t.Fatalf("expected fragment %q in %q", fragment, message)
		}
	}
}

func TestHistoryCommandRegression_ShowsParticipantActivityHistory(t *testing.T) {
	bot, store := newTestBot(t, testCastawayAPI{
		instances: []castaway.Instance{{ID: "instance-50", Name: "Historical Season 50", Season: 50, Episodes: []castaway.InstanceEpisode{
			{ID: "e0", EpisodeNumber: 0, Label: "Episode 0", AirsAt: "2026-03-01T00:00:00Z"},
			{ID: "e1", EpisodeNumber: 1, Label: "Episode 1", AirsAt: "2026-03-10T00:00:00Z"},
		}}},
		participantsByInstance:      map[string][]castaway.Participant{"instance-50": {{ID: "participant-mooney", Name: "Mooney"}}},
		linkedParticipantByInstance: map[string]map[string]castaway.Participant{"instance-50": {"user-1": {ID: "participant-mooney", Name: "Mooney"}}},
		historyByParticipant: map[string]castaway.ParticipantActivityHistory{"participant-mooney": {
			Participant: castaway.Participant{ID: "participant-mooney", Name: "Mooney"},
			Instance:    castaway.Instance{ID: "instance-50", Name: "Historical Season 50", Season: 50},
			Activities: []castaway.ParticipantActivityHistoryActivity{{
				Activity: castaway.Activity{ID: "act-1", Name: "Journey 1", ActivityType: "journey"},
				Occurrences: []castaway.ParticipantActivityHistoryOccurrence{{
					Occurrence:  castaway.Occurrence{ID: "occ-1", Name: "Lost for Words — Mooney", EffectiveAt: "2026-03-14T02:00:00Z"},
					Involvement: &castaway.ParticipantOccurrenceInvolvement{Role: "delegate", Result: "risked", ParticipantGroupName: "Leaf"},
					Ledger:      []castaway.BonusLedgerEntry{{ParticipantName: "Mooney", Points: 1, Visibility: "secret"}},
				}},
			}},
		}},
	})
	if err := store.SetUserDefault("guild-1", "user-1", "instance-50"); err != nil {
		t.Fatalf("set user default: %v", err)
	}

	message, err := bot.executeCommand(context.Background(), testInteraction("guild-1", "user-1", 0), commandSpec{name: "history", options: []*discordgo.ApplicationCommandInteractionDataOption{stringOption("participant", "Mooney")}})
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}
	for _, fragment := range []string{"**Season 50: Mooney History**", "**Episode 0**", "n/a", "**Episode 1**", "Journey 1", "impact: +1 secret"} {
		if !strings.Contains(message, fragment) {
			t.Fatalf("expected fragment %q in %q", fragment, message)
		}
	}
}

func TestHistoryCommandRegression_HidesSecretImpactForPublicCaller(t *testing.T) {
	bot, store := newTestBot(t, testCastawayAPI{
		instances:              []castaway.Instance{{ID: "instance-50", Name: "Historical Season 50", Season: 50}},
		participantsByInstance: map[string][]castaway.Participant{"instance-50": {{ID: "participant-mooney", Name: "Mooney"}}},
		historyByParticipant: map[string]castaway.ParticipantActivityHistory{"participant-mooney": {
			Participant: castaway.Participant{ID: "participant-mooney", Name: "Mooney"},
			Instance:    castaway.Instance{ID: "instance-50", Name: "Historical Season 50", Season: 50},
			Activities: []castaway.ParticipantActivityHistoryActivity{{
				Activity: castaway.Activity{ID: "act-1", Name: "Journey 1", ActivityType: "journey"},
				Occurrences: []castaway.ParticipantActivityHistoryOccurrence{{
					Occurrence: castaway.Occurrence{ID: "occ-1", Name: "Lost for Words — Mooney", EffectiveAt: "2026-03-14T02:00:00Z"},
					Ledger: []castaway.BonusLedgerEntry{
						{ParticipantName: "Mooney", Points: 1, Visibility: "public"},
						{ParticipantName: "Mooney", Points: 2, Visibility: "secret"},
					},
				}},
			}},
		}},
	})
	if err := store.SetUserDefault("guild-1", "user-2", "instance-50"); err != nil {
		t.Fatalf("set user default: %v", err)
	}

	message, err := bot.executeCommand(context.Background(), testInteraction("guild-1", "user-2", 0), commandSpec{name: "history", options: []*discordgo.ApplicationCommandInteractionDataOption{stringOption("participant", "Mooney")}})
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}
	if strings.Contains(message, "+2 secret") {
		t.Fatalf("expected public history to hide secret impact, got %q", message)
	}
	if !strings.Contains(message, "+1 public") {
		t.Fatalf("expected public history to retain visible impact, got %q", message)
	}
}

func TestLinkAndUnlinkCommandsRegression(t *testing.T) {
	bot, store := newTestBot(t, testCastawayAPI{
		instances:              []castaway.Instance{{ID: "instance-50", Name: "Historical Season 50", Season: 50}},
		participantsByInstance: map[string][]castaway.Participant{"instance-50": {{ID: "participant-bryan", Name: "Bryan"}}},
	})
	if err := store.SetUserDefault("guild-1", "user-1", "instance-50"); err != nil {
		t.Fatalf("set user default: %v", err)
	}

	message, err := bot.executeCommand(context.Background(), testInteraction("guild-1", "user-1", 0), commandSpec{name: "link", options: []*discordgo.ApplicationCommandInteractionDataOption{stringOption("participant", "Bryan"), userIDOption("user", "user-2")}})
	if err != nil {
		t.Fatalf("link command: %v", err)
	}
	if message != "Linked Bryan to <@user-2> in Season 50 — Historical Season 50." {
		t.Fatalf("unexpected link message: %q", message)
	}

	message, err = bot.executeCommand(context.Background(), testInteraction("guild-1", "user-1", 0), commandSpec{name: "unlink", options: []*discordgo.ApplicationCommandInteractionDataOption{stringOption("participant", "Bryan")}})
	if err != nil {
		t.Fatalf("unlink command: %v", err)
	}
	if message != "Unlinked Discord account from Bryan in Season 50 — Historical Season 50." {
		t.Fatalf("unexpected unlink message: %q", message)
	}
}

func TestOccurrencesCommandRegression_ActivityNotFound(t *testing.T) {
	bot, store := newTestBot(t, testCastawayAPI{instances: []castaway.Instance{{ID: "instance-50", Name: "Historical Season 50", Season: 50}}, activitiesByInstance: map[string][]castaway.Activity{"instance-50": {{ID: "act-1", InstanceID: "instance-50", ActivityType: "tribal_pony", Name: "Tribal Pony", Status: "active"}}}})
	if err := store.SetUserDefault("guild-1", "user-1", "instance-50"); err != nil {
		t.Fatalf("set user default: %v", err)
	}

	_, err := bot.executeCommand(context.Background(), testInteraction("guild-1", "user-1", 0), commandSpec{name: "occurrences", options: []*discordgo.ApplicationCommandInteractionDataOption{stringOption("activity", "Nonexistent")}})
	if err == nil {
		t.Fatal("expected error for nonexistent activity")
	}
	if !strings.Contains(err.Error(), "no activities matched") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func newTestBot(t *testing.T, api testCastawayAPI) (*Bot, *state.BoltStore) {
	t.Helper()
	server := httptest.NewServer(api.handler(t))
	t.Cleanup(server.Close)

	client, err := castaway.NewClient(server.URL, server.Client(), castaway.Options{})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	store, err := state.OpenBolt(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("open state store: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("close state store: %v", err)
		}
	})

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return &Bot{castaway: client, state: store, log: logger}, store
}

func (api testCastawayAPI) handler(t *testing.T) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Helper()
		writeJSON := func(status int, payload any) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			if err := json.NewEncoder(w).Encode(payload); err != nil {
				t.Fatalf("encode response: %v", err)
			}
		}

		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) == 2 && parts[0] == "activities" && r.Method == http.MethodGet {
			detail, ok := api.activityDetails[parts[1]]
			if !ok {
				writeJSON(http.StatusNotFound, map[string]any{"error": "activity not found"})
				return
			}
			writeJSON(http.StatusOK, detail)
			return
		}
		if len(parts) == 3 && parts[0] == "activities" && parts[2] == "occurrences" && r.Method == http.MethodGet {
			occurrences := api.occurrencesByActivity[parts[1]]
			if occurrences == nil {
				occurrences = []castaway.Occurrence{}
			}
			writeJSON(http.StatusOK, map[string]any{"occurrences": occurrences})
			return
		}
		if len(parts) == 2 && parts[0] == "occurrences" && r.Method == http.MethodGet {
			detail, ok := api.occurrenceDetails[parts[1]]
			if !ok {
				writeJSON(http.StatusNotFound, map[string]any{"error": "occurrence not found"})
				return
			}
			writeJSON(http.StatusOK, detail)
			return
		}
		if len(parts) == 1 && parts[0] == "instances" {
			instances := api.instances
			if seasonRaw := strings.TrimSpace(r.URL.Query().Get("season")); seasonRaw != "" {
				season, err := strconv.Atoi(seasonRaw)
				if err != nil {
					writeJSON(http.StatusBadRequest, map[string]any{"error": "invalid season"})
					return
				}
				filtered := make([]castaway.Instance, 0, len(instances))
				for _, instance := range instances {
					if int(instance.Season) == season {
						filtered = append(filtered, instance)
					}
				}
				instances = filtered
			}
			if nameFilter := strings.TrimSpace(r.URL.Query().Get("name")); nameFilter != "" {
				filtered := make([]castaway.Instance, 0, len(instances))
				for _, instance := range instances {
					if containsFold(instance.Name, nameFilter) {
						filtered = append(filtered, instance)
					}
				}
				instances = filtered
			}
			writeJSON(http.StatusOK, map[string]any{"instances": instances})
			return
		}

		if len(parts) < 2 || parts[0] != "instances" {
			writeJSON(http.StatusNotFound, map[string]any{"error": "not found"})
			return
		}
		instanceID := parts[1]
		instance, ok := api.instanceByID(instanceID)
		if !ok {
			writeJSON(http.StatusNotFound, map[string]any{"error": "instance not found"})
			return
		}

		switch {
		case len(parts) == 2 && r.Method == http.MethodGet:
			writeJSON(http.StatusOK, map[string]any{"instance": instance, "episodes": instance.Episodes})
		case len(parts) == 3 && parts[2] == "contestants" && r.Method == http.MethodGet:
			contestants := api.contestantsByInstance[instanceID]
			if contestants == nil {
				contestants = []castaway.Contestant{}
			}
			writeJSON(http.StatusOK, map[string]any{"contestants": contestants})
		case len(parts) == 3 && parts[2] == "participants" && r.Method == http.MethodGet:
			participants := api.participantsByInstance[instanceID]
			if nameFilter := strings.TrimSpace(r.URL.Query().Get("name")); nameFilter != "" {
				filtered := make([]castaway.Participant, 0, len(participants))
				for _, participant := range participants {
					if containsFold(participant.Name, nameFilter) {
						filtered = append(filtered, participant)
					}
				}
				participants = filtered
			}
			writeJSON(http.StatusOK, map[string]any{"participants": participants})
		case len(parts) == 4 && parts[2] == "participants" && parts[3] == "me" && r.Method == http.MethodGet:
			participant, ok := api.linkedParticipantByInstance[instanceID][r.Header.Get("X-Discord-User-ID")]
			if !ok {
				writeJSON(http.StatusNotFound, map[string]any{"error": "participant not linked"})
				return
			}
			writeJSON(http.StatusOK, map[string]any{"participant": participant})
		case len(parts) == 5 && parts[2] == "participants" && parts[4] == "discord-link" && r.Method == http.MethodPut:
			participantID := parts[3]
			participant, ok := api.participantByID(instanceID, participantID)
			if !ok {
				writeJSON(http.StatusNotFound, map[string]any{"error": "participant not found"})
				return
			}
			if api.linkedParticipantByInstance == nil {
				api.linkedParticipantByInstance = map[string]map[string]castaway.Participant{}
			}
			if api.linkedParticipantByInstance[instanceID] == nil {
				api.linkedParticipantByInstance[instanceID] = map[string]castaway.Participant{}
			}
			api.linkedParticipantByInstance[instanceID][r.Header.Get("X-Discord-User-ID")] = participant
			writeJSON(http.StatusOK, map[string]any{"participant": participant})
		case len(parts) == 5 && parts[2] == "participants" && parts[4] == "discord-link" && r.Method == http.MethodDelete:
			participantID := parts[3]
			participant, ok := api.participantByID(instanceID, participantID)
			if !ok {
				writeJSON(http.StatusNotFound, map[string]any{"error": "participant not found"})
				return
			}
			if links := api.linkedParticipantByInstance[instanceID]; links != nil {
				delete(links, r.Header.Get("X-Discord-User-ID"))
			}
			writeJSON(http.StatusOK, map[string]any{"participant": participant})
		case len(parts) == 3 && parts[2] == "activities" && r.Method == http.MethodGet:
			activities := api.activitiesByInstance[instanceID]
			if activities == nil {
				activities = []castaway.Activity{}
			}
			writeJSON(http.StatusOK, map[string]any{"activities": activities})
		case len(parts) == 3 && parts[2] == "leaderboard" && r.Method == http.MethodGet:
			rows := api.leaderboardByInstance[instanceID]
			if participantID := strings.TrimSpace(r.URL.Query().Get("participant_id")); participantID != "" {
				filtered := make([]castaway.LeaderboardRow, 0, len(rows))
				for _, row := range rows {
					if row.ParticipantID == participantID {
						filtered = append(filtered, row)
					}
				}
				rows = filtered
			}
			writeJSON(http.StatusOK, map[string]any{"leaderboard": rows})
		case len(parts) == 4 && parts[2] == "drafts" && r.Method == http.MethodGet:
			draft, ok := api.draftsByInstance[instanceID][parts[3]]
			if !ok {
				writeJSON(http.StatusNotFound, map[string]any{"error": "draft not found"})
				return
			}
			writeJSON(http.StatusOK, draft)
		case len(parts) == 5 && parts[2] == "participants" && parts[4] == "bonus-ledger" && r.Method == http.MethodGet:
			ledger, ok := api.bonusLedgerByParticipant[parts[3]]
			if !ok {
				rows := api.leaderboardByInstance[instanceID]
				for _, row := range rows {
					if row.ParticipantID == parts[3] {
						ledger = castaway.ParticipantBonusLedger{Participant: castaway.Participant{ID: row.ParticipantID, Name: row.ParticipantName}, BonusPoints: row.BonusPoints}
						ok = true
						break
					}
				}
			}
			if !ok {
				writeJSON(http.StatusNotFound, map[string]any{"error": "bonus ledger not found"})
				return
			}
			if !api.canViewSecretParticipantData(instanceID, parts[3], r.Header.Get("X-Discord-User-ID")) {
				ledger = api.publicBonusLedger(ledger)
			}
			writeJSON(http.StatusOK, ledger)
		case len(parts) == 5 && parts[2] == "participants" && parts[4] == "activity-history" && r.Method == http.MethodGet:
			history, ok := api.historyByParticipant[parts[3]]
			if !ok {
				writeJSON(http.StatusNotFound, map[string]any{"error": "history not found"})
				return
			}
			if !api.canViewSecretParticipantData(instanceID, parts[3], r.Header.Get("X-Discord-User-ID")) {
				history = api.publicHistory(history)
			}
			writeJSON(http.StatusOK, history)
		case len(parts) == 4 && parts[2] == "stir-the-pot" && parts[3] == "me" && r.Method == http.MethodGet:
			writeJSON(http.StatusOK, api.stirThePotStatusByInstance[instanceID])
		case len(parts) == 5 && parts[2] == "stir-the-pot" && parts[3] == "tribes" && parts[4] == "show" && r.Method == http.MethodGet:
			if r.Header.Get("X-Discord-User-ID") != "admin-1" {
				writeJSON(http.StatusForbidden, map[string]any{"error": "forbidden"})
				return
			}
			tribeName := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("name")))
			status, ok := api.stirThePotTribeStatusByInstance[instanceID][tribeName]
			if !ok {
				writeJSON(http.StatusNotFound, map[string]any{"error": "tribe not found"})
				return
			}
			writeJSON(http.StatusOK, status)
		case len(parts) == 4 && parts[2] == "stir-the-pot" && parts[3] == "start" && r.Method == http.MethodPost:
			writeJSON(http.StatusOK, api.stirThePotStartByInstance[instanceID])
		case len(parts) == 5 && parts[2] == "stir-the-pot" && parts[3] == "me" && parts[4] == "contributions" && r.Method == http.MethodPost:
			var req struct {
				ParticipantID string `json:"participant_id"`
				Points        int    `json:"points"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode stir the pot contribution: %v", err)
			}
			if strings.TrimSpace(req.ParticipantID) != "" && r.Header.Get("X-Discord-User-ID") != "admin-1" {
				writeJSON(http.StatusForbidden, map[string]any{"error": "forbidden"})
				return
			}
			if strings.TrimSpace(req.ParticipantID) == "" {
				if _, ok := api.linkedParticipantByInstance[instanceID][r.Header.Get("X-Discord-User-ID")]; !ok {
					writeJSON(http.StatusNotFound, map[string]any{"error": "participant not linked"})
					return
				}
			}
			writeJSON(http.StatusOK, api.stirThePotContributionByInstance[instanceID])
		case len(parts) == 4 && parts[2] == "auction" && parts[3] == "me" && r.Method == http.MethodGet:
			writeJSON(http.StatusOK, api.auctionStatusByInstance[instanceID])
		case len(parts) == 5 && parts[2] == "auction" && parts[3] == "lots" && parts[4] == "start" && r.Method == http.MethodPost:
			var req struct {
				ContestantID string `json:"contestant_id"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode auction start: %v", err)
			}
			writeJSON(http.StatusOK, api.auctionLotStartByContestant[req.ContestantID])
		case len(parts) == 6 && parts[2] == "auction" && parts[3] == "lots" && parts[5] == "stop" && r.Method == http.MethodPost:
			writeJSON(http.StatusOK, api.auctionLotStopByContestant[parts[4]])
		case len(parts) == 7 && parts[2] == "auction" && parts[3] == "contestants" && parts[5] == "bid" && parts[6] == "me" && r.Method == http.MethodPut:
			var req struct {
				ParticipantID string `json:"participant_id"`
				Points        int    `json:"points"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode auction bid: %v", err)
			}
			if strings.TrimSpace(req.ParticipantID) != "" && r.Header.Get("X-Discord-User-ID") != "admin-1" {
				writeJSON(http.StatusForbidden, map[string]any{"error": "forbidden"})
				return
			}
			if strings.TrimSpace(req.ParticipantID) == "" {
				if _, ok := api.linkedParticipantByInstance[instanceID][r.Header.Get("X-Discord-User-ID")]; !ok {
					writeJSON(http.StatusNotFound, map[string]any{"error": "participant not linked"})
					return
				}
			}
			writeJSON(http.StatusOK, api.auctionBidByContestant[parts[4]])
		case len(parts) == 4 && parts[2] == "ponies" && parts[3] == "me" && r.Method == http.MethodGet:
			writeJSON(http.StatusOK, api.ponyListByInstance[instanceID])
		case len(parts) == 4 && parts[2] == "loan-shark" && parts[3] == "me" && r.Method == http.MethodGet:
			writeJSON(http.StatusOK, api.loanStatusByInstance[instanceID])
		case len(parts) == 5 && parts[2] == "loan-shark" && parts[3] == "me" && parts[4] == "borrow" && r.Method == http.MethodPost:
			writeJSON(http.StatusOK, api.loanBorrowByInstance[instanceID])
		case len(parts) == 5 && parts[2] == "loan-shark" && parts[3] == "me" && parts[4] == "repay" && r.Method == http.MethodPost:
			writeJSON(http.StatusOK, api.loanRepayByInstance[instanceID])
		case len(parts) == 4 && parts[2] == "individual-pony" && parts[3] == "immunity" && r.Method == http.MethodPost:
			var req struct {
				ContestantID string `json:"contestant_id"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode pony immunity: %v", err)
			}
			writeJSON(http.StatusOK, api.individualPonyByContestant[req.ContestantID])
		default:
			writeJSON(http.StatusNotFound, map[string]any{"error": "not found"})
		}
	})
}

func (api testCastawayAPI) instanceByID(id string) (castaway.Instance, bool) {
	for _, instance := range api.instances {
		if instance.ID == id {
			return instance, true
		}
	}
	return castaway.Instance{}, false
}

func (api testCastawayAPI) participantByID(instanceID, participantID string) (castaway.Participant, bool) {
	for _, participant := range api.participantsByInstance[instanceID] {
		if participant.ID == participantID {
			return participant, true
		}
	}
	return castaway.Participant{}, false
}

func (api testCastawayAPI) canViewSecretParticipantData(instanceID, participantID, discordUserID string) bool {
	if strings.TrimSpace(discordUserID) == "admin-1" {
		return true
	}
	linkedParticipant, ok := api.linkedParticipantByInstance[instanceID][discordUserID]
	return ok && linkedParticipant.ID == participantID
}

func (api testCastawayAPI) publicBonusLedger(ledger castaway.ParticipantBonusLedger) castaway.ParticipantBonusLedger {
	if len(ledger.Ledger) == 0 {
		return ledger
	}
	filtered := make([]castaway.BonusLedgerEntry, 0, len(ledger.Ledger))
	bonusPoints := 0
	for _, entry := range ledger.Ledger {
		if entry.Visibility == "secret" {
			continue
		}
		filtered = append(filtered, entry)
		bonusPoints += entry.Points
	}
	ledger.Ledger = filtered
	ledger.BonusPoints = bonusPoints
	return ledger
}

func (api testCastawayAPI) publicHistory(history castaway.ParticipantActivityHistory) castaway.ParticipantActivityHistory {
	filteredActivities := make([]castaway.ParticipantActivityHistoryActivity, 0, len(history.Activities))
	for _, activity := range history.Activities {
		filteredOccurrences := make([]castaway.ParticipantActivityHistoryOccurrence, 0, len(activity.Occurrences))
		for _, occurrence := range activity.Occurrences {
			visibleLedger := make([]castaway.BonusLedgerEntry, 0, len(occurrence.Ledger))
			for _, entry := range occurrence.Ledger {
				if entry.Visibility == "secret" {
					continue
				}
				visibleLedger = append(visibleLedger, entry)
			}
			occurrence.Ledger = visibleLedger
			filteredOccurrences = append(filteredOccurrences, occurrence)
		}
		activity.Occurrences = filteredOccurrences
		filteredActivities = append(filteredActivities, activity)
	}
	history.Activities = filteredActivities
	return history
}

func testInteraction(guildID, userID string, permissions int64) *discordgo.InteractionCreate {
	interaction := &discordgo.Interaction{GuildID: guildID}
	if guildID == "" {
		interaction.User = &discordgo.User{ID: userID}
	} else {
		interaction.Member = &discordgo.Member{Permissions: permissions, User: &discordgo.User{ID: userID}}
	}
	return &discordgo.InteractionCreate{Interaction: interaction}
}

func stringOption(name, value string) *discordgo.ApplicationCommandInteractionDataOption {
	return &discordgo.ApplicationCommandInteractionDataOption{Name: name, Type: discordgo.ApplicationCommandOptionString, Value: value}
}

func intOption(name string, value int64) *discordgo.ApplicationCommandInteractionDataOption {
	return &discordgo.ApplicationCommandInteractionDataOption{Name: name, Type: discordgo.ApplicationCommandOptionInteger, Value: float64(value)}
}

func userIDOption(name, userID string) *discordgo.ApplicationCommandInteractionDataOption {
	return &discordgo.ApplicationCommandInteractionDataOption{Name: name, Type: discordgo.ApplicationCommandOptionUser, Value: userID}
}

func containsFold(candidate, filter string) bool {
	return strings.Contains(strings.ToLower(candidate), strings.ToLower(filter))
}
