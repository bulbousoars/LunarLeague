// Package draft implements the live draft state machine + REST API.
//
// Two modes:
//   - snake: pick order reverses each round, fixed pick clock per slot.
//   - auction: rotating nominator, all teams bid until clock hits zero.
//
// State is persisted in `drafts` and `draft_picks`; the in-process Hub fans
// out events to subscribers (`draft:<id>` topic).
package draft

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/bulbousoars/lunarleague/apps/api/internal/db"
	"github.com/bulbousoars/lunarleague/apps/api/internal/httpx"
	"github.com/bulbousoars/lunarleague/apps/api/internal/player"
	"github.com/bulbousoars/lunarleague/apps/api/internal/ws"
	"github.com/go-chi/chi/v5"
)

type Service struct {
	pool *db.DB
	hub  *ws.Hub

	mu     sync.Mutex
	timers map[string]*time.Timer // draftID -> active pick clock
}

func NewService(pool *db.DB, hub *ws.Hub) *Service {
	return &Service{pool: pool, hub: hub, timers: map[string]*time.Timer{}}
}

func (s *Service) Mount(r chi.Router) {
	r.Get("/leagues/{leagueID}/draft", s.get)
	r.Post("/leagues/{leagueID}/draft", s.create)
	r.Post("/leagues/{leagueID}/draft/start", s.start)
	r.Post("/leagues/{leagueID}/draft/pause", s.pause)
	r.Post("/leagues/{leagueID}/draft/resume", s.resume)
	r.Post("/leagues/{leagueID}/draft/pick", s.makePick)
	r.Post("/leagues/{leagueID}/draft/queue", s.setQueue)
	r.Get("/leagues/{leagueID}/draft/queue", s.getQueue)

	// Auction-only:
	r.Post("/leagues/{leagueID}/draft/nominate", s.handleNominate)
	r.Post("/leagues/{leagueID}/draft/bid", s.handleBid)

	// Keeper designation (pre-draft).
	r.Get("/leagues/{leagueID}/draft/keepers", s.listKeepers)
	r.Post("/leagues/{leagueID}/draft/keepers", s.designateKeeper)
	r.Delete("/leagues/{leagueID}/draft/keepers/{keeperID}", s.removeKeeper)
}

// --- Models ---

type Draft struct {
	ID                string    `json:"id"`
	LeagueID          string    `json:"league_id"`
	Type              string    `json:"type"`
	Status            string    `json:"status"`
	Rounds            int       `json:"rounds"`
	PickSeconds       int       `json:"pick_seconds"`
	NominationSeconds int       `json:"nomination_seconds"`
	BiddingSeconds    int       `json:"bidding_seconds"`
	StartsAt          *string   `json:"starts_at,omitempty"`
	StartedAt         *string   `json:"started_at,omitempty"`
	CompletedAt       *string   `json:"completed_at,omitempty"`
	DraftOrder        []string  `json:"draft_order"`
	Config            JSONMap   `json:"config"`
	Picks             []Pick    `json:"picks"` // never omit: clients must always get an array (empty is [])
	OnTheClock        *OnClock  `json:"on_the_clock,omitempty"`
}

type Pick struct {
	ID         string  `json:"id"`
	PickNo     int     `json:"pick_no"`
	Round      int     `json:"round"`
	TeamID     string  `json:"team_id"`
	PlayerID   *string `json:"player_id"`
	Price      *int    `json:"price,omitempty"`
	IsKeeper   bool    `json:"is_keeper"`
	IsAutopick bool    `json:"is_autopick"`
	PickedAt   *string `json:"picked_at,omitempty"`
}

type OnClock struct {
	TeamID    string `json:"team_id"`
	PickNo    int    `json:"pick_no"`
	Round     int    `json:"round"`
	DeadlineISO string `json:"deadline"`
}

type JSONMap map[string]any

// --- Handlers ---

type createReq struct {
	Type              string   `json:"type"`
	Rounds            int      `json:"rounds"`
	PickSeconds       int      `json:"pick_seconds"`
	NominationSeconds int      `json:"nomination_seconds"`
	BiddingSeconds    int      `json:"bidding_seconds"`
	StartsAt          *string  `json:"starts_at"`
	DraftOrder        []string `json:"draft_order"` // team ids; empty = randomize at start
	AuctionBudget     int      `json:"auction_budget"`
}

