package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// buttonAwards scores Press the Button. Players with zero presses have no result (they aren't in presses).
// Players who share a press count earn the group's size, at most 3. Everyone else is ranked by distinct
// press count: the most earns +2, the second most -1 and the least +1. A count gets only the first that
// applies, so with two distinct counts the lower one is "second most".
func buttonAwards(presses map[string]int64) map[string]int {
	byCount := map[int64][]string{}
	for player, n := range presses {
		byCount[n] = append(byCount[n], player)
	}
	counts := make([]int64, 0, len(byCount))
	for n := range byCount {
		counts = append(counts, n)
	}
	sort.Slice(counts, func(i, j int) bool { return counts[i] > counts[j] })
	awards := map[string]int{}
	for i, n := range counts {
		group := byCount[n]
		points := 0
		switch {
		case len(group) > 1:
			points = min(len(group), 3)
		case i == 0:
			points = 2
		case i == 1:
			points = -1
		case i == len(counts)-1:
			points = 1
		}
		for _, player := range group {
			if points != 0 {
				awards[player] = points
			}
		}
	}
	return awards
}

type buttonGame struct {
	ID       uuid.UUID `json:"id"`
	Episode  *int32    `json:"episode_number"` // nil for an admin-only, unscored test game
	OpensAt  time.Time `json:"opens_at"`
	CutoffAt time.Time `json:"cutoff_at"`
	Resolved bool      `json:"resolved"`
}

