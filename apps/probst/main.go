package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

func main() {
	if err := newCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newCommand() *cobra.Command {
	var server, actor, instance, guild, discordUser string
	var asJSON, yes bool
	root := &cobra.Command{Use: "probst", Short: "Castaway operator client", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().StringVar(&server, "server", os.Getenv("PROBST_API_URL"), "Castaway API URL (PROBST_API_URL)")
	root.PersistentFlags().StringVar(&actor, "actor", os.Getenv("PROBST_DISCORD_USER_ID"), "Admin Discord ID asserted by the trusted service")
	root.PersistentFlags().StringVar(&instance, "instance", "", "Instance UUID")
	root.PersistentFlags().StringVar(&guild, "guild", "", "Discord guild ID")
	root.PersistentFlags().StringVar(&discordUser, "discord-user", "", "Discord account to link")
	root.PersistentFlags().BoolVar(&asJSON, "json", false, "Print machine-readable JSON")
	root.PersistentFlags().BoolVar(&yes, "yes", false, "Confirm rebinding, unlinking or unbinding")
	request := func(cmd *cobra.Command, method, path string, body any) error {
		token := os.Getenv("PROBST_TOKEN")
		if token == "" {
			return fmt.Errorf("set PROBST_TOKEN through your credential provider")
		}
		parsed, err := url.Parse(server)
		if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			return fmt.Errorf("set --server or PROBST_API_URL to an API URL")
		}
		local := parsed.Hostname() == "localhost" || parsed.Hostname() == "127.0.0.1" || parsed.Hostname() == "::1"
		if parsed.Scheme != "https" && !(parsed.Scheme == "http" && local) {
			return fmt.Errorf("HTTPS required except for loopback test servers")
		}
		if strings.TrimSpace(actor) == "" {
			return fmt.Errorf("set --actor or PROBST_DISCORD_USER_ID; this is trusted-service delegation, not human login")
		}
		var reader io.Reader
		if body != nil {
			data, err := json.Marshal(body)
			if err != nil {
				return err
			}
			reader = bytes.NewReader(data)
		}
		req, err := http.NewRequestWithContext(cmd.Context(), method, strings.TrimRight(server, "/")+path, reader)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Discord-User-ID", actor)
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		response, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("API request failed: %w", err)
		}
		defer func() { _ = response.Body.Close() }()
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			var failure struct {
				Error string `json:"error"`
			}
			_ = json.NewDecoder(io.LimitReader(response.Body, 65536)).Decode(&failure)
			return fmt.Errorf("API HTTP %d: %s", response.StatusCode, failure.Error)
		}
		var result map[string]any
		if err := json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(&result); err != nil {
			return err
		}
		return printResult(cmd.OutOrStdout(), result, asJSON)
	}
	instancePath := func() (string, error) {
		if instance == "" {
			return "", fmt.Errorf("--instance is required")
		}
		return "/instances/" + url.PathEscape(instance), nil
	}
	channelPath := func(channel string) (string, error) {
		if guild == "" {
			return "", fmt.Errorf("--guild is required")
		}
		return "/discord/guilds/" + url.PathEscape(guild) + "/channels/" + url.PathEscape(channel), nil
	}
	add := func(parent *cobra.Command, use string, count int, run func(*cobra.Command, []string) error) {
		parent.AddCommand(&cobra.Command{Use: use, Args: cobra.ExactArgs(count), RunE: run})
	}
	auth := &cobra.Command{Use: "auth"}
	root.AddCommand(auth)
	add(auth, "status", 0, func(c *cobra.Command, _ []string) error { return request(c, "GET", "/admin/session", nil) })
	instances := &cobra.Command{Use: "instance"}
	root.AddCommand(instances)
	add(instances, "list", 0, func(c *cobra.Command, _ []string) error { return request(c, "GET", "/instances", nil) })
	add(instances, "show INSTANCE", 1, func(c *cobra.Command, a []string) error {
		return request(c, "GET", "/instances/"+url.PathEscape(a[0]), nil)
	})
	add(instances, "bootstrap-admin INSTANCE", 1, func(c *cobra.Command, a []string) error {
		return request(c, "POST", "/instances/"+url.PathEscape(a[0])+"/admins/bootstrap", nil)
	})
	channels := &cobra.Command{Use: "channel"}
	root.AddCommand(channels)
	add(channels, "show CHANNEL", 1, func(c *cobra.Command, a []string) error {
		p, e := channelPath(a[0])
		if e != nil {
			return e
		}
		return request(c, "GET", p, nil)
	})
	add(channels, "bind CHANNEL", 1, func(c *cobra.Command, a []string) error {
		p, e := channelPath(a[0])
		if e != nil {
			return e
		}
		if instance == "" {
			return fmt.Errorf("--instance is required")
		}
		return request(c, "PUT", p, map[string]any{"instance_id": instance, "replace": yes})
	})
	add(channels, "unbind CHANNEL", 1, func(c *cobra.Command, a []string) error {
		if !yes {
			return fmt.Errorf("unbinding requires --yes")
		}
		p, e := channelPath(a[0])
		if e != nil {
			return e
		}
		return request(c, "DELETE", p, nil)
	})
	players := &cobra.Command{Use: "player"}
	root.AddCommand(players)
	add(players, "list", 0, func(c *cobra.Command, _ []string) error {
		p, e := instancePath()
		if e != nil {
			return e
		}
		return request(c, "GET", p+"/participants", nil)
	})
	add(players, "link PARTICIPANT", 1, func(c *cobra.Command, a []string) error {
		p, e := instancePath()
		if e != nil {
			return e
		}
		if discordUser == "" {
			return fmt.Errorf("--discord-user is required")
		}
		return request(c, "PUT", p+"/participants/"+url.PathEscape(a[0])+"/discord-link", map[string]string{"discord_user_id": discordUser})
	})
	add(players, "unlink PARTICIPANT", 1, func(c *cobra.Command, a []string) error {
		if !yes {
			return fmt.Errorf("unlinking requires --yes")
		}
		p, e := instancePath()
		if e != nil {
			return e
		}
		return request(c, "DELETE", p+"/participants/"+url.PathEscape(a[0])+"/discord-link", nil)
	})
	drafts := &cobra.Command{Use: "draft"}
	root.AddCommand(drafts)
	add(drafts, "show PARTICIPANT", 1, func(c *cobra.Command, a []string) error {
		p, e := instancePath()
		if e != nil {
			return e
		}
		return request(c, "GET", p+"/drafts/"+url.PathEscape(a[0]), nil)
	})
	add(root, "scores", 0, func(c *cobra.Command, _ []string) error {
		p, e := instancePath()
		if e != nil {
			return e
		}
		return request(c, "GET", p+"/leaderboard", nil)
	})
	return root
}

