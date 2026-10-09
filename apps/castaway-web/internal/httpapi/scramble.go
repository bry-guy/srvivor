package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Island Scramble: arrange letter tiles (the phrase's letters plus random decoys) into a hidden phrase. A
// player's tiles stay hidden until they press Start, which starts their clock, so working it out somewhere
// else still counts as time. Checks only say right or wrong. At cutoff the three fastest solves earn
// +3/+2/+1; fewer wrong checks breaks a tie on time, and exact ties share the place.

const (
	scrambleMaxLetters = 30 // phrase letters
	scrambleMaxDecoys  = 10 // so the tray is at most 40 tiles, about five rows on an iPhone SE
	scrambleRowWidth   = 9  // slots per row on a 320px phone; longer words wrap
)

var errScramblePhrase = fmt.Errorf("phrase must be letters A–Z and spaces, at most %d letters, with at most %d decoys", scrambleMaxLetters, scrambleMaxDecoys)

// scramblePhrase upper-cases a phrase, collapses its spaces, and checks the letter and decoy limits.
func scramblePhrase(raw string, decoys int) (string, error) {
	phrase := strings.Join(strings.Fields(strings.ToUpper(raw)), " ")
	letters := strings.ReplaceAll(phrase, " ", "")
	if letters == "" || decoys < 0 || decoys > scrambleMaxDecoys || len(letters) > scrambleMaxLetters || strings.Trim(letters, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") != "" {
		return "", errScramblePhrase
	}
	return phrase, nil
}

func randomLetters(n int) string {
	var b strings.Builder
	for range n {
		b.WriteByte(byte('A' + rand.IntN(26))) // #nosec G404 -- decoys and tile order needn't be unpredictable
	}
	return b.String()
}

// scrambleTray is the phrase's letters plus the decoys, shuffled.
func scrambleTray(phrase, decoys string) string {
	tiles := []byte(strings.ReplaceAll(phrase, " ", "") + decoys)
	rand.Shuffle(len(tiles), func(i, j int) { tiles[i], tiles[j] = tiles[j], tiles[i] }) // #nosec G404
	return string(tiles)
}

// fromTray reports whether guess can be made from the tray's tiles, each used at most once.
func fromTray(tray, guess string) bool {
	left := map[rune]int{}
	for _, r := range tray {
		left[r]++
	}
	for _, r := range guess {
		if left[r]--; left[r] < 0 {
			return false
		}
	}
	return true
}

// scrambleSlots lays out a phrase as words of rows of slot numbers; words longer than a row wrap.
func scrambleSlots(phrase string) [][][]int {
	var words [][][]int
	slot := 0
	for _, word := range strings.Fields(phrase) {
		var rows [][]int
		for len(word) > 0 {
			n := min(len(word), scrambleRowWidth)
			row := make([]int, n)
			for i := range row {
				row[i] = slot
				slot++
			}
			rows = append(rows, row)
			word = word[n:]
		}
		words = append(words, rows)
	}
	return words
}

type scrambleSolve struct {
	ID          string
	Seconds     int64
	WrongChecks int
}

// scramblePlaces ranks solves by time, then wrong checks; exact ties share a place. Places 1–3 earn 3, 2
// and 1 points (so a tie for first gives both +3 and the next solver is third).
func scramblePlaces(solves []scrambleSolve) map[string]int {
	sort.SliceStable(solves, func(i, j int) bool {
		if solves[i].Seconds != solves[j].Seconds {
			return solves[i].Seconds < solves[j].Seconds
		}
		return solves[i].WrongChecks < solves[j].WrongChecks
	})
	places := map[string]int{}
	for i, s := range solves {
		place := i + 1
		if i > 0 && s.Seconds == solves[i-1].Seconds && s.WrongChecks == solves[i-1].WrongChecks {
			place = places[solves[i-1].ID]
		}
		places[s.ID] = place
	}
	return places
}

func placePoints(place int) int { return max(4-place, 0) }

func scrambleDuration(seconds int64) string {
	d := time.Duration(seconds) * time.Second
	if d >= time.Hour {
		return fmt.Sprintf("%d:%02d:%02d", int(d.Hours()), int(d.Minutes())%60, seconds%60)
	}
	return fmt.Sprintf("%d:%02d", int(d.Minutes()), seconds%60)
}

type scrambleGame struct {
	ID       uuid.UUID `json:"id"`
	Episode  *int32    `json:"episode_number"`
	OpensAt  time.Time `json:"opens_at"`
	CutoffAt time.Time `json:"cutoff_at"`
	Resolved bool      `json:"resolved"`
}

// createScrambleGame creates (or, before it resolves, replaces) an instance's game for an episode; without
// an episode it creates an admin-only test. decoys is a count of random letters, or decoy_letters lists them.
func (s *Server) createScrambleGame(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok || !s.requireInstanceAdminRequest(c, instanceID) {
		return
	}
	var req struct {
		Episode      *int32    `json:"episode_number"`
		Phrase       string    `json:"phrase" binding:"required"`
		Decoys       int       `json:"decoys"`
		DecoyLetters string    `json:"decoy_letters"`
		OpensAt      time.Time `json:"opens_at" binding:"required"`
		CutoffAt     time.Time `json:"cutoff_at" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || !req.CutoffAt.After(req.OpensAt) {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "phrase, opens_at and a later cutoff_at are required"})
		return
	}
	decoys := strings.ToUpper(req.DecoyLetters)
	if decoys == "" {
		decoys = randomLetters(max(req.Decoys, 0))
	}
	phrase, err := scramblePhrase(req.Phrase, len(decoys))
	if err != nil || strings.Trim(decoys, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") != "" {
		c.JSON(http.StatusBadRequest, errorResponse{Error: errScramblePhrase.Error()})
		return
	}
	game := scrambleGame{Episode: req.Episode}
	conflict := ""
	if req.Episode != nil { // re-applying replaces the puzzle, so nobody may have started it yet
		conflict = `ON CONFLICT (instance_id, episode_number) DO UPDATE SET phrase = EXCLUDED.phrase, decoys = EXCLUDED.decoys,
			opens_at = EXCLUDED.opens_at, cutoff_at = EXCLUDED.cutoff_at
			WHERE scramble_games.resolved_at IS NULL AND NOT EXISTS (SELECT 1 FROM scramble_plays p WHERE p.game_id = scramble_games.id)`
	}
	err = s.pool.QueryRow(c.Request.Context(), `INSERT INTO scramble_games (instance_id, episode_number, phrase, decoys, opens_at, cutoff_at)
		SELECT id, $2, $3, $4, $5, $6 FROM instances WHERE public_id = $1 `+conflict+`
		RETURNING public_id, opens_at, cutoff_at, resolved_at IS NOT NULL`, instanceID, req.Episode, phrase, decoys, req.OpensAt, req.CutoffAt).
		Scan(&game.ID, &game.OpensAt, &game.CutoffAt, &game.Resolved)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusConflict, errorResponse{Error: "instance not found, or this episode's game already started or resolved"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "could not save game"})
		return
	}
	c.JSON(http.StatusOK, game)
}

func (s *Server) resolveScrambleGameRequest(c *gin.Context) {
	gameID, ok := parseUUIDPath(c, "gameID")
	if !ok {
		return
	}
	var instanceID uuid.UUID
	err := s.pool.QueryRow(c.Request.Context(), `SELECT i.public_id FROM scramble_games g JOIN instances i ON i.id = g.instance_id WHERE g.public_id = $1`, gameID).Scan(&instanceID)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, errorResponse{Error: "game not found"})
		return
	}
	if err != nil || !s.requireInstanceAdminRequest(c, instanceID) {
		if err != nil {
			c.JSON(http.StatusInternalServerError, errorResponse{Error: "could not load game"})
		}
		return
	}
	awards, err := s.resolveScrambleGame(c.Request.Context(), gameID)
	if err != nil {
		c.JSON(http.StatusConflict, errorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"awards": awards})
}

type scrambleQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

type scramblePlayer struct {
	ID, Name, Tribe string
	Solved          bool
	Seconds         int64
	WrongChecks     int
}

// loadScramblePlayers reads everyone who started a game (solves count only before cutoff).
func loadScramblePlayers(ctx context.Context, q scrambleQuerier, gameID int64) ([]scramblePlayer, error) {
	rows, err := q.Query(ctx, `SELECT p.public_id::text, p.name, COALESCE(t.name, ''), sp.solved_at IS NOT NULL AND sp.solved_at <= g.cutoff_at,
			COALESCE(EXTRACT(EPOCH FROM sp.solved_at - sp.started_at)::bigint, 0), sp.wrong_checks
		FROM scramble_plays sp JOIN participants p ON p.id = sp.participant_id JOIN scramble_games g ON g.id = sp.game_id
		LEFT JOIN LATERAL (SELECT pg.name FROM participant_group_membership_periods m JOIN participant_groups pg ON pg.id = m.participant_group_id
			WHERE m.participant_id = p.id AND pg.kind = 'tribe' AND m.starts_at <= g.cutoff_at AND (m.ends_at IS NULL OR m.ends_at > g.cutoff_at)
			ORDER BY m.starts_at DESC LIMIT 1) t ON true
		WHERE sp.game_id = $1 ORDER BY p.name`, gameID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (scramblePlayer, error) {
		var p scramblePlayer
		return p, row.Scan(&p.ID, &p.Name, &p.Tribe, &p.Solved, &p.Seconds, &p.WrongChecks)
	})
}

func scramblePlayerPlaces(players []scramblePlayer) map[string]int {
	var solves []scrambleSolve
	for _, p := range players {
		if p.Solved {
			solves = append(solves, scrambleSolve{ID: p.ID, Seconds: p.Seconds, WrongChecks: p.WrongChecks})
		}
	}
	return scramblePlaces(solves)
}

var errScrambleOpen = errors.New("game cannot resolve before its cutoff")

// resolveScrambleGame writes the top three's awards to the bonus ledger once; later calls return nil.
func (s *Server) resolveScrambleGame(ctx context.Context, gameID uuid.UUID) (map[string]int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			requestLogger.Error("rollback scramble game", "error", err)
		}
	}()
	var (
		id            int64
		instance      uuid.UUID
		episode       *int32
		opens, cutoff time.Time
		resolved      bool
	)
	if err := tx.QueryRow(ctx, `SELECT g.id, i.public_id, g.episode_number, g.opens_at, g.cutoff_at, g.resolved_at IS NOT NULL
		FROM scramble_games g JOIN instances i ON i.id = g.instance_id WHERE g.public_id = $1 FOR UPDATE OF g`, gameID).
		Scan(&id, &instance, &episode, &opens, &cutoff, &resolved); err != nil {
		return nil, err
	}
	if resolved {
		return nil, nil
	}
	if s.now().Before(cutoff) {
		return nil, errScrambleOpen
	}
	players, err := loadScramblePlayers(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	places := scramblePlayerPlaces(players)
	awards := map[string]int{}
	for player, place := range places {
		if points := placePoints(place); points > 0 {
			awards[player] = points
		}
	}
	if len(awards) > 0 && episode != nil { // tests report what they would award, but never score
		q := s.queries.WithTx(tx)
		at := pgtype.Timestamptz{Time: cutoff, Valid: true}
		name := fmt.Sprintf("Island Scramble: Episode %d", *episode)
		activity, err := q.CreateInstanceActivity(ctx, db.CreateInstanceActivityParams{
			ActivityType: "spell_it_out", Name: name, Status: "completed", Metadata: []byte("{}"),
			StartsAt: pgtype.Timestamptz{Time: opens, Valid: true}, EndsAt: at, InstanceID: toPGUUID(instance),
		})
		if err != nil {
			return nil, err
		}
		occurrence, err := q.CreateActivityOccurrence(ctx, db.CreateActivityOccurrenceParams{
			OccurrenceType: "spell_it_out", Name: name, EffectiveAt: at, StartsAt: activity.StartsAt, EndsAt: at,
			Status: "resolved", Metadata: []byte("{}"), ActivityID: activity.ID,
		})
		if err != nil {
			return nil, err
		}
		for _, p := range players {
			points := awards[p.ID]
			if points == 0 {
				continue
			}
			metadata, err := json.Marshal(map[string]int64{"place": int64(places[p.ID]), "seconds": p.Seconds, "wrong_checks": int64(p.WrongChecks)})
			if err != nil {
				return nil, err
			}
			if _, err := q.CreateBonusPointLedgerEntry(ctx, db.CreateBonusPointLedgerEntryParams{
				EntryKind: "award", Points: int32(points), Visibility: "public", Reason: name, EffectiveAt: at, // #nosec G115 -- points are 1–3
				AwardKey: pgtype.Text{String: "spell_it_out", Valid: true}, Metadata: metadata,
				InstanceID: toPGUUID(instance), ParticipantID: toPGUUID(uuid.MustParse(p.ID)), ActivityOccurrenceID: occurrence.ID,
			}); err != nil {
				return nil, err
			}
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE scramble_games SET resolved_at = $2 WHERE id = $1`, id, s.now()); err != nil {
		return nil, err
	}
	return awards, tx.Commit(ctx)
}

// resolveDueScrambleGames scores every scheduled game past its cutoff (RunButtonResolver calls it).
func (s *Server) resolveDueScrambleGames(ctx context.Context) error {
	rows, err := s.pool.Query(ctx, `SELECT public_id FROM scramble_games WHERE resolved_at IS NULL AND cutoff_at <= $1 AND episode_number IS NOT NULL`, s.now())
	if err != nil {
		return err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := s.resolveScrambleGame(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

type scrambleView struct {
	ID          string
	Episode     *int32
	Test        bool
	Open        bool
	Upcoming    bool
	OpensAt     time.Time
	CutoffAt    time.Time
	Action      string
	Words       [][][]int // words → rows → slot numbers
	Started     bool
	StartedAt   int64 // unix ms, for the on-page clock
	Tiles       []string
	Solved      bool
	Phrase      string   // shown once solved, or once results are out
	Letters     []string // the phrase's letter for each slot, when Phrase is shown
	Time        string
	WrongChecks int
	Results     []scrambleResult
	ResultsWait bool
}

type scrambleResult struct {
	Place  int // 0: not solved
	ID     string
	Name   string
	Tribe  string
	Time   string
	Wrong  int
	Points int
}

func (v *scrambleView) reveal(phrase string) {
	v.Phrase = phrase
	v.Letters = strings.Split(strings.ReplaceAll(phrase, " ", ""), "")
}

var errScrambleNotFound = errors.New("game not found")

// loadScramble loads /games/scramble/:gameID (or the open scheduled game) for the signed-in player.
func (s *Server) loadScramble(c *gin.Context, data sitePageData) (*scrambleView, int64, string, bool, error) {
	ctx := c.Request.Context()
	var (
		dbID     int64
		phrase   string
		resolved bool
		view     scrambleView
	)
	query := `SELECT g.id, g.public_id::text, g.episode_number, g.opens_at, g.cutoff_at, g.phrase, g.resolved_at IS NOT NULL
		FROM scramble_games g JOIN instances i ON i.id = g.instance_id WHERE i.public_id = $1 AND `
	args := []any{s.public.InstanceID}
	if id := c.Param("gameID"); id != "" {
		if _, err := uuid.Parse(id); err != nil {
			return nil, 0, "", false, errScrambleNotFound
		}
		query += `g.public_id::text = $2 AND (g.episode_number IS NOT NULL OR $3)`
		args = append(args, id, data.Admin)
	} else {
		query += `g.episode_number IS NOT NULL AND g.opens_at <= $2 AND g.cutoff_at > $2 ORDER BY g.opens_at DESC LIMIT 1`
		args = append(args, s.now())
	}
	err := s.pool.QueryRow(ctx, query, args...).Scan(&dbID, &view.ID, &view.Episode, &view.OpensAt, &view.CutoffAt, &phrase, &resolved)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, 0, "", false, errScrambleNotFound
	}
	if err != nil {
		return nil, 0, "", false, err
	}
	now := s.now()
	view.Test = view.Episode == nil
	view.Open = !now.Before(view.OpensAt) && now.Before(view.CutoffAt)
	view.Upcoming = now.Before(view.OpensAt)
	view.OpensAt, view.CutoffAt = easternTime(view.OpensAt), easternTime(view.CutoffAt)
	view.Action = "/games/scramble/" + view.ID
	view.Words = scrambleSlots(phrase)
	return &view, dbID, phrase, resolved, nil
}

// fillScramble adds the player's play (open) or everyone's results (once scored; tests once closed).
func (s *Server) fillScramble(ctx context.Context, view *scrambleView, dbID int64, phrase string, resolved bool, discordID string) error {
	var tiles string
	var started time.Time
	var solved *time.Time
	err := s.pool.QueryRow(ctx, `SELECT sp.tiles, sp.started_at, sp.solved_at, sp.wrong_checks FROM scramble_plays sp
		JOIN participants p ON p.id = sp.participant_id JOIN instances i ON i.id = p.instance_id
		WHERE sp.game_id = $1 AND i.public_id = $2 AND p.discord_user_id = $3`, dbID, s.public.InstanceID, discordID).
		Scan(&tiles, &started, &solved, &view.WrongChecks)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if err == nil {
		view.Started, view.StartedAt = true, started.UnixMilli()
		view.Tiles = strings.Split(tiles, "")
		if solved != nil {
			view.Solved = true
			view.reveal(phrase)
			view.Time = scrambleDuration(int64(solved.Sub(started) / time.Second))
		}
	}
	if view.Open || view.Upcoming {
		return nil
	}
	if !resolved && !view.Test {
		view.ResultsWait = true
		return nil
	}
	view.reveal(phrase)
	players, err := loadScramblePlayers(ctx, s.pool, dbID)
	if err != nil {
		return err
	}
	places := scramblePlayerPlaces(players)
	for _, p := range players {
		r := scrambleResult{Place: places[p.ID], ID: p.ID, Name: p.Name, Tribe: p.Tribe, Wrong: p.WrongChecks, Points: placePoints(places[p.ID])}
		if p.Solved {
			r.Time = scrambleDuration(p.Seconds)
		}
		view.Results = append(view.Results, r)
	}
	sort.SliceStable(view.Results, func(i, j int) bool {
		a, b := view.Results[i], view.Results[j]
		return a.Place != 0 && (b.Place == 0 || a.Place < b.Place)
	})
	return nil
}

func (s *Server) scramblePage(c *gin.Context) {
	data, ok := s.siteData(c)
	if !ok {
		return
	}
	if !data.Allowed {
		renderSite(c, "home.html", data)
		return
	}
	view, dbID, phrase, resolved, err := s.loadScramble(c, data)
	if errors.Is(err, errScrambleNotFound) {
		if c.Param("gameID") == "" {
			c.Redirect(http.StatusFound, "/games")
			return
		}
		c.String(http.StatusNotFound, "game not found")
		return
	}
	if err == nil {
		err = s.fillScramble(c.Request.Context(), view, dbID, phrase, resolved, data.User.DiscordUserID)
	}
	if err != nil {
		c.String(http.StatusInternalServerError, "could not load the page")
		return
	}
	data.Scramble = view
	renderSite(c, "scramble.html", data)
}

// startScramble reveals the player's tiles and starts their clock (once; later calls change nothing).
func (s *Server) startScramble(c *gin.Context) {
	data, ok := s.siteData(c)
	if !ok {
		return
	}
	view, dbID, phrase, _, err := s.loadScramble(c, data)
	if errors.Is(err, errScrambleNotFound) {
		c.Status(http.StatusNotFound)
		return
	}
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	if !data.Allowed || !view.Open {
		c.Status(http.StatusConflict)
		return
	}
	var decoys string
	if err := s.pool.QueryRow(c.Request.Context(), `SELECT decoys FROM scramble_games WHERE id = $1`, dbID).Scan(&decoys); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	tag, err := s.pool.Exec(c.Request.Context(), `INSERT INTO scramble_plays (game_id, participant_id, tiles, started_at)
		SELECT $1, p.id, $4, $5 FROM participants p JOIN instances i ON i.id = p.instance_id WHERE i.public_id = $2 AND p.discord_user_id = $3
		ON CONFLICT DO NOTHING`, dbID, s.public.InstanceID, data.User.DiscordUserID, scrambleTray(phrase, decoys), s.now())
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	if tag.RowsAffected() == 0 { // already started, or not a player in this season
		var exists bool
		if err := s.pool.QueryRow(c.Request.Context(), `SELECT EXISTS (SELECT 1 FROM scramble_plays sp JOIN participants p ON p.id = sp.participant_id
			WHERE sp.game_id = $1 AND p.discord_user_id = $2)`, dbID, data.User.DiscordUserID).Scan(&exists); err != nil || !exists {
			c.Status(http.StatusConflict)
			return
		}
	}
	c.Redirect(http.StatusSeeOther, view.Action)
}

// checkScramble checks a full arrangement. Right: the solve time is saved. Wrong: one more wrong check.
// Either way it says nothing about which letters are right.
func (s *Server) checkScramble(c *gin.Context) {
	data, ok := s.siteData(c)
	if !ok {
		return
	}
	view, dbID, phrase, _, err := s.loadScramble(c, data)
	if errors.Is(err, errScrambleNotFound) {
		c.Status(http.StatusNotFound)
		return
	}
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	if !data.Allowed || !view.Open {
		c.JSON(http.StatusConflict, errorResponse{Error: "this game is closed"})
		return
	}
	var req struct {
		Guess string `json:"guess"`
	}
	answer := strings.ReplaceAll(phrase, " ", "")
	guess := ""
	if c.ShouldBindJSON(&req) == nil {
		guess = strings.ToUpper(req.Guess)
	}
	ctx := c.Request.Context()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			requestLogger.Error("rollback scramble check", "error", err)
		}
	}()
	var (
		participant int64
		tiles       string
		started     time.Time
		solved      *time.Time
		wrong       int
	)
	err = tx.QueryRow(ctx, `SELECT sp.participant_id, sp.tiles, sp.started_at, sp.solved_at, sp.wrong_checks FROM scramble_plays sp
		JOIN participants p ON p.id = sp.participant_id JOIN instances i ON i.id = p.instance_id
		WHERE sp.game_id = $1 AND i.public_id = $2 AND p.discord_user_id = $3 FOR UPDATE OF sp`, dbID, s.public.InstanceID, data.User.DiscordUserID).
		Scan(&participant, &tiles, &started, &solved, &wrong)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusConflict, errorResponse{Error: "press Start first"})
		return
	}
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	if solved == nil {
		if len(guess) != len(answer) || !fromTray(tiles, guess) {
			c.JSON(http.StatusBadRequest, errorResponse{Error: "fill every slot with your tiles"})
			return
		}
		if guess == answer {
			now := s.now()
			solved = &now
			_, err = tx.Exec(ctx, `UPDATE scramble_plays SET solved_at = $3 WHERE game_id = $1 AND participant_id = $2`, dbID, participant, now)
		} else {
			wrong++
			_, err = tx.Exec(ctx, `UPDATE scramble_plays SET wrong_checks = wrong_checks + 1 WHERE game_id = $1 AND participant_id = $2`, dbID, participant)
		}
		if err == nil {
			err = tx.Commit(ctx)
		}
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
	}
	body := gin.H{"solved": solved != nil, "wrong_checks": wrong}
	if solved != nil {
		body["time"] = scrambleDuration(int64(solved.Sub(started) / time.Second))
	}
	c.JSON(http.StatusOK, body)
}