func (s *Service) create(w http.ResponseWriter, r *http.Request) {
	leagueID := chi.URLParam(r, "leagueID")
	var req createReq
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err)
		return
	}
	if req.Type == "" {
		req.Type = "snake"
	}
	if req.PickSeconds == 0 {
		req.PickSeconds = 90
	}
	if req.Rounds == 0 {
		req.Rounds = 16
	}
	cfg := JSONMap{}
	if req.AuctionBudget > 0 {
		cfg["auction_budget"] = req.AuctionBudget
	}
	cfgJSON, _ := json.Marshal(cfg)

	orderJSON, _ := json.Marshal(req.DraftOrder)

	var d Draft
	d.LeagueID = leagueID
	err := s.pool.QueryRow(r.Context(), `
		INSERT INTO drafts (league_id, type, status, rounds, pick_seconds, nomination_seconds, bidding_seconds, starts_at, draft_order, config)
		VALUES ($1, $2, 'pending', $3, $4, $5, $6, $7, $8::jsonb, $9::jsonb)
		RETURNING id, type, status, rounds, pick_seconds, nomination_seconds, bidding_seconds, starts_at`,
		leagueID, req.Type, req.Rounds, req.PickSeconds,
		req.NominationSeconds, req.BiddingSeconds, req.StartsAt,
		string(orderJSON), string(cfgJSON)).
		Scan(&d.ID, &d.Type, &d.Status, &d.Rounds, &d.PickSeconds,
			&d.NominationSeconds, &d.BiddingSeconds, &d.StartsAt)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	d.DraftOrder = req.DraftOrder
	d.Config = cfg
	d.Picks = []Pick{}
	httpx.WriteJSON(w, http.StatusCreated, d)
}

func (s *Service) get(w http.ResponseWriter, r *http.Request) {
	leagueID := chi.URLParam(r, "leagueID")
	d, err := s.loadCurrent(r.Context(), leagueID)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, err)
		return
	}
	picks, _ := s.loadPicks(r.Context(), d.ID)
	if picks == nil {
		picks = []Pick{}
	}
	d.Picks = picks
	if d.Status == "in_progress" {
		clk := s.computeOnClock(d)
		d.OnTheClock = clk
	}
	httpx.WriteJSON(w, http.StatusOK, d)
}

func (s *Service) start(w http.ResponseWriter, r *http.Request) {
	leagueID := chi.URLParam(r, "leagueID")
	d, err := s.loadCurrent(r.Context(), leagueID)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, err)
		return
	}
	if d.Status != "pending" && d.Status != "paused" {
		httpx.WriteError(w, http.StatusConflict, errors.New("draft cannot start in this state"))
		return
	}
	// Randomize order if missing.
	if len(d.DraftOrder) == 0 {
		teams, err := s.teamIDs(r.Context(), leagueID)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, err)
			return
		}
		d.DraftOrder = teams
	}
	orderJSON, _ := json.Marshal(d.DraftOrder)
	_, err = s.pool.Exec(r.Context(), `
		UPDATE drafts SET status = 'in_progress', started_at = COALESCE(started_at, now()),
		                  draft_order = $2::jsonb
		WHERE id = $1`, d.ID, string(orderJSON))
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	d.Status = "in_progress"

	// Pre-create the rows for all picks (snake) so we know who's on the clock
	// without recomputing each call.
	if d.Type == "snake" {
		if err := s.seedSnakePicks(r.Context(), d); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, err)
			return
		}
	}

	s.hub.Publish(r.Context(), "draft:"+d.ID, "started", d)
	s.armClock(d)
	httpx.WriteJSON(w, http.StatusOK, d)
}

