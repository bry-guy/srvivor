package config

import "testing"

func TestLoadDefaultsToBoltState(t *testing.T) {
	t.Setenv("CASTAWAY_DISCORD_BOT_TOKEN", "token")
	t.Setenv("CASTAWAY_DISCORD_APPLICATION_ID", "app-id")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.StateBackend != "bolt" {
		t.Fatalf("expected bolt backend, got %q", cfg.StateBackend)
	}
	if cfg.StatePath == "" {
		t.Fatal("expected default state path")
	}
}

func TestLoadRequiresPostgresURLForPostgresBackend(t *testing.T) {
	t.Setenv("CASTAWAY_DISCORD_BOT_TOKEN", "token")
	t.Setenv("CASTAWAY_DISCORD_APPLICATION_ID", "app-id")
	t.Setenv("BOT_STATE_BACKEND", "postgres")
	t.Setenv("BOT_STATE_DATABASE_URL", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected config error")
	}
	if got := err.Error(); got != "BOT_STATE_DATABASE_URL is required when BOT_STATE_BACKEND=postgres" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadAcceptsPostgresBackendAndAPIAuthToken(t *testing.T) {
	t.Setenv("CASTAWAY_DISCORD_BOT_TOKEN", "token")
	t.Setenv("CASTAWAY_DISCORD_APPLICATION_ID", "app-id")
	t.Setenv("BOT_STATE_BACKEND", "postgres")
	t.Setenv("BOT_STATE_DATABASE_URL", "postgres://bot:secret@localhost:5432/castaway_discord_bot?sslmode=disable")
	t.Setenv("CASTAWAY_API_AUTH_TOKEN", "shared-token")
	t.Setenv("DISCORD_ADMIN_USER_IDS", " admin-1, admin-2 ,admin-1 ,, ")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.StateBackend != "postgres" {
		t.Fatalf("expected postgres backend, got %q", cfg.StateBackend)
	}
	if cfg.CastawayAPIAuthToken != "shared-token" {
		t.Fatalf("unexpected api auth token: %q", cfg.CastawayAPIAuthToken)
	}
	if len(cfg.DiscordAdminUserIDs) != 2 || cfg.DiscordAdminUserIDs[0] != "admin-1" || cfg.DiscordAdminUserIDs[1] != "admin-2" {
		t.Fatalf("unexpected discord admin user ids: %v", cfg.DiscordAdminUserIDs)
	}
}