// createButtonGame creates (or, before it resolves, reschedules) an instance's game for an episode. Without
// an episode it creates a new admin-only test game, which never scores.
func (s *Server) createButtonGame(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok || !s.requireInstanceAdminRequest(c, instanceID) {
		return
	}
	var req struct {
		Episode  *int32    `json:"episode_number"`
		OpensAt  time.Time `json:"opens_at" binding:"required"`
		CutoffAt time.Time `json:"cutoff_at" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || !req.CutoffAt.After(req.OpensAt) {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "opens_at and a later cutoff_at are required"})
		return
	}
	game := buttonGame{Episode: req.Episode}
	conflict := ""
	if req.Episode != nil {
		conflict = `ON CONFLICT (instance_id, episode_number) DO UPDATE SET opens_at = EXCLUDED.opens_at, cutoff_at = EXCLUDED.cutoff_at
			WHERE button_games.resolved_at IS NULL`
	}
	err := s.pool.QueryRow(c.Request.Context(), `INSERT INTO button_games (instance_id, episode_number, opens_at, cutoff_at)
		SELECT id, $2, $3, $4 FROM instances WHERE public_id = $1 `+conflict+`
		RETURNING public_id, opens_at, cutoff_at, resolved_at IS NOT NULL`, instanceID, req.Episode, req.OpensAt, req.CutoffAt).
		Scan(&game.ID, &game.OpensAt, &game.CutoffAt, &game.Resolved)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusConflict, errorResponse{Error: "instance not found, or this episode's game already resolved"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "could not save game"})
		return
	}
	c.JSON(http.StatusOK, game)
}

func (s *Server) resolveButtonGameRequest(c *gin.Context) {
	gameID, ok := parseUUIDPath(c, "gameID")
	if !ok {
		return
	}
	var instanceID uuid.UUID
	err := s.pool.QueryRow(c.Request.Context(), `SELECT i.public_id FROM button_games g JOIN instances i ON i.id = g.instance_id WHERE g.public_id = $1`, gameID).Scan(&instanceID)
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
	awards, err := s.resolveButtonGame(c.Request.Context(), gameID)
	if err != nil {
		c.JSON(http.StatusConflict, errorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"awards": awards})
}

var errButtonOpen = errors.New("game cannot resolve before its cutoff")

// resolveButtonGame writes each player's award to the bonus ledger once; later calls return nil awards.
func (s *Server) resolveButtonGame(ctx context.Context, gameID uuid.UUID) (map[string]int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			requestLogger.Error("rollback button game", "error", err)
		}
	}()
	var game struct {
		id            int64
		instance      uuid.UUID
		episode       *int32
		opens, cutoff time.Time
		resolved      bool
	}
	if err := tx.QueryRow(ctx, `SELECT g.id, i.public_id, g.episode_number, g.opens_at, g.cutoff_at, g.resolved_at IS NOT NULL
		FROM button_games g JOIN instances i ON i.id = g.instance_id WHERE g.public_id = $1 FOR UPDATE OF g`, gameID).
		Scan(&game.id, &game.instance, &game.episode, &game.opens, &game.cutoff, &game.resolved); err != nil {
		return nil, err
	}
	if game.resolved {
		return nil, nil
	}
	if s.now().Before(game.cutoff) {
		return nil, errButtonOpen
	}
	rows, err := tx.Query(ctx, `SELECT p.public_id::text, b.presses FROM button_presses b JOIN participants p ON p.id = b.participant_id WHERE b.game_id = $1`, game.id)
	if err != nil {
		return nil, err
	}
	presses := map[string]int64{}
	for rows.Next() {
		var player string
		var n int64
		if err := rows.Scan(&player, &n); err != nil {
			rows.Close()
			return nil, err
		}
		presses[player] = n
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	awards := buttonAwards(presses)
	if len(awards) > 0 && game.episode != nil { // test games report what they would award, but never score
		q := s.queries.WithTx(tx)
		at := pgtype.Timestamptz{Time: game.cutoff, Valid: true}
		name := fmt.Sprintf("Press the Button: Episode %d", *game.episode)
		activity, err := q.CreateInstanceActivity(ctx, db.CreateInstanceActivityParams{
			ActivityType: "press_the_button", Name: name, Status: "completed", Metadata: []byte("{}"),
			StartsAt: pgtype.Timestamptz{Time: game.opens, Valid: true}, EndsAt: at, InstanceID: toPGUUID(game.instance),
		})
		if err != nil {
			return nil, err
		}
		occurrence, err := q.CreateActivityOccurrence(ctx, db.CreateActivityOccurrenceParams{
			OccurrenceType: "press_the_button", Name: name, EffectiveAt: at, StartsAt: activity.StartsAt, EndsAt: at,
			Status: "resolved", Metadata: []byte("{}"), ActivityID: activity.ID,
		})
		if err != nil {
			return nil, err
		}
		for player, points := range awards {
			metadata, err := json.Marshal(map[string]int64{"presses": presses[player]})
			if err != nil {
				return nil, err
			}
			if _, err := q.CreateBonusPointLedgerEntry(ctx, db.CreateBonusPointLedgerEntryParams{
				EntryKind: "award", Points: int32(points), Visibility: "public", Reason: name, EffectiveAt: at, // #nosec G115 -- points are in [-1, 3]
				AwardKey: pgtype.Text{String: "press_the_button", Valid: true}, Metadata: metadata,
				InstanceID: toPGUUID(game.instance), ParticipantID: toPGUUID(uuid.MustParse(player)), ActivityOccurrenceID: occurrence.ID,
			}); err != nil {
				return nil, err
			}
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE button_games SET resolved_at = $2 WHERE id = $1`, game.id, s.now()); err != nil {
		return nil, err
	}
	return awards, tx.Commit(ctx)
}

