package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	checkConfig := flag.Bool("check-config", false, "validate configuration and local state setup, then exit")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)

	store, err := state.Open(cfg.StatePath)
	if err != nil {
		return fmt.Errorf("open state store: %w", err)
	}
	defer store.Close()

	client, err := castaway.NewClient(cfg.CastawayAPIBaseURL, &http.Client{Timeout: 10 * time.Second})
	if err != nil {
		return fmt.Errorf("create castaway client: %w", err)
	}

	if *checkConfig {
		logger.Info("configuration valid", "api_base_url", cfg.CastawayAPIBaseURL, "state_path", cfg.StatePath)
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

	logger.Info("castaway-discord-bot running")
	<-ctx.Done()
	logger.Info("castaway-discord-bot shutting down")
	return nil
}
