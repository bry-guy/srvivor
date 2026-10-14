package discord

import (
	"context"
	"errors"

	"github.com/bry-guy/srvivor/apps/castaway-discord-bot/internal/castaway"
	"github.com/bry-guy/srvivor/apps/castaway-discord-bot/internal/format"
	"github.com/bry-guy/srvivor/apps/castaway-discord-bot/internal/messages"
	"github.com/bwmarrin/discordgo"
)

// setupError tells the player where to get help; problem names a setup-* template.
func (b *Bot) setupError(problem string) error {
	return messages.Err("setup-error", map[string]any{"Problem": messages.Fragment(problem, nil), "Admin": b.adminContactID})
}

func (b *Bot) executePlayerCommand(ctx context.Context, interaction *discordgo.InteractionCreate, command commandSpec) (string, error) {
	if interaction.GuildID == "" || interaction.ChannelID == "" || interaction.Member == nil || interaction.Member.User == nil {
		return "", messages.Err("server-channel-only", nil)
	}
	if command.group != "" || (command.name != "score" && command.name != "scores" && command.name != "draft") {
		return "", messages.Err("command-retired", nil)
	}
	selected := interaction.Member.User.ID
	if len(command.options) > 1 {
		return "", messages.Err("unsupported-options", nil)
	}
	for _, option := range command.options {
		if command.name == "scores" || option.Name != "player" || option.Type != discordgo.ApplicationCommandOptionUser {
			return "", messages.Err("unsupported-options", nil)
		}
		value, ok := option.Value.(string)
		if !ok || value == "" {
			return "", messages.Err("select-player", nil)
		}
		selected = value
	}
	instanceID, err := b.channelInstance(ctx, interaction)
	if err != nil {
		return "", err
	}
	instance, err := b.castaway.GetInstance(ctx, instanceID)
	if err != nil {
		return "", b.setupError("setup-instance-unavailable")
	}
	if command.name == "scores" {
		rows, err := b.castaway.GetLeaderboard(ctx, instance.ID, "")
		if err != nil {
			return "", err
		}
		return format.Leaderboard(instance, castaway.Drafted(rows)), nil
	}
	participant, err := b.castaway.GetLinkedParticipant(ctx, instance.ID, selected)
	if err != nil {
		var apiErr *castaway.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			return "", b.setupError("setup-player-not-linked")
		}
		return "", err
	}
	if command.name == "draft" {
		draft, err := b.castaway.GetDraft(ctx, instance.ID, participant.ID)
		if err != nil {
			return "", err
		}
		return format.Draft(instance, draft), nil
	}
	rows, err := b.castaway.GetLeaderboard(ctx, instance.ID, "")
	if err != nil {
		return "", err
	}
	for i, row := range castaway.Drafted(rows) {
		if row.ParticipantID == participant.ID {
			return format.SingleScore(instance, row, i+1), nil
		}
	}
	return "", messages.Err("no-draft", map[string]any{"Season": instance.Season})
}

func (b *Bot) channelInstance(ctx context.Context, interaction *discordgo.InteractionCreate) (string, error) {
	id, err := b.castaway.GetChannelInstance(ctx, interaction.GuildID, interaction.ChannelID)
	if err == nil {
		return id, nil
	}
	var apiErr *castaway.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 404 {
		return "", b.setupError("setup-channel-config")
	}
	channel, err := b.session.Channel(interaction.ChannelID, discordgo.WithContext(ctx))
	if err != nil {
		return "", b.setupError("setup-channel-verify")
	}
	if channel.GuildID != interaction.GuildID {
		return "", b.setupError("setup-channel-server")
	}
	switch channel.Type {
	case discordgo.ChannelTypeGuildNewsThread, discordgo.ChannelTypeGuildPublicThread, discordgo.ChannelTypeGuildPrivateThread:
		if channel.ParentID != "" {
			id, err = b.castaway.GetChannelInstance(ctx, interaction.GuildID, channel.ParentID)
			if err == nil {
				return id, nil
			}
			if !errors.As(err, &apiErr) || apiErr.StatusCode != 404 {
				return "", b.setupError("setup-parent-config")
			}
		}
	}
	return "", b.setupError("setup-not-bound")
}
