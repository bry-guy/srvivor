package config

import (
	"reflect"
	"testing"
)

func TestLoadServiceAuthDefaults(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.ServiceAuthEnabled {
		t.Fatalf("expected service auth disabled by default")
	}
	if len(cfg.ServiceAuthBearerTokens) != 0 {
		t.Fatalf("expected no default service auth tokens, got %v", cfg.ServiceAuthBearerTokens)
	}
	if cfg.ServiceAuthPrincipal != "castaway-discord-bot" {
		t.Fatalf("service auth principal = %q, want %q", cfg.ServiceAuthPrincipal, "castaway-discord-bot")
	}
}

func TestLoadRequiresBearerTokensWhenServiceAuthEnabled(t *testing.T) {
	t.Setenv("SERVICE_AUTH_ENABLED", "true")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when service auth enabled without bearer tokens")
	}
}

func TestLoadParsesBearerTokens(t *testing.T) {
	t.Setenv("SERVICE_AUTH_ENABLED", "true")
	t.Setenv("SERVICE_AUTH_BEARER_TOKENS", " token-a, token-b ,token-a ,, ")
	t.Setenv("SERVICE_AUTH_PRINCIPAL", "castaway-discord-bot")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	want := []string{"token-a", "token-b"}
	if !reflect.DeepEqual(cfg.ServiceAuthBearerTokens, want) {
		t.Fatalf("service auth bearer tokens = %v, want %v", cfg.ServiceAuthBearerTokens, want)
	}
	if cfg.ServiceAuthPrincipal != "castaway-discord-bot" {
		t.Fatalf("service auth principal = %q, want %q", cfg.ServiceAuthPrincipal, "castaway-discord-bot")
	}
}
