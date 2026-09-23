package config

import "testing"

func TestParseTargetServerIDsPrefersAllowlistAndFallsBackToLegacy(t *testing.T) {
	got, err := parseTargetServerIDs("1078197143501819915, 521073437779689474", "999999999999999999")
	if err != nil {
		t.Fatalf("parse allowlist: %v", err)
	}
	if len(got) != 2 || got[0] != "1078197143501819915" || got[1] != "521073437779689474" {
		t.Fatalf("unexpected allowlist: %#v", got)
	}

	got, err = parseTargetServerIDs("", "521073437779689474")
	if err != nil || len(got) != 1 || got[0] != "521073437779689474" {
		t.Fatalf("legacy fallback: %#v, %v", got, err)
	}
}

func TestParseTargetServerIDsRejectsInvalidAndDuplicateIDs(t *testing.T) {
	for _, value := range []string{"not-a-guild", "0", "521073437779689474,521073437779689474", "521073437779689474,"} {
		if _, err := parseTargetServerIDs(value, ""); err == nil {
			t.Errorf("expected invalid target guild list %q to fail", value)
		}
	}
}

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
}
