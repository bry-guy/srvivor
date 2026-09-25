package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"text/tabwriter"
	"time"
	"unicode/utf8"

	"github.com/spf13/cobra"
)

type announcement struct {
	ID          string     `json:"id"`
	ChannelID   string     `json:"channel_id"`
	RequestKey  string     `json:"request_key"`
	Body        string     `json:"body"`
	ScheduledAt *time.Time `json:"scheduled_at"`
	DueAt       time.Time  `json:"due_at"`
	Status      string     `json:"status"`
	MessageID   *string    `json:"message_id"`
}

func readMessageFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(data) || strings.TrimSpace(string(data)) == "" || utf8.RuneCount(data) > 2000 {
		return "", fmt.Errorf("message must be nonempty UTF-8, at most 2000 characters")
	}
	return string(data), nil
}

// findAnnouncement accepts an announcement's name (its request key) or ID.
func findAnnouncement(ctx context.Context, call apiCall, instancePath, ref string) (announcement, error) {
	var res struct {
		Announcements []announcement `json:"announcements"`
	}
	if err := call(ctx, "GET", instancePath+"/announcements", nil, &res); err != nil {
		return announcement{}, err
	}
	for _, a := range res.Announcements {
		if a.ID == ref || a.RequestKey == ref {
			return a, nil
		}
	}
	return announcement{}, fmt.Errorf("no announcement named %q; see `probst announcement list`", ref)
}

// addAnnouncementDraftCommands adds the save → edit → schedule workflow. The bot reads the text when it
// sends, so edits apply until the moment it goes out.
func addAnnouncementDraftCommands(parent *cobra.Command, call apiCall, instancePath func() (string, error), checkBinding func(ctx context.Context, channel string) error, guild *string, yes *bool) {
	out := func(c *cobra.Command, format string, args ...any) error {
		_, err := fmt.Fprintf(c.OutOrStdout(), format, args...)
		return err
	}
	var file string
	save := &cobra.Command{Use: "save NAME CHANNEL", Short: "Save a draft announcement (not sent until scheduled)", Args: cobra.ExactArgs(2), RunE: func(c *cobra.Command, a []string) error {
		p, err := instancePath()
		if err != nil {
			return err
		}
		text, err := readMessageFile(file)
		if err != nil {
			return err
		}
		if err := checkBinding(c.Context(), a[1]); err != nil {
			return err
		}
		if !*yes {
			return out(c, "Dry run — draft %q for channel %s:\n\n%s\n\nRe-run with --yes to save it.\n", a[0], a[1], text)
		}
		body := map[string]any{"guild_id": *guild, "channel_id": a[1], "request_key": a[0], "body": text, "draft": true}
		if err := call(c.Context(), "POST", p+"/announcements", body, nil); err != nil {
			return fmt.Errorf("%w (to change a saved draft, use `announcement edit`)", err)
		}
		return out(c, "Saved draft %q. Schedule it with `probst announcement schedule %s --at ...`.\n", a[0], a[0])
	}}
	save.Flags().StringVar(&file, "file", "", "UTF-8 Markdown file with the announcement text")
	parent.AddCommand(save)

	edit := &cobra.Command{Use: "edit NAME", Short: "Replace an unsent announcement's text", Args: cobra.ExactArgs(1), RunE: func(c *cobra.Command, a []string) error {
		p, err := instancePath()
		if err != nil {
			return err
		}
		text, err := readMessageFile(file)
		if err != nil {
			return err
		}
		found, err := findAnnouncement(c.Context(), call, p, a[0])
		if err != nil {
			return err
		}
		if !*yes {
			return out(c, "Dry run — %q (%s) will read:\n\n%s\n\nRe-run with --yes to save it.\n", a[0], found.Status, text)
		}
		if err := call(c.Context(), "PUT", p+"/announcements/"+url.PathEscape(found.ID)+"/body", map[string]string{"body": text}, nil); err != nil {
			return err
		}
		return out(c, "Updated %q.\n", a[0])
	}}
	edit.Flags().StringVar(&file, "file", "", "UTF-8 Markdown file with the new text")
	parent.AddCommand(edit)

	parent.AddCommand(&cobra.Command{Use: "show NAME", Short: "Print an announcement's current text", Args: cobra.ExactArgs(1), RunE: func(c *cobra.Command, a []string) error {
		p, err := instancePath()
		if err != nil {
			return err
		}
		found, err := findAnnouncement(c.Context(), call, p, a[0])
		if err != nil {
			return err
		}
		return out(c, "%s — %s, channel %s\n\n%s\n", found.RequestKey, describeStatus(found), found.ChannelID, found.Body)
	}})

	var at string
	schedule := &cobra.Command{Use: "schedule NAME", Aliases: []string{"reschedule"}, Short: "Send a saved announcement at --at, or now with --yes", Args: cobra.ExactArgs(1), RunE: func(c *cobra.Command, a []string) error {
		p, err := instancePath()
		if err != nil {
			return err
		}
		body := map[string]any{}
		if at != "" {
			when, err := parseAnnouncementTime(at)
			if err != nil {
				return err
			}
			body["scheduled_at"] = when.UTC().Format(time.RFC3339)
		} else if !*yes {
			return fmt.Errorf("give --at, or --yes to send now")
		}
		found, err := findAnnouncement(c.Context(), call, p, a[0])
		if err != nil {
			return err
		}
		var res struct {
			Announcement announcement `json:"announcement"`
		}
		if err := call(c.Context(), "PUT", p+"/announcements/"+url.PathEscape(found.ID)+"/schedule", body, &res); err != nil {
			return err
		}
		return out(c, "%q: %s.\n", a[0], describeStatus(res.Announcement))
	}}
	schedule.Flags().StringVar(&at, "at", "", `Send at "2006-01-02 15:04" America/New_York, or RFC3339`)
	parent.AddCommand(schedule)

	simple := func(use, short, method, suffix, done string, confirm bool) {
		parent.AddCommand(&cobra.Command{Use: use, Short: short, Args: cobra.ExactArgs(1), RunE: func(c *cobra.Command, a []string) error {
			if confirm && !*yes {
				return fmt.Errorf("%s requires --yes", strings.Fields(use)[0])
			}
			p, err := instancePath()
			if err != nil {
				return err
			}
			found, err := findAnnouncement(c.Context(), call, p, a[0])
			if err != nil {
				return err
			}
			if err := call(c.Context(), method, p+"/announcements/"+url.PathEscape(found.ID)+suffix, nil, nil); err != nil {
				return err
			}
			return out(c, "%q %s.\n", a[0], done)
		}})
	}
	simple("unschedule NAME", "Take an announcement off the schedule and keep it as a draft", "DELETE", "/schedule", "is a draft again", false)
	simple("delete NAME", "Delete an unsent announcement", "DELETE", "", "deleted", true)

	parent.AddCommand(&cobra.Command{Use: "list", Short: "List this instance's announcements", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		p, err := instancePath()
		if err != nil {
			return err
		}
		var res struct {
			Announcements []announcement `json:"announcements"`
		}
		if err := call(c.Context(), "GET", p+"/announcements", nil, &res); err != nil {
			return err
		}
		table := tabwriter.NewWriter(c.OutOrStdout(), 0, 4, 2, ' ', 0)
		if _, err := fmt.Fprintln(table, "NAME\tSTATUS\tCHANNEL\tSTART OF TEXT"); err != nil {
			return err
		}
		for _, a := range res.Announcements {
			preview := strings.Join(strings.Fields(a.Body), " ")
			if r := []rune(preview); len(r) > 40 {
				preview = string(r[:40]) + "…"
			}
			if _, err := fmt.Fprintf(table, "%s\t%s\t%s\t%s\n", a.RequestKey, describeStatus(a), a.ChannelID, preview); err != nil {
				return err
			}
		}
		return table.Flush()
	}})
}

