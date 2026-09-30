package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/castawordle"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type castawordleGameView struct {
	ID         string    `json:"id"`
	InstanceID string    `json:"instance_id"`
	Name       string    `json:"name"`
	WordLength int       `json:"word_length"`
	GuessLimit int       `json:"guess_limit"`
	OpensAt    time.Time `json:"opens_at"`
	CutoffAt   time.Time `json:"cutoff_at"`
	Unscored   bool      `json:"unscored"`
}

type castawordleGuessView struct {
	Word     string   `json:"word"`
	Feedback []string `json:"feedback"`
}

type castawordlePlayView struct {
	Game    castawordleGameView    `json:"game"`
	Guesses []castawordleGuessView `json:"guesses"`
	Status  string                 `json:"status"`
	Answer  string                 `json:"answer,omitempty"`
}

func castawordleGame(row db.GetCastawordleGameRow) castawordleGameView {
	return castawordleGameView{
		ID: pgUUIDString(row.ID), InstanceID: pgUUIDString(row.InstanceID), Name: row.Name,
		WordLength: len(row.Answer), GuessLimit: castawordle.GuessLimit,
		OpensAt: row.OpensAt.Time, CutoffAt: row.CutoffAt.Time, Unscored: true,
	}
}

func (s *Server) castawordlePlay(row db.GetCastawordleGameRow, guesses []string, status string) castawordlePlayView {
	view := castawordlePlayView{Game: castawordleGame(row), Guesses: make([]castawordleGuessView, 0, len(guesses)), Status: status}
	for _, guess := range guesses {
		view.Guesses = append(view.Guesses, castawordleGuessView{Word: guess, Feedback: castawordle.Feedback(row.Answer, guess)})
	}
	switch {
	case status == "solved" || status == "exhausted":
		view.Answer = row.Answer
	case !s.now().Before(row.CutoffAt.Time):
		view.Status = "expired"
	case s.now().Before(row.OpensAt.Time):
		view.Status = "not_open"
	}
	return view
}

func castawordleError(c *gin.Context, err error) {
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, errorResponse{Error: "game not found"})
		return
	}
	requestLogger.Error("castawordle request", "error", err)
	c.JSON(http.StatusInternalServerError, errorResponse{Error: "could not load or save the game"})
}

