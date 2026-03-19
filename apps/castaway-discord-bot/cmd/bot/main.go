package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bry-guy/srvivor/apps/castaway-discord-bot/internal/buildinfo"
	"github.com/bry-guy/srvivor/apps/castaway-discord-bot/internal/castaway"
	"github.com/bry-guy/srvivor/apps/castaway-discord-bot/internal/config"
	discordbot "github.com/bry-guy/srvivor/apps/castaway-discord-bot/internal/discord"
	"github.com/bry-guy/srvivor/apps/castaway-discord-bot/internal/state"
)

func main() {
	if err := run(); err != nil {
		slog.Error("castaway-discord-bot failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	checkConfig := flag.Bool("check-config", false, "validate configuration and state setup, then exit")
	showVersion := flag.Bool("version", false, "print version and exit")
	importBoltStateFrom := flag.String("import-bolt-state-from", "", "import guild/user defaults from a BoltDB file into the configured postgres state backend, then exit")
	flag.Parse()
	if *showVersion {
		fmt.Println(buildinfo.String())
		return nil
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)

	if sourcePath := strings.TrimSpace(*importBoltStateFrom); sourcePath != "" {
		if state.Backend(cfg.StateBackend) != state.BackendPostgres {
			return fmt.Errorf("--import-bolt-state-from requires BOT_STATE_BACKEND=postgres")
		}
		result, err := state.ImportBoltToPostgres(context.Background(), sourcePath, cfg.StateDatabaseURL)
		if err != nil {
			return fmt.Errorf("import bolt state: %w", err)
		}
		logger.Info("imported bolt state into postgres", "guild_defaults", result.GuildDefaultsImported, "user_defaults", result.UserDefaultsImported)
		return nil
	}

	store, err := state.Open(context.Background(), state.Options{
		Backend:     state.Backend(cfg.StateBackend),
		BoltPath:    cfg.StatePath,
		PostgresURL: cfg.StateDatabaseURL,
		AutoMigrate: true,
	})
	if err != nil {
		return fmt.Errorf("open state store: %w", err)
	}
	defer store.Close()

	client, err := castaway.NewClient(cfg.CastawayAPIBaseURL, &http.Client{Timeout: 10 * time.Second}, castaway.Options{BearerToken: cfg.CastawayAPIAuthToken})
	if err != nil {
		return fmt.Errorf("create castaway client: %w", err)
	}

	if *checkConfig {
		attrs := []any{"version", buildinfo.String(), "api_base_url", cfg.CastawayAPIBaseURL, "state_backend", cfg.StateBackend}
		if state.Backend(cfg.StateBackend) == state.BackendBolt {
			attrs = append(attrs, "state_path", cfg.StatePath)
		}
		if cfg.CastawayAPIAuthToken != "" {
			attrs = append(attrs, "api_auth", "configured")
		}
		logger.Info("configuration valid", attrs...)
		return nil
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	bot, err := discordbot.New(cfg, client, store, logger)
	if err != nil {
		return fmt.Errorf("create discord bot: %w", err)
	}
	defer bot.Close()

	if err := bot.Start(ctx); err != nil {
		return err
	}

	logger.Info("castaway-discord-bot running", "version", buildinfo.String())
	<-ctx.Done()
	logger.Info("castaway-discord-bot shutting down")
	return nil
}