// createScrambleTest starts a one-hour admin-only test puzzle and opens it.
func (s *Server) createScrambleTest(c *gin.Context) {
	data, ok := s.siteData(c)
	if !ok {
		return
	}
	if !data.Admin {
		c.Status(http.StatusNotFound)
		return
	}
	var decoys int
	if _, err := fmt.Sscan(c.PostForm("decoys"), &decoys); err != nil {
		decoys = 0
	}
	letters := randomLetters(max(decoys, 0))
	phrase, err := scramblePhrase(c.PostForm("phrase"), len(letters))
	if err != nil {
		c.String(http.StatusBadRequest, errScramblePhrase.Error())
		return
	}
	var id uuid.UUID
	now := s.now()
	if err := s.pool.QueryRow(c.Request.Context(), `INSERT INTO scramble_games (instance_id, phrase, decoys, opens_at, cutoff_at)
		SELECT id, $2, $3, $4, $5 FROM instances WHERE public_id = $1 RETURNING public_id`, s.public.InstanceID, phrase, letters, now, now.Add(time.Hour)).Scan(&id); err != nil {
		c.String(http.StatusInternalServerError, "could not create a test game")
		return
	}
	c.Redirect(http.StatusSeeOther, "/games/scramble/"+id.String())
}

// addScrambleGames lists an instance's Island Scramble games (tests only for admins).
func (s *Server) addScrambleGames(ctx context.Context, data *sitePageData, instanceID string) error {
	rows, err := s.pool.Query(ctx, `SELECT g.public_id::text, g.episode_number, g.opens_at, g.cutoff_at, g.resolved_at IS NOT NULL
		FROM scramble_games g JOIN instances i ON i.id = g.instance_id WHERE i.public_id::text = $1 AND (g.episode_number IS NOT NULL OR $2)
		ORDER BY g.opens_at`, instanceID, data.Admin)
	if err != nil {
		return err
	}
	defer rows.Close()
	now := s.now()
	for rows.Next() {
		var id string
		var episode *int32
		var opens, cutoff time.Time
		var resolved bool
		if err := rows.Scan(&id, &episode, &opens, &cutoff, &resolved); err != nil {
			return err
		}
		badge := "Scored"
		if episode == nil {
			badge, resolved = "Admin-only test", true
		}
		data.addGame(episode, gameEntry{Kind: "Island Scramble", Name: "Island Scramble", URL: "/games/scramble/" + id, Badge: badge, Status: gameStatus(now, opens, cutoff, resolved)})
	}
	return rows.Err()
}

