package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/castawordle"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type previewGameFields struct {
	InstanceID                   int64
	Name, Answer                 string
	OpensAt, CutoffAt, CreatedAt string
}

type previewPlayState struct {
	Guesses, Status, UpdatedAt string
}

func TestUpgradePreviewDictionaryCommand(t *testing.T) {
	if castawordle.ValidWord("JEFF") {
		t.Fatal("JEFF must be absent from the embedded SCOWL dictionary for this regression test")
	}
	ctx, pool := integrationPool(t)
	resetDatabase(t, ctx, pool)
	q := db.New(pool)
	instance := createInstanceForTest(t, ctx, q, "Dictionary command", 53)
	player := createParticipantForTest(t, ctx, q, instance.ID, "upgrade-player")
	now := time.Date(2026, 9, 30, 17, 0, 0, 0, time.UTC)
	createGame := func(name, answer, version string) db.CreateCastawordleGameRow {
		t.Helper()
		game, err := q.CreateCastawordleGame(ctx, db.CreateCastawordleGameParams{
			InstanceID: instance.ID, Name: name, Answer: answer, DictionaryVersion: version,
			OpensAt: pgtype.Timestamptz{Time: now, Valid: true}, CutoffAt: pgtype.Timestamptz{Time: now.Add(time.Hour), Valid: true},
		})
		if err != nil {
			t.Fatal(err)
		}
		return game
	}
	legacy := createGame("Legacy compatible", "COLOR", legacyPreviewDictionaryVersion)
	controlVersion := "another-dictionary-version"
	control := createGame("Other dictionary", "TORCH", controlVersion)
	progressTime := now.Add(time.Minute)
	if _, err := pool.Exec(ctx, `INSERT INTO castawordle_plays (game_id, participant_id, guesses, status, updated_at)
		SELECT g.id, p.id, $3::jsonb, 'in_progress', $4 FROM castawordle_games g
		JOIN participants p ON p.instance_id = g.instance_id
		WHERE g.public_id = $1 AND p.public_id = $2`, legacy.ID, player.ID, `["CRANE","SLATE"]`, progressTime); err != nil {
		t.Fatal(err)
	}
	legacyFields := getPreviewGameFields(t, ctx, pool, legacy.ID)
	legacyProgress := getPreviewPlayState(t, ctx, pool, legacy.ID, player.ID)
	controlFields := getPreviewGameFields(t, ctx, pool, control.ID)

	databaseURL := upgradeCommandDatabaseURL(t, pool)
	runUpgrade := func(apply bool, rollbackFile string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, "go", "run", "../../cmd/upgrade-preview-dictionary")
		if apply {
			cmd.Args = append(cmd.Args, "--apply", "--rollback-file="+rollbackFile)
		}
		cmd.Env = upgradeCommandEnv(databaseURL)
		return cmd.CombinedOutput()
	}
	if _, err := runUpgrade(false, ""); err != nil {
		t.Fatalf("read-only dictionary check failed: %v", err)
	}
	if got := getPreviewDictionaryVersion(t, ctx, pool, legacy.ID); got != legacyPreviewDictionaryVersion {
		t.Fatalf("dry run changed legacy version to %q", got)
	}
	if got := getPreviewGameFields(t, ctx, pool, legacy.ID); got != legacyFields {
		t.Fatalf("dry run changed legacy game metadata: %#v", got)
	}
	if got := getPreviewPlayState(t, ctx, pool, legacy.ID, player.ID); got != legacyProgress {
		t.Fatalf("dry run changed saved progress: %#v", got)
	}
	if got := getPreviewGameFields(t, ctx, pool, control.ID); got != controlFields || getPreviewDictionaryVersion(t, ctx, pool, control.ID) != controlVersion {
		t.Fatalf("dry run changed other-version game: %#v", got)
	}

	backupPath := filepath.Join(t.TempDir(), "rollback.json")
	if _, err := runUpgrade(true, backupPath); err != nil {
		t.Fatalf("compatible dictionary upgrade failed: %v", err)
	}
	if got := getPreviewDictionaryVersion(t, ctx, pool, legacy.ID); got != castawordle.DictionaryVersion {
		t.Fatalf("apply left legacy version at %q", got)
	}
	if got := getPreviewGameFields(t, ctx, pool, legacy.ID); got != legacyFields {
		t.Fatalf("apply changed non-dictionary game metadata: %#v", got)
	}
	if got := getPreviewPlayState(t, ctx, pool, legacy.ID, player.ID); got != legacyProgress {
		t.Fatalf("apply changed saved progress: %#v", got)
	}
	if got := getPreviewGameFields(t, ctx, pool, control.ID); got != controlFields || getPreviewDictionaryVersion(t, ctx, pool, control.ID) != controlVersion {
		t.Fatalf("apply changed other-version game: %#v", got)
	}
	backupInfo, err := os.Stat(backupPath)
	if err != nil {
		t.Fatal("successful apply did not create its rollback file")
	}
	if backupInfo.Mode().Perm() != 0o600 {
		t.Fatalf("rollback file mode is %o, want 600", backupInfo.Mode().Perm())
	}
	backup, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatal(err)
	}
	var rollback struct {
		From    string   `json:"from"`
		To      string   `json:"to"`
		GameIDs []string `json:"game_ids"`
	}
	decoder := json.NewDecoder(bytes.NewReader(backup))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&rollback); err != nil {
		t.Fatal(err)
	}
	if rollback.From != legacyPreviewDictionaryVersion || rollback.To != castawordle.DictionaryVersion || len(rollback.GameIDs) != 1 || rollback.GameIDs[0] != uuid.UUID(legacy.ID.Bytes).String() {
		t.Fatalf("rollback file does not contain only the upgraded game ID: %#v", rollback)
	}
	if strings.Contains(string(backup), "COLOR") || strings.Contains(string(backup), "CRANE") {
		t.Fatal("rollback file contains game answers or saved guesses")
	}

	blockedCandidate := createGame("Blocked candidate", "TORCH", legacyPreviewDictionaryVersion)
	incompatible := createGame("Incompatible answer", "JEFF", legacyPreviewDictionaryVersion)
	blockedCandidateFields := getPreviewGameFields(t, ctx, pool, blockedCandidate.ID)
	incompatibleFields := getPreviewGameFields(t, ctx, pool, incompatible.ID)
	blockedBackupPath := filepath.Join(t.TempDir(), "must-not-exist.json")
	output, err := runUpgrade(true, blockedBackupPath)
	if err == nil || !strings.Contains(string(output), "dictionary upgrade blocked") {
		t.Fatal("apply did not block the incompatible answer")
	}
	for _, game := range []struct {
		id     pgtype.UUID
		fields previewGameFields
	}{{blockedCandidate.ID, blockedCandidateFields}, {incompatible.ID, incompatibleFields}} {
		if got := getPreviewDictionaryVersion(t, ctx, pool, game.id); got != legacyPreviewDictionaryVersion {
			t.Fatalf("blocked apply changed dictionary version to %q", got)
		}
		if got := getPreviewGameFields(t, ctx, pool, game.id); got != game.fields {
			t.Fatalf("blocked apply changed game metadata: %#v", got)
		}
	}
	if _, err := os.Stat(blockedBackupPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("blocked apply created a rollback file")
	}
}

