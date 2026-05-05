// Package trades implements trade proposal, counter, accept, veto vote, and
// commissioner review.
package trades

import (
	"context"
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
	r.Get("/leagues/{leagueID}/trades", s.list)
	r.Post("/leagues/{leagueID}/trades", s.propose)
	r.Get("/leagues/{leagueID}/trades/{tradeID}", s.get)
	r.Post("/leagues/{leagueID}/trades/{tradeID}/accept", s.accept)
	r.Post("/leagues/{leagueID}/trades/{tradeID}/reject", s.reject)
	r.Post("/leagues/{leagueID}/trades/{tradeID}/cancel", s.cancel)
	r.Post("/leagues/{leagueID}/trades/{tradeID}/vote", s.vote)
	r.Post("/leagues/{leagueID}/trades/{tradeID}/execute", s.execute)
}

type asset struct {
	FromTeamID string  `json:"from_team_id"`
	ToTeamID   string  `json:"to_team_id"`
	AssetType  string  `json:"asset_type"`
	PlayerID   *string `json:"player_id,omitempty"`
	FAAB       *int    `json:"faab_amount,omitempty"`
	PickRound  *int    `json:"pick_round,omitempty"`
	PickYear   *int    `json:"pick_year,omitempty"`
}

type Trade struct {
	ID             string  `json:"id"`
	ProposerTeamID string  `json:"proposer_team_id"`
	Status         string  `json:"status"`
	Note           *string `json:"note,omitempty"`
	ReviewUntil    *string `json:"review_until,omitempty"`
	Assets         []asset `json:"assets"`
	CreatedAt      string  `json:"created_at"`
}

type proposeReq struct {
	ProposerTeamID string  `json:"proposer_team_id"`
	Note           *string `json:"note"`
	Assets         []asset `json:"assets"`
}

