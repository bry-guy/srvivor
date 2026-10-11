package discord

import (
	"context"
	"slices"
	"strings"

	"github.com/bwmarrin/discordgo"
)

// championPostMarker identifies the bot's merge post: replying to it picks (or changes) your Champion.
const championPostMarker = "Pick Your Champion"

// handleChampionReply turns a player's reply to the merge post into their Champion pick and answers in-line.
func (b *Bot) handleChampionReply(ctx context.Context, m *discordgo.Message) {
	if m == nil || m.GuildID == "" || m.Author == nil || m.Author.Bot || m.MessageReference == nil || strings.TrimSpace(m.Content) == "" ||
		!slices.Contains(b.targetServerIDs, m.GuildID) || b.session.State == nil || b.session.State.User == nil {
		return
	}
	post, err := b.session.ChannelMessage(m.ChannelID, m.MessageReference.MessageID, discordgo.WithContext(ctx))
	if err != nil || post.Author == nil || post.Author.ID != b.session.State.User.ID || !strings.Contains(post.Content, championPostMarker) {
		return
	}
	text := "🏆 Your Champion: **%s**. You can change it until picks lock."
	name, err := b.castaway.PickChampion(ctx, m.GuildID, m.ChannelID, m.Author.ID, strings.TrimSpace(m.Content))
	if err != nil {
		text = "Not picked: " + err.Error()
	} else {
		text = strings.Replace(text, "%s", name, 1)
	}
	if _, err := b.session.ChannelMessageSendReply(m.ChannelID, text, m.Reference(), discordgo.WithContext(ctx)); err != nil {
		b.log.Error("champion reply", "error", err)
	}
}