func upgradeCommandDatabaseURL(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	database := pool.Config().ConnConfig.Database
	if !strings.HasPrefix(database, "castaway_httpapi_test_") {
		t.Fatal("upgrade command must target the disposable integration database")
	}
	dsn := pool.Config().ConnString()
	if strings.HasPrefix(strings.ToLower(dsn), "postgres://") || strings.HasPrefix(strings.ToLower(dsn), "postgresql://") {
		parsed, err := url.Parse(dsn)
		if err != nil {
			t.Fatal("could not form a URI for the disposable integration database")
		}
		parsed.Path = "/" + database
		parsed.RawPath = ""
		query := parsed.Query()
		query.Set("dbname", database)
		parsed.RawQuery = query.Encode()
		dsn = parsed.String()
	} else {
		database = strings.ReplaceAll(database, `\`, `\\`)
		database = strings.ReplaceAll(database, `'`, `\'`)
		dsn = strings.TrimSpace(dsn) + " dbname='" + database + "'"
	}
	parsed, err := pgx.ParseConfig(dsn)
	if err != nil || parsed.Database != pool.Config().ConnConfig.Database {
		t.Fatal("upgrade command connection does not target the disposable integration database")
	}
	return dsn
}

func upgradeCommandEnv(databaseURL string) []string {
	env := make([]string, 0, len(os.Environ())+2)
	for _, value := range os.Environ() {
		key, _, _ := strings.Cut(value, "=")
		if key != "DATABASE_URL" && key != "PUBLIC_PORT" {
			env = append(env, value)
		}
	}
	return append(env, "DATABASE_URL="+databaseURL, "PUBLIC_PORT=")
}

func getPreviewDictionaryVersion(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id pgtype.UUID) string {
	t.Helper()
	var version string
	if err := pool.QueryRow(ctx, `SELECT dictionary_version FROM castawordle_games WHERE public_id = $1`, id).Scan(&version); err != nil {
		t.Fatal(err)
	}
	return version
}

func getPreviewGameFields(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id pgtype.UUID) previewGameFields {
	t.Helper()
	var fields previewGameFields
	if err := pool.QueryRow(ctx, `SELECT instance_id, name, answer, opens_at::text, cutoff_at::text, created_at::text
		FROM castawordle_games WHERE public_id = $1`, id).Scan(
		&fields.InstanceID, &fields.Name, &fields.Answer, &fields.OpensAt, &fields.CutoffAt, &fields.CreatedAt,
	); err != nil {
		t.Fatal(err)
	}
	return fields
}

func getPreviewPlayState(t *testing.T, ctx context.Context, pool *pgxpool.Pool, gameID, playerID pgtype.UUID) previewPlayState {
	t.Helper()
	var state previewPlayState
	if err := pool.QueryRow(ctx, `SELECT guesses::text, status, updated_at::text FROM castawordle_plays
		WHERE game_id = (SELECT id FROM castawordle_games WHERE public_id = $1)
		AND participant_id = (SELECT id FROM participants WHERE public_id = $2)`, gameID, playerID).Scan(
		&state.Guesses, &state.Status, &state.UpdatedAt,
	); err != nil {
		t.Fatal(err)
	}
	return state
}
