// Package matchup owns the league schedule, scoreboard, standings, and
// playoff bracket.
package matchup

import (
	"net/http"

	"github.com/bulbousoars/lunarleague/apps/api/internal/db"
	"github.com/bulbousoars/lunarleague/apps/api/internal/httpx"
	"github.com/go-chi/chi/v5"
)

type Service struct {
	pool *db.DB
}

func NewService(pool *db.DB) *Service { return &Service{pool: pool} }

func (s *Service) Mount(r chi.Router) {
	r.Get("/leagues/{leagueID}/schedule", s.schedule)
	r.Get("/leagues/{leagueID}/scoreboard", s.scoreboard)
	r.Get("/leagues/{leagueID}/standings", s.standings)
	r.Post("/leagues/{leagueID}/schedule/generate", s.generate)
}

type matchup struct {
	ID              string  `json:"id"`
	Week            int     `json:"week"`
	Season          int     `json:"season"`
	HomeTeamID      string  `json:"home_team_id"`
	AwayTeamID      string  `json:"away_team_id"`
	HomeScore       string  `json:"home_score"`
	AwayScore       string  `json:"away_score"`
	HomeProjected   *string `json:"home_projected,omitempty"`
	AwayProjected   *string `json:"away_projected,omitempty"`
	IsPlayoff       bool    `json:"is_playoff"`
	IsConsolation   bool    `json:"is_consolation"`
	IsFinal         bool    `json:"is_final"`
}

func (s *Service) schedule(w http.ResponseWriter, r *http.Request) {
	leagueID := chi.URLParam(r, "leagueID")
	rows, err := s.pool.Query(r.Context(), `
		SELECT id, week, season, home_team_id, away_team_id,
		       home_score::text, away_score::text,
		       home_projected::text, away_projected::text,
		       is_playoff, is_consolation, is_final
		FROM matchups WHERE league_id = $1
		ORDER BY week, home_team_id`, leagueID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()
	out := []matchup{}
	for rows.Next() {
		var m matchup
		if err := rows.Scan(&m.ID, &m.Week, &m.Season, &m.HomeTeamID, &m.AwayTeamID,
			&m.HomeScore, &m.AwayScore, &m.HomeProjected, &m.AwayProjected,
			&m.IsPlayoff, &m.IsConsolation, &m.IsFinal); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, err)
			return
		}
		out = append(out, m)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"matchups": out})
}

func (s *Service) scoreboard(w http.ResponseWriter, r *http.Request) {
	leagueID := chi.URLParam(r, "leagueID")
	week := r.URL.Query().Get("week")
	rows, err := s.pool.Query(r.Context(), `
		SELECT id, week, season, home_team_id, away_team_id,
		       home_score::text, away_score::text,
		       home_projected::text, away_projected::text,
		       is_playoff, is_consolation, is_final
		FROM matchups WHERE league_id = $1 AND week::text = $2`, leagueID, week)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()
	out := []matchup{}
	for rows.Next() {
		var m matchup
		if err := rows.Scan(&m.ID, &m.Week, &m.Season, &m.HomeTeamID, &m.AwayTeamID,
			&m.HomeScore, &m.AwayScore, &m.HomeProjected, &m.AwayProjected,
			&m.IsPlayoff, &m.IsConsolation, &m.IsFinal); err != nil {
			continue
		}
		out = append(out, m)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"matchups": out})
}

type standing struct {
	TeamID        string `json:"team_id"`
	Name          string `json:"name"`
	Abbreviation  string `json:"abbreviation"`
	Wins          int    `json:"wins"`
	Losses        int    `json:"losses"`
	Ties          int    `json:"ties"`
	PointsFor     string `json:"points_for"`
	PointsAgainst string `json:"points_against"`
}

func (s *Service) standings(w http.ResponseWriter, r *http.Request) {
	leagueID := chi.URLParam(r, "leagueID")
	rows, err := s.pool.Query(r.Context(), `
		SELECT id, name, abbreviation, record_wins, record_losses, record_ties,
		       points_for::text, points_against::text
		FROM teams WHERE league_id = $1
		ORDER BY record_wins DESC, points_for DESC`, leagueID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()
	out := []standing{}
	for rows.Next() {
		var s standing
		if err := rows.Scan(&s.TeamID, &s.Name, &s.Abbreviation, &s.Wins, &s.Losses,
			&s.Ties, &s.PointsFor, &s.PointsAgainst); err != nil {
			continue
		}
		out = append(out, s)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"standings": out})
}

// generate builds a round-robin schedule using the circle method. Commissioner-
// only. Body: {"season": 2026, "regular_weeks": 14, "playoff_start_week": 15,
// "playoff_team_count": 6}.
type generateReq struct {
	Season           int `json:"season"`
	RegularWeeks     int `json:"regular_weeks"`
	PlayoffStartWeek int `json:"playoff_start_week"`
	PlayoffTeamCount int `json:"playoff_team_count"`
}

func (s *Service) generate(w http.ResponseWriter, r *http.Request) {
	leagueID := chi.URLParam(r, "leagueID")
	var req generateReq
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err)
		return
	}
	if req.RegularWeeks <= 0 {
		req.RegularWeeks = 14
	}
	if req.PlayoffStartWeek == 0 {
		req.PlayoffStartWeek = req.RegularWeeks + 1
	}

	rows, err := s.pool.Query(r.Context(),
		`SELECT id FROM teams WHERE league_id = $1 ORDER BY abbreviation`, leagueID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()
	teams := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, err)
			return
		}
		teams = append(teams, id)
	}
	if len(teams) < 2 {
		httpx.WriteError(w, http.StatusBadRequest, nil)
		return
	}

	pairs := roundRobin(teams, req.RegularWeeks)
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	defer tx.Rollback(r.Context())
	if _, err := tx.Exec(r.Context(),
		`DELETE FROM matchups WHERE league_id = $1 AND season = $2 AND NOT is_final`,
		leagueID, req.Season); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	for week, ms := range pairs {
		for _, m := range ms {
			_, err := tx.Exec(r.Context(), `
				INSERT INTO matchups (league_id, season, week, home_team_id, away_team_id)
				VALUES ($1, $2, $3, $4, $5)
				ON CONFLICT DO NOTHING`,
				leagueID, req.Season, week+1, m[0], m[1])
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, err)
				return
			}
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"weeks": req.RegularWeeks})
}

// roundRobin generates a round-robin schedule. If the number of teams is odd,
// a "BYE" sentinel is inserted (skipped at write time).
func roundRobin(teams []string, weeks int) [][][2]string {
	t := append([]string{}, teams...)
	if len(t)%2 == 1 {
		t = append(t, "")
	}
	n := len(t)
	rounds := make([][][2]string, weeks)
	for w := 0; w < weeks; w++ {
		ms := [][2]string{}
		for i := 0; i < n/2; i++ {
			a, b := t[i], t[n-1-i]
			if a != "" && b != "" {
				if w%2 == 1 {
					a, b = b, a
				}
				ms = append(ms, [2]string{a, b})
			}
		}
		rounds[w] = ms
		// rotate (keep [0] fixed)
		first := t[0]
		t = append([]string{first}, append([]string{t[n-1]}, t[1:n-1]...)...)
	}
	return rounds
}
