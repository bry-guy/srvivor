package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// A profile is one player's seasons in this league, joined by Discord account. Each page shows one season
// (score + draft) and lists the player's other league seasons.
type profileSeason struct {
	ParticipantID, InstanceID, InstanceName  string
	Season                                   int32
	Current, Selected                        bool
	Scored                                   bool // has a draft on the leaderboard; otherwise "Unavailable"
	Rank, Players, Total, DraftPoints, Bonus int
	Tribe                                    string
}

type profilePick struct {
	Position int32
	Name     string
}

type profileView struct {
	PlayerName, LeagueName string
	Self                   bool
	Season                 profileSeason
	Draft                  []profilePick
	DraftHidden            string
	Seasons                []profileSeason
}

type leaderboardRow struct {
	ParticipantID string `json:"participant_id"`
	Name          string `json:"participant_name"`
	Tribe         string `json:"current_tribe_name"`
	Draft         int    `json:"draft_points"`
	Bonus         int    `json:"bonus_points"`
	Total         int    `json:"total_points"`
	HasDraft      *bool  `json:"has_draft"`
	Rank          int    `json:"-"`
}

// leaderboardRows runs the leaderboard handler in-process (visible bonus only; secret points never count)
// and returns players with drafts, ranked by total with ties sharing a rank.
// ponytail: one leaderboard computation per season shown; cache or extract if profiles get slow.
func (s *Server) leaderboardRows(ctx context.Context, instanceID string) ([]leaderboardRow, error) {
	rec := httptest.NewRecorder()
	gc, _ := gin.CreateTestContext(rec)
	gc.Request = httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	gc.Params = gin.Params{{Key: "instanceID", Value: instanceID}}
	s.leaderboard(gc)
	if rec.Code != http.StatusOK {
		return nil, fmt.Errorf("leaderboard HTTP %d", rec.Code)
	}
	var res struct {
		Leaderboard []leaderboardRow `json:"leaderboard"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		return nil, err
	}
	rows := res.Leaderboard[:0]
	for _, r := range res.Leaderboard {
		if r.HasDraft == nil || *r.HasDraft {
			rows = append(rows, r)
		}
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Total > rows[j].Total })
	for i := range rows {
		rows[i].Rank = i + 1
		if i > 0 && rows[i].Total == rows[i-1].Total {
			rows[i].Rank = rows[i-1].Rank
		}
	}
	return rows, nil
}

func (s *Server) leagueInstanceIDs() []string {
	return append([]string{s.public.InstanceID}, s.public.LeagueInstanceIDs...)
}

func (s *Server) mePage(c *gin.Context) {
	data, ok := s.siteData(c)
	if !ok {
		return
	}
	var participantID string
	err := s.pool.QueryRow(c.Request.Context(), `SELECT p.public_id::text FROM participants p JOIN instances i ON i.id = p.instance_id
		WHERE i.public_id = $1 AND p.discord_user_id = $2`, s.public.InstanceID, data.User.DiscordUserID).Scan(&participantID)
	if !data.Allowed || errors.Is(err, pgx.ErrNoRows) {
		c.Redirect(http.StatusFound, "/") // not a player this season: Scores (or the access request) instead
		return
	}
	if err != nil {
		c.String(http.StatusInternalServerError, "could not load the page")
		return
	}
	s.renderProfile(c, data, participantID)
}

func (s *Server) playerPage(c *gin.Context) {
	data, ok := s.siteData(c)
	if !ok {
		return
	}
	if !data.Allowed {
		c.Redirect(http.StatusFound, "/")
		return
	}
	id, err := uuid.Parse(c.Param("participantID"))
	if err != nil {
		c.String(http.StatusNotFound, "player not found")
		return
	}
	s.renderProfile(c, data, id.String())
}

func (s *Server) renderProfile(c *gin.Context, data sitePageData, participantID string) {
	ctx := c.Request.Context()
	league := s.leagueInstanceIDs()
	var targetInstance, discordID string
	var name string
	// Only league seasons, and only players of the current season (plus anyone in it), are viewable.
	err := s.pool.QueryRow(ctx, `SELECT p.name, i.public_id::text, COALESCE(p.discord_user_id, '') FROM participants p JOIN instances i ON i.id = p.instance_id
		WHERE p.public_id = $1 AND i.public_id::text = ANY($2)
		  AND (i.public_id = $3 OR EXISTS (SELECT 1 FROM participants cur JOIN instances ci ON ci.id = cur.instance_id
		       WHERE ci.public_id = $3 AND cur.discord_user_id = p.discord_user_id))`,
		participantID, league, s.public.InstanceID).Scan(&name, &targetInstance, &discordID)
	if errors.Is(err, pgx.ErrNoRows) {
		c.String(http.StatusNotFound, "player not found")
		return
	}
	if err != nil {
		c.String(http.StatusInternalServerError, "could not load the page")
		return
	}
	view := profileView{PlayerName: name, LeagueName: s.public.LeagueName, Self: discordID != "" && discordID == data.User.DiscordUserID}
	rows, err := s.pool.Query(ctx, `SELECT p.public_id::text, i.public_id::text, i.name, i.season FROM participants p JOIN instances i ON i.id = p.instance_id
		WHERE i.public_id::text = ANY($1) AND (p.public_id = $2 OR ($3 <> '' AND p.discord_user_id = $3))
		ORDER BY i.season DESC, i.created_at DESC`, league, participantID, discordID)
	if err == nil {
		view.Seasons, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (profileSeason, error) {
			var season profileSeason
			err := row.Scan(&season.ParticipantID, &season.InstanceID, &season.InstanceName, &season.Season)
			return season, err
		})
	}
	for i := range view.Seasons {
		if err != nil {
			break
		}
		season := &view.Seasons[i]
		season.Current = season.InstanceID == s.public.InstanceID
		season.Selected = season.ParticipantID == participantID
		var board []leaderboardRow
		board, err = s.leaderboardRows(ctx, season.InstanceID)
		season.Players = len(board)
		for _, r := range board {
			if r.ParticipantID == season.ParticipantID {
				season.Scored, season.Rank, season.Total, season.DraftPoints, season.Bonus, season.Tribe = true, r.Rank, r.Total, r.Draft, r.Bonus, r.Tribe
			}
		}
		if season.Selected {
			view.Season = *season
		}
	}
	if err == nil {
		view.Draft, view.DraftHidden, err = s.profileDraft(ctx, data, view, targetInstance, participantID)
	}
	if err != nil {
		c.String(http.StatusInternalServerError, "could not load the page")
		return
	}
	data.Profile = &view
	renderSite(c, "profile.html", data)
}

// profileDraft returns the selected season's draft, or why it's hidden. Drafts are soft-closed (late drafts
// are allowed), so another player's current-season draft is shown only once submissions have closed and the
// viewer has saved their own. Past seasons are open; admins and the owner always see it.
func (s *Server) profileDraft(ctx context.Context, data sitePageData, view profileView, instanceID, participantID string) ([]profilePick, string, error) {
	if instanceID == s.public.InstanceID && !view.Self && !data.Admin {
		activity, _, err := draftSubmissionActivity(ctx, s.queries, toPGUUID(uuid.MustParse(instanceID)))
		if err != nil {
			return nil, "", err
		}
		if activity != nil && activity.Status == "active" {
			return nil, "Drafts are revealed after drafts close.", nil
		}
		var drafted bool
		if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM draft_picks dp JOIN participants p ON p.id = dp.participant_id
			JOIN instances i ON i.id = p.instance_id WHERE i.public_id = $1 AND p.discord_user_id = $2)`, instanceID, data.User.DiscordUserID).Scan(&drafted); err != nil {
			return nil, "", err
		}
		if !drafted {
			return nil, "Submit your own draft to see everyone else's.", nil
		}
	}
	picks, err := s.queries.ListDraftPicksForParticipant(ctx, toPGUUID(uuid.MustParse(participantID)))
	if err != nil {
		return nil, "", err
	}
	contestants, err := s.queries.ListContestantsByInstance(ctx, toPGUUID(uuid.MustParse(instanceID)))
	if err != nil {
		return nil, "", err
	}
	names := make(map[[16]byte]string, len(contestants))
	for _, contestant := range contestants {
		names[contestant.ID.Bytes] = contestant.Name
	}
	draft := make([]profilePick, 0, len(picks))
	for _, pick := range picks {
		draft = append(draft, profilePick{Position: pick.Position, Name: names[pick.ContestantID.Bytes]})
	}
	if len(draft) == 0 {
		return nil, "No draft on record.", nil
	}
	return draft, "", nil
}
