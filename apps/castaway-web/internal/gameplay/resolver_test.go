package gameplay

import (
	"context"
	"testing"
	"time"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestResolveActivityOccurrenceTribalPonyAwardsWinningTribeMembers(t *testing.T) {
	instanceID := testUUID()
	activityID := testUUID()
	occurrenceID := testUUID()
	winningGroupID := testUUID()
	losingGroupID := testUUID()
	aliceID := testUUID()
	bobID := testUUID()
	effectiveAt := time.Date(2026, time.March, 11, 20, 0, 0, 0, time.UTC)

	fake := &fakeQuerier{
		activityOccurrence: db.GetActivityOccurrenceRow{
			ID:             occurrenceID,
			ActivityID:     activityID,
			OccurrenceType: "immunity_result",
			Name:           "Episode 2 Immunity",
			EffectiveAt:    timestamptz(effectiveAt),
			Metadata:       []byte(`{"winning_survivor_tribes":["vatu"]}`),
		},
		instanceActivity: db.GetInstanceActivityRow{
			ID:           activityID,
			InstanceID:   instanceID,
			ActivityType: "tribal_pony",
			Name:         "Pony Tribes",
		},
		activeActivityGroupAssignments: []db.ListActiveActivityGroupAssignmentsAtRow{
			{
				ActivityID:           activityID,
				ParticipantGroupID:   winningGroupID,
				ParticipantGroupName: "Lotus",
				Role:                 "tribe",
				Configuration:        []byte(`{"pony_survivor_tribe":"vatu"}`),
			},
			{
				ActivityID:           activityID,
				ParticipantGroupID:   losingGroupID,
				ParticipantGroupName: "Leaf",
				Role:                 "tribe",
				Configuration:        []byte(`{"pony_survivor_tribe":"kalo"}`),
			},
		},
		activeMembershipsByGroup: map[[16]byte][]db.ListActiveParticipantGroupMembershipsAtRow{
			winningGroupID.Bytes: {
				{ParticipantGroupID: winningGroupID, ParticipantID: aliceID, ParticipantName: "Alice"},
				{ParticipantGroupID: winningGroupID, ParticipantID: bobID, ParticipantName: "Bob"},
			},
		},
	}

	service := NewService(fake)
	created, err := service.ResolveActivityOccurrence(context.Background(), occurrenceID)
	if err != nil {
		t.Fatalf("resolve activity occurrence: %v", err)
	}
	if got := len(created); got != 2 {
		t.Fatalf("expected 2 created ledger entries, got %d", got)
	}
	if got := len(fake.createdBonusLedgerEntries); got != 2 {
		t.Fatalf("expected 2 persisted ledger entries, got %d", got)
	}
	for _, entry := range fake.createdBonusLedgerEntries {
		if entry.InstanceID != instanceID {
			t.Fatalf("expected instance id %v, got %v", instanceID, entry.InstanceID)
		}
		if entry.Points != 1 || entry.EntryKind != bonusEntryKindAward || entry.Visibility != bonusVisibilityPublic {
			t.Fatalf("unexpected tribal pony entry: %+v", entry)
		}
		if entry.SourceGroupID != winningGroupID {
			t.Fatalf("expected source group %v, got %v", winningGroupID, entry.SourceGroupID)
		}
	}
}

func TestResolveActivityOccurrenceTribeWordleAwardsTopThreeTieWinners(t *testing.T) {
	instanceID := testUUID()
	activityID := testUUID()
	occurrenceID := testUUID()
	lotusID := testUUID()
	leafID := testUUID()
	tangerineID := testUUID()
	lotusAliceID := testUUID()
	lotusBobID := testUUID()
	leafCarolID := testUUID()
	leafDanID := testUUID()
	effectiveAt := time.Date(2026, time.March, 18, 20, 0, 0, 0, time.UTC)

	fake := &fakeQuerier{
		activityOccurrence: db.GetActivityOccurrenceRow{
			ID:             occurrenceID,
			ActivityID:     activityID,
			OccurrenceType: "challenge_result",
			Name:           "Week 2 Wordle",
			EffectiveAt:    timestamptz(effectiveAt),
		},
		instanceActivity: db.GetInstanceActivityRow{
			ID:           activityID,
			InstanceID:   instanceID,
			ActivityType: "tribe_wordle",
			Name:         "Weekly Wordle",
		},
		occurrenceParticipants: []db.ListActivityOccurrenceParticipantsRow{
			{ParticipantID: testUUID(), ParticipantName: "Lotus 1", ParticipantGroupID: lotusID, ParticipantGroupName: textValue("Lotus"), Metadata: []byte(`{"guess_count":2}`)},
			{ParticipantID: testUUID(), ParticipantName: "Lotus 2", ParticipantGroupID: lotusID, ParticipantGroupName: textValue("Lotus"), Metadata: []byte(`{"guess_count":3}`)},
			{ParticipantID: testUUID(), ParticipantName: "Lotus 3", ParticipantGroupID: lotusID, ParticipantGroupName: textValue("Lotus"), Metadata: []byte(`{"guess_count":4}`)},
			{ParticipantID: testUUID(), ParticipantName: "Leaf 1", ParticipantGroupID: leafID, ParticipantGroupName: textValue("Leaf"), Metadata: []byte(`{"guess_count":1}`)},
			{ParticipantID: testUUID(), ParticipantName: "Leaf 2", ParticipantGroupID: leafID, ParticipantGroupName: textValue("Leaf"), Metadata: []byte(`{"guess_count":4}`)},
			{ParticipantID: testUUID(), ParticipantName: "Leaf 3", ParticipantGroupID: leafID, ParticipantGroupName: textValue("Leaf"), Metadata: []byte(`{"guess_count":4}`)},
			{ParticipantID: testUUID(), ParticipantName: "Tangerine 1", ParticipantGroupID: tangerineID, ParticipantGroupName: textValue("Tangerine"), Metadata: []byte(`{"guess_count":5}`)},
			{ParticipantID: testUUID(), ParticipantName: "Tangerine 2", ParticipantGroupID: tangerineID, ParticipantGroupName: textValue("Tangerine"), Metadata: []byte(`{"guess_count":5}`)},
			{ParticipantID: testUUID(), ParticipantName: "Tangerine 3", ParticipantGroupID: tangerineID, ParticipantGroupName: textValue("Tangerine"), Metadata: []byte(`{"guess_count":5}`)},
		},
		activeMembershipsByGroup: map[[16]byte][]db.ListActiveParticipantGroupMembershipsAtRow{
			lotusID.Bytes: {
				{ParticipantGroupID: lotusID, ParticipantID: lotusAliceID, ParticipantName: "Alice"},
				{ParticipantGroupID: lotusID, ParticipantID: lotusBobID, ParticipantName: "Bob"},
			},
			leafID.Bytes: {
				{ParticipantGroupID: leafID, ParticipantID: leafCarolID, ParticipantName: "Carol"},
				{ParticipantGroupID: leafID, ParticipantID: leafDanID, ParticipantName: "Dan"},
			},
			tangerineID.Bytes: {
				{ParticipantGroupID: tangerineID, ParticipantID: testUUID(), ParticipantName: "Eve"},
			},
		},
	}

	service := NewService(fake)
	created, err := service.ResolveActivityOccurrence(context.Background(), occurrenceID)
	if err != nil {
		t.Fatalf("resolve activity occurrence: %v", err)
	}
	if got := len(created); got != 4 {
		t.Fatalf("expected 4 created ledger entries, got %d", got)
	}

	awardedGroups := make(map[[16]byte]int)
	for _, entry := range fake.createdBonusLedgerEntries {
		awardedGroups[entry.SourceGroupID.Bytes]++
		if entry.Points != 1 || entry.Visibility != bonusVisibilityPublic {
			t.Fatalf("unexpected wordle entry: %+v", entry)
		}
	}
	if got := awardedGroups[lotusID.Bytes]; got != 2 {
		t.Fatalf("expected lotus to receive 2 awards, got %d", got)
	}
	if got := awardedGroups[leafID.Bytes]; got != 2 {
		t.Fatalf("expected leaf to receive 2 awards, got %d", got)
	}
	if got := awardedGroups[tangerineID.Bytes]; got != 0 {
		t.Fatalf("expected tangerine to receive 0 awards, got %d", got)
	}
}

func TestResolveActivityOccurrenceJourneyAttendanceAwardsDelegates(t *testing.T) {
	instanceID := testUUID()
	activityID := testUUID()
	occurrenceID := testUUID()
	aliceID := testUUID()
	bobID := testUUID()
	effectiveAt := time.Date(2026, time.March, 20, 20, 0, 0, 0, time.UTC)

	fake := &fakeQuerier{
		activityOccurrence: db.GetActivityOccurrenceRow{
			ID:             occurrenceID,
			ActivityID:     activityID,
			OccurrenceType: "journey_attendance",
			Name:           "Journey 1",
			EffectiveAt:    timestamptz(effectiveAt),
		},
		instanceActivity: db.GetInstanceActivityRow{
			ID:           activityID,
			InstanceID:   instanceID,
			ActivityType: "journey",
			Name:         "Journey 1",
		},
		activeActivityParticipantAssignments: []db.ListActiveActivityParticipantAssignmentsAtRow{
			{ParticipantID: aliceID, ParticipantName: "Alice", Role: "delegate"},
			{ParticipantID: bobID, ParticipantName: "Bob", Role: "delegate"},
		},
	}

	service := NewService(fake)
	created, err := service.ResolveActivityOccurrence(context.Background(), occurrenceID)
	if err != nil {
		t.Fatalf("resolve activity occurrence: %v", err)
	}
	if got := len(created); got != 2 {
		t.Fatalf("expected 2 created ledger entries, got %d", got)
	}
	for _, entry := range fake.createdBonusLedgerEntries {
		if entry.Points != 1 || entry.EntryKind != bonusEntryKindAward || entry.Visibility != bonusVisibilityPublic {
			t.Fatalf("unexpected journey attendance entry: %+v", entry)
		}
		if entry.SourceGroupID.Valid {
			t.Fatalf("expected attendance award to have no source group, got %+v", entry.SourceGroupID)
		}
	}
}

func TestResolveActivityOccurrenceJourneyDiplomacyAwardsShareTribeWhenTwoSteal(t *testing.T) {
	instanceID := testUUID()
	activityID := testUUID()
	occurrenceID := testUUID()
	tangerineID := testUUID()
	leafID := testUUID()
	lotusID := testUUID()
	lotusAliceID := testUUID()
	lotusBobID := testUUID()
	effectiveAt := time.Date(2026, time.March, 21, 12, 0, 0, 0, time.UTC)

	fake := &fakeQuerier{
		activityOccurrence: db.GetActivityOccurrenceRow{
			ID:             occurrenceID,
			ActivityID:     activityID,
			OccurrenceType: "journey_resolution",
			Name:           "Journey 1 Tribal Diplomacy",
			EffectiveAt:    timestamptz(effectiveAt),
		},
		instanceActivity: db.GetInstanceActivityRow{
			ID:           activityID,
			InstanceID:   instanceID,
			ActivityType: "journey",
			Name:         "Journey 1",
		},
		occurrenceParticipants: []db.ListActivityOccurrenceParticipantsRow{
			{ParticipantID: testUUID(), ParticipantName: "Tangerine Delegate", ParticipantGroupID: tangerineID, Result: "steal"},
			{ParticipantID: testUUID(), ParticipantName: "Leaf Delegate", ParticipantGroupID: leafID, Result: "STEAL"},
			{ParticipantID: testUUID(), ParticipantName: "Lotus Delegate", ParticipantGroupID: lotusID, Result: "share"},
		},
		activeActivityParticipantAssignments: []db.ListActiveActivityParticipantAssignmentsAtRow{},
		activeMembershipsByGroup: map[[16]byte][]db.ListActiveParticipantGroupMembershipsAtRow{
			lotusID.Bytes: {
				{ParticipantGroupID: lotusID, ParticipantID: lotusAliceID, ParticipantName: "Alice"},
				{ParticipantGroupID: lotusID, ParticipantID: lotusBobID, ParticipantName: "Bob"},
			},
		},
	}

	service := NewService(fake)
	created, err := service.ResolveActivityOccurrence(context.Background(), occurrenceID)
	if err != nil {
		t.Fatalf("resolve activity occurrence: %v", err)
	}
	if got := len(created); got != 2 {
		t.Fatalf("expected 2 created ledger entries, got %d", got)
	}
	for _, entry := range fake.createdBonusLedgerEntries {
		if entry.SourceGroupID != lotusID {
			t.Fatalf("expected lotus to receive tribe awards, got %+v", entry.SourceGroupID)
		}
		if entry.Points != 1 || entry.Visibility != bonusVisibilityPublic {
			t.Fatalf("unexpected journey diplomacy entry: %+v", entry)
		}
	}
}

func TestResolveActivityOccurrenceJourneySecretRiskSpendsSecretBeforePublic(t *testing.T) {
	instanceID := testUUID()
	activityID := testUUID()
	occurrenceID := testUUID()
	participantID := testUUID()
	effectiveAt := time.Date(2026, time.March, 21, 13, 0, 0, 0, time.UTC)

	fake := &fakeQuerier{
		activityOccurrence: db.GetActivityOccurrenceRow{
			ID:             occurrenceID,
			ActivityID:     activityID,
			OccurrenceType: "secret_risk_result",
			Name:           "Lost for Words",
			EffectiveAt:    timestamptz(effectiveAt),
		},
		instanceActivity: db.GetInstanceActivityRow{
			ID:           activityID,
			InstanceID:   instanceID,
			ActivityType: "journey",
			Name:         "Journey 1",
		},
		occurrenceParticipants: []db.ListActivityOccurrenceParticipantsRow{
			{ParticipantID: participantID, ParticipantName: "Alice", Metadata: []byte(`{"guess_count":5}`)},
		},
		availableSecretTotal: 1,
		visibleTotalAsOf:     2,
	}

	service := NewService(fake)
	created, err := service.ResolveActivityOccurrence(context.Background(), occurrenceID)
	if err != nil {
		t.Fatalf("resolve activity occurrence: %v", err)
	}
	if got := len(created); got != 3 {
		t.Fatalf("expected 3 created ledger entries, got %d", got)
	}

	if fake.createdBonusLedgerEntries[0].Points != 3 || fake.createdBonusLedgerEntries[0].Visibility != bonusVisibilitySecret {
		t.Fatalf("expected first entry to award 3 secret points, got %+v", fake.createdBonusLedgerEntries[0])
	}
	if fake.createdBonusLedgerEntries[1].Points != -4 || fake.createdBonusLedgerEntries[1].Visibility != bonusVisibilitySecret {
		t.Fatalf("expected second entry to spend 4 secret points, got %+v", fake.createdBonusLedgerEntries[1])
	}
	if fake.createdBonusLedgerEntries[2].Points != -1 || fake.createdBonusLedgerEntries[2].Visibility != bonusVisibilityPublic {
		t.Fatalf("expected third entry to spend 1 public point, got %+v", fake.createdBonusLedgerEntries[2])
	}
}

func TestResolveActivityOccurrenceManualAdjustmentCreatesCorrectionRows(t *testing.T) {
	instanceID := testUUID()
	activityID := testUUID()
	occurrenceID := testUUID()
	participantID := testUUID()
	groupID := testUUID()
	effectiveAt := time.Date(2026, time.March, 22, 9, 0, 0, 0, time.UTC)

	fake := &fakeQuerier{
		activityOccurrence: db.GetActivityOccurrenceRow{
			ID:             occurrenceID,
			ActivityID:     activityID,
			OccurrenceType: "manual_correction",
			Name:           "Manual Correction",
			EffectiveAt:    timestamptz(effectiveAt),
		},
		instanceActivity: db.GetInstanceActivityRow{
			ID:           activityID,
			InstanceID:   instanceID,
			ActivityType: "manual_adjustment",
			Name:         "Manual Adjustments",
		},
		occurrenceParticipants: []db.ListActivityOccurrenceParticipantsRow{
			{
				ID:                 42,
				ParticipantID:      participantID,
				ParticipantName:    "Alice",
				ParticipantGroupID: groupID,
				Metadata:           []byte(`{"points":-2,"visibility":"public","reason":"manual correction"}`),
			},
		},
	}

	service := NewService(fake)
	created, err := service.ResolveActivityOccurrence(context.Background(), occurrenceID)
	if err != nil {
		t.Fatalf("resolve activity occurrence: %v", err)
	}
	if got := len(created); got != 1 {
		t.Fatalf("expected 1 created ledger entry, got %d", got)
	}
	entry := fake.createdBonusLedgerEntries[0]
	if entry.EntryKind != bonusEntryKindCorrection || entry.Points != -2 || entry.Visibility != bonusVisibilityPublic {
		t.Fatalf("unexpected manual adjustment entry: %+v", entry)
	}
	if entry.SourceGroupID != groupID {
		t.Fatalf("expected manual adjustment to preserve source group, got %+v", entry.SourceGroupID)
	}
}

func TestResolveActivityOccurrenceStirThePotWithoutAdvantage(t *testing.T) {
	instanceID := testUUID()
	activityID := testUUID()
	occurrenceID := testUUID()
	tangerineID := testUUID()
	aliceID := testUUID()
	bobID := testUUID()
	carolID := testUUID()
	effectiveAt := time.Date(2026, time.March, 25, 20, 0, 0, 0, time.UTC)

	fake := &fakeQuerier{
		activityOccurrence: db.GetActivityOccurrenceRow{
			ID:             occurrenceID,
			ActivityID:     activityID,
			OccurrenceType: "stir_the_pot_result",
			Name:           "Stir the Pot Round 1",
			EffectiveAt:    timestamptz(effectiveAt),
		},
		instanceActivity: db.GetInstanceActivityRow{
			ID:           activityID,
			InstanceID:   instanceID,
			ActivityType: "stir_the_pot",
			Name:         "Stir the Pot",
		},
		occurrenceParticipants: []db.ListActivityOccurrenceParticipantsRow{
			{ParticipantID: aliceID, ParticipantName: "Alice", ParticipantGroupID: tangerineID, ParticipantGroupName: textValue("Tangerine"), Metadata: []byte(`{"contribution":2}`)},
			{ParticipantID: bobID, ParticipantName: "Bob", ParticipantGroupID: tangerineID, ParticipantGroupName: textValue("Tangerine"), Metadata: []byte(`{"contribution":2}`)},
		},
		activeMembershipsByGroup: map[[16]byte][]db.ListActiveParticipantGroupMembershipsAtRow{
			tangerineID.Bytes: {
				{ParticipantGroupID: tangerineID, ParticipantID: aliceID, ParticipantName: "Alice"},
				{ParticipantGroupID: tangerineID, ParticipantID: bobID, ParticipantName: "Bob"},
				{ParticipantGroupID: tangerineID, ParticipantID: carolID, ParticipantName: "Carol"},
			},
		},
		activeAdvantagesByGroup: nil, // no advantages
	}

	service := NewService(fake)
	created, err := service.ResolveActivityOccurrence(context.Background(), occurrenceID)
	if err != nil {
		t.Fatalf("resolve activity occurrence: %v", err)
	}

	// 4 points contributed / 4 cost = 1 point each for 3 tribe members = 3 awards
	// Plus 2 spend entries for Alice and Bob
	if got := len(created); got != 5 {
		t.Fatalf("expected 5 created ledger entries, got %d", got)
	}

	var spends, awards int
	for _, entry := range fake.createdBonusLedgerEntries {
		if entry.EntryKind == "spend" {
			spends++
		} else if entry.EntryKind == "award" {
			awards++
			if entry.Points != 1 {
				t.Fatalf("expected award of 1 point, got %d", entry.Points)
			}
		}
	}
	if spends != 2 {
		t.Fatalf("expected 2 spend entries, got %d", spends)
	}
	if awards != 3 {
		t.Fatalf("expected 3 award entries, got %d", awards)
	}
}

func TestResolveActivityOccurrenceStirThePotWithAdvantage(t *testing.T) {
	instanceID := testUUID()
	activityID := testUUID()
	occurrenceID := testUUID()
	leafID := testUUID()
	aliceID := testUUID()
	bobID := testUUID()
	effectiveAt := time.Date(2026, time.March, 25, 20, 0, 0, 0, time.UTC)

	fake := &fakeQuerier{
		activityOccurrence: db.GetActivityOccurrenceRow{
			ID:             occurrenceID,
			ActivityID:     activityID,
			OccurrenceType: "stir_the_pot_result",
			Name:           "Stir the Pot Round 1",
			EffectiveAt:    timestamptz(effectiveAt),
		},
		instanceActivity: db.GetInstanceActivityRow{
			ID:           activityID,
			InstanceID:   instanceID,
			ActivityType: "stir_the_pot",
			Name:         "Stir the Pot",
		},
		occurrenceParticipants: []db.ListActivityOccurrenceParticipantsRow{
			{ParticipantID: aliceID, ParticipantName: "Alice", ParticipantGroupID: leafID, ParticipantGroupName: textValue("Leaf"), Metadata: []byte(`{"contribution":2}`)},
			{ParticipantID: bobID, ParticipantName: "Bob", ParticipantGroupID: leafID, ParticipantGroupName: textValue("Leaf"), Metadata: []byte(`{"contribution":1}`)},
		},
		activeMembershipsByGroup: map[[16]byte][]db.ListActiveParticipantGroupMembershipsAtRow{
			leafID.Bytes: {
				{ParticipantGroupID: leafID, ParticipantID: aliceID, ParticipantName: "Alice"},
				{ParticipantGroupID: leafID, ParticipantID: bobID, ParticipantName: "Bob"},
			},
		},
		activeAdvantagesByGroup: []db.ListActiveAdvantagesByTypeForGroupRow{
			{ID: testUUID(), InstanceID: instanceID, ParticipantGroupID: leafID, AdvantageType: "stir_the_pot_advantage"},
		},
	}

	service := NewService(fake)
	created, err := service.ResolveActivityOccurrence(context.Background(), occurrenceID)
	if err != nil {
		t.Fatalf("resolve activity occurrence: %v", err)
	}

	// 3 points contributed / 3 cost (advantage!) = 1 point each for 2 tribe members = 2 awards
	// Plus 2 spend entries for Alice and Bob
	if got := len(created); got != 4 {
		t.Fatalf("expected 4 created ledger entries, got %d", got)
	}

	var spends, awards int
	for _, entry := range fake.createdBonusLedgerEntries {
		if entry.EntryKind == "spend" {
			spends++
		} else if entry.EntryKind == "award" {
			awards++
			if entry.Points != 1 {
				t.Fatalf("expected award of 1 point, got %d", entry.Points)
			}
		}
	}
	if spends != 2 {
		t.Fatalf("expected 2 spend entries, got %d", spends)
	}
	if awards != 2 {
		t.Fatalf("expected 2 award entries, got %d", awards)
	}
}

func TestResolveActivityOccurrenceStirThePotInsufficientContribution(t *testing.T) {
	instanceID := testUUID()
	activityID := testUUID()
	occurrenceID := testUUID()
	tangerineID := testUUID()
	aliceID := testUUID()
	effectiveAt := time.Date(2026, time.March, 25, 20, 0, 0, 0, time.UTC)

	fake := &fakeQuerier{
		activityOccurrence: db.GetActivityOccurrenceRow{
			ID:             occurrenceID,
			ActivityID:     activityID,
			OccurrenceType: "stir_the_pot_result",
			Name:           "Stir the Pot Round 1",
			EffectiveAt:    timestamptz(effectiveAt),
		},
		instanceActivity: db.GetInstanceActivityRow{
			ID:           activityID,
			InstanceID:   instanceID,
			ActivityType: "stir_the_pot",
			Name:         "Stir the Pot",
		},
		occurrenceParticipants: []db.ListActivityOccurrenceParticipantsRow{
			{ParticipantID: aliceID, ParticipantName: "Alice", ParticipantGroupID: tangerineID, ParticipantGroupName: textValue("Tangerine"), Metadata: []byte(`{"contribution":3}`)},
		},
		activeMembershipsByGroup: map[[16]byte][]db.ListActiveParticipantGroupMembershipsAtRow{
			tangerineID.Bytes: {
				{ParticipantGroupID: tangerineID, ParticipantID: aliceID, ParticipantName: "Alice"},
			},
		},
		activeAdvantagesByGroup: nil,
	}

	service := NewService(fake)
	created, err := service.ResolveActivityOccurrence(context.Background(), occurrenceID)
	if err != nil {
		t.Fatalf("resolve activity occurrence: %v", err)
	}

	// 3 points contributed / 4 cost = 0 reward, but 1 spend entry
	if got := len(created); got != 1 {
		t.Fatalf("expected 1 created ledger entry (spend only), got %d", got)
	}
	if fake.createdBonusLedgerEntries[0].EntryKind != "spend" || fake.createdBonusLedgerEntries[0].Points != -3 {
		t.Fatalf("expected spend of -3, got %+v", fake.createdBonusLedgerEntries[0])
	}
}

func textValue(value string) pgtype.Text {
	return pgtype.Text{String: value, Valid: true}
}
