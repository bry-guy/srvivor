package discord

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

// Admin DMs: the bot DMs every instance admin about a problem draft or a post waiting for approval, and an
// admin answers by replying (Discord's Reply) to that DM. Each DM carries what a reply acts on, so the bot
// keeps no state:
//   - an approval DM ends with "approval <id> <revision>"; "yes" approves exactly that revision.
//   - a draft DM links the player's post and (after a fix) quotes the working draft; "7. Thien An" lines
//     replace those ranks in the working draft (the post on first reply) and resubmit it.
//
// The server rejects stale actions: an approval of a changed post, or a fix after the draft was saved.

var (
	draftLink   = regexp.MustCompile(`discord\.com/channels/\d+/(\d+)/(\d+)`)
	approvalRef = regexp.MustCompile("approval `([0-9a-f-]{36})` `([0-9a-f]{32})`")
	rankLine    = regexp.MustCompile(`^\s*0*(\d{1,2})\s*[.):\-]?\s+(\S.*?)\s*$`)
	workingCopy = regexp.MustCompile("(?s)Working draft:\n```\n(.*)\n```")
)

// dmAdmins sends text to each admin, reporting failures.
func (b *Bot) dmAdmins(ctx context.Context, admins []string, text string) error {
	var failed []string
	for _, admin := range admins {
		dm, err := b.session.UserChannelCreate(admin, discordgo.WithContext(ctx))
		if err == nil {
			_, err = b.session.ChannelMessageSendComplex(dm.ID, &discordgo.MessageSend{Content: text, AllowedMentions: allowedMentions(false)}, discordgo.WithContext(ctx))
		}
		if err != nil {
			b.log.Error("admin DM", "admin", admin, "error", err)
			failed = append(failed, admin)
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("couldn't DM admins %v", failed)
	}
	return nil
}

// applyFixes replaces (or adds) the draft's numbered lines with the reply's "N. Name" lines. It rejects a
// reply with no fixes, any line that isn't a fix, or the same rank twice, rather than guess.
func applyFixes(draft, reply string) (string, error) {
	fixes := map[int]string{}
	for _, line := range strings.Split(strings.TrimSpace(reply), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		m := rankLine.FindStringSubmatch(line)
		if m == nil {
			return "", fmt.Errorf("%q isn't a fix; use one per line, like `7. Thien An`", strings.TrimSpace(line))
		}
		rank, err := strconv.Atoi(m[1])
		if err != nil || rank < 1 {
			return "", fmt.Errorf("rank %s isn't valid", m[1])
		}
		if _, dup := fixes[rank]; dup {
			return "", fmt.Errorf("rank %d appears twice", rank)
		}
		fixes[rank] = m[2]
	}
	if len(fixes) == 0 {
		return "", fmt.Errorf("reply with one fix per line, like `7. Thien An`")
	}
	lines := strings.Split(draft, "\n")
	for i, line := range lines {
		if m := rankLine.FindStringSubmatch(line); m != nil {
			rank, err := strconv.Atoi(m[1])
			if name, ok := fixes[rank]; err == nil && ok {
				lines[i] = fmt.Sprintf("%d. %s", rank, name)
				delete(fixes, rank)
			}
		}
	}
	var added []int
	for rank := range fixes {
		added = append(added, rank)
	}
	sort.Ints(added)
	for _, rank := range added {
		lines = append(lines, fmt.Sprintf("%d. %s", rank, fixes[rank]))
	}
	return strings.Join(lines, "\n"), nil
}

// handleAdminReply acts on a DM reply to one of the bot's own admin DMs.
func (b *Bot) handleAdminReply(ctx context.Context, m *discordgo.Message) {
	if m == nil || m.GuildID != "" || m.Author == nil || m.Author.Bot || m.MessageReference == nil || b.session.State == nil || b.session.State.User == nil {
		return
	}
	asked, err := b.session.ChannelMessage(m.ChannelID, m.MessageReference.MessageID, discordgo.WithContext(ctx))
	if err != nil || asked.Author == nil || asked.Author.ID != b.session.State.User.ID {
		return
	}
	answer := func(text string) {
		if _, err := b.session.ChannelMessageSendReply(m.ChannelID, text, m.Reference(), discordgo.WithContext(ctx)); err != nil {
			b.log.Error("admin DM answer", "error", err)
		}
	}
	if ref := approvalRef.FindStringSubmatch(asked.Content); ref != nil {
		if !strings.EqualFold(strings.Trim(strings.TrimSpace(m.Content), ".!"), "yes") {
			answer("Reply **yes** to approve. To change it, edit it with `probst announcement edit` and you'll get a new DM.")
			return
		}
		at, err := b.castaway.ApproveAnnouncement(ctx, ref[1], ref[2], m.Author.ID)
		if err != nil {
			answer("Not approved: " + err.Error())
			return
		}
		answer("✅ Approved. It posts " + eastern(at) + ".")
		return
	}
	link := draftLink.FindStringSubmatch(asked.Content)
	if link == nil {
		return
	}
	original, err := b.session.ChannelMessage(link[1], link[2], discordgo.WithContext(ctx))
	if err != nil {
		answer("Couldn't read that draft post: " + err.Error())
		return
	}
	working := original.Content
	if w := workingCopy.FindStringSubmatch(asked.Content); w != nil {
		working = w[1]
	}
	fixed, err := applyFixes(working, m.Content)
	if err != nil {
		answer("Nothing changed: " + err.Error())
		return
	}
	result, err := b.castaway.FixDraftThreadMessage(ctx, link[1], original.ID, "fix-"+m.ID, original.Author.ID, fixed, m.Author.ID)
	if err != nil {
		answer("Nothing changed: " + err.Error())
		return
	}
	switch result.Status {
	case "saved", "unchanged":
		answer(fmt.Sprintf("✅ Saved %s's draft.", result.Player))
	case "problem":
		text := fmt.Sprintf("Still not right (%s): https://discord.com/channels/%s/%s/%s\n- %s\nReply to this with more fixes, like `7. Thien An`.\n\nWorking draft:\n```\n%s\n```",
			result.Player, original.GuildID, link[1], link[2], strings.Join(result.Problems, "\n- "), fixed)
		if len(text) > 2000 {
			answer("Still not right, and the draft is too long to show here. Fix it with `probst draft import`.")
			return
		}
		answer(text)
	default:
		answer("That didn't read as a draft (" + result.Status + ").")
	}
}

func eastern(t time.Time) string {
	if loc, err := time.LoadLocation("America/New_York"); err == nil {
		t = t.In(loc)
	}
	return t.Format("Mon Jan 2 at 3:04pm MST")
}