func describeStatus(a announcement) string {
	switch a.Status {
	case "pending":
		return "scheduled " + a.DueAt.In(eastern()).Format("Mon 01/02 15:04 MST")
	case "sent":
		if a.MessageID != nil {
			return "sent (message " + *a.MessageID + ")"
		}
	}
	return a.Status
}

// sendDiscordMessage posts text as the bot straight to Discord, without saving it. Mentions render but never notify.
func sendDiscordMessage(ctx context.Context, token, channelID, text, replyTo string) (string, error) {
	if token == "" {
		return "", fmt.Errorf("set CASTAWAY_DISCORD_BOT_TOKEN (or discord_bot_token in the config file) to post to Discord")
	}
	base := os.Getenv("PROBST_DISCORD_API_URL")
	if base == "" {
		base = "https://discord.com/api/v10"
	}
	payload := map[string]any{"content": text, "allowed_mentions": map[string]any{"parse": []string{}}}
	if replyTo != "" {
		payload["message_reference"] = map[string]any{"message_id": replyTo, "fail_if_not_exists": false}
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", base+"/channels/"+url.PathEscape(channelID)+"/messages", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bot "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		// Discord may have posted it anyway; say so instead of retrying.
		return "", fmt.Errorf("Discord request failed (check the channel before resending): %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Discord HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var sent struct {
		ID string `json:"id"`
	}
	return sent.ID, json.Unmarshal(body, &sent)
}

func addMessageCommand(root *cobra.Command, token *string, yes *bool) {
	var file, replyTo string
	message := &cobra.Command{
		Use:   "message CHANNEL [TEXT]",
		Short: "Post a one-off message as the bot right now (not saved); dry run unless --yes",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(c *cobra.Command, a []string) error {
			var text string
			switch {
			case len(a) == 2 && file == "":
				text = a[1]
			case len(a) == 1 && file != "":
				var err error
				if text, err = readMessageFile(file); err != nil {
					return err
				}
			default:
				return fmt.Errorf("give the TEXT or --file, not both")
			}
			if strings.TrimSpace(text) == "" || utf8.RuneCountInString(text) > 2000 {
				return fmt.Errorf("message must be nonblank and at most 2000 characters")
			}
			if !*yes {
				_, err := fmt.Fprintf(c.OutOrStdout(), "Dry run — channel %s (mentions will not notify):\n\n%s\n\nRe-run with --yes to post it.\n", a[0], text)
				return err
			}
			id, err := sendDiscordMessage(c.Context(), *token, a[0], text, replyTo)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(c.OutOrStdout(), "Posted message %s.\n", id)
			return err
		},
	}
	message.Flags().StringVar(&file, "file", "", "UTF-8 Markdown file to post")
	message.Flags().StringVar(&replyTo, "reply-to", "", "Discord message ID to reply to")
	root.AddCommand(message)
}
