package discord

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) pollAnnouncements(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		if err := b.deliverNextAnnouncement(ctx); err != nil && ctx.Err() == nil {
			b.log.Error("announcement delivery", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// deliverNextAnnouncement sends at most one due announcement. A failed send is
// recorded as failed and never retried; the operator re-sends manually.
func (b *Bot) deliverNextAnnouncement(ctx context.Context) error {
	a, err := b.castaway.ClaimAnnouncement(ctx, b.targetServerIDs)
	if err != nil || a == nil {
		return err
	}
	finish := func(messageID string, failed bool) error {
		ackCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return b.castaway.FinishAnnouncement(ackCtx, a.ID, messageID, failed)
	}
	fail := func(reason error) error {
		return fmt.Errorf("announcement %s to channel %s failed: %v (record: %v)", a.ID, a.ChannelID, reason, finish("", true))
	}
	if !slices.Contains(b.targetServerIDs, a.GuildID) {
		return fail(fmt.Errorf("guild %s is not allowed", a.GuildID))
	}
	boundInstance, err := b.castaway.GetChannelInstance(ctx, a.GuildID, a.ChannelID)
	if err != nil {
		return fail(err)
	}
	if boundInstance != a.InstanceID {
		return fail(fmt.Errorf("channel is no longer bound to instance %s", a.InstanceID))
	}
	// 429s are retried by discordgo; other failures are not, so a lost 5xx cannot post twice.
	message, err := b.session.ChannelMessageSendComplex(a.ChannelID, &discordgo.MessageSend{
		Content:         a.Body,
		AllowedMentions: &discordgo.MessageAllowedMentions{Parse: []discordgo.AllowedMentionType{}},
	}, discordgo.WithRestRetries(0))
	if err != nil {
		return fail(err)
	}
	if err := finish(message.ID, false); err != nil {
		return fmt.Errorf("announcement %s sent as message %s but recording it failed: %w", a.ID, message.ID, err)
	}
	b.log.Info("announcement sent", "announcement_id", a.ID, "message_id", message.ID, "guild_id", a.GuildID, "channel_id", a.ChannelID)
	return nil
}
