package discord

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/bry-guy/srvivor/apps/castaway-discord-bot/internal/castaway"
	"github.com/bwmarrin/discordgo"
)

func (b *Bot) pollAnnouncements(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		if err := b.deliverNextAnnouncement(ctx); err != nil && ctx.Err() == nil {
			b.log.Error("announcement delivery", "error", err)
		}
		if err := b.notifyAccessRequest(ctx); err != nil && ctx.Err() == nil {
			b.log.Error("access request DM", "error", err)
		}
		if err := b.notifyApproval(ctx); err != nil && ctx.Err() == nil {
			b.log.Error("approval DM", "error", err)
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
	threadID := ""
	finish := func(messageID string, failed bool) error {
		ackCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return b.castaway.FinishAnnouncement(ackCtx, a.ID, messageID, threadID, failed)
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
	target := a.ChannelID
	if a.Thread != nil {
		if target, err = b.announcementThread(a); err != nil {
			return fail(err)
		}
		threadID = target
	}
	// 429s are retried by discordgo; other failures are not, so a lost 5xx cannot post twice.
	send := func() (*discordgo.Message, error) {
		return b.session.ChannelMessageSendComplex(target, &discordgo.MessageSend{
			Content:         a.Body,
			AllowedMentions: allowedMentions(a.NotifyUsers),
		}, discordgo.WithRestRetries(0))
	}
	message, err := send()
	// A deleted thread is the one failure where nothing was posted, so opening a new thread and retrying is safe.
	if a.Thread != nil && a.Thread.ID != "" && isUnknownChannel(err) {
		a.Thread.ID = ""
		if target, err = b.announcementThread(a); err != nil {
			return fail(err)
		}
		threadID = target
		message, err = send()
	}
	if err != nil {
		return fail(err)
	}
	if err := finish(message.ID, false); err != nil {
		return fmt.Errorf("announcement %s sent as message %s but recording it failed: %w", a.ID, message.ID, err)
	}
	b.log.Info("announcement sent", "announcement_id", a.ID, "message_id", message.ID, "guild_id", a.GuildID, "channel_id", a.ChannelID)
	return nil
}

// announcementThread returns the announcement's thread, opening it from a starter message in the channel
// when it does not exist yet. The starter never pings anyone.
func (b *Bot) announcementThread(a *castaway.Announcement) (string, error) {
	if a.Thread.ID != "" {
		return a.Thread.ID, nil
	}
	starter, err := b.session.ChannelMessageSendComplex(a.ChannelID, &discordgo.MessageSend{
		Content: a.Thread.Starter, AllowedMentions: allowedMentions(false),
	}, discordgo.WithRestRetries(0))
	if err != nil {
		return "", fmt.Errorf("thread starter: %w", err)
	}
	thread, err := b.session.MessageThreadStartComplex(a.ChannelID, starter.ID, &discordgo.ThreadStart{Name: a.Thread.Name, AutoArchiveDuration: 10080})
	if err != nil {
		return "", fmt.Errorf("open thread: %w", err)
	}
	return thread.ID, nil
}

func isUnknownChannel(err error) bool {
	var rest *discordgo.RESTError
	return errors.As(err, &rest) && rest.Message != nil && rest.Message.Code == discordgo.ErrCodeUnknownChannel
}

func allowedMentions(notifyUsers bool) *discordgo.MessageAllowedMentions {
	if notifyUsers {
		return &discordgo.MessageAllowedMentions{Parse: []discordgo.AllowedMentionType{discordgo.AllowedMentionTypeUsers}}
	}
	return &discordgo.MessageAllowedMentions{Parse: []discordgo.AllowedMentionType{}}
}

// notifyAccessRequest DMs the admin contact about one new website access request (at most once each).
func (b *Bot) notifyAccessRequest(ctx context.Context) error {
	r, err := b.castaway.ClaimAccessRequest(ctx)
	if err != nil || r == nil {
		return err
	}
	text := fmt.Sprintf("🔑 Castaway website access request from <@%s> (%s, `%s`).\nApprove: `probst access approve %s NAME --instance INSTANCE`\nDeny: `probst access deny %s`", r.DiscordUserID, r.DiscordUsername, r.DiscordUserID, r.DiscordUserID, r.DiscordUserID)
	dm, err := b.session.UserChannelCreate(b.adminContactID, discordgo.WithContext(ctx))
	if err == nil {
		_, err = b.session.ChannelMessageSendComplex(dm.ID, &discordgo.MessageSend{Content: text, AllowedMentions: allowedMentions(false)}, discordgo.WithContext(ctx))
	}
	return err
}

// notifyApproval DMs every admin a post waiting for approval (once per version of its text): the full text
// first, then a short message saying where and when it posts. Replying "yes" to that one approves exactly
// this text; it's never truncated, so admins approve what will post.
func (b *Bot) notifyApproval(ctx context.Context) error {
	a, err := b.castaway.ClaimAnnouncementApproval(ctx, b.targetServerIDs)
	if err != nil || a == nil {
		return err
	}
	if len(a.Admins) == 0 {
		return fmt.Errorf("announcement %s needs approval but its season has no admins", a.Announcement.ID)
	}
	pings := "no pings"
	if a.Announcement.NotifyUsers {
		pings = "@mentions ping those players"
	}
	ask := fmt.Sprintf("📝 The post above goes to <#%s> %s (%s). Reply **yes** to this message to approve it.\n\napproval `%s` `%s`",
		a.Announcement.ChannelID, eastern(a.SendAt), pings, a.Announcement.ID, a.Revision)
	err1 := b.dmAdmins(ctx, a.Admins, a.Announcement.Body)
	err2 := b.dmAdmins(ctx, a.Admins, ask)
	if err1 != nil || err2 != nil {
		return fmt.Errorf("approval DM for %s: %v %v (re-run `probst season post --yes` to re-send)", a.Announcement.ID, err1, err2)
	}
	return nil
}