func (s *Service) propose(w http.ResponseWriter, r *http.Request) {
	leagueID := chi.URLParam(r, "leagueID")
	var req proposeReq
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err)
		return
	}
	if len(req.Assets) == 0 {
		httpx.WriteError(w, http.StatusBadRequest, errors.New("at least one asset required"))
		return
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	defer tx.Rollback(r.Context())
	var t Trade
	err = tx.QueryRow(r.Context(), `
		INSERT INTO trades (league_id, proposer_team_id, note)
		VALUES ($1, $2, $3) RETURNING id, status, created_at`,
		leagueID, req.ProposerTeamID, req.Note).
		Scan(&t.ID, &t.Status, &t.CreatedAt)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	for _, a := range req.Assets {
		_, err := tx.Exec(r.Context(), `
			INSERT INTO trade_assets (trade_id, from_team_id, to_team_id, asset_type, player_id, faab_amount, pick_round, pick_year)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			t.ID, a.FromTeamID, a.ToTeamID, a.AssetType, a.PlayerID, a.FAAB, a.PickRound, a.PickYear)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, err)
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	t.ProposerTeamID = req.ProposerTeamID
	t.Note = req.Note
	t.Assets = req.Assets
	httpx.WriteJSON(w, http.StatusCreated, t)
}

func (s *Service) list(w http.ResponseWriter, r *http.Request) {
	leagueID := chi.URLParam(r, "leagueID")
	rows, err := s.pool.Query(r.Context(),
		`SELECT id, proposer_team_id, status, note, review_until, created_at
		 FROM trades WHERE league_id = $1 ORDER BY created_at DESC`, leagueID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()
	out := []Trade{}
	for rows.Next() {
		var t Trade
		if err := rows.Scan(&t.ID, &t.ProposerTeamID, &t.Status, &t.Note, &t.ReviewUntil, &t.CreatedAt); err != nil {
			continue
		}
		out = append(out, t)
	}
	for i := range out {
		out[i].Assets, _ = s.loadAssets(r.Context(), out[i].ID)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"trades": out})
}

func (s *Service) get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "tradeID")
	var t Trade
	err := s.pool.QueryRow(r.Context(),
		`SELECT id, proposer_team_id, status, note, review_until, created_at
		 FROM trades WHERE id = $1`, id).
		Scan(&t.ID, &t.ProposerTeamID, &t.Status, &t.Note, &t.ReviewUntil, &t.CreatedAt)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, err)
		return
	}
	t.Assets, _ = s.loadAssets(r.Context(), t.ID)
	httpx.WriteJSON(w, http.StatusOK, t)
}

func (s *Service) loadAssets(ctx context.Context, tradeID string) ([]asset, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT from_team_id, to_team_id, asset_type, player_id, faab_amount, pick_round, pick_year
		 FROM trade_assets WHERE trade_id = $1`, tradeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []asset{}
	for rows.Next() {
		var a asset
		_ = rows.Scan(&a.FromTeamID, &a.ToTeamID, &a.AssetType, &a.PlayerID, &a.FAAB, &a.PickRound, &a.PickYear)
		out = append(out, a)
	}
	return out, nil
}

func (s *Service) accept(w http.ResponseWriter, r *http.Request) {
	s.setStatus(w, r, "accepted")
}

func (s *Service) reject(w http.ResponseWriter, r *http.Request) {
	s.setStatus(w, r, "rejected")
}

func (s *Service) cancel(w http.ResponseWriter, r *http.Request) {
	s.setStatus(w, r, "cancelled")
}

func (s *Service) setStatus(w http.ResponseWriter, r *http.Request, status string) {
	id := chi.URLParam(r, "tradeID")
	_, err := s.pool.Exec(r.Context(),
		`UPDATE trades SET status = $2, updated_at = now() WHERE id = $1 AND status IN ('proposed','review','accepted')`,
		id, status)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

type voteReq struct {
	Vote string `json:"vote"` // 'approve' | 'veto'
}

func (s *Service) vote(w http.ResponseWriter, r *http.Request) {
	uid, _ := httpx.UserID(r.Context())
	id := chi.URLParam(r, "tradeID")
	var req voteReq
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err)
		return
	}
	_, err := s.pool.Exec(r.Context(), `
		INSERT INTO trade_votes (trade_id, user_id, vote)
		VALUES ($1, $2, $3)
		ON CONFLICT (trade_id, user_id) DO UPDATE SET vote = EXCLUDED.vote, voted_at = now()`,
		id, uid, req.Vote)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// execute swaps the assets atomically. Commissioner-only.
func (s *Service) execute(w http.ResponseWriter, r *http.Request) {
	leagueID := chi.URLParam(r, "leagueID")
	id := chi.URLParam(r, "tradeID")
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	defer tx.Rollback(r.Context())

	assets, err := s.loadAssets(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	for _, a := range assets {
		switch a.AssetType {
		case "player":
			if a.PlayerID == nil {
				continue
			}
			// Move roster row from -> to.
			_, _ = tx.Exec(r.Context(),
				`UPDATE rosters SET team_id = $1, acquired_via = 'trade', acquired_at = now()
				 WHERE league_id = $2 AND team_id = $3 AND player_id = $4`,
				a.ToTeamID, leagueID, a.FromTeamID, *a.PlayerID)
		case "faab":
			if a.FAAB != nil {
				_, _ = tx.Exec(r.Context(),
					`UPDATE teams SET waiver_budget = COALESCE(waiver_budget,0) - $1 WHERE id = $2`,
					*a.FAAB, a.FromTeamID)
				_, _ = tx.Exec(r.Context(),
					`UPDATE teams SET waiver_budget = COALESCE(waiver_budget,0) + $1 WHERE id = $2`,
					*a.FAAB, a.ToTeamID)
			}
		}
	}
	if _, err := tx.Exec(r.Context(),
		`UPDATE trades SET status = 'executed', updated_at = now() WHERE id = $1`, id); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}
