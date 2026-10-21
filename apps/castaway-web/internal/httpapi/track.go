package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/bits"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Inside Track: flick 12 balls up three foam tracks into three cups (site/assets/inside-track.js). One run
// per player per game. The page reports which cups are filled and how many balls are used after every ball;
// the server only checks that progress moves forward, so the physics runs on the player's device
// (shortcut: a determined player could post a fake score; fine for a friends' league).
// In-game score: cups filled, plus balls left once all three are filled (max 3 + 11 = 14).
// League points: best score +3, second +2, everyone else who played +1; ties share the place.

const trackBalls = 12

type trackPlayer struct {
	ID, Name, Tribe string
	CupsMask        int
	BallsUsed       int
	Finished        bool
}

func trackScore(mask, used int) int {
	cups := bits.OnesCount(uint(mask))
	if cups == 3 {
		return cups + trackBalls - used
	}
	return cups
}

// trackPoints ranks played runs by score: first +3, second +2, the rest +1; ties share the place.
func trackPoints(players []trackPlayer) (map[string]int, map[string]int) {
	places, points := map[string]int{}, map[string]int{}
	for _, p := range players {
		place := 1
		for _, o := range players {
			if trackScore(o.CupsMask, o.BallsUsed) > trackScore(p.CupsMask, p.BallsUsed) {
				place++
			}
		}
		places[p.ID], points[p.ID] = place, max(4-place, 1)
	}
	return places, points
}

func loadTrackPlayers(ctx context.Context, q scrambleQuerier, gameID int64) ([]trackPlayer, error) {
	rows, err := q.Query(ctx, `SELECT p.public_id::text, p.name, COALESCE(t.name, ''), tp.cups_mask, tp.balls_used, tp.finished_at IS NOT NULL
		FROM track_plays tp JOIN participants p ON p.id = tp.participant_id JOIN track_games g ON g.id = tp.game_id
		LEFT JOIN LATERAL (SELECT pg.name FROM participant_group_membership_periods m JOIN participant_groups pg ON pg.id = m.participant_group_id
			WHERE m.participant_id = p.id AND pg.kind = 'tribe' AND m.starts_at <= g.cutoff_at AND (m.ends_at IS NULL OR m.ends_at > g.cutoff_at)
			ORDER BY m.starts_at DESC LIMIT 1) t ON true
		WHERE tp.game_id = $1 ORDER BY p.name`, gameID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (trackPlayer, error) {
		var p trackPlayer
		return p, row.Scan(&p.ID, &p.Name, &p.Tribe, &p.CupsMask, &p.BallsUsed, &p.Finished)
	})
}

type trackGame struct {
	ID       uuid.UUID `json:"id"`
	Episode  int32     `json:"episode_number"`
	OpensAt  time.Time `json:"opens_at"`
	CutoffAt time.Time `json:"cutoff_at"`
	Resolved bool      `json:"resolved"`
}

