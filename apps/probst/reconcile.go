package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// `probst season reconcile` is the scheduled job: each run looks at the season file and the live season
// and does whatever is due, once. Every step is safe to repeat (the server dedupes), and steps run
// independently, so one failure doesn't stop the rest. It never sends anything: scores posts go to the
// admins for approval and alerts go to the admins' DMs.

type automationConfig struct {
	Enabled  bool   `yaml:"enabled"`
	FromWeek int    `yaml:"from_week"` // first week automation handles (results, scores posts)
	Guild    string `yaml:"guild"`     // the Discord server the season's channel is in
	NudgeAt  string `yaml:"nudge_at"`  // daily reminder time, ET "15:04" (default 10:00)
}

type reconciler struct {
	ctx  context.Context
	call apiCall
	f    seasonFile
	airs []time.Time
	now  time.Time
	path string
	tmpl string
	out  io.Writer
	open []string // this run's still-open issues, for the daily nudge
	// fetchPlan is fetchEpisodePlan; tests swap it.
	fetchPlan func(episode int) (episodePlan, error)
}

func (r *reconciler) logf(format string, args ...any) {
	_, _ = fmt.Fprintf(r.out, format+"\n", args...)
}

// alert DMs the season's admins once per key.
// Every alert but an expired post is also an open issue for the daily nudge.
func (r *reconciler) alert(key, body string) error {
	if !strings.HasSuffix(key, "-expired") {
		r.open = append(r.open, body)
	}
	var res struct {
		Created bool `json:"created"`
	}
	if err := r.call(r.ctx, "PUT", r.path+"/admin-alerts/"+url.PathEscape(key), map[string]string{"body": body}, &res); err != nil {
		return err
	}
	if res.Created {
		r.logf("alerted admins (%s)", key)
	}
	return nil
}

func hash8(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])[:8]
}

func (r *reconciler) rules() importRules {
	rules := importRules{}
	if v, ok := r.f.Scoring.Tribe["from_episode"].(int); ok {
		rules.TribeFrom = v
	}
	if v, ok := r.f.Scoring.MergeEpisode.(int); ok {
		rules.MergeEpisode = v
	}
	return rules
}

func (r *reconciler) imports() (map[int][]int, error) {
	var res struct {
		Imports []struct {
			Episode   int   `json:"episode_number"`
			Positions []int `json:"boot_positions"`
		} `json:"imports"`
	}
	if err := r.call(r.ctx, "GET", r.path+"/episode-imports", nil, &res); err != nil {
		return nil, err
	}
	done := map[int][]int{}
	for _, i := range res.Imports {
		done[i.Episode] = i.Positions
	}
	return done, nil
}