func printResult(out io.Writer, result map[string]any, asJSON bool) error {
	if !asJSON {
		for _, key := range []string{"instances", "participants", "leaderboard"} {
			rows, ok := result[key].([]any)
			if !ok {
				continue
			}
			table := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
			if key == "leaderboard" {
				if _, err := fmt.Fprintln(table, "PLAYER\tDRAFT\tBONUS\tTOTAL"); err != nil {
					return err
				}
			} else {
				if _, err := fmt.Fprintln(table, "ID\tNAME\tDISCORD USER"); err != nil {
					return err
				}
			}
			for _, value := range rows {
				row, ok := value.(map[string]any)
				if !ok {
					return fmt.Errorf("invalid API row")
				}
				var err error
				if key == "leaderboard" {
					_, err = fmt.Fprintf(table, "%v\t%v\t%v\t%v\n", row["participant_name"], row["draft_points"], row["bonus_points"], row["total_points"])
				} else {
					user := row["discord_user_id"]
					if user == nil {
						user = ""
					}
					_, err = fmt.Fprintf(table, "%v\t%v\t%v\n", row["id"], row["name"], user)
				}
				if err != nil {
					return err
				}
			}
			return table.Flush()
		}
	}
	encoder := json.NewEncoder(out)
	if !asJSON {
		encoder.SetIndent("", "  ")
	}
	return encoder.Encode(result)
}