// createTrackGame creates (or, before anyone plays it, reschedules) an instance's game for an episode.
func (s *Server) createTrackGame(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok || !s.requireInstanceAdminRequest(c, instanceID) {
		return
	}
	var req struct {
		Episode  int32     `json:"episode_number" binding:"required"`
		OpensAt  time.Time `json:"opens_at" binding:"required"`
		CutoffAt time.Time `json:"cutoff_at" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || !req.CutoffAt.After(req.OpensAt) {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "episode_number, opens_at and a later cutoff_at are required"})
		return
	}
	game := trackGame{Episode: req.Episode}
	err := s.pool.QueryRow(c.Request.Context(), `INSERT INTO track_games (instance_id, episode_number, opens_at, cutoff_at)
		SELECT id, $2, $3, $4 FROM instances WHERE public_id = $1
		ON CONFLICT (instance_id, episode_number) DO UPDATE SET opens_at = EXCLUDED.opens_at, cutoff_at = EXCLUDED.cutoff_at
			WHERE track_games.resolved_at IS NULL AND NOT EXISTS (SELECT 1 FROM track_plays p WHERE p.game_id = track_games.id)
		RETURNING public_id, opens_at, cutoff_at, resolved_at IS NOT NULL`, instanceID, req.Episode, req.OpensAt, req.CutoffAt).
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

func (s *Server) resolveTrackGameRequest(c *gin.Context) {
	gameID, ok := parseUUIDPath(c, "gameID")
	if !ok {
		return
	}
	var instanceID uuid.UUID
	err := s.pool.QueryRow(c.Request.Context(), `SELECT i.public_id FROM track_games g JOIN instances i ON i.id = g.instance_id WHERE g.public_id = $1`, gameID).Scan(&instanceID)
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
	awards, err := s.resolveTrackGame(c.Request.Context(), gameID)
	if err != nil {
		c.JSON(http.StatusConflict, errorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"awards": awards})
}

// resolveTrackGame writes every player's points to the bonus ledger once; later calls return nil.
func (s *Server) resolveTrackGame(ctx context.Context, gameID uuid.UUID) (map[string]int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			requestLogger.Error("rollback track game", "error", err)
		}
	}()
	var (
		id            int64
		instance      uuid.UUID
		episode       int32
		opens, cutoff time.Time
		resolved      bool
	)
	if err := tx.QueryRow(ctx, `SELECT g.id, i.public_id, g.episode_number, g.opens_at, g.cutoff_at, g.resolved_at IS NOT NULL
		FROM track_games g JOIN instances i ON i.id = g.instance_id WHERE g.public_id = $1 FOR UPDATE OF g`, gameID).
		Scan(&id, &instance, &episode, &opens, &cutoff, &resolved); err != nil {
		return nil, err
	}
	if resolved {
		return nil, nil
	}
	if s.now().Before(cutoff) {
		return nil, errScrambleOpen
	}
	players, err := loadTrackPlayers(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	places, awards := trackPoints(players)
	if len(players) > 0 {
		q := s.queries.WithTx(tx)
		at := pgtype.Timestamptz{Time: cutoff, Valid: true}
		name := fmt.Sprintf("Inside Track: Episode %d", episode)
		activity, err := q.CreateInstanceActivity(ctx, db.CreateInstanceActivityParams{
			ActivityType: "inside_track", Name: name, Status: "completed", Metadata: []byte("{}"),
			StartsAt: pgtype.Timestamptz{Time: opens, Valid: true}, EndsAt: at, InstanceID: toPGUUID(instance),
		})
		if err != nil {
			return nil, err
		}
		occurrence, err := q.CreateActivityOccurrence(ctx, db.CreateActivityOccurrenceParams{
			OccurrenceType: "inside_track", Name: name, EffectiveAt: at, StartsAt: activity.StartsAt, EndsAt: at,
			Status: "resolved", Metadata: []byte("{}"), ActivityID: activity.ID,
		})
		if err != nil {
			return nil, err
		}
		for _, p := range players {
			metadata, err := json.Marshal(map[string]int{"place": places[p.ID], "score": trackScore(p.CupsMask, p.BallsUsed),
				"cups": bits.OnesCount(uint(p.CupsMask)), "balls_used": p.BallsUsed})
			if err != nil {
				return nil, err
			}
			if _, err := q.CreateBonusPointLedgerEntry(ctx, db.CreateBonusPointLedgerEntryParams{
				EntryKind: "award", Points: int32(awards[p.ID]), Visibility: "public", Reason: name, EffectiveAt: at, // #nosec G115 -- points are 1–3
				AwardKey: pgtype.Text{String: "inside_track", Valid: true}, Metadata: metadata,
				InstanceID: toPGUUID(instance), ParticipantID: toPGUUID(uuid.MustParse(p.ID)), ActivityOccurrenceID: occurrence.ID,
			}); err != nil {
				return nil, err
			}
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE track_games SET resolved_at = $2 WHERE id = $1`, id, s.now()); err != nil {
		return nil, err
	}
	return awards, tx.Commit(ctx)
}

