// Package waivers handles waiver claims (FAAB + rolling) and free-agent
// pickups outside of the waiver window.
package waivers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/bulbousoars/lunarleague/apps/api/internal/db"
	"github.com/bulbousoars/lunarleague/apps/api/internal/httpx"
	"github.com/bulbousoars/lunarleague/apps/api/internal/roster"
	"github.com/go-chi/chi/v5"
)

type Service struct {
	pool *db.DB
}

func NewService(pool *db.DB) *Service { return &Service{pool: pool} }

func (s *Service) Mount(r chi.Router) {
	r.Get("/leagues/{leagueID}/waivers", s.list)
	r.Post("/leagues/{leagueID}/waivers", s.create)
	r.Delete("/leagues/{leagueID}/waivers/{claimID}", s.cancel)
	r.Get("/leagues/{leagueID}/free-agents", s.freeAgents)
	r.Post("/leagues/{leagueID}/teams/{teamID}/free-agents/add", s.addFreeAgent)
}

type claim struct {
	ID            string  `json:"id"`
	TeamID        string  `json:"team_id"`
	AddPlayerID   string  `json:"add_player_id"`
	DropPlayerID  *string `json:"drop_player_id,omitempty"`
	BidAmount     *int    `json:"bid_amount,omitempty"`
	Priority      int     `json:"priority"`
	Status        string  `json:"status"`
	ProcessAt     string  `json:"process_at"`
	ProcessedAt   *string `json:"processed_at,omitempty"`
	FailureReason *string `json:"failure_reason,omitempty"`
}

func (s *Service) list(w http.ResponseWriter, r *http.Request) {
	leagueID := chi.URLParam(r, "leagueID")
	rows, err := s.pool.Query(r.Context(), `
		SELECT id, team_id, add_player_id, drop_player_id, bid_amount, priority,
		       status, process_at, processed_at, failure_reason
		FROM waiver_claims WHERE league_id = $1 ORDER BY created_at DESC LIMIT 200`,
		leagueID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()
	out := []claim{}
	for rows.Next() {
		var c claim
		if err := rows.Scan(&c.ID, &c.TeamID, &c.AddPlayerID, &c.DropPlayerID,
			&c.BidAmount, &c.Priority, &c.Status, &c.ProcessAt, &c.ProcessedAt, &c.FailureReason); err != nil {
			continue
		}
		out = append(out, c)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"claims": out})
}

type createReq struct {
	TeamID       string  `json:"team_id"`
	AddPlayerID  string  `json:"add_player_id"`
	DropPlayerID *string `json:"drop_player_id"`
	BidAmount    *int    `json:"bid_amount"`
}

func (s *Service) create(w http.ResponseWriter, r *http.Request) {
	leagueID := chi.URLParam(r, "leagueID")
	var req createReq
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err)
		return
	}
	processAt, err := s.nextWaiverRun(r.Context(), leagueID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	var prio int
	_ = s.pool.QueryRow(r.Context(),
		`SELECT COALESCE(MAX(priority),0)+1 FROM waiver_claims
		 WHERE league_id = $1 AND team_id = $2 AND status = 'pending'`,
		leagueID, req.TeamID).Scan(&prio)
	_, err = s.pool.Exec(r.Context(), `
		INSERT INTO waiver_claims (league_id, team_id, add_player_id, drop_player_id,
		                           bid_amount, priority, process_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		leagueID, req.TeamID, req.AddPlayerID, req.DropPlayerID,
		req.BidAmount, prio, processAt)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"ok": true, "process_at": processAt})
}

func (s *Service) cancel(w http.ResponseWriter, r *http.Request) {
	leagueID := chi.URLParam(r, "leagueID")
	id := chi.URLParam(r, "claimID")
	_, err := s.pool.Exec(r.Context(),
		`UPDATE waiver_claims SET status = 'cancelled' WHERE id = $1 AND league_id = $2 AND status = 'pending'`,
		id, leagueID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// freeAgents lists undrafted/un-rostered players in the league's sport.
func (s *Service) freeAgents(w http.ResponseWriter, r *http.Request) {
	leagueID := chi.URLParam(r, "leagueID")
	rows, err := s.pool.Query(r.Context(), `
		SELECT p.id, p.full_name, p.position, p.nfl_team, p.injury_status, p.headshot_url
		FROM players p
		JOIN leagues l ON l.id = $1
		WHERE p.sport_id = l.sport_id
		  AND NOT EXISTS (SELECT 1 FROM rosters r WHERE r.league_id = $1 AND r.player_id = p.id)
		ORDER BY p.full_name
		LIMIT 200`, leagueID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()
	type fa struct {
		ID          string  `json:"id"`
		FullName    string  `json:"full_name"`
		Position    *string `json:"position"`
		NFLTeam     *string `json:"nfl_team"`
		InjuryStatus *string `json:"injury_status,omitempty"`
		HeadshotURL *string `json:"headshot_url,omitempty"`
	}
	out := []fa{}
	for rows.Next() {
		var f fa
		_ = rows.Scan(&f.ID, &f.FullName, &f.Position, &f.NFLTeam, &f.InjuryStatus, &f.HeadshotURL)
		out = append(out, f)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"free_agents": out})
}

type addFAReq struct {
	PlayerID     string  `json:"player_id"`
	DropPlayerID *string `json:"drop_player_id"`
}

func (s *Service) addFreeAgent(w http.ResponseWriter, r *http.Request) {
	leagueID := chi.URLParam(r, "leagueID")
	teamID := chi.URLParam(r, "teamID")
	var req addFAReq
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err)
		return
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	defer tx.Rollback(r.Context())
	if req.DropPlayerID != nil && *req.DropPlayerID != "" {
		if err := roster.DropPlayer(r.Context(), s.pool, leagueID, teamID, *req.DropPlayerID); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, err)
			return
		}
	}
	if err := roster.AddPlayer(r.Context(), s.pool, leagueID, teamID, req.PlayerID, "free_agent"); err != nil {
		httpx.WriteError(w, http.StatusConflict, err)
		return
	}
	_ = tx.Commit(r.Context())
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// --- worker side ---

// ProcessDue scans for due waiver claims and resolves them. Called by the
// worker on a 5-minute tick. Each league processes its claims in priority
// order: highest bid wins (FAAB) or lowest priority number wins (rolling).
func (s *Service) ProcessDue(ctx context.Context) error {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT league_id FROM waiver_claims
		WHERE status = 'pending' AND process_at <= now()`)
	if err != nil {
		return err
	}
	leagues := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		leagues = append(leagues, id)
	}
	rows.Close()

	for _, lid := range leagues {
		if err := s.processLeague(ctx, lid); err != nil {
			fmt.Println("waiver process league:", lid, err)
		}
	}
	return nil
}