// scrambleProfileGames is a player's scored Island Scramble results.
func (s *Server) scrambleProfileGames(ctx context.Context, participantID string) ([]profileGame, error) {
	rows, err := s.pool.Query(ctx, `SELECT g.id, g.public_id::text, g.episode_number FROM scramble_games g JOIN scramble_plays sp ON sp.game_id = g.id
		JOIN participants p ON p.id = sp.participant_id WHERE p.public_id = $1 AND g.episode_number IS NOT NULL AND g.resolved_at IS NOT NULL`, participantID)
	if err != nil {
		return nil, err
	}
	type ref struct {
		dbID    int64
		id      string
		episode int32
	}
	refs, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (ref, error) {
		var r ref
		return r, row.Scan(&r.dbID, &r.id, &r.episode)
	})
	if err != nil {
		return nil, err
	}
	var games []profileGame
	for _, r := range refs {
		players, err := loadScramblePlayers(ctx, s.pool, r.dbID)
		if err != nil {
			return nil, err
		}
		result := "Unsolved"
		for _, p := range players {
			if p.ID == participantID && p.Solved {
				place := scramblePlayerPlaces(players)[p.ID]
				result = fmt.Sprintf("Solved in %s · %+d pts", scrambleDuration(p.Seconds), placePoints(place))
			}
		}
		games = append(games, profileGame{Episode: r.episode, Kind: "Island Scramble", URL: "/games/scramble/" + r.id, Result: result})
	}
	return games, nil
}