func (s *Server) resolveDueTrackGames(ctx context.Context) error {
	rows, err := s.pool.Query(ctx, `SELECT public_id FROM track_games WHERE resolved_at IS NULL AND cutoff_at <= $1`, s.now())
	if err != nil {
		return err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := s.resolveTrackGame(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

type trackView struct {
	ID          string
	Episode     int32
	Open        bool
	Upcoming    bool
	OpensAt     time.Time
	CutoffAt    time.Time
	Played      bool // finished this run
	Score       int
	Cups        int
	BallsLeft   int
	Results     []trackResult
	ResultsWait bool
}

type trackResult struct {
	Place                  int
	ID, Name, Tribe        string
	Score, Cups, BallsLeft int
	Points                 int
}

type trackPlay struct {
	mask, used int
	finished   bool
	exists     bool
}

// loadTrack finds /games/inside-track/:gameID, or the scheduled game that's open now (else the latest one).
func (s *Server) loadTrack(c *gin.Context) (*trackView, int64, error) {
	var view trackView
	var dbID int64
	var resolved bool
	query := `SELECT g.id, g.public_id::text, g.episode_number, g.opens_at, g.cutoff_at, g.resolved_at IS NOT NULL
		FROM track_games g JOIN instances i ON i.id = g.instance_id WHERE i.public_id = $1 AND `
	args := []any{s.public.InstanceID}
	if id := c.Param("gameID"); id != "" {
		if _, err := uuid.Parse(id); err != nil {
			return nil, 0, errScrambleNotFound
		}
		query += `g.public_id::text = $2`
		args = append(args, id)
	} else {
		query += `g.opens_at <= $2 ORDER BY g.opens_at DESC LIMIT 1`
		args = append(args, s.now())
	}
	err := s.pool.QueryRow(c.Request.Context(), query, args...).Scan(&dbID, &view.ID, &view.Episode, &view.OpensAt, &view.CutoffAt, &resolved)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, 0, errScrambleNotFound
	}
	if err != nil {
		return nil, 0, err
	}
	now := s.now()
	view.Open = !now.Before(view.OpensAt) && now.Before(view.CutoffAt)
	view.Upcoming = now.Before(view.OpensAt)
	view.ResultsWait = !view.Open && !view.Upcoming && !resolved
	view.OpensAt, view.CutoffAt = easternTime(view.OpensAt), easternTime(view.CutoffAt)
	return &view, dbID, nil
}

func (s *Server) loadTrackPlay(ctx context.Context, dbID int64, discordID string) (trackPlay, error) {
	var p trackPlay
	err := s.pool.QueryRow(ctx, `SELECT tp.cups_mask, tp.balls_used, tp.finished_at IS NOT NULL FROM track_plays tp
		JOIN participants p ON p.id = tp.participant_id JOIN instances i ON i.id = p.instance_id
		WHERE tp.game_id = $1 AND i.public_id = $2 AND p.discord_user_id = $3`, dbID, s.public.InstanceID, discordID).Scan(&p.mask, &p.used, &p.finished)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, nil
	}
	p.exists = err == nil
	return p, err
}

// insideTrackPage: the game while it's open and the player hasn't finished, otherwise their score or the
// results. Admins with no open game get the unscored practice board.
func (s *Server) insideTrackPage(c *gin.Context) {
	data, ok := s.siteData(c)
	if !ok {
		return
	}
	if !data.Allowed {
		renderSite(c, "home.html", data)
		return
	}
	ctx := c.Request.Context()
	view, dbID, err := s.loadTrack(c)
	if errors.Is(err, errScrambleNotFound) {
		if c.Param("gameID") == "" && data.Admin {
			s.serveTrackBoard(c, "")
			return
		}
		if c.Param("gameID") == "" {
			c.Redirect(http.StatusFound, "/games")
			return
		}
		c.String(http.StatusNotFound, "game not found")
		return
	}
	if err != nil {
		c.String(http.StatusInternalServerError, "could not load the page")
		return
	}
	play, err := s.loadTrackPlay(ctx, dbID, data.User.DiscordUserID)
	if err != nil {
		c.String(http.StatusInternalServerError, "could not load the page")
		return
	}
	if view.Open && !play.finished {
		attrs := fmt.Sprintf(` data-action="/games/inside-track/%s" data-cups="%d" data-used="%d"`, view.ID, play.mask, play.used)
		if play.exists {
			attrs += ` data-resume="1"`
		}
		s.serveTrackBoard(c, attrs)
		return
	}
	if play.exists {
		view.Played, view.Score = true, trackScore(play.mask, play.used)
		view.Cups = bits.OnesCount(uint(play.mask))
		if view.Cups == 3 {
			view.BallsLeft = trackBalls - play.used
		}
	}
	if !view.Open && !view.Upcoming && !view.ResultsWait {
		players, err := loadTrackPlayers(ctx, s.pool, dbID)
		if err != nil {
			c.String(http.StatusInternalServerError, "could not load the page")
			return
		}
		view.Results = trackResults(players)
	}
	data.Track = view
	renderSite(c, "track.html", data)
}

func trackResults(players []trackPlayer) []trackResult {
	places, points := trackPoints(players)
	out := make([]trackResult, 0, len(players))
	for _, p := range players {
		r := trackResult{Place: places[p.ID], ID: p.ID, Name: p.Name, Tribe: p.Tribe, Score: trackScore(p.CupsMask, p.BallsUsed),
			Cups: bits.OnesCount(uint(p.CupsMask)), Points: points[p.ID]}
		if r.Cups == 3 {
			r.BallsLeft = trackBalls - p.BallsUsed
		}
		out = append(out, r)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Place < out[j].Place })
	return out
}

