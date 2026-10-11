package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// syncedSeason is the season config stored in the database (the scheduled job's source of truth).
type syncedSeason struct {
	SeasonYAML     string    `json:"season_yaml"`
	ScoresTemplate string    `json:"scores_template"`
	Revision       string    `json:"revision"`
	UpdatedAt      time.Time `json:"updated_at"`
	UpdatedBy      string    `json:"updated_by"`
}

// getSeasonConfig returns the synced config, or nil if none was synced yet.
func getSeasonConfig(ctx context.Context, call apiCall, instance string) (*syncedSeason, error) {
	var cfg syncedSeason
	err := call(ctx, "GET", "/instances/"+url.PathEscape(instance)+"/season-config", nil, &cfg)
	if err != nil && strings.Contains(err.Error(), "404") {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// `probst season sync FILE`: the file is only Probst's input. Sync refuses while the live instance disagrees
// with the file (run `season apply` first), then saves the file and its scores template to the database,
// replacing only the revision it read (a concurrent sync loses with 409).
func addSeasonSyncCommand(season *cobra.Command, call apiCall, yes *bool) {
	season.AddCommand(&cobra.Command{
		Use:   "sync FILE",
		Short: "Save the season file + scores template to the database for the scheduled job (dry run unless --yes)",
		Args:  cobra.ExactArgs(1),
		RunE:  func(c *cobra.Command, a []string) error { return syncSeason(c, call, a[0], *yes) },
	})
}

// syncSeason saves file (and its scores template) as the instance's synced season config; see sync.
func syncSeason(c *cobra.Command, call apiCall, file string, yes bool) error {
	a := []string{file}
	raw, err := os.ReadFile(a[0])
	if err != nil {
		return err
	}
	f, err := parseSeason(raw, a[0])
	if err != nil {
		return err
	}
	tmplPath, err := seasonFilePaths(a[0])
	if err != nil {
		return err
	}
	tmpl, err := os.ReadFile(tmplPath)
	if err != nil {
		return err
	}
	_, airs, err := expandSeason(f, time.Now())
	if err != nil {
		return err
	}
	diffs, err := seasonDiff(c.Context(), call, f, airs)
	if err != nil {
		return fmt.Errorf("compare with live instance: %w", err)
	}
	// Episodes must agree (the job dates everything from them); live-only posts, like held scores drafts,
	// are the job's own work and don't block.
	var conflicts []string
	for _, d := range diffs {
		if strings.HasPrefix(d, "Episode ") {
			conflicts = append(conflicts, d)
		}
	}
	if len(conflicts) > 0 {
		return fmt.Errorf("the live episodes differ from %s; fix the file or the live schedule first:\n  %s", a[0], strings.Join(conflicts, "\n  "))
	}
	cur, err := getSeasonConfig(c.Context(), call, f.Instance)
	if err != nil {
		return err
	}
	out := c.OutOrStdout()
	base := ""
	if cur != nil {
		if cur.SeasonYAML == string(raw) && cur.ScoresTemplate == string(tmpl) {
			fmt.Fprintf(out, "Already synced (revision %s).\n", cur.Revision)
			return nil
		}
		base = cur.Revision
		fmt.Fprintf(out, "Replaces revision %s (synced %s by %s).\n", cur.Revision, cur.UpdatedAt.In(eastern()).Format("Jan 2 3:04pm"), cur.UpdatedBy)
	} else {
		fmt.Fprintln(out, "First sync for this instance.")
	}
	if !yes {
		fmt.Fprintln(out, "Dry run — re-run with --yes to save.")
		return nil
	}
	var res struct {
		Revision string `json:"revision"`
	}
	err = call(c.Context(), "PUT", "/instances/"+url.PathEscape(f.Instance)+"/season-config",
		map[string]string{"season_yaml": string(raw), "scores_template": string(tmpl), "base_revision": base}, &res)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Synced revision %s; the scheduled job uses it from its next run.\n", res.Revision)
	return nil
}