func (s *Service) pause(w http.ResponseWriter, r *http.Request) {
	d, err := s.loadCurrent(r.Context(), chi.URLParam(r, "leagueID"))
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, err)
		return
	}
	if _, err := s.pool.Exec(r.Context(),
		`UPDATE drafts SET status = 'paused' WHERE id = $1`, d.ID); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	s.disarmClock(d.ID)
	s.hub.Publish(r.Context(), "draft:"+d.ID, "paused", nil)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Service) resume(w http.ResponseWriter, r *http.Request) {
	d, err := s.loadCurrent(r.Context(), chi.URLParam(r, "leagueID"))
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, err)
		return
	}
	if _, err := s.pool.Exec(r.Context(),
		`UPDATE drafts SET status = 'in_progress' WHERE id = $1`, d.ID); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	d.Status = "in_progress"
	s.armClock(d)
	s.hub.Publish(r.Context(), "draft:"+d.ID, "resumed", nil)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

type pickReq struct {
	PlayerID string `json:"player_id"`
	Price    *int   `json:"price,omitempty"` // auction
}

func (s *Service) makePick(w http.ResponseWriter, r *http.Request) {
	leagueID := chi.URLParam(r, "leagueID")
	d, err := s.loadCurrent(r.Context(), leagueID)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, err)
		return
	}
	if d.Status != "in_progress" {
		httpx.WriteError(w, http.StatusConflict, errors.New("draft not running"))
		return
	}
	var req pickReq
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err)
		return
	}
	clk := s.computeOnClock(d)
	if clk == nil {
		httpx.WriteError(w, http.StatusConflict, errors.New("draft complete"))
		return
	}
	if err := s.recordPick(r.Context(), d, clk, req.PlayerID, req.Price, false); err != nil {
		httpx.WriteError(w, http.StatusConflict, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

type queueReq struct {
	TeamID    string   `json:"team_id"`
	PlayerIDs []string `json:"player_ids"`
}

func (s *Service) setQueue(w http.ResponseWriter, r *http.Request) {
	leagueID := chi.URLParam(r, "leagueID")
	d, err := s.loadCurrent(r.Context(), leagueID)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, err)
		return
	}
	var req queueReq
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
	if _, err := tx.Exec(r.Context(),
		`DELETE FROM draft_queues WHERE draft_id = $1 AND team_id = $2`,
		d.ID, req.TeamID); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	for i, pid := range req.PlayerIDs {
		if _, err := tx.Exec(r.Context(), `
			INSERT INTO draft_queues (draft_id, team_id, player_id, rank)
			VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`,
			d.ID, req.TeamID, pid, i+1); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, err)
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Service) getQueue(w http.ResponseWriter, r *http.Request) {
	leagueID := chi.URLParam(r, "leagueID")
	teamID := r.URL.Query().Get("team_id")
	d, err := s.loadCurrent(r.Context(), leagueID)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, err)
		return
	}
	rows, err := s.pool.Query(r.Context(), fmt.Sprintf(`
		SELECT q.player_id, q.rank, %s, p.position, p.nfl_team
		FROM draft_queues q JOIN players p ON p.id = q.player_id
		WHERE q.draft_id = $1 AND q.team_id = $2
		ORDER BY q.rank`, player.DisplayNameP), d.ID, teamID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()
	type entry struct {
		PlayerID string  `json:"player_id"`
		Rank     int     `json:"rank"`
		Name     string  `json:"name"`
		Position *string `json:"position"`
		Team     *string `json:"team"`
	}
	out := []entry{}
	for rows.Next() {
		var e entry
		_ = rows.Scan(&e.PlayerID, &e.Rank, &e.Name, &e.Position, &e.Team)
		out = append(out, e)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"queue": out})
}

// --- Keeper designation ---

type keeperReq struct {
	TeamID    string `json:"team_id"`
	PlayerID  string `json:"player_id"`
	RoundCost int    `json:"round_cost"`
}

func (s *Service) listKeepers(w http.ResponseWriter, r *http.Request) {
	leagueID := chi.URLParam(r, "leagueID")
	rows, err := s.pool.Query(r.Context(), fmt.Sprintf(`
		SELECT r.id, r.team_id, r.player_id, %s, r.keeper_round_cost
		FROM rosters r JOIN players p ON p.id = r.player_id
		WHERE r.league_id = $1 AND r.keeper_round_cost IS NOT NULL`, player.DisplayNameP), leagueID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()
	type k struct {
		ID, TeamID, PlayerID, PlayerName string
		RoundCost                        int
	}
	out := []k{}
	for rows.Next() {
		var x k
		_ = rows.Scan(&x.ID, &x.TeamID, &x.PlayerID, &x.PlayerName, &x.RoundCost)
		out = append(out, x)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"keepers": out})
}