// serveTrackBoard sends the standalone game page; attrs (data-action etc.) go on <body> for the script.
func (s *Server) serveTrackBoard(c *gin.Context, attrs string) {
	page, err := siteFiles.ReadFile("site/assets/inside-track.html")
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(strings.Replace(string(page), "<body>", "<body"+attrs+">", 1)))
}

// trackProgress saves a run after each real ball: which cups are filled and how many balls are used. The
// first call starts the run. Progress may only move forward; once finished it can't change.
func (s *Server) trackProgress(c *gin.Context) {
	data, ok := s.siteData(c)
	if !ok {
		return
	}
	view, dbID, err := s.loadTrack(c)
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
		CupsMask  int  `json:"cups_mask"`
		BallsUsed int  `json:"balls_used"`
		Finished  bool `json:"finished"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.CupsMask < 0 || req.CupsMask > 7 || req.BallsUsed < 0 || req.BallsUsed > trackBalls ||
		bits.OnesCount(uint(req.CupsMask)) > req.BallsUsed || (req.Finished && req.CupsMask != 7 && req.BallsUsed != trackBalls) {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "bad progress"})
		return
	}
	var finished any
	if req.Finished {
		finished = s.now()
	}
	tag, err := s.pool.Exec(c.Request.Context(), `INSERT INTO track_plays (game_id, participant_id, started_at, cups_mask, balls_used, finished_at)
		SELECT $1, p.id, $4, $5, $6, $7 FROM participants p JOIN instances i ON i.id = p.instance_id WHERE i.public_id = $2 AND p.discord_user_id = $3
		ON CONFLICT (game_id, participant_id) DO UPDATE SET cups_mask = EXCLUDED.cups_mask, balls_used = EXCLUDED.balls_used, finished_at = EXCLUDED.finished_at
			WHERE track_plays.finished_at IS NULL AND track_plays.balls_used <= EXCLUDED.balls_used
				AND track_plays.cups_mask & EXCLUDED.cups_mask = track_plays.cups_mask`,
		dbID, s.public.InstanceID, data.User.DiscordUserID, s.now(), req.CupsMask, req.BallsUsed, finished)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	if tag.RowsAffected() == 0 {
		c.JSON(http.StatusConflict, errorResponse{Error: "not a player, already finished, or progress went backwards"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"score": trackScore(req.CupsMask, req.BallsUsed)})
}

// addTrackGames lists an instance's Inside Track games on /games.
func (s *Server) addTrackGames(ctx context.Context, data *sitePageData, instanceID string) error {
	rows, err := s.pool.Query(ctx, `SELECT g.public_id::text, g.episode_number, g.opens_at, g.cutoff_at, g.resolved_at IS NOT NULL
		FROM track_games g JOIN instances i ON i.id = g.instance_id WHERE i.public_id::text = $1 ORDER BY g.opens_at`, instanceID)
	if err != nil {
		return err
	}
	defer rows.Close()
	now := s.now()
	for rows.Next() {
		var id string
		var episode int32
		var opens, cutoff time.Time
		var resolved bool
		if err := rows.Scan(&id, &episode, &opens, &cutoff, &resolved); err != nil {
			return err
		}
		data.addGame(&episode, gameEntry{Kind: "Inside Track", Name: "Inside Track", URL: "/games/inside-track/" + id, Badge: "Scored", Status: gameStatus(now, opens, cutoff, resolved)})
	}
	return rows.Err()
}

// trackProfileGames is a player's scored Inside Track results.
func (s *Server) trackProfileGames(ctx context.Context, participantID string) ([]profileGame, error) {
	rows, err := s.pool.Query(ctx, `SELECT g.id, g.public_id::text, g.episode_number FROM track_games g JOIN track_plays tp ON tp.game_id = g.id
		JOIN participants p ON p.id = tp.participant_id WHERE p.public_id = $1 AND g.resolved_at IS NOT NULL`, participantID)
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
		players, err := loadTrackPlayers(ctx, s.pool, r.dbID)
		if err != nil {
			return nil, err
		}
		_, points := trackPoints(players)
		for _, p := range players {
			if p.ID == participantID {
				games = append(games, profileGame{Episode: r.episode, Kind: "Inside Track", URL: "/games/inside-track/" + r.id,
					Result: strconv.Itoa(trackScore(p.CupsMask, p.BallsUsed)) + fmt.Sprintf(" in-game · %+d pts", points[p.ID])})
			}
		}
	}
	return games, nil
}
