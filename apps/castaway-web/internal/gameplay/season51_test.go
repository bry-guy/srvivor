package gameplay

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestResolveWordleIndividualAndTribeAverage(t *testing.T) {
	instanceID, activityID, occurrenceID := testUUID(), testUUID(), testUUID()
	savu, toka := testUUID(), testUUID()
	a, b, c, d, e := testUUID(), testUUID(), testUUID(), testUUID(), testUUID()
	row := func(p pgtype.UUID, name string, group pgtype.UUID, groupName string, guesses int) db.ListActivityOccurrenceParticipantsRow {
		return db.ListActivityOccurrenceParticipantsRow{ParticipantID: p, ParticipantName: name, ParticipantGroupID: group, ParticipantGroupName: textValue(groupName), Metadata: []byte(fmt.Sprintf(`{"guess_count":%d}`, guesses))}
	}
	fake := &fakeQuerier{
		activityOccurrence: db.GetActivityOccurrenceRow{ID: occurrenceID, ActivityID: activityID, OccurrenceType: "tribe_wordle", EffectiveAt: timestamptz(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC))},
		instanceActivity:   db.GetInstanceActivityRow{ID: activityID, InstanceID: instanceID, ActivityType: "tribe_wordle", Metadata: []byte(`{"scoring":"individual_and_tribe_average"}`)},
		// Savu average 3.5 (2, 5); Toka average 3 (3, 3); A and E tie for best individual at 2.
		occurrenceParticipants: []db.ListActivityOccurrenceParticipantsRow{
			row(a, "A", savu, "Savu", 2),
			row(b, "B", savu, "Savu", 5),
			row(c, "C", toka, "Toka", 3),
			row(d, "D", toka, "Toka", 3),
		},
		activeMembershipsByGroup: map[[16]byte][]db.ListActiveParticipantGroupMembershipsAtRow{
			toka.Bytes: {{ParticipantGroupID: toka, ParticipantID: c}, {ParticipantGroupID: toka, ParticipantID: d}, {ParticipantGroupID: toka, ParticipantID: e}},
		},
	}
	if _, err := NewService(fake).ResolveActivityOccurrence(context.Background(), occurrenceID); err != nil {
		t.Fatal(err)
	}
	got := map[[16]byte]int32{}
	for _, entry := range fake.createdBonusLedgerEntries {
		got[entry.ParticipantID.Bytes] += entry.Points
	}
	// A: best individual +2. C, D, E (non-submitter still in the tribe): tribe average +1. B: nothing.
	want := map[[16]byte]int32{a.Bytes: 2, c.Bytes: 1, d.Bytes: 1, e.Bytes: 1}
	if len(got) != len(want) {
		t.Fatalf("awards = %v, want %v", got, want)
	}
	for id, points := range want {
		if got[id] != points {
			t.Fatalf("awards = %v, want %v", got, want)
		}
	}
}

func TestResolveTribeChallengeAwardsWinningTribeMembers(t *testing.T) {
	for kind, points := range map[string]int32{"immunity": 2, "reward": 1} {
		instanceID, activityID, occurrenceID, savu := testUUID(), testUUID(), testUUID(), testUUID()
		a, b := testUUID(), testUUID()
		fake := &fakeQuerier{
			activityOccurrence: db.GetActivityOccurrenceRow{ID: occurrenceID, ActivityID: activityID, OccurrenceType: kind, EffectiveAt: timestamptz(time.Date(2026, 10, 1, 1, 0, 0, 0, time.UTC)), Metadata: []byte(fmt.Sprintf(`{"winning_tribe_ids":[%q]}`, pgUUIDString(savu)))},
			instanceActivity:   db.GetInstanceActivityRow{ID: activityID, InstanceID: instanceID, ActivityType: TribeChallengeActivityType},
			participantGroup:   db.GetParticipantGroupRow{ID: savu, InstanceID: instanceID, Name: "Savu", Kind: "tribe"},
			activeMembershipsByGroup: map[[16]byte][]db.ListActiveParticipantGroupMembershipsAtRow{
				savu.Bytes: {{ParticipantGroupID: savu, ParticipantID: a}, {ParticipantGroupID: savu, ParticipantID: b}},
			},
		}
		if _, err := NewService(fake).ResolveActivityOccurrence(context.Background(), occurrenceID); err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		if len(fake.createdBonusLedgerEntries) != 2 {
			t.Fatalf("%s: got %d entries", kind, len(fake.createdBonusLedgerEntries))
		}
		for _, entry := range fake.createdBonusLedgerEntries {
			if entry.Points != points || entry.Visibility != bonusVisibilityPublic {
				t.Fatalf("%s: unexpected entry %+v", kind, entry)
			}
		}
	}
}
