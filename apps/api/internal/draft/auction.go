package draft

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/bulbousoars/lunarleague/apps/api/internal/httpx"
	"github.com/go-chi/chi/v5"
)

// Auction state held in-process. Lost across restarts (acceptable for MVP);
// a Redis-backed store will replace this when we run multiple replicas.
type auctionState struct {
	DraftID            string `json:"draft_id"`
	NominatorIdx       int    `json:"nominator_idx"`
	NomineePlayer      string `json:"nominee_player_id,omitempty"`
	HighBidTeam        string `json:"high_bid_team_id,omitempty"`
	HighBid            int    `json:"high_bid"`
	BiddingDeadline    string `json:"bidding_deadline,omitempty"`
	NominationDeadline string `json:"nomination_deadline,omitempty"`
	PicksThisRound     int    `json:"picks_this_round"`
}

type auctionStateStore struct {
	mu      sync.Mutex
	byDraft map[string]*auctionState
}

var auctionStates = &auctionStateStore{byDraft: map[string]*auctionState{}}

func (s *auctionStateStore) get(id string) *auctionState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.byDraft[id]
}

func (s *auctionStateStore) set(id string, v *auctionState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byDraft[id] = v
}

type nominateReq struct {
	TeamID     string `json:"team_id"`
	PlayerID   string `json:"player_id"`
	OpeningBid int    `json:"opening_bid"`
}

type bidReq struct {
	TeamID string `json:"team_id"`
	Amount int    `json:"amount"`
}

func (s *Service) handleNominate(w http.ResponseWriter, r *http.Request) {
	leagueID := chi.URLParam(r, "leagueID")
	d, err := s.loadCurrent(r.Context(), leagueID)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, err)
		return
	}
	if d.Type != "auction" {
		httpx.WriteError(w, http.StatusBadRequest, errors.New("not an auction draft"))
		return
	}
	if d.Status != "in_progress" {
		httpx.WriteError(w, http.StatusConflict, errors.New("draft not running"))
		return
	}
	var req nominateReq
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err)
		return
	}
	st := auctionStates.get(d.ID)
	if st == nil {
		st = &auctionState{DraftID: d.ID}
	}
	if len(d.DraftOrder) == 0 {
		httpx.WriteError(w, http.StatusConflict, errors.New("draft order missing"))
		return
	}
	expectedTeam := d.DraftOrder[st.NominatorIdx%len(d.DraftOrder)]
	if expectedTeam != req.TeamID {
		httpx.WriteError(w, http.StatusForbidden, errors.New("not your turn to nominate"))
		return
	}
	if st.NomineePlayer != "" {
		httpx.WriteError(w, http.StatusConflict, errors.New("a player is already nominated"))
		return
	}
	st.NomineePlayer = req.PlayerID
	open := req.OpeningBid
	if open < 1 {
		open = 1
	}
	st.HighBid = open
	st.HighBidTeam = req.TeamID
	deadline := time.Now().Add(time.Duration(d.BiddingSeconds) * time.Second)
	st.BiddingDeadline = deadline.UTC().Format(time.RFC3339)
	auctionStates.set(d.ID, st)

	s.hub.Publish(r.Context(), "draft:"+d.ID, "auction_nominated", st)
	go s.scheduleAuctionFinalize(d, deadline)
	httpx.WriteJSON(w, http.StatusOK, st)
}

func (s *Service) handleBid(w http.ResponseWriter, r *http.Request) {
	leagueID := chi.URLParam(r, "leagueID")
	d, err := s.loadCurrent(r.Context(), leagueID)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, err)
		return
	}
	if d.Type != "auction" {
		httpx.WriteError(w, http.StatusBadRequest, errors.New("not an auction draft"))
		return
	}
	var req bidReq
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err)
		return
	}
	st := auctionStates.get(d.ID)
	if st == nil || st.NomineePlayer == "" {
		httpx.WriteError(w, http.StatusConflict, errors.New("nothing nominated"))
		return
	}
	if req.Amount <= st.HighBid {
		httpx.WriteError(w, http.StatusBadRequest, fmt.Errorf("bid must exceed $%d", st.HighBid))
		return
	}
	st.HighBid = req.Amount
	st.HighBidTeam = req.TeamID
	deadline := time.Now().Add(time.Duration(d.BiddingSeconds) * time.Second)
	st.BiddingDeadline = deadline.UTC().Format(time.RFC3339)
	auctionStates.set(d.ID, st)

	s.hub.Publish(r.Context(), "draft:"+d.ID, "auction_bid", st)
	go s.scheduleAuctionFinalize(d, deadline)
	httpx.WriteJSON(w, http.StatusOK, st)
}

func (s *Service) scheduleAuctionFinalize(d Draft, deadline time.Time) {
	time.Sleep(time.Until(deadline) + time.Second)
	st := auctionStates.get(d.ID)
	if st == nil {
		return
	}
	parsed, _ := time.Parse(time.RFC3339, st.BiddingDeadline)
	if !parsed.Equal(deadline) {
		return
	}
	ctx := context.Background()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return
	}
	defer tx.Rollback(ctx)
	pickNo := st.PicksThisRound + 1
	round := 1
	if _, err := tx.Exec(ctx, `
		INSERT INTO draft_picks (draft_id, pick_no, round, team_id, player_id, price, picked_at)
		VALUES ($1, $2, $3, $4, $5, $6, now())`,
		d.ID, pickNo, round, st.HighBidTeam, st.NomineePlayer, st.HighBid); err != nil {
		return
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO rosters (league_id, team_id, player_id, slot, acquired_via)
		VALUES ($1, $2, $3, 'BN', 'draft') ON CONFLICT DO NOTHING`,
		d.LeagueID, st.HighBidTeam, st.NomineePlayer); err != nil {
		return
	}
	if _, err := tx.Exec(ctx, `
		UPDATE teams SET auction_budget = COALESCE(auction_budget,0) - $1 WHERE id = $2`,
		st.HighBid, st.HighBidTeam); err != nil {
		return
	}
	_ = tx.Commit(ctx)
	s.hub.Publish(ctx, "draft:"+d.ID, "auction_won", map[string]any{
		"player_id": st.NomineePlayer, "team_id": st.HighBidTeam, "price": st.HighBid,
	})
	st.NominatorIdx = (st.NominatorIdx + 1) % len(d.DraftOrder)
	st.NomineePlayer = ""
	st.HighBid = 0
	st.HighBidTeam = ""
	st.BiddingDeadline = ""
	st.PicksThisRound++
	auctionStates.set(d.ID, st)
}
