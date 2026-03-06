package state

import (
	"path/filepath"
	"testing"
)

func TestStoreGuildDefaults(t *testing.T) {
	store := openTestStore(t)
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

func TestStoreUserDefaultsAreScopedByGuild(t *testing.T) {
	store := openTestStore(t)
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

func openTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("open state store: %v", err)
	}
	return store
}