func (s *Service) designateKeeper(w http.ResponseWriter, r *http.Request) {
	leagueID := chi.URLParam(r, "leagueID")
	var req keeperReq
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.pool.Exec(r.Context(), `
		UPDATE rosters SET keeper_round_cost = $4
		WHERE league_id = $1 AND team_id = $2 AND player_id = $3`,
		leagueID, req.TeamID, req.PlayerID, req.RoundCost)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	if res.RowsAffected() == 0 {
		httpx.WriteError(w, http.StatusNotFound, errors.New("player not on this team's roster"))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Service) removeKeeper(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "keeperID")
	_, err := s.pool.Exec(r.Context(),
		`UPDATE rosters SET keeper_round_cost = NULL WHERE id = $1`, id)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// --- internals ---

func (s *Service) loadCurrent(ctx context.Context, leagueID string) (Draft, error) {
	var d Draft
	d.LeagueID = leagueID
	var orderRaw, cfgRaw []byte
	err := s.pool.QueryRow(ctx, `
		SELECT id, type, status, rounds, pick_seconds, nomination_seconds, bidding_seconds,
		       starts_at, started_at, completed_at, draft_order, config
		FROM drafts WHERE league_id = $1
		ORDER BY created_at DESC LIMIT 1`, leagueID).
		Scan(&d.ID, &d.Type, &d.Status, &d.Rounds, &d.PickSeconds,
			&d.NominationSeconds, &d.BiddingSeconds,
			&d.StartsAt, &d.StartedAt, &d.CompletedAt,
			&orderRaw, &cfgRaw)
	if err != nil {
		return d, err
	}
	_ = json.Unmarshal(orderRaw, &d.DraftOrder)
	_ = json.Unmarshal(cfgRaw, &d.Config)
	return d, nil
}

func (s *Service) loadPicks(ctx context.Context, draftID string) ([]Pick, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, pick_no, round, team_id, player_id, price, is_keeper, is_autopick, picked_at
		FROM draft_picks WHERE draft_id = $1
		ORDER BY pick_no`, draftID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Pick{}
	for rows.Next() {
		var p Pick
		if err := rows.Scan(&p.ID, &p.PickNo, &p.Round, &p.TeamID, &p.PlayerID, &p.Price,
			&p.IsKeeper, &p.IsAutopick, &p.PickedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

// computeOnClock returns the next un-filled pick.
func (s *Service) computeOnClock(d Draft) *OnClock {
	picks, err := s.loadPicks(context.Background(), d.ID)
	if err != nil {
		return nil
	}
	for _, p := range picks {
		if p.PlayerID == nil {
			return &OnClock{
				TeamID:      p.TeamID,
				PickNo:      p.PickNo,
				Round:       p.Round,
				DeadlineISO: time.Now().Add(time.Duration(d.PickSeconds) * time.Second).UTC().Format(time.RFC3339),
			}
		}
	}
	return nil
}

func (s *Service) recordPick(ctx context.Context, d Draft, clk *OnClock, playerID string, price *int, autopick bool) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	res, err := tx.Exec(ctx, `
		UPDATE draft_picks SET player_id = $4, price = $5, is_autopick = $6, picked_at = now()
		WHERE draft_id = $1 AND pick_no = $2 AND team_id = $3 AND player_id IS NULL`,
		d.ID, clk.PickNo, clk.TeamID, playerID, price, autopick)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return errors.New("pick already taken or not on the clock")
	}
	// Mirror onto rosters.
	if _, err := tx.Exec(ctx, `
		INSERT INTO rosters (league_id, team_id, player_id, slot, acquired_via)
		VALUES ($1, $2, $3, 'BN', 'draft')
		ON CONFLICT DO NOTHING`,
		d.LeagueID, clk.TeamID, playerID); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	// Mark draft complete if no nulls left.
	var remaining int
	_ = s.pool.QueryRow(ctx,
		`SELECT count(*) FROM draft_picks WHERE draft_id = $1 AND player_id IS NULL`, d.ID).
		Scan(&remaining)
	if remaining == 0 {
		_, _ = s.pool.Exec(ctx,
			`UPDATE drafts SET status = 'complete', completed_at = now() WHERE id = $1`, d.ID)
		_, _ = s.pool.Exec(ctx,
			`UPDATE leagues SET status = 'in_season' WHERE id = $1`, d.LeagueID)
		s.disarmClock(d.ID)
		s.hub.Publish(ctx, "draft:"+d.ID, "complete", nil)
		return nil
	}
	s.hub.Publish(ctx, "draft:"+d.ID, "pick", map[string]any{
		"pick_no": clk.PickNo, "team_id": clk.TeamID, "player_id": playerID, "price": price, "autopick": autopick,
	})
	d.Status = "in_progress"
	s.armClock(d)
	return nil
}

func (s *Service) seedSnakePicks(ctx context.Context, d Draft) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	pickNo := 1
	for round := 1; round <= d.Rounds; round++ {
		order := append([]string{}, d.DraftOrder...)
		if round%2 == 0 {
			// reverse for even rounds (snake)
			for i, j := 0, len(order)-1; i < j; i, j = i+1, j-1 {
				order[i], order[j] = order[j], order[i]
			}
		}
		for _, teamID := range order {
			if _, err := tx.Exec(ctx, `
				INSERT INTO draft_picks (draft_id, pick_no, round, team_id)
				VALUES ($1, $2, $3, $4)
				ON CONFLICT DO NOTHING`,
				d.ID, pickNo, round, teamID); err != nil {
				return err
			}
			pickNo++
		}
	}
	return tx.Commit(ctx)
}

func (s *Service) teamIDs(ctx context.Context, leagueID string) ([]string, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id FROM teams WHERE league_id = $1 ORDER BY abbreviation`, leagueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, nil
}

// armClock starts a timer for the current pick. When it fires, autopick the
// top-queued player (or the highest-projected available if queue empty).
func (s *Service) armClock(d Draft) {
	s.disarmClock(d.ID)
	clk := s.computeOnClock(d)
	if clk == nil {
		return
	}
	dur := time.Duration(d.PickSeconds) * time.Second
	t := time.AfterFunc(dur, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		s.autopick(ctx, d, clk)
	})
	s.mu.Lock()
	s.timers[d.ID] = t
	s.mu.Unlock()
	s.hub.Publish(context.Background(), "draft:"+d.ID, "on_clock", clk)
}

