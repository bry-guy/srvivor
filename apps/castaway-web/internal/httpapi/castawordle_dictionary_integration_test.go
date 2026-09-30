package httpapi_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/castawordle"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/httpapi"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

const legacyPreviewDictionaryVersion = "cmudict-74790861f652b15e4ac49015a90074ad62a27690"

func TestCastawordleDictionaryUpgradeReplayAndGuess(t *testing.T) {
	if castawordle.ValidWord("JEFF") {
		t.Fatal("JEFF must be absent from the embedded SCOWL dictionary for this regression test")
	}
	ctx, pool := integrationPool(t)
	resetDatabase(t, ctx, pool)
	q := db.New(pool)
	instance := createInstanceForTest(t, ctx, q, "Dictionary upgrade", 52)
	player := createParticipantForTest(t, ctx, q, instance.ID, "cw-player")
	if _, err := q.SetParticipantDiscordUserID(ctx, db.SetParticipantDiscordUserIDParams{
		ID: player.ID, DiscordUserID: pgtype.Text{String: "cw-player", Valid: true},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := q.CreateInstanceAdmin(ctx, db.CreateInstanceAdminParams{InstanceID: instance.ID, DiscordUserID: "cw-player"}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 30, 17, 0, 0, 0, time.UTC)
	token := uuid.NewString()
	hash := sha256.Sum256([]byte(token))
	if _, err := pool.Exec(ctx, `INSERT INTO web_sessions (token_hash, discord_user_id, discord_username, kind, expires_at) VALUES ($1,$2,$2,'browser',$3)`, hash[:], "cw-player", now.AddDate(0, 0, 30)); err != nil {
		t.Fatal(err)
	}
	const base = "https://castaway.example"
	router := httpapi.New(pool, httpapi.WithClock(func() time.Time { return now }), httpapi.WithPublic(httpapi.PublicConfig{
		BaseURL: base, InstanceID: uuid.UUID(instance.ID.Bytes).String(),
	})).PublicRouter()
	postGuess := func(gameID, word string, position int) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/api/castawordle/"+gameID+"/play/guesses", strings.NewReader(fmt.Sprintf(`{"guess":%q,"position":%d}`, word, position)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", base)
		req.Header.Set("X-Discord-User-ID", "cw-other")
		req.AddCookie(&http.Cookie{Name: "castaway_session", Value: token})
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	createGame := func(name, answer, version string) db.CreateCastawordleGameRow {
		t.Helper()
		game, err := q.CreateCastawordleGame(ctx, db.CreateCastawordleGameParams{
			InstanceID: instance.ID, Name: name, Answer: answer, DictionaryVersion: version,
			OpensAt: pgtype.Timestamptz{Time: now, Valid: true}, CutoffAt: pgtype.Timestamptz{Time: now.Add(time.Hour), Valid: true},
		})
		if err != nil {
			t.Fatal(err)
		}
		return game
	}

	historical := createGame("Historical", "MATH", legacyPreviewDictionaryVersion)
	if _, err := pool.Exec(ctx, `INSERT INTO castawordle_plays (game_id, participant_id, guesses, status, updated_at)
		SELECT g.id, p.id, $3::jsonb, 'in_progress', $4 FROM castawordle_games g
		JOIN participants p ON p.instance_id = g.instance_id
		WHERE g.public_id = $1 AND p.public_id = $2`, historical.ID, player.ID, `["TEAM","JEFF"]`, now); err != nil {
		t.Fatal(err)
	}
	result, err := pool.Exec(ctx, `UPDATE castawordle_games SET dictionary_version = $2 WHERE public_id = $1`, historical.ID, castawordle.DictionaryVersion)
	if err != nil {
		t.Fatal(err)
	}
	if result.RowsAffected() != 1 {
		t.Fatalf("dictionary metadata upgrade affected %d games, want 1", result.RowsAffected())
	}
	gameID := uuid.UUID(historical.ID.Bytes).String()
	replay := postGuess(gameID, "JEFF", 2)
	if replay.Code != http.StatusOK {
		t.Fatalf("saved guess replay returned HTTP %d: %s", replay.Code, replay.Body)
	}
	var replayed struct {
		Guesses []struct {
			Word string `json:"word"`
		} `json:"guesses"`
	}
	if err := json.Unmarshal(replay.Body.Bytes(), &replayed); err != nil {
		t.Fatal(err)
	}
	if len(replayed.Guesses) != 2 || replayed.Guesses[0].Word != "TEAM" || replayed.Guesses[1].Word != "JEFF" {
		t.Fatalf("replay did not preserve saved position: %#v", replayed.Guesses)
	}
	newGuess := postGuess(gameID, "JEFF", 3)
	if newGuess.Code != http.StatusBadRequest || !strings.Contains(newGuess.Body.String(), "invalid word, try again") {
		t.Fatalf("new historical word returned HTTP %d: %s", newGuess.Code, newGuess.Body)
	}
	var saved []string
	var raw []byte
	if err := pool.QueryRow(ctx, `SELECT p.guesses FROM castawordle_plays p JOIN castawordle_games g ON g.id = p.game_id
		JOIN participants player ON player.id = p.participant_id WHERE g.public_id = $1 AND player.public_id = $2`, historical.ID, player.ID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &saved); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(saved, []string{"TEAM", "JEFF"}) {
		t.Fatalf("new invalid guess changed persisted progress: %v", saved)
	}

	sevenLetter := createGame("Seven letters", "OUTCAST", castawordle.DictionaryVersion)
	accepted := postGuess(uuid.UUID(sevenLetter.ID.Bytes).String(), "SWADDLE", 1)
	if accepted.Code != http.StatusOK {
		t.Fatalf("SWADDLE returned HTTP %d: %s", accepted.Code, accepted.Body)
	}
	var sevenLetterPlay struct {
		Guesses []struct {
			Word string `json:"word"`
		} `json:"guesses"`
	}
	if err := json.Unmarshal(accepted.Body.Bytes(), &sevenLetterPlay); err != nil {
		t.Fatal(err)
	}
	if len(sevenLetterPlay.Guesses) != 1 || sevenLetterPlay.Guesses[0].Word != "SWADDLE" {
		t.Fatalf("SWADDLE was not saved as the seven-letter guess: %#v", sevenLetterPlay.Guesses)
	}
}
