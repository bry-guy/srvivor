package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// loginSite turns an API URL (https://host/api) into the site that serves /auth.
func loginSite(apiURL string) (string, error) {
	u, err := url.Parse(strings.TrimRight(apiURL, "/"))
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("set --server or api_url to the site's /api URL, e.g. https://castaway.bry-guy.net/api")
	}
	u.Path = strings.TrimSuffix(u.Path, "/api")
	return u.String(), nil
}

// saveLogin rewrites the config file with a session token and Discord ID, keeping every other field.
func saveLogin(apiURL, token, discordUserID string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(home, ".config", "probst", "config.json")
	cfg, err := loadConfig()
	if err != nil {
		return "", err
	}
	cfg.APIURL, cfg.Token, cfg.DiscordUserID = apiURL, token, discordUserID
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return "", err
	}
	return path, os.Rename(tmp, path)
}

func openBrowser(link string) {
	name := "xdg-open"
	if runtime.GOOS == "darwin" {
		name = "open"
	}
	_ = exec.Command(name, link).Start()
}

func addLoginCommands(root *cobra.Command, server *string) {
	root.AddCommand(&cobra.Command{
		Use:   "login",
		Short: "Log in with Discord in your browser and save a probst session to the config file",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			apiURL := strings.TrimRight(*server, "/")
			site, err := loginSite(apiURL)
			if err != nil {
				return err
			}
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				return err
			}
			defer func() { _ = listener.Close() }()
			stateBytes := make([]byte, 16)
			if _, err := rand.Read(stateBytes); err != nil {
				return err
			}
			state := hex.EncodeToString(stateBytes)
			type result struct{ code, err string }
			done := make(chan result, 1)
			srv := &http.Server{ReadHeaderTimeout: 5 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/callback" || r.URL.Query().Get("state") != state {
					http.Error(w, "unexpected request", http.StatusBadRequest)
					return
				}
				res := result{code: r.URL.Query().Get("code"), err: r.URL.Query().Get("error")}
				if res.err != "" {
					_, _ = fmt.Fprintln(w, "Login refused: your Discord account isn't linked to a Castaway player or admin. You can close this tab.")
				} else {
					_, _ = fmt.Fprintln(w, "Logged in to probst. You can close this tab.")
				}
				select {
				case done <- res:
				default:
				}
			})}
			go func() { _ = srv.Serve(listener) }()
			defer func() { _ = srv.Close() }()
			port := listener.Addr().(*net.TCPAddr).Port
			link := fmt.Sprintf("%s/auth/cli?port=%d&state=%s", site, port, state)
			_, _ = fmt.Fprintf(c.ErrOrStderr(), "Opening %s\n", link)
			openBrowser(link)
			ctx, cancel := context.WithTimeout(c.Context(), 5*time.Minute)
			defer cancel()
			var res result
			select {
			case res = <-done:
			case <-ctx.Done():
				return fmt.Errorf("timed out waiting for the browser login")
			}
			if res.err != "" || res.code == "" {
				return fmt.Errorf("login refused (%s)", res.err)
			}
			body, _ := json.Marshal(map[string]string{"code": res.code})
			req, err := http.NewRequestWithContext(ctx, "POST", apiURL+"/auth/cli/exchange", bytes.NewReader(body))
			if err != nil {
				return err
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
			if err != nil {
				return err
			}
			defer func() { _ = resp.Body.Close() }()
			var out struct {
				Token     string `json:"token"`
				DiscordID string `json:"discord_user_id"`
				Username  string `json:"discord_username"`
				ExpiresAt string `json:"expires_at"`
				Error     string `json:"error"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || resp.StatusCode != http.StatusOK {
				return fmt.Errorf("exchange failed: HTTP %d %s", resp.StatusCode, out.Error)
			}
			path, err := saveLogin(apiURL, out.Token, out.DiscordID)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(c.OutOrStdout(), "Logged in as %s (%s) until %s; saved to %s.\n", out.Username, out.DiscordID, out.ExpiresAt, path)
			return err
		},
	})
}
