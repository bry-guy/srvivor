package discord

import (
	"context"
	"errors"
	"fmt"

	"github.com/bry-guy/srvivor/apps/castaway-discord-bot/internal/castaway"
	"github.com/bry-guy/srvivor/apps/castaway-discord-bot/internal/format"
	"github.com/bwmarrin/discordgo"
)

func (b *Bot) setupError(message string) error {
	return fmt.Errorf("%s Ask <@%s> in Podracing Discord", message, b.adminContactID)
}

func (b *Bot) executePlayerCommand(ctx context.Context, interaction *discordgo.InteractionCreate, command commandSpec) (string, error) {
	if interaction.GuildID == "" || interaction.ChannelID == "" || interaction.Member == nil || interaction.Member.User == nil {
		return "", fmt.Errorf("use this command in a server channel, not a DM")
	}
	if command.group != "" || (command.name != "score" && command.name != "scores" && command.name != "draft") {
		return "", fmt.Errorf("this command is retired; use /castaway score, scores, or draft")
	}
	selected := interaction.Member.User.ID
	if len(command.options) > 1 {
		return "", fmt.Errorf("unsupported command options; refresh Discord commands")
	}
	for _, option := range command.options {
		if command.name == "scores" || option.Name != "player" || option.Type != discordgo.ApplicationCommandOptionUser {
			return "", fmt.Errorf("unsupported command options; refresh Discord commands")
		}
		value, ok := option.Value.(string)
		if !ok || value == "" {
			return "", fmt.Errorf("select a Discord player")
		}
		selected = value
	}
	instanceID, err := b.channelInstance(ctx, interaction)
	if err != nil {
		return "", err
	}
	instance, err := b.castaway.GetInstance(ctx, instanceID)
	if err != nil {
		return "", b.setupError("The bound instance is unavailable.")
	}
	if command.name == "scores" {
		rows, err := b.castaway.GetLeaderboard(ctx, instance.ID, "")
		if err != nil {
			return "", err
		}
		return format.Leaderboard(instance, rows), nil
	}
	participant, err := b.castaway.GetLinkedParticipant(ctx, instance.ID, selected)
	if err != nil {
		var apiErr *castaway.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			return "", b.setupError("That Discord player is not linked in this season.")
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
	for i, row := range rows {
		if row.ParticipantID == participant.ID {
			return format.SingleScore(instance, row, i+1), nil
		}
	}
	return "", fmt.Errorf("no published score for this player in Season %d", instance.Season)
}

func (b *Bot) channelInstance(ctx context.Context, interaction *discordgo.InteractionCreate) (string, error) {
	id, err := b.castaway.GetChannelInstance(ctx, interaction.GuildID, interaction.ChannelID)
	if err == nil {
		return id, nil
	}
	var apiErr *castaway.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 404 {
		return "", b.setupError("Cannot read this channel's configuration.")
	}
	channel, err := b.session.Channel(interaction.ChannelID, discordgo.WithContext(ctx))
	if err != nil {
		return "", b.setupError("Cannot verify this channel's context.")
	}
	if channel.GuildID != interaction.GuildID {
		return "", b.setupError("Channel server does not match.")
	}
	switch channel.Type {
	case discordgo.ChannelTypeGuildNewsThread, discordgo.ChannelTypeGuildPublicThread, discordgo.ChannelTypeGuildPrivateThread:
		if channel.ParentID != "" {
			id, err = b.castaway.GetChannelInstance(ctx, interaction.GuildID, channel.ParentID)
			if err == nil {
				return id, nil
			}
			if !errors.As(err, &apiErr) || apiErr.StatusCode != 404 {
				return "", b.setupError("Cannot read the parent channel's configuration.")
			}
		}
	}
	return "", b.setupError("This channel is not bound to a season.")
}
