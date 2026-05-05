// Package roster owns team rosters and weekly lineup management.
package roster

import (
	"context"
	"encoding/json"
	"errors"
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
	r.Get("/leagues/{leagueID}/teams/{teamID}/roster", s.list)
	r.Patch("/leagues/{leagueID}/teams/{teamID}/roster/slot", s.setSlot)
	r.Post("/leagues/{leagueID}/teams/{teamID}/roster/add", s.add)
	r.Post("/leagues/{leagueID}/teams/{teamID}/roster/drop", s.drop)
	r.Get("/leagues/{leagueID}/teams/{teamID}/lineup", s.getLineup)
	r.Put("/leagues/{leagueID}/teams/{teamID}/lineup", s.saveLineup)
}

type rosterEntry struct {
	ID         string  `json:"id"`
	PlayerID   string  `json:"player_id"`
	PlayerName string  `json:"player_name"`
	Position   *string `json:"position"`
	NFLTeam    *string `json:"nfl_team"`
	Slot       string  `json:"slot"`
	Acquired   string  `json:"acquired_via"`
	AcquiredAt string  `json:"acquired_at"`
	KeeperRound *int   `json:"keeper_round_cost,omitempty"`
}

func (s *Service) list(w http.ResponseWriter, r *http.Request) {
	teamID := chi.URLParam(r, "teamID")
	rows, err := s.pool.Query(r.Context(), `
		SELECT r.id, r.player_id, p.full_name, p.position, p.nfl_team, r.slot, r.acquired_via, r.acquired_at, r.keeper_round_cost
		FROM rosters r JOIN players p ON p.id = r.player_id
		WHERE r.team_id = $1
		ORDER BY p.position NULLS LAST, p.full_name`, teamID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()
	out := []rosterEntry{}
	for rows.Next() {
		var e rosterEntry
		if err := rows.Scan(&e.ID, &e.PlayerID, &e.PlayerName, &e.Position, &e.NFLTeam,
			&e.Slot, &e.Acquired, &e.AcquiredAt, &e.KeeperRound); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, err)
			return
		}
		out = append(out, e)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"roster": out})
}

type setSlotReq struct {
	PlayerID string `json:"player_id"`
	Slot     string `json:"slot"`
}

func (s *Service) setSlot(w http.ResponseWriter, r *http.Request) {
	teamID := chi.URLParam(r, "teamID")
	var req setSlotReq
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err)
		return
	}
	_, err := s.pool.Exec(r.Context(),
		`UPDATE rosters SET slot = $1 WHERE team_id = $2 AND player_id = $3`,
		req.Slot, teamID, req.PlayerID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

type addReq struct {
	PlayerID string `json:"player_id"`
	Acquired string `json:"acquired_via"`
}

func (s *Service) add(w http.ResponseWriter, r *http.Request) {
	leagueID := chi.URLParam(r, "leagueID")
	teamID := chi.URLParam(r, "teamID")
	var req addReq
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err)
		return
	}
	if req.Acquired == "" {
		req.Acquired = "free_agent"
	}
	if err := AddPlayer(r.Context(), s.pool, leagueID, teamID, req.PlayerID, req.Acquired); err != nil {
		httpx.WriteError(w, http.StatusConflict, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

type dropReq struct {
	PlayerID string `json:"player_id"`
}

func (s *Service) drop(w http.ResponseWriter, r *http.Request) {
	leagueID := chi.URLParam(r, "leagueID")
	teamID := chi.URLParam(r, "teamID")
	var req dropReq
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err)
		return
	}
	if err := DropPlayer(r.Context(), s.pool, leagueID, teamID, req.PlayerID); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// AddPlayer is a reusable transaction for waiver / FA / draft / trade flows.
func AddPlayer(ctx context.Context, pool *db.DB, leagueID, teamID, playerID, acquired string) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO rosters (league_id, team_id, player_id, slot, acquired_via)
		VALUES ($1, $2, $3, 'BN', $4)`,
		leagueID, teamID, playerID, acquired)
	if err != nil {
		return errors.New("could not add player (already rostered?)")
	}
	_, _ = pool.Exec(ctx, `
		INSERT INTO transactions (league_id, team_id, type, detail)
		VALUES ($1, $2, 'add', jsonb_build_object('player_id', $3::text, 'via', $4::text))`,
		leagueID, teamID, playerID, acquired)
	return nil
}

func DropPlayer(ctx context.Context, pool *db.DB, leagueID, teamID, playerID string) error {
	_, err := pool.Exec(ctx,
		`DELETE FROM rosters WHERE league_id = $1 AND team_id = $2 AND player_id = $3`,
		leagueID, teamID, playerID)
	if err != nil {
		return err
	}
	_, _ = pool.Exec(ctx, `
		INSERT INTO transactions (league_id, team_id, type, detail)
		VALUES ($1, $2, 'drop', jsonb_build_object('player_id', $3::text))`,
		leagueID, teamID, playerID)
	return nil
}

// --- Lineup ---

type lineupSlot struct {
	Slot     string `json:"slot"`
	PlayerID string `json:"player_id"`
}

type lineupResp struct {
	Starters []lineupSlot `json:"starters"`
	Bench    []lineupSlot `json:"bench"`
	LockedAt *string      `json:"locked_at"`
}

func (s *Service) getLineup(w http.ResponseWriter, r *http.Request) {
	teamID := chi.URLParam(r, "teamID")
	q := r.URL.Query()
	season := q.Get("season")
	week := q.Get("week")
	var startersRaw, benchRaw []byte
	var lockedAt *string
	err := s.pool.QueryRow(r.Context(), `
		SELECT starters, bench, locked_at FROM lineups
		WHERE team_id = $1 AND season::text = $2 AND week::text = $3`,
		teamID, season, week).Scan(&startersRaw, &benchRaw, &lockedAt)
	if err != nil {
		httpx.WriteJSON(w, http.StatusOK, lineupResp{Starters: []lineupSlot{}, Bench: []lineupSlot{}})
		return
	}
	var resp lineupResp
	_ = json.Unmarshal(startersRaw, &resp.Starters)
	_ = json.Unmarshal(benchRaw, &resp.Bench)
	resp.LockedAt = lockedAt
	httpx.WriteJSON(w, http.StatusOK, resp)
}

type saveLineupReq struct {
	Season   int          `json:"season"`
	Week     int          `json:"week"`
	Starters []lineupSlot `json:"starters"`
	Bench    []lineupSlot `json:"bench"`
}

func (s *Service) saveLineup(w http.ResponseWriter, r *http.Request) {
	leagueID := chi.URLParam(r, "leagueID")
	teamID := chi.URLParam(r, "teamID")
	var req saveLineupReq
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err)
		return
	}
	st, _ := json.Marshal(req.Starters)
	bn, _ := json.Marshal(req.Bench)
	_, err := s.pool.Exec(r.Context(), `
		INSERT INTO lineups (league_id, team_id, season, week, starters, bench)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6::jsonb)
		ON CONFLICT (team_id, season, week) DO UPDATE SET
			starters = EXCLUDED.starters, bench = EXCLUDED.bench`,
		leagueID, teamID, req.Season, req.Week, string(st), string(bn))
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}
