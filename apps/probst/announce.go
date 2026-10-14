package main

import (
	"context"
	"fmt"
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
func addAnnouncementDraftCommands(parent *cobra.Command, call apiCall, instancePath func() (string, error), checkBinding func(ctx context.Context, channel string) error, guild *string, yes *bool, resolveChannel func(string) string) {
	out := func(c *cobra.Command, format string, args ...any) error {
		_, err := fmt.Fprintf(c.OutOrStdout(), format, args...)
		return err
	}
	var file string
	var notify bool
	save := &cobra.Command{Use: "save NAME [CHANNEL]", Short: "Save a draft announcement (not sent until scheduled)", Args: cobra.RangeArgs(1, 2), RunE: func(c *cobra.Command, a []string) error {
		a = []string{a[0], resolveChannel(strings.Join(a[1:], ""))}
		if a[1] == "" {
			return fmt.Errorf("give a CHANNEL, or an --instance alias that has one")
		}
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
			return out(c, "Dry run — draft %q for channel %s (%s):\n\n%s\n\nRe-run with --yes to save it.\n", a[0], a[1], mentionNote(notify), text)
		}
		body := map[string]any{"guild_id": *guild, "channel_id": a[1], "request_key": a[0], "body": text, "draft": true, "notify_users": notify}
		if err := call(c.Context(), "POST", p+"/announcements", body, nil); err != nil {
			return fmt.Errorf("%w (to change a saved draft, use `announcement edit`)", err)
		}
		return out(c, "Saved draft %q. Schedule it with `probst announcement schedule %s --at ...`.\n", a[0], a[0])
	}}
	save.Flags().StringVar(&file, "file", "", "UTF-8 Markdown file with the announcement text")
	save.Flags().BoolVar(&notify, "notify", false, "Let <@user> mentions ping those players (never @everyone or roles)")
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
	var approval bool
	schedule := &cobra.Command{Use: "schedule NAME", Aliases: []string{"reschedule"}, Short: "Request approval to send a saved announcement at --at (admins get a DM; it sends after a \"yes\")", Args: cobra.ExactArgs(1), RunE: func(c *cobra.Command, a []string) error {
		p, err := instancePath()
		if err != nil {
			return err
		}
		if at == "" {
			return fmt.Errorf("give --at: every announcement waits for an admin's \"yes\", then sends at that time")
		}
		when, err := parseAnnouncementTime(at)
		if err != nil {
			return err
		}
		found, err := findAnnouncement(c.Context(), call, p, a[0])
		if err != nil {
			return err
		}
		if err := call(c.Context(), "PUT", p+"/announcements/"+url.PathEscape(found.ID)+"/approval", map[string]any{"send_at": when.UTC().Format(time.RFC3339)}, nil); err != nil {
			return err
		}
		return out(c, "%q is waiting for approval: every admin gets a DM, and it posts at %s once one replies \"yes\".\n", a[0], at)
	}}
	schedule.Flags().StringVar(&at, "at", "", `Send at "2006-01-02 15:04" America/New_York, or RFC3339`)
	schedule.Flags().BoolVar(&approval, "approval", true, "Always on: every announcement waits for approval")
	_ = schedule.Flags().MarkHidden("approval")
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
	var sentMessage string
	markSent := &cobra.Command{Use: "mark-sent NAME", Short: "Record that you posted an announcement yourself, so the bot never sends it", Args: cobra.ExactArgs(1), RunE: func(c *cobra.Command, a []string) error {
		p, err := instancePath()
		if err != nil {
			return err
		}
		found, err := findAnnouncement(c.Context(), call, p, a[0])
		if err != nil {
			return err
		}
		if err := call(c.Context(), "POST", p+"/announcements/"+url.PathEscape(found.ID)+"/sent", map[string]string{"message_id": sentMessage}, nil); err != nil {
			return err
		}
		return out(c, "%q marked sent.\n", a[0])
	}}
	markSent.Flags().StringVar(&sentMessage, "message", "", "Discord message ID of the post, if you have it")
	parent.AddCommand(markSent)
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

// addMessageCommand saves a one-off message and requests approval: it posts once an admin replies "yes"
// (right away if that's after --at, which defaults to a minute from now).
func addMessageCommand(root *cobra.Command, call apiCall, instancePath func() (string, error), guild *string, yes *bool, resolveChannel func(string) string) {
	var file, at string
	var notify bool
	message := &cobra.Command{
		Use:   "message [CHANNEL] [TEXT]",
		Short: "Post a one-off message as the bot after an admin approves it by DM; dry run unless --yes. CHANNEL defaults to the --instance alias's channel",
		Args:  cobra.MaximumNArgs(2),
		RunE: func(c *cobra.Command, a []string) error {
			var channel, text string
			switch {
			case len(a) == 2 && file == "":
				channel, text = a[0], a[1]
			case len(a) == 1 && file == "":
				text = a[0]
			case len(a) <= 1 && file != "":
				channel = strings.Join(a, "")
				var err error
				if text, err = readMessageFile(file); err != nil {
					return err
				}
			default:
				return fmt.Errorf("give the TEXT or --file, not both")
			}
			channel = resolveChannel(channel)
			if channel == "" {
				return fmt.Errorf("give a CHANNEL, or an --instance alias that has one")
			}
			if strings.TrimSpace(text) == "" || utf8.RuneCountInString(text) > 2000 {
				return fmt.Errorf("message must be nonblank and at most 2000 characters")
			}
			when := time.Now().Add(time.Minute)
			if at != "" {
				var err error
				if when, err = parseAnnouncementTime(at); err != nil {
					return err
				}
			}
			if !*yes {
				_, err := fmt.Fprintf(c.OutOrStdout(), "Dry run — channel %s (%s):\n\n%s\n\nRe-run with --yes to request approval.\n", channel, mentionNote(notify), text)
				return err
			}
			p, err := instancePath()
			if err != nil {
				return err
			}
			key := fmt.Sprintf("message-%d", time.Now().UnixNano())
			var saved struct {
				Announcement announcement `json:"announcement"`
			}
			if err := call(c.Context(), "POST", p+"/announcements", map[string]any{"guild_id": *guild, "channel_id": channel, "request_key": key, "body": text, "draft": true, "notify_users": notify}, &saved); err != nil {
				return err
			}
			if err := call(c.Context(), "PUT", p+"/announcements/"+url.PathEscape(saved.Announcement.ID)+"/approval", map[string]any{"send_at": when.UTC().Format(time.RFC3339)}, nil); err != nil {
				return err
			}
			_, err = fmt.Fprintf(c.OutOrStdout(), "Message %q is waiting for approval: every admin gets a DM, and it posts once one replies \"yes\".\n", key)
			return err
		},
	}
	message.Flags().StringVar(&file, "file", "", "UTF-8 Markdown file to post")
	message.Flags().StringVar(&at, "at", "", `Post no earlier than "2006-01-02 15:04" America/New_York (default: on approval)`)
	message.Flags().BoolVar(&notify, "notify", false, "Let <@user> mentions ping those players (never @everyone or roles)")
	root.AddCommand(message)
}
func mentionNote(notify bool) string {
	if notify {
		return "<@user> mentions WILL ping those players"
	}
	return "mentions will not notify"
}