func (s *Service) processLeague(ctx context.Context, leagueID string) error {
	var waiverType string
	err := s.pool.QueryRow(ctx,
		`SELECT waiver_type FROM league_settings WHERE league_id = $1`, leagueID).
		Scan(&waiverType)
	if err != nil {
		return err
	}

	var orderClause string
	switch waiverType {
	case "faab":
		orderClause = "ORDER BY add_player_id, bid_amount DESC NULLS LAST, priority"
	case "rolling", "reverse_standings":
		orderClause = "ORDER BY add_player_id, priority"
	default:
		orderClause = "ORDER BY add_player_id, priority"
	}

	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, team_id, add_player_id, drop_player_id, bid_amount, priority
		FROM waiver_claims
		WHERE league_id = $1 AND status = 'pending' AND process_at <= now()
		%s`, orderClause), leagueID)
	if err != nil {
		return err
	}
	type c struct {
		id, teamID, addID string
		dropID            *string
		bid               *int
	}
	claims := []c{}
	for rows.Next() {
		var x c
		var prio int
		if err := rows.Scan(&x.id, &x.teamID, &x.addID, &x.dropID, &x.bid, &prio); err != nil {
			rows.Close()
			return err
		}
		claims = append(claims, x)
	}
	rows.Close()

	awarded := map[string]bool{} // add_player_id -> done
	for _, x := range claims {
		if awarded[x.addID] {
			_, _ = s.pool.Exec(ctx,
				`UPDATE waiver_claims SET status = 'lost', processed_at = now() WHERE id = $1`,
				x.id)
			continue
		}
		if x.dropID != nil && *x.dropID != "" {
			_ = roster.DropPlayer(ctx, s.pool, leagueID, x.teamID, *x.dropID)
		}
		if err := roster.AddPlayer(ctx, s.pool, leagueID, x.teamID, x.addID, "waiver"); err != nil {
			_, _ = s.pool.Exec(ctx,
				`UPDATE waiver_claims SET status = 'failed', processed_at = now(), failure_reason = $2 WHERE id = $1`,
				x.id, err.Error())
			continue
		}
		// Deduct FAAB.
		if waiverType == "faab" && x.bid != nil {
			_, _ = s.pool.Exec(ctx,
				`UPDATE teams SET waiver_budget = COALESCE(waiver_budget,0) - $1 WHERE id = $2`,
				*x.bid, x.teamID)
		}
		_, _ = s.pool.Exec(ctx,
			`UPDATE waiver_claims SET status = 'won', processed_at = now() WHERE id = $1`,
			x.id)
		awarded[x.addID] = true
	}
	return nil
}

func (s *Service) nextWaiverRun(ctx context.Context, leagueID string) (time.Time, error) {
	var dow, hour int
	err := s.pool.QueryRow(ctx,
		`SELECT waiver_run_dow, waiver_run_hour FROM league_settings WHERE league_id = $1`,
		leagueID).Scan(&dow, &hour)
	if err != nil {
		return time.Time{}, err
	}
	loc, _ := time.LoadLocation("America/New_York")
	now := time.Now().In(loc)
	target := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, loc)
	for {
		if int(target.Weekday()) == dow && target.After(now) {
			return target, nil
		}
		target = target.Add(time.Hour)
		if target.Sub(now) > 7*24*time.Hour {
			return time.Time{}, errors.New("could not find next waiver slot")
		}
	}
}
