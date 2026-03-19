package state

import (
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"
	pgxmock "github.com/pashagolub/pgxmock/v4"
)

func TestBoltStoreGuildDefaults(t *testing.T) {
	store := openTestBoltStore(t)
	defer store.Close()

	if err := store.SetGuildDefault("guild-1", "instance-1"); err != nil {
		t.Fatalf("set guild default: %v", err)
	}
	got, err := store.GetGuildDefault("guild-1")
	if err != nil {
		t.Fatalf("get guild default: %v", err)
	}
	if got != "instance-1" {
		t.Fatalf("unexpected guild default: %q", got)
	}
	if err := store.ClearGuildDefault("guild-1"); err != nil {
		t.Fatalf("clear guild default: %v", err)
	}
	got, err = store.GetGuildDefault("guild-1")
	if err != nil {
		t.Fatalf("get cleared guild default: %v", err)
	}
	if got != "" {
		t.Fatalf("expected cleared guild default, got %q", got)
	}
}

func TestBoltStoreUserDefaultsAreScopedByGuild(t *testing.T) {
	store := openTestBoltStore(t)
	defer store.Close()

	if err := store.SetUserDefault("guild-1", "user-1", "instance-a"); err != nil {
		t.Fatalf("set user default: %v", err)
	}
	if err := store.SetUserDefault("guild-2", "user-1", "instance-b"); err != nil {
		t.Fatalf("set second user default: %v", err)
	}

	gotA, err := store.GetUserDefault("guild-1", "user-1")
	if err != nil {
		t.Fatalf("get user default A: %v", err)
	}
	gotB, err := store.GetUserDefault("guild-2", "user-1")
	if err != nil {
		t.Fatalf("get user default B: %v", err)
	}
	if gotA != "instance-a" || gotB != "instance-b" {
		t.Fatalf("unexpected user defaults: %q %q", gotA, gotB)
	}
}

func TestPostgresStoreGuildDefaults(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("new pgxmock pool: %v", err)
	}
	defer mock.Close()

	store := &PostgresStore{pool: mock}

	mock.ExpectExec("INSERT INTO guild_defaults").WithArgs("guild-1", "instance-1").WillReturnResult(pgxmock.NewResult("INSERT", 1))
	if err := store.SetGuildDefault("guild-1", "instance-1"); err != nil {
		t.Fatalf("set guild default: %v", err)
	}

	mock.ExpectQuery("SELECT instance_id").WithArgs("guild-1").WillReturnRows(pgxmock.NewRows([]string{"instance_id"}).AddRow("instance-1"))
	got, err := store.GetGuildDefault("guild-1")
	if err != nil {
		t.Fatalf("get guild default: %v", err)
	}
	if got != "instance-1" {
		t.Fatalf("unexpected guild default: %q", got)
	}

	mock.ExpectExec("DELETE FROM guild_defaults").WithArgs("guild-1").WillReturnResult(pgxmock.NewResult("DELETE", 1))
	if err := store.ClearGuildDefault("guild-1"); err != nil {
		t.Fatalf("clear guild default: %v", err)
	}

	mock.ExpectQuery("SELECT instance_id").WithArgs("guild-1").WillReturnError(pgx.ErrNoRows)
	got, err = store.GetGuildDefault("guild-1")
	if err != nil {
		t.Fatalf("get cleared guild default: %v", err)
	}
	if got != "" {
		t.Fatalf("expected cleared guild default, got %q", got)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestPostgresStoreUserDefaultsAreScopedByGuild(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("new pgxmock pool: %v", err)
	}
	defer mock.Close()

	store := &PostgresStore{pool: mock}

	mock.ExpectExec("INSERT INTO user_defaults").WithArgs("guild-1", "user-1", "instance-a").WillReturnResult(pgxmock.NewResult("INSERT", 1))
	if err := store.SetUserDefault("guild-1", "user-1", "instance-a"); err != nil {
		t.Fatalf("set user default A: %v", err)
	}
	mock.ExpectExec("INSERT INTO user_defaults").WithArgs("guild-2", "user-1", "instance-b").WillReturnResult(pgxmock.NewResult("INSERT", 1))
	if err := store.SetUserDefault("guild-2", "user-1", "instance-b"); err != nil {
		t.Fatalf("set user default B: %v", err)
	}

	mock.ExpectQuery("SELECT instance_id").WithArgs("guild-1", "user-1").WillReturnRows(pgxmock.NewRows([]string{"instance_id"}).AddRow("instance-a"))
	gotA, err := store.GetUserDefault("guild-1", "user-1")
	if err != nil {
		t.Fatalf("get user default A: %v", err)
	}
	mock.ExpectQuery("SELECT instance_id").WithArgs("guild-2", "user-1").WillReturnRows(pgxmock.NewRows([]string{"instance_id"}).AddRow("instance-b"))
	gotB, err := store.GetUserDefault("guild-2", "user-1")
	if err != nil {
		t.Fatalf("get user default B: %v", err)
	}
	if gotA != "instance-a" || gotB != "instance-b" {
		t.Fatalf("unexpected user defaults: %q %q", gotA, gotB)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestSplitUserKey(t *testing.T) {
	guildID, userID, ok := splitUserKey("guild-1:user-2")
	if !ok {
		t.Fatal("expected valid split")
	}
	if guildID != "guild-1" || userID != "user-2" {
		t.Fatalf("unexpected split values: %q %q", guildID, userID)
	}

	if _, _, ok := splitUserKey("invalid"); ok {
		t.Fatal("expected invalid split for malformed key")
	}
}

func openTestBoltStore(t *testing.T) *BoltStore {
	t.Helper()
	store, err := OpenBolt(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("open state store: %v", err)
	}
	return store
}