// importResults imports each aired episode (from from_week on) once survivoR has it complete. Missing
// results past the file's due time, holds, and conflicts become one-time admin alerts.
func (r *reconciler) importResults(done map[int][]int) error {
	var errs []error
	for ep := max(1, r.f.Automation.FromWeek); ep <= len(r.airs); ep++ {
		if _, ok := done[ep]; ok || r.now.Before(r.airs[ep-1].Add(3*time.Hour)) {
			continue
		}
		plan, err := r.fetchPlan(ep)
		if err != nil {
			errs = append(errs, fmt.Errorf("episode %d: %w", ep, err))
			continue
		}
		r.logf("%s", plan.describe())
		key := fmt.Sprintf("s%d-ep%d", r.f.Season, ep)
		switch {
		case plan.Status != "ready":
			due, err := planTime(r.f.Weekly.Results.Due, r.airs[ep-1])
			if err == nil && r.now.After(due) {
				errs = append(errs, r.alert(key+"-results-missing", fmt.Sprintf("⚠️ **Episode %d results aren't in survivoR yet** (%s). The Week %d scores post waits for them. "+
					"Enter them by hand (`probst boot` / `probst challenge`), then run `probst season import seasons/%d.yaml --episode %d --resolve-holds --yes`.", ep, plan.Why, ep, r.f.Season, ep)))
			}
		case len(plan.Holds) > 0:
			errs = append(errs, r.alert(key+"-holds-"+hash8(strings.Join(plan.Holds, "\n")), fmt.Sprintf("⚠️ **Episode %d needs a decision before it can be scored:**\n• %s\n\n"+
				"Record those by hand (`probst boot` / `probst challenge`), then run `probst season import seasons/%d.yaml --episode %d --resolve-holds --yes` to record the rest. "+
				"The Week %d scores post waits until then.", ep, strings.Join(plan.Holds, "\n• "), r.f.Season, ep, ep)))
		default:
			if err := r.applyPlan(plan, false); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

func (r *reconciler) applyPlan(plan episodePlan, manual bool) error {
	// Challenge times come from the server's episode schedule, as `probst challenge` does, so the same
	// challenge entered either way lands at the same moment and is caught as a duplicate.
	episodes, err := loadEpisodes(r.ctx, r.call, r.path)
	if err != nil {
		return err
	}
	ep, err := findEpisode(episodes, plan.Episode)
	if err != nil {
		return err
	}
	var res struct {
		Status string `json:"status"`
	}
	err = r.call(r.ctx, "POST", r.path+"/episode-imports", plan.importBody(ep.AirsAt, manual), &res)
	if err != nil && strings.Contains(err.Error(), "409") {
		return r.alert(fmt.Sprintf("s%d-ep%d-conflict-%s", r.f.Season, plan.Episode, hash8(err.Error())), fmt.Sprintf(
			"⚠️ **Episode %d results conflict with what's already recorded**, so nothing was imported: %v\n\nCheck the outcomes and challenges for that episode.", plan.Episode, err))
	}
	if err == nil {
		r.logf("episode %d: %s", plan.Episode, res.Status)
	}
	return err
}

type liveGame struct {
	Type    string `json:"type"`
	Episode int    `json:"episode_number"`
	Scored  bool   `json:"scored"`
}

// draftScoresPosts drafts each due week's scores post once its game is scored and its episode imported,
// and keeps it matching the standings until it's sent. Drafting stops 3 hours after the post's time.
func (r *reconciler) draftScoresPosts(done map[int][]int) error {
	var games struct {
		Games []liveGame `json:"games"`
	}
	if err := r.call(r.ctx, "GET", r.path+"/scheduled-games", nil, &games); err != nil {
		return err
	}
	var errs []error
	for week := max(1, r.f.Automation.FromWeek); week <= len(r.airs); week++ {
		at, err := weekPostTime(r.f, r.airs, week)
		if err != nil || r.now.Before(at) || r.f.Weeks[week].ScoresPost != nil && r.f.Weeks[week].ScoresPost.Sent {
			continue
		}
		key := scoresKey(r.f.Season, week)
		if r.now.After(at.Add(3 * time.Hour)) {
			if existing, err := findAnnouncement(r.ctx, r.call, r.path, key); err != nil || existing.Status != "sent" {
				errs = append(errs, r.alert(key+"-expired", fmt.Sprintf("⚠️ The Week %d scores post didn't go out within 3 hours of %s ET, so automation stopped. Post it by hand if you still want it.",
					week, at.Format("Mon Jan 2 3:04pm"))))
			}
			continue
		}
		var waiting []string
		if _, ok := done[week]; !ok {
			waiting = append(waiting, fmt.Sprintf("Episode %d results", week))
		}
		want := weekGame(r.f, week) // a configured game must exist and be scored first
		found := false
		for _, g := range games.Games {
			if g.Episode == week && !g.Scored {
				waiting = append(waiting, gameNames[g.Type]+" scoring")
			}
			found = found || (g.Episode == week && g.Type == want)
		}
		if want != "" && !found {
			waiting = append(waiting, gameNames[want]+" (not created)")
		}
		if len(waiting) > 0 {
			r.logf("week %d scores post waiting for %s", week, strings.Join(waiting, " and "))
			if r.now.After(at.Add(15 * time.Minute)) {
				errs = append(errs, r.alert(key+"-waiting", fmt.Sprintf("⏳ The Week %d scores post is waiting for %s; it's drafted for your approval as soon as they're in (until %s ET).",
					week, strings.Join(waiting, " and "), at.Add(3*time.Hour).Format("3:04pm"))))
			}
			continue
		}
		if err := r.draftWeek(week, at, key, done[week]); err != nil {
			errs = append(errs, fmt.Errorf("week %d scores post: %w", week, err))
		}
	}
	return errors.Join(errs...)
}

func (r *reconciler) draftWeek(week int, at time.Time, key string, bootPositions []int) error {
	var snap struct {
		Fingerprint string     `json:"fingerprint"`
		Leaderboard []scoreRow `json:"leaderboard"`
		Frozen      bool       `json:"frozen"`
	}
	if err := r.call(r.ctx, "PUT", r.path+"/score-snapshots/"+strconv.Itoa(week), map[string]string{"request_key": key}, &snap); err != nil {
		return err
	}
	if snap.Frozen {
		return nil // already sent
	}
	var prev struct {
		Leaderboard []scoreRow `json:"leaderboard"`
	}
	if week > 1 {
		if err := r.call(r.ctx, "GET", r.path+"/score-snapshots/"+strconv.Itoa(week-1), nil, &prev); err != nil {
			if !strings.Contains(err.Error(), "404") {
				return err
			}
			before, err := weekPostTime(r.f, r.airs, week-1) // weeks posted by hand: the board as of that post
			if err != nil {
				return err
			}
			if err := r.call(r.ctx, "GET", r.path+"/leaderboard?at="+url.QueryEscape(before.UTC().Format(time.RFC3339)), nil, &prev); err != nil {
				return err
			}
		}
	}
	booted, err := r.bootNames(bootPositions)
	if err != nil {
		return err
	}
	p, err := buildScoresPost(r.f.Season, week, snap.Leaderboard, prev.Leaderboard, booted, siteURL, r.nextGameLine(week))
	if err != nil {
		return err
	}
	text, err := renderScoresPost(r.tmpl, p)
	if err != nil {
		return err
	}
	var res struct {
		Action string `json:"action"`
	}
	err = r.call(r.ctx, "PUT", r.path+"/score-posts/"+url.PathEscape(key), map[string]any{
		"guild_id": r.f.Automation.Guild, "channel_id": r.f.Channel, "body": text, "send_at": at.UTC().Format(time.RFC3339),
		"score_fingerprint": snap.Fingerprint, "notify_users": true,
	}, &res)
	if err != nil && strings.Contains(err.Error(), "wasn't written by automation") {
		r.logf("week %d: %s was saved by hand; leaving it alone", week, key)
		return nil
	}
	if err == nil && res.Action != "unchanged" {
		r.logf("week %d scores post %s; admins get the approval DM", week, res.Action)
	}
	return err
}

const siteURL = "https://castaway.bry-guy.net"

// bootNames are this episode's boots (by place), spoiler-tagged.
func (r *reconciler) bootNames(positions []int) ([]string, error) {
	if len(positions) == 0 {
		return nil, nil
	}
	var outs struct {
		Outcomes []struct {
			Position int    `json:"position"`
			Name     string `json:"contestant_name"`
		} `json:"outcomes"`
	}
	if err := r.call(r.ctx, "GET", r.path+"/outcomes", nil, &outs); err != nil {
		return nil, err
	}
	sort.Sort(sort.Reverse(sort.IntSlice(positions)))
	var names []string
	for _, pos := range positions {
		for _, o := range outs.Outcomes {
			if o.Position == pos && o.Name != "" {
				names = append(names, fmt.Sprintf("||%-14s||", "  "+shortName(o.Name)))
			}
		}
	}
	return names, nil
}

func (r *reconciler) nextGameLine(week int) string {
	g := weekGame(r.f, week+1)
	line, ok := gameLines[g]
	if !ok || week >= len(r.airs) {
		return ""
	}
	opens, err := planTime(r.f.Weekly.Game.Opens, r.airs[week])
	if err != nil {
		return ""
	}
	return fmt.Sprintf(line, "tonight at "+strings.TrimSuffix(opens.Format("3:04pm"), ":00pm")+"pm ET", siteURL)
}

func (r *reconciler) run() error {
	if !r.f.Automation.Enabled {
		r.logf("automation is off for season %d (automation.enabled in the season file)", r.f.Season)
		return nil
	}
	r.open = nil
	var errs []error
	did, err := checkNextGame(r.ctx, r.call, r.f, r.airs, r.now)
	if err == nil {
		r.logf("%s", did)
		if p, ok := strings.CutPrefix(did, "alerted admins: "); ok {
			r.open = append(r.open, "⚠️ **Next week's game isn't ready.** "+p)
		} else if p, ok := strings.CutPrefix(did, "already alerted: "); ok {
			r.open = append(r.open, "⚠️ **Next week's game isn't ready.** "+p)
		}
	}
	errs = append(errs, err)
	done, err := r.imports()
	if err != nil {
		return errors.Join(append(errs, err)...)
	}
	errs = append(errs, r.importResults(done))
	if done, err = r.imports(); err != nil { // pick up what was just imported
		return errors.Join(append(errs, err)...)
	}
	errs = append(errs, r.draftScoresPosts(done))
	errs = append(errs, r.nudge())
	return errors.Join(errs...)
}

// nudge sends one daily DM, from nudge_at ET, listing every issue still open on that run. Issues are
// re-found each run, so anything fixed by then drops out; nothing open, no DM.
func (r *reconciler) nudge() error {
	at := r.f.Automation.NudgeAt
	if at == "" {
		at = "10:00"
	}
	t, err := time.Parse("15:04", at)
	if err != nil {
		return fmt.Errorf("automation.nudge_at %q: want HH:MM", at)
	}
	et := eastern()
	now := r.now.In(et)
	due := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, et)
	if len(r.open) == 0 || now.Before(due) {
		return nil
	}
	issues := r.open
	r.open = nil // the nudge isn't itself an open issue
	return r.alert(fmt.Sprintf("s%d-nudge-%s", r.f.Season, now.Format("2006-01-02")),
		"📋 **Daily reminder — still open:**\n\n"+strings.Join(issues, "\n\n"))
}

func newReconciler(c *cobra.Command, call apiCall, file string) (*reconciler, error) {
	f, err := loadSeasonFile(file)
	if err != nil {
		return nil, err
	}
	tmpl, err := seasonFilePaths(file)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	_, airs, err := expandSeason(f, now)
	if err != nil {
		return nil, err
	}
	r := &reconciler{ctx: c.Context(), call: call, f: f, airs: airs, now: now, tmpl: tmpl, out: c.OutOrStdout(), path: "/instances/" + url.PathEscape(f.Instance)}
	r.fetchPlan = func(ep int) (episodePlan, error) {
		return fetchEpisodePlan(r.ctx, call, r.path, f.Season, ep, r.rules())
	}
	return r, nil
}

func addSeasonReconcileCommand(season *cobra.Command, call apiCall) {
	season.AddCommand(&cobra.Command{
		Use:   "reconcile FILE",
		Short: "Scheduled job: readiness alerts, survivoR results import, scores-post drafts for approval (never sends)",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, a []string) error {
			r, err := newReconciler(c, call, a[0])
			if err != nil {
				return err
			}
			return r.run()
		},
	})
}

// `probst season import`: the same import by hand, e.g. after recording held items.
func addEpisodeImportCommand(season *cobra.Command, call apiCall, yes *bool) {
	var episode int
	var resolveHolds bool
	cmd := &cobra.Command{
		Use:   "import FILE --episode N",
		Short: "Import an episode's results from survivoR all at once (dry run unless --yes)",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, a []string) error {
			r, err := newReconciler(c, call, a[0])
			if err != nil {
				return err
			}
			if episode < 1 || episode > len(r.airs) {
				return fmt.Errorf("--episode must be 1-%d", len(r.airs))
			}
			plan, err := r.fetchPlan(episode)
			if err != nil {
				return err
			}
			r.logf("%s", plan.describe())
			switch {
			case plan.Status != "ready" && !resolveHolds:
				return fmt.Errorf("episode %d isn't complete in survivoR yet", episode)
			case len(plan.Holds) > 0 && !resolveHolds:
				return fmt.Errorf("episode %d has holds: record them by hand, then re-run with --resolve-holds", episode)
			case !*yes:
				r.logf("Dry run — re-run with --yes to import.")
				return nil
			}
			return r.applyPlan(plan, resolveHolds)
		},
	}
	cmd.Flags().IntVar(&episode, "episode", 0, "Episode number")
	cmd.Flags().BoolVar(&resolveHolds, "resolve-holds", false, "Held items were recorded by hand: import the rest and mark the episode done")
	season.AddCommand(cmd)
}
