package discord

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bry-guy/srvivor/apps/castaway-discord-bot/internal/castaway"
	"github.com/bry-guy/srvivor/apps/castaway-discord-bot/internal/config"
	"github.com/bry-guy/srvivor/apps/castaway-discord-bot/internal/state"
	"github.com/bwmarrin/discordgo"
)

type Bot struct {
	appID                 string
	targetServerIDs       []string
	announcementChannelID string
	adminContactID        string
	log                   *slog.Logger

	castaway *castaway.Client
	state    state.Store
	session  *discordgo.Session
}

func New(cfg *config.Config, client *castaway.Client, store state.Store, logger *slog.Logger) (*Bot, error) {
	session, err := discordgo.New("Bot " + cfg.DiscordBotToken)
	if err != nil {
		return nil, fmt.Errorf("create discord session: %w", err)
	}
	session.Identify.Intents = discordgo.IntentsGuilds

	bot := &Bot{
		appID:                 cfg.DiscordApplicationID,
		targetServerIDs:       cfg.TargetServerIDs(),
		announcementChannelID: cfg.AnnouncementChannelID,
		adminContactID:        cfg.AdminContactDiscordUserID,
		log:                   logger,
		castaway:              client,
		state:                 store,
		session:               session,
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

	botDiscordGatewayConnected.Set(1)
	b.log.Info("discord session opened", "command_scope", commandScope)
	return nil
}

func (b *Bot) Close() error {
	botDiscordGatewayConnected.Set(0)
	if b.session == nil {
		return nil
	}
	return b.session.Close()
}

func (b *Bot) syncCommands() (string, error) {
	if len(b.targetServerIDs) == 0 {
		return "", fmt.Errorf("explicit Discord target guilds are required; global registration is disabled")
	}
	for _, serverID := range b.targetServerIDs {
		if _, err := b.session.ApplicationCommandBulkOverwrite(b.appID, serverID, applicationCommands()); err != nil {
			return "", fmt.Errorf("sync guild %s application commands: %w", serverID, err)
		}
	}
	return "guild", nil
}
