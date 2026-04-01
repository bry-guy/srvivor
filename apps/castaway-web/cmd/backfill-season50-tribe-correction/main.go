package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/app"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/config"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/gameplay"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("backfill-season50-tribe-correction: %v", err)
	}
}

type groupSpec struct {
	Name    string
	Members []string
}

type adjustmentParticipant struct {
	Name   string
	Points int32
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("create db pool: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}
	if err := app.RunMigrations(ctx, pool, cfg.MigrationsDir); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil && rollbackErr != pgx.ErrTxClosed {
			log.Printf("backfill-season50-tribe-correction: rollback tx: %v", rollbackErr)
		}
	}()

	q := db.New(tx)
	svc := gameplay.NewService(q)

	instance, err := season50Instance(ctx, q)
	if err != nil {
		return err
	}

	participantByName, err := participantMap(ctx, q, instance.ID)
	if err != nil {
		return err
	}

	groups := []groupSpec{
		{Name: "Tangerine", Members: []string{"Adam", "Grant", "Kyle", "Kate", "Keith"}},
		{Name: "Leaf", Members: []string{"Bryan", "Lauren", "Amanda", "Yacob", "Riley", "Mooney"}},
		{Name: "Lotus", Members: []string{"Katie", "Kenny", "Marv", "Keeling", "Sarah"}},
	}
	preseasonStart := gameplay.DefaultEpisodeScheduleForSeason(50)[0].AirsAt
	groupByName, err := ensureGroups(ctx, q, svc, instance.ID, participantByName, groups, preseasonStart)
	if err != nil {
		return err
	}
	if err := correctMemberships(ctx, tx, instance.ID, participantByName, groupByName, groups, preseasonStart, preseasonStart.Add(time.Second)); err != nil {
		return err
	}

	correctionActivity, err := ensureActivity(ctx, q, instance.ID, "manual_adjustment", "Season 50 Tribe Correction", "completed", mustTime("2026-03-25T20:00:00-04:00"))
	if err != nil {
		return err
	}

	if _, err := ensureManualAdjustmentOccurrence(ctx, svc, q, correctionActivity.ID, "Season 50 tribe correction — Week 4 public fixes", mustTime("2026-03-25T20:00:00-04:00"), []adjustmentParticipant{
		{Name: "Kate", Points: 2},
		{Name: "Mooney", Points: -2},
		{Name: "Kenny", Points: 1},
		{Name: "Marv", Points: 1},
		{Name: "Keeling", Points: 1},
		{Name: "Sarah", Points: 1},
		{Name: "Amanda", Points: -1},
		{Name: "Yacob", Points: -1},
		{Name: "Lauren", Points: -1},
		{Name: "Bryan", Points: -1},
	}, "public", "Season 50 tribe correction — Week 4 public fixes", "season50:tribe-correction-week4-public", participantByName); err != nil {
		return err
	}

	if _, err := ensureManualAdjustmentOccurrence(ctx, svc, q, correctionActivity.ID, "Season 50 tribe correction — Week 4 secret fixes", mustTime("2026-03-25T20:00:01-04:00"), []adjustmentParticipant{
		{Name: "Bryan", Points: 1},
		{Name: "Lauren", Points: 1},
		{Name: "Amanda", Points: 1},
		{Name: "Yacob", Points: 1},
		{Name: "Mooney", Points: 1},
		{Name: "Keeling", Points: -1},
		{Name: "Kate", Points: -1},
		{Name: "Kenny", Points: -1},
		{Name: "Sarah", Points: -1},
		{Name: "Marv", Points: -1},
	}, "secret", "Season 50 tribe correction — Week 4 secret fixes", "season50:tribe-correction-week4-secret", participantByName); err != nil {
		return err
	}

	if err := expireWrongLoanSharkAdvantages(ctx, tx, instance.ID, participantByName, []string{"Keeling", "Kate", "Kenny", "Sarah", "Marv"}); err != nil {
		return err
	}
	for _, name := range []string{"Bryan", "Lauren", "Amanda", "Yacob", "Riley", "Mooney"} {
		participantID, ok := participantByName[normalize(name)]
		if !ok {
			return fmt.Errorf("resolve participant %q for corrected advantage", name)
		}
		if err := ensureAdvantage(ctx, q, instance.ID, participantID, groupByName["Leaf"].ID, pgtype.UUID{}, name); err != nil {
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	fmt.Println("season 50 tribe correction backfill complete")
	return nil
}

func season50Instance(ctx context.Context, q *db.Queries) (db.ListInstancesRow, error) {
	instances, err := q.ListInstances(ctx)
	if err != nil {
		return db.ListInstancesRow{}, fmt.Errorf("list instances: %w", err)
	}
	matches := make([]db.ListInstancesRow, 0)
	for _, instance := range instances {
		if instance.Season == 50 {
			matches = append(matches, instance)
		}
	}
	if len(matches) != 1 {
		return db.ListInstancesRow{}, fmt.Errorf("expected exactly one season 50 instance, found %d", len(matches))
	}
	return matches[0], nil
}

func participantMap(ctx context.Context, q *db.Queries, instanceID pgtype.UUID) (map[string]pgtype.UUID, error) {
	participants, err := q.ListParticipantsByInstance(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("list participants: %w", err)
	}
	result := make(map[string]pgtype.UUID, len(participants))
	for _, p := range participants {
		result[normalize(p.Name)] = p.ID
	}
	return result, nil
}

func ensureGroups(ctx context.Context, q *db.Queries, svc *gameplay.Service, instanceID pgtype.UUID, participantByName map[string]pgtype.UUID, specs []groupSpec, startsAt time.Time) (map[string]db.ListParticipantGroupsByInstanceRow, error) {
	existing, err := q.ListParticipantGroupsByInstance(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("list participant groups: %w", err)
	}
	groupByName := make(map[string]db.ListParticipantGroupsByInstanceRow, len(existing))
	for _, group := range existing {
		groupByName[group.Name] = group
	}

	for _, spec := range specs {
		group, ok := groupByName[spec.Name]
		if !ok {
			created, err := q.CreateParticipantGroup(ctx, db.CreateParticipantGroupParams{
				InstanceID: instanceID,
				Name:       spec.Name,
				Kind:       "tribe",
				Metadata:   []byte("{}"),
			})
			if err != nil {
				return nil, fmt.Errorf("create group %q: %w", spec.Name, err)
			}
			group = db.ListParticipantGroupsByInstanceRow(created)
			groupByName[spec.Name] = group
			fmt.Printf("created group %s\n", spec.Name)
		}

		periods, err := q.ListParticipantGroupMembershipPeriods(ctx, group.ID)
		if err != nil {
			return nil, fmt.Errorf("list memberships for group %q: %w", spec.Name, err)
		}
		existingMembership := make(map[string]bool, len(periods))
		for _, period := range periods {
			existingMembership[string(period.ParticipantID.Bytes[:])+period.StartsAt.Time.UTC().Format(time.RFC3339)] = true
		}

		for _, memberName := range spec.Members {
			participantID, ok := participantByName[normalize(memberName)]
			if !ok {
				return nil, fmt.Errorf("resolve participant %q for group %q", memberName, spec.Name)
			}
			key := string(participantID.Bytes[:]) + startsAt.UTC().Format(time.RFC3339)
			if existingMembership[key] {
				continue
			}
			if _, err := svc.CreateMembershipPeriod(ctx, gameplay.CreateMembershipPeriodParams{
				ParticipantGroupID: group.ID,
				ParticipantID:      participantID,
				Role:               "member",
				StartsAt:           startsAt,
			}); err != nil {
				return nil, fmt.Errorf("create membership %q in group %q: %w", memberName, spec.Name, err)
			}
			fmt.Printf("created membership %s -> %s\n", memberName, spec.Name)
		}
	}

	return groupByName, nil
}

func correctMemberships(ctx context.Context, tx pgx.Tx, instanceID pgtype.UUID, participantByName map[string]pgtype.UUID, groupByName map[string]db.ListParticipantGroupsByInstanceRow, specs []groupSpec, startsAt, endsAt time.Time) error {
	desiredByGroup := make(map[string]map[string]struct{}, len(specs))
	for _, spec := range specs {
		members := make(map[string]struct{}, len(spec.Members))
		for _, name := range spec.Members {
			members[normalize(name)] = struct{}{}
		}
		desiredByGroup[spec.Name] = members
	}

	for groupName, group := range groupByName {
		if group.Kind != "tribe" {
			continue
		}
		desired, ok := desiredByGroup[groupName]
		if !ok {
			continue
		}
		memberships, err := db.New(tx).ListParticipantGroupMembershipPeriods(ctx, group.ID)
		if err != nil {
			return fmt.Errorf("list memberships for correction in group %q: %w", groupName, err)
		}
		for _, membership := range memberships {
			if membership.Role != "member" || !membership.StartsAt.Valid || !membership.StartsAt.Time.Equal(startsAt) {
				continue
			}
			participantName, ok := participantNameByID(participantByName, membership.ParticipantID)
			if !ok {
				continue
			}
			if _, keep := desired[participantName]; keep {
				continue
			}
			if membership.EndsAt.Valid && !membership.EndsAt.Time.After(endsAt) {
				continue
			}
			if _, err := tx.Exec(ctx, `
UPDATE participant_group_membership_periods
SET ends_at = $1
WHERE participant_group_id = (
    SELECT pg.id
    FROM participant_groups pg
    JOIN instances i ON i.id = pg.instance_id
    WHERE i.public_id = $2
      AND pg.public_id = $3
)
  AND participant_id = (
    SELECT p.id
    FROM participants p
    JOIN instances i ON i.id = p.instance_id
    WHERE i.public_id = $2
      AND p.public_id = $4
)
  AND role = 'member'
  AND starts_at = $5
  AND (ends_at IS NULL OR ends_at > $1)
`, endsAt, instanceID, group.ID, membership.ParticipantID, startsAt); err != nil {
				return fmt.Errorf("end incorrect membership %q in %q: %w", participantName, groupName, err)
			}
			fmt.Printf("ended incorrect membership %s -> %s\n", displayName(participantName), groupName)
		}
	}

	return nil
}

func participantNameByID(participantByName map[string]pgtype.UUID, id pgtype.UUID) (string, bool) {
	for name, candidate := range participantByName {
		if candidate == id {
			return name, true
		}
	}
	return "", false
}

func displayName(normalized string) string {
	if normalized == "" {
		return normalized
	}
	return strings.ToUpper(normalized[:1]) + normalized[1:]
}

func ensureActivity(ctx context.Context, q *db.Queries, instanceID pgtype.UUID, activityType, name, status string, startsAt time.Time) (db.ListInstanceActivitiesByInstanceRow, error) {
	activities, err := q.ListInstanceActivitiesByInstance(ctx, instanceID)
	if err != nil {
		return db.ListInstanceActivitiesByInstanceRow{}, fmt.Errorf("list instance activities: %w", err)
	}
	for _, activity := range activities {
		if activity.ActivityType == activityType && activity.Name == name {
			return activity, nil
		}
	}
	created, err := q.CreateInstanceActivity(ctx, db.CreateInstanceActivityParams{
		InstanceID:   instanceID,
		ActivityType: activityType,
		Name:         name,
		Status:       status,
		StartsAt:     timestamptz(startsAt),
		Metadata:     []byte("{}"),
	})
	if err != nil {
		return db.ListInstanceActivitiesByInstanceRow{}, fmt.Errorf("create activity %q: %w", name, err)
	}
	fmt.Printf("created activity %s\n", name)
	return db.ListInstanceActivitiesByInstanceRow(created), nil
}

func ensureManualAdjustmentOccurrence(ctx context.Context, svc *gameplay.Service, q *db.Queries, activityID pgtype.UUID, name string, effectiveAt time.Time, participants []adjustmentParticipant, visibility, reason, awardKey string, participantByName map[string]pgtype.UUID) (db.ListActivityOccurrencesByActivityRow, error) {
	occurrences, err := q.ListActivityOccurrencesByActivity(ctx, activityID)
	if err != nil {
		return db.ListActivityOccurrencesByActivityRow{}, fmt.Errorf("list activity occurrences for %q: %w", name, err)
	}
	for _, occurrence := range occurrences {
		if occurrence.Name == name {
			return occurrence, nil
		}
	}

	occurrence, err := q.CreateActivityOccurrence(ctx, db.CreateActivityOccurrenceParams{
		ActivityID:     activityID,
		OccurrenceType: "manual_correction",
		Name:           name,
		EffectiveAt:    timestamptz(effectiveAt),
		Status:         "resolved",
		Metadata:       []byte("{}"),
	})
	if err != nil {
		return db.ListActivityOccurrencesByActivityRow{}, fmt.Errorf("create occurrence %q: %w", name, err)
	}

	for _, participant := range participants {
		participantID, ok := participantByName[normalize(participant.Name)]
		if !ok {
			return db.ListActivityOccurrencesByActivityRow{}, fmt.Errorf("resolve participant %q for occurrence %q", participant.Name, name)
		}
		metadata, err := json.Marshal(map[string]any{
			"points":     participant.Points,
			"visibility": visibility,
			"reason":     reason,
			"award_key":  awardKey,
		})
		if err != nil {
			return db.ListActivityOccurrencesByActivityRow{}, fmt.Errorf("marshal participant metadata for %q: %w", participant.Name, err)
		}
		if _, err := q.CreateActivityOccurrenceParticipant(ctx, db.CreateActivityOccurrenceParticipantParams{
			ActivityOccurrenceID: occurrence.ID,
			ParticipantID:        participantID,
			Role:                 "adjustment",
			Metadata:             metadata,
		}); err != nil {
			return db.ListActivityOccurrencesByActivityRow{}, fmt.Errorf("create occurrence participant %q for %q: %w", participant.Name, name, err)
		}
	}

	if _, err := svc.ResolveActivityOccurrence(ctx, occurrence.ID); err != nil {
		return db.ListActivityOccurrencesByActivityRow{}, fmt.Errorf("resolve occurrence %q: %w", name, err)
	}
	fmt.Printf("created and resolved occurrence %s\n", name)
	return db.ListActivityOccurrencesByActivityRow(occurrence), nil
}

func expireWrongLoanSharkAdvantages(ctx context.Context, tx pgx.Tx, instanceID pgtype.UUID, participantByName map[string]pgtype.UUID, names []string) error {
	for _, name := range names {
		participantID, ok := participantByName[normalize(name)]
		if !ok {
			return fmt.Errorf("resolve participant %q for advantage expiry", name)
		}
		commandTag, err := tx.Exec(ctx, `
UPDATE participant_advantages pa
SET status = 'expired',
    updated_at = NOW(),
    metadata = pa.metadata || jsonb_build_object('expired_by', 'backfill-season50-tribe-correction')
FROM instances i, participants p
WHERE pa.instance_id = i.id
  AND pa.participant_id = p.id
  AND i.public_id = $1
  AND p.public_id = $2
  AND pa.advantage_type = 'stir_the_pot_advantage'
  AND pa.name = 'Stir the Pot Advantage Scroll'
  AND pa.status = 'active'
`, instanceID, participantID)
		if err != nil {
			return fmt.Errorf("expire wrong advantage for %q: %w", name, err)
		}
		if commandTag.RowsAffected() > 0 {
			fmt.Printf("expired incorrect stir the pot advantage for %s\n", name)
		}
	}
	return nil
}

func ensureAdvantage(ctx context.Context, q *db.Queries, instanceID, participantID, groupID, sourceOccurrenceID pgtype.UUID, participantName string) error {
	active, err := q.ListActiveAdvantagesByTypeForParticipant(ctx, db.ListActiveAdvantagesByTypeForParticipantParams{
		InstanceID:    instanceID,
		ParticipantID: participantID,
		AdvantageType: "stir_the_pot_advantage",
		At:            timestamptz(mustTime("2026-03-25T20:00:00-04:00")),
	})
	if err != nil {
		return fmt.Errorf("list active advantages for %q: %w", participantName, err)
	}
	if len(active) > 0 {
		return nil
	}
	if _, err := q.CreateParticipantAdvantage(ctx, db.CreateParticipantAdvantageParams{
		InstanceID:                 instanceID,
		ParticipantID:              participantID,
		ParticipantGroupID:         groupID,
		AdvantageType:              "stir_the_pot_advantage",
		Name:                       "Stir the Pot Advantage Scroll",
		Status:                     "active",
		SourceActivityOccurrenceID: sourceOccurrenceID,
		GrantedAt:                  timestamptz(mustTime("2026-03-19T01:02:00Z")),
		EffectiveAt:                timestamptz(mustTime("2026-03-25T20:00:00-04:00")),
		EffectiveUntil:             optionalTimestamptz(ptrTime(mustTime("2026-04-08T20:00:00-04:00"))),
		Metadata:                   []byte("{}"),
	}); err != nil {
		return fmt.Errorf("create advantage for %q: %w", participantName, err)
	}
	fmt.Printf("created stir the pot advantage for %s\n", participantName)
	return nil
}

func normalize(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func mustTime(raw string) time.Time {
	value, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		panic(err)
	}
	return value
}

func ptrTime(value time.Time) *time.Time {
	return &value
}

func timestamptz(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}

func optionalTimestamptz(value *time.Time) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}
	return timestamptz(*value)
}