func (s *Service) disarmClock(draftID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.timers[draftID]; ok {
		t.Stop()
		delete(s.timers, draftID)
	}
}

func (s *Service) autopick(ctx context.Context, d Draft, clk *OnClock) {
	// Take the top-queued *available* player; if queue is empty fall back to
	// any not-yet-drafted player (worst-case lottery so the draft doesn't
	// stall).
	var playerID string
	err := s.pool.QueryRow(ctx, `
		SELECT q.player_id FROM draft_queues q
		WHERE q.draft_id = $1 AND q.team_id = $2
		  AND NOT EXISTS (
			SELECT 1 FROM rosters r WHERE r.league_id = $3 AND r.player_id = q.player_id
		  )
		ORDER BY q.rank LIMIT 1`,
		d.ID, clk.TeamID, d.LeagueID).Scan(&playerID)
	if err != nil {
		// fallback
		_ = s.pool.QueryRow(ctx, `
			SELECT p.id FROM players p JOIN sports sp ON sp.id = p.sport_id
			WHERE sp.code = (SELECT code FROM sports WHERE id = (SELECT sport_id FROM leagues WHERE id = $1))
			  AND NOT EXISTS (SELECT 1 FROM rosters r WHERE r.league_id = $1 AND r.player_id = p.id)
			ORDER BY `+player.DisplayNameP+` LIMIT 1`, d.LeagueID).Scan(&playerID)
	}
	if playerID == "" {
		fmt.Println("autopick: no eligible player")
		return
	}
	_ = s.recordPick(ctx, d, clk, playerID, nil, true)
}