// resolveDueButtonGames scores every game past its cutoff; RunButtonResolver calls it each minute.
func (s *Server) resolveDueButtonGames(ctx context.Context) error {
	rows, err := s.pool.Query(ctx, `SELECT public_id FROM button_games WHERE resolved_at IS NULL AND cutoff_at <= $1 AND episode_number IS NOT NULL`, s.now())
	if err != nil {
		return err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := s.resolveButtonGame(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// RunButtonResolver resolves Press the Button games at their cutoff until ctx ends.
func (s *Server) RunButtonResolver(ctx context.Context) {
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	for {
		if err := s.resolveDueButtonGames(ctx); err != nil && ctx.Err() == nil {
			requestLogger.Error("resolve button games", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

// openButtonGame is the current season's game that's open now, if any.
func (s *Server) openButtonGame(ctx context.Context) (int64, bool, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `SELECT g.id FROM button_games g JOIN instances i ON i.id = g.instance_id
		WHERE i.public_id = $1 AND g.episode_number IS NOT NULL AND g.opens_at <= $2 AND g.cutoff_at > $2 ORDER BY g.opens_at DESC LIMIT 1`, s.public.InstanceID, s.now()).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	return id, err == nil, err
}

var errButtonNotFound = errors.New("game not found")

// buttonGameFor is the game a request plays: the scheduled game for /button, or for /button/:gameID an
// admin-only test game (anyone else gets not found).
func (s *Server) buttonGameFor(c *gin.Context, data sitePageData) (int64, bool, error) {
	testID := c.Param("gameID")
	if testID == "" {
		return s.openButtonGame(c.Request.Context())
	}
	if !data.Admin {
		return 0, false, errButtonNotFound
	}
	var id int64
	var open bool
	err := s.pool.QueryRow(c.Request.Context(), `SELECT g.id, g.opens_at <= $3 AND g.cutoff_at > $3 FROM button_games g JOIN instances i ON i.id = g.instance_id
		WHERE g.public_id::text = $1 AND i.public_id = $2 AND g.episode_number IS NULL`, testID, s.public.InstanceID, s.now()).Scan(&id, &open)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, errButtonNotFound
	}
	return id, open, err
}

func (s *Server) buttonPage(c *gin.Context) {
	data, ok := s.siteData(c)
	if !ok {
		return
	}
	data.ButtonAction = c.Request.URL.Path
	if data.Allowed {
		_, open, err := s.buttonGameFor(c, data)
		if errors.Is(err, errButtonNotFound) {
			c.String(http.StatusNotFound, "game not found")
			return
		}
		if err != nil {
			c.String(http.StatusInternalServerError, "could not load the page")
			return
		}
		data.ButtonOpen = open
	}
	renderSite(c, "button.html", data)
}

// pressButton counts one press for the signed-in player. It answers 204 to the page's script and redirects
// plain form posts back to the button.
func (s *Server) pressButton(c *gin.Context) {
	data, ok := s.siteData(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	gameID, open, err := s.buttonGameFor(c, data)
	if errors.Is(err, errButtonNotFound) {
		c.Status(http.StatusNotFound)
		return
	}
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	if !data.Allowed || !open {
		c.Status(http.StatusConflict)
		return
	}
	tag, err := s.pool.Exec(ctx, `INSERT INTO button_presses (game_id, participant_id, presses)
		SELECT $1, p.id, 1 FROM participants p JOIN instances i ON i.id = p.instance_id
		WHERE i.public_id = $2 AND p.discord_user_id = $3
		ON CONFLICT (game_id, participant_id) DO UPDATE SET presses = button_presses.presses + 1`,
		gameID, s.public.InstanceID, data.User.DiscordUserID)
	if err != nil || tag.RowsAffected() != 1 {
		c.Status(http.StatusConflict)
		return
	}
	if c.GetHeader("X-Press") != "" {
		c.Status(http.StatusNoContent)
		return
	}
	c.Redirect(http.StatusSeeOther, c.Request.URL.Path)
}

// createButtonTest starts a one-hour admin-only test game and opens it.
func (s *Server) createButtonTest(c *gin.Context) {
	data, ok := s.siteData(c)
	if !ok {
		return
	}
	if !data.Admin {
		c.Status(http.StatusNotFound)
		return
	}
	var id uuid.UUID
	now := s.now()
	if err := s.pool.QueryRow(c.Request.Context(), `INSERT INTO button_games (instance_id, opens_at, cutoff_at)
		SELECT id, $2, $3 FROM instances WHERE public_id = $1 RETURNING public_id`, s.public.InstanceID, now, now.Add(time.Hour)).Scan(&id); err != nil {
		c.String(http.StatusInternalServerError, "could not create a test game")
		return
	}
	c.Redirect(http.StatusSeeOther, "/button/"+id.String())
}