func (s *Server) createCastawordle(c *gin.Context) {
	instanceID, ok := parseUUIDPath(c, "instanceID")
	if !ok || !s.requireInstanceAdminRequest(c, instanceID) {
		return
	}
	var req struct {
		Name     string    `json:"name"`
		Answer   string    `json:"answer"`
		OpensAt  time.Time `json:"opens_at"`
		CutoffAt time.Time `json:"cutoff_at"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2048)
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid game configuration"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Answer = castawordle.Normalize(req.Answer)
	if req.Name == "" || len(req.Name) > 80 || !castawordle.ValidWord(req.Answer) {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "use a name up to 80 characters and a dictionary answer of 4–8 letters"})
		return
	}
	if req.OpensAt.IsZero() {
		req.OpensAt = s.now()
	}
	if req.CutoffAt.IsZero() {
		req.CutoffAt = req.OpensAt.AddDate(0, 0, 7)
	}
	if !req.CutoffAt.After(req.OpensAt) {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "cutoff must be after opening"})
		return
	}
	row, err := s.queries.CreateCastawordleGame(c.Request.Context(), db.CreateCastawordleGameParams{
		InstanceID: toPGUUID(instanceID), Name: req.Name, Answer: req.Answer, DictionaryVersion: castawordle.DictionaryVersion,
		OpensAt: pgtype.Timestamptz{Time: req.OpensAt, Valid: true}, CutoffAt: pgtype.Timestamptz{Time: req.CutoffAt, Valid: true},
	})
	if err != nil {
		castawordleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, castawordleGame(db.GetCastawordleGameRow(row)))
}

func (s *Server) castawordlePlayer(c *gin.Context) (db.GetCastawordleGameRow, pgtype.UUID, bool) {
	id, ok := parseUUIDPath(c, "gameID")
	if !ok {
		return db.GetCastawordleGameRow{}, pgtype.UUID{}, false
	}
	game, err := s.queries.GetCastawordleGame(c.Request.Context(), toPGUUID(id))
	if err != nil {
		castawordleError(c, err)
		return game, pgtype.UUID{}, false
	}
	if strings.HasPrefix(c.FullPath(), "/api/") && s.public.InstanceID != pgUUIDString(game.InstanceID) {
		c.JSON(http.StatusForbidden, errorResponse{Error: "this game belongs to a different season"})
		return game, pgtype.UUID{}, false
	}
	player, err := s.queries.GetParticipantByDiscordUserID(c.Request.Context(), db.GetParticipantByDiscordUserIDParams{
		InstanceID:    game.InstanceID,
		DiscordUserID: pgtype.Text{String: discordUserIDFromRequest(c.Request), Valid: true},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusForbidden, errorResponse{Error: "link your Discord account to a player in this season to play"})
		return game, pgtype.UUID{}, false
	}
	if err != nil {
		castawordleError(c, err)
		return game, pgtype.UUID{}, false
	}
	return game, player.ID, true
}

func (s *Server) getCastawordlePlay(c *gin.Context) {
	game, participantID, ok := s.castawordlePlayer(c)
	if !ok {
		return
	}
	play, err := s.queries.GetCastawordlePlay(c.Request.Context(), db.GetCastawordlePlayParams{GameID: game.ID, ParticipantID: participantID})
	if errors.Is(err, pgx.ErrNoRows) {
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusOK, s.castawordlePlay(game, nil, "not_started"))
		return
	}
	if err != nil {
		castawordleError(c, err)
		return
	}
	var guesses []string
	if err := json.Unmarshal(play.Guesses, &guesses); err != nil {
		castawordleError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, s.castawordlePlay(game, guesses, play.Status))
}

func (s *Server) guessCastawordle(c *gin.Context) {
	game, participantID, ok := s.castawordlePlayer(c)
	if !ok {
		return
	}
	var req struct {
		Guess    string `json:"guess"`
		Position int    `json:"position"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024)
	if err := c.ShouldBindJSON(&req); err != nil || req.Position < 1 || req.Position > castawordle.GuessLimit {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid guess request"})
		return
	}
	req.Guess = castawordle.Normalize(req.Guess)
	if len(req.Guess) != len(game.Answer) || !castawordle.ValidWord(req.Guess) {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "enter a dictionary word of the correct length"})
		return
	}
	tx, err := s.pool.Begin(c.Request.Context())
	if err != nil {
		castawordleError(c, err)
		return
	}
	defer rollbackTx(c, tx)
	qtx := s.queries.WithTx(tx)
	if err := qtx.EnsureCastawordlePlay(c.Request.Context(), db.EnsureCastawordlePlayParams{GameID: game.ID, ParticipantID: participantID}); err != nil {
		castawordleError(c, err)
		return
	}
	play, err := qtx.LockCastawordlePlay(c.Request.Context(), db.LockCastawordlePlayParams{GameID: game.ID, ParticipantID: participantID})
	if err != nil {
		castawordleError(c, err)
		return
	}
	var guesses []string
	if err := json.Unmarshal(play.Guesses, &guesses); err != nil {
		castawordleError(c, err)
		return
	}
	if req.Position <= len(guesses) && guesses[req.Position-1] == req.Guess {
		c.JSON(http.StatusOK, s.castawordlePlay(game, guesses, play.Status))
		return
	}
	if req.Position != len(guesses)+1 || play.Status != "in_progress" {
		c.JSON(http.StatusConflict, errorResponse{Error: "your game changed; reload your saved progress"})
		return
	}
	now := s.now()
	if now.Before(game.OpensAt.Time) || !now.Before(game.CutoffAt.Time) {
		c.JSON(http.StatusConflict, errorResponse{Error: "this game is not open"})
		return
	}
	guesses = append(guesses, req.Guess)
	status := "in_progress"
	if req.Guess == game.Answer {
		status = "solved"
	} else if len(guesses) == castawordle.GuessLimit {
		status = "exhausted"
	}
	encoded, err := json.Marshal(guesses)
	if err != nil {
		castawordleError(c, err)
		return
	}
	if err := qtx.UpdateCastawordlePlay(c.Request.Context(), db.UpdateCastawordlePlayParams{
		GameID: game.ID, ParticipantID: participantID, Guesses: encoded, Status: status,
		UpdatedAt: pgtype.Timestamptz{Time: now, Valid: true},
	}); err != nil {
		castawordleError(c, err)
		return
	}
	if err := tx.Commit(c.Request.Context()); err != nil {
		castawordleError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, s.castawordlePlay(game, guesses, status))
}

func (s *Server) castawordleListPage(c *gin.Context) {
	data, ok := s.siteData(c)
	if !ok {
		return
	}
	if !data.Allowed {
		renderSite(c, "home.html", data)
		return
	}
	id, err := uuid.Parse(s.public.InstanceID)
	if err != nil {
		castawordleError(c, err)
		return
	}
	rows, err := s.queries.ListCastawordleGames(c.Request.Context(), toPGUUID(id))
	if err != nil {
		castawordleError(c, err)
		return
	}
	for _, row := range rows {
		data.Games = append(data.Games, castawordleGameView{
			ID: pgUUIDString(row.ID), InstanceID: pgUUIDString(row.InstanceID), Name: row.Name,
			WordLength: int(row.WordLength), GuessLimit: castawordle.GuessLimit,
			OpensAt: row.OpensAt.Time, CutoffAt: row.CutoffAt.Time, Unscored: true,
		})
	}
	renderSite(c, "games.html", data)
}

func (s *Server) castawordlePage(c *gin.Context) {
	data, ok := s.siteData(c)
	if !ok {
		return
	}
	if !data.Allowed {
		renderSite(c, "home.html", data)
		return
	}
	id, ok := parseUUIDPath(c, "gameID")
	if !ok {
		return
	}
	row, err := s.queries.GetCastawordleGame(c.Request.Context(), toPGUUID(id))
	if err != nil {
		castawordleError(c, err)
		return
	}
	if pgUUIDString(row.InstanceID) != s.public.InstanceID {
		c.String(http.StatusNotFound, "game not found")
		return
	}
	game := castawordleGame(row)
	data.Game = &game
	renderSite(c, "game.html", data)
}
