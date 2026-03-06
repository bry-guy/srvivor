package discord

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/bry-guy/srvivor/apps/castaway-discord-bot/internal/castaway"
	"github.com/bry-guy/srvivor/apps/castaway-discord-bot/internal/config"
	"github.com/bry-guy/srvivor/apps/castaway-discord-bot/internal/state"
	"github.com/bwmarrin/discordgo"
)

type Bot struct {
	appID      string
	devGuildID string
	log        *slog.Logger

	castaway *castaway.Client
	state    *state.Store
	session  *discordgo.Session
}

func New(cfg *config.Config, client *castaway.Client, store *state.Store, logger *slog.Logger) (*Bot, error) {
	session, err := discordgo.New("Bot " + cfg.DiscordBotToken)
	if err != nil {
		return nil, fmt.Errorf("create discord session: %w", err)
	}
	session.Identify.Intents = discordgo.IntentsGuilds

	bot := &Bot{
		appID:      cfg.DiscordApplicationID,
		devGuildID: cfg.DiscordDevGuildID,
		log:        logger,
		castaway:   client,
		state:      store,
		session:    session,
	}

	session.AddHandler(bot.handleInteraction)
	return bot, nil
}

func (b *Bot) Start(ctx context.Context) error {
	if err := b.session.Open(); err != nil {
		return fmt.Errorf("open discord session: %w", err)
	}

	commandScope, err := b.syncCommands()
	if err != nil {
		_ = b.session.Close()
		return err
	}

	go func() {
		<-ctx.Done()
		if err := b.session.Close(); err != nil {
			b.log.Warn("close discord session", "error", err)
		}
	}()

	b.log.Info("discord session opened", "command_scope", commandScope)
	return nil
}

func (b *Bot) Close() error {
	if b.session == nil {
		return nil
	}
	return b.session.Close()
}

func (b *Bot) syncCommands() (string, error) {
	commands := applicationCommands()
	if b.devGuildID != "" {
		if _, err := b.session.ApplicationCommandBulkOverwrite(b.appID, b.devGuildID, commands); err == nil {
			return "guild", nil
		} else if !isDiscordMissingAccess(err) {
			return "", fmt.Errorf("sync guild application commands: %w", err)
		} else {
			b.log.Warn("guild command sync failed; falling back to global commands", "guild_id", b.devGuildID, "error", err)
		}
	}

	if _, err := b.session.ApplicationCommandBulkOverwrite(b.appID, "", commands); err != nil {
		return "", fmt.Errorf("sync global application commands: %w", err)
	}
	return "global", nil
}

func isDiscordMissingAccess(err error) bool {
	var restErr *discordgo.RESTError
	if !errors.As(err, &restErr) {
		return false
	}
	return restErr.Response != nil && restErr.Response.StatusCode == 403 && restErr.Message != nil && restErr.Message.Code == 50001
}
