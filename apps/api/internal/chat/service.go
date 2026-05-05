// Package chat owns league chat, message board, polls, and reactions.
package chat

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/bulbousoars/lunarleague/apps/api/internal/db"
	"github.com/bulbousoars/lunarleague/apps/api/internal/httpx"
	"github.com/bulbousoars/lunarleague/apps/api/internal/ws"
	"github.com/go-chi/chi/v5"
)

type Service struct {
	pool *db.DB
	hub  *ws.Hub
}

func NewService(pool *db.DB, hub *ws.Hub) *Service { return &Service{pool: pool, hub: hub} }

func (s *Service) Mount(r chi.Router) {
	r.Get("/leagues/{leagueID}/messages", s.list)
	r.Post("/leagues/{leagueID}/messages", s.post)
	r.Delete("/leagues/{leagueID}/messages/{messageID}", s.delete)
	r.Post("/leagues/{leagueID}/messages/{messageID}/react", s.react)

	r.Get("/leagues/{leagueID}/polls", s.listPolls)
	r.Post("/leagues/{leagueID}/polls", s.createPoll)
	r.Post("/leagues/{leagueID}/polls/{pollID}/vote", s.votePoll)
}

type message struct {
	ID          string         `json:"id"`
	Channel     string         `json:"channel"`
	UserID      *string        `json:"user_id"`
	DisplayName *string        `json:"display_name,omitempty"`
	Body        string         `json:"body"`
	Refs        map[string]any `json:"refs"`
	CreatedAt   string         `json:"created_at"`
}

func (s *Service) list(w http.ResponseWriter, r *http.Request) {
	leagueID := chi.URLParam(r, "leagueID")
	q := r.URL.Query()
	channel := q.Get("channel")
	if channel == "" {
		channel = "main"
	}
	limit := 50
	if v, _ := strconv.Atoi(q.Get("limit")); v > 0 && v <= 200 {
		limit = v
	}
	rows, err := s.pool.Query(r.Context(), `
		SELECT m.id, m.channel, m.user_id, u.display_name, m.body, m.refs, m.created_at
		FROM messages m LEFT JOIN users u ON u.id = m.user_id
		WHERE m.league_id = $1 AND m.channel = $2 AND m.deleted_at IS NULL
		ORDER BY m.created_at DESC LIMIT $3`, leagueID, channel, limit)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()
	out := []message{}
	for rows.Next() {
		var m message
		var refsRaw []byte
		if err := rows.Scan(&m.ID, &m.Channel, &m.UserID, &m.DisplayName, &m.Body, &refsRaw, &m.CreatedAt); err != nil {
			continue
		}
		_ = json.Unmarshal(refsRaw, &m.Refs)
		out = append(out, m)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"messages": out})
}

type postReq struct {
	Channel string         `json:"channel"`
	Body    string         `json:"body"`
	Refs    map[string]any `json:"refs"`
}

func (s *Service) post(w http.ResponseWriter, r *http.Request) {
	uid, err := httpx.UserID(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, err)
		return
	}
	leagueID := chi.URLParam(r, "leagueID")
	var req postReq
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err)
		return
	}
	if req.Channel == "" {
		req.Channel = "main"
	}
	if strings.TrimSpace(req.Body) == "" {
		httpx.WriteError(w, http.StatusBadRequest, errors.New("body required"))
		return
	}
	refs, _ := json.Marshal(req.Refs)
	var m message
	err = s.pool.QueryRow(r.Context(), `
		INSERT INTO messages (league_id, channel, user_id, body, refs)
		VALUES ($1, $2, $3, $4, $5::jsonb)
		RETURNING id, channel, user_id, body, refs, created_at`,
		leagueID, req.Channel, uid, req.Body, string(refs)).
		Scan(&m.ID, &m.Channel, &m.UserID, &m.Body, &refs, &m.CreatedAt)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	_ = json.Unmarshal(refs, &m.Refs)
	s.hub.Publish(r.Context(), "league:"+leagueID, "chat", m)
	httpx.WriteJSON(w, http.StatusCreated, m)
}

func (s *Service) delete(w http.ResponseWriter, r *http.Request) {
	uid, _ := httpx.UserID(r.Context())
	id := chi.URLParam(r, "messageID")
	_, err := s.pool.Exec(r.Context(),
		`UPDATE messages SET deleted_at = now() WHERE id = $1 AND user_id = $2`,
		id, uid)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

type reactReq struct {
	Emoji  string `json:"emoji"`
	Remove bool   `json:"remove"`
}

func (s *Service) react(w http.ResponseWriter, r *http.Request) {
	uid, _ := httpx.UserID(r.Context())
	id := chi.URLParam(r, "messageID")
	var req reactReq
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err)
		return
	}
	if req.Remove {
		_, _ = s.pool.Exec(r.Context(),
			`DELETE FROM message_reactions WHERE message_id = $1 AND user_id = $2 AND emoji = $3`,
			id, uid, req.Emoji)
	} else {
		_, _ = s.pool.Exec(r.Context(), `
			INSERT INTO message_reactions (message_id, user_id, emoji) VALUES ($1, $2, $3)
			ON CONFLICT DO NOTHING`, id, uid, req.Emoji)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

type poll struct {
	ID        string           `json:"id"`
	Question  string           `json:"question"`
	Options   []map[string]any `json:"options"`
	ClosesAt  *string          `json:"closes_at,omitempty"`
	CreatedBy *string          `json:"created_by"`
	CreatedAt string           `json:"created_at"`
}

func (s *Service) listPolls(w http.ResponseWriter, r *http.Request) {
	leagueID := chi.URLParam(r, "leagueID")
	rows, err := s.pool.Query(r.Context(),
		`SELECT id, question, options, closes_at, created_by, created_at
		 FROM polls WHERE league_id = $1 ORDER BY created_at DESC`, leagueID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()
	out := []poll{}
	for rows.Next() {
		var p poll
		var optsRaw []byte
		if err := rows.Scan(&p.ID, &p.Question, &optsRaw, &p.ClosesAt, &p.CreatedBy, &p.CreatedAt); err != nil {
			continue
		}
		_ = json.Unmarshal(optsRaw, &p.Options)
		out = append(out, p)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"polls": out})
}

type createPollReq struct {
	Question string           `json:"question"`
	Options  []map[string]any `json:"options"`
	ClosesAt *string          `json:"closes_at"`
}

func (s *Service) createPoll(w http.ResponseWriter, r *http.Request) {
	uid, _ := httpx.UserID(r.Context())
	leagueID := chi.URLParam(r, "leagueID")
	var req createPollReq
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err)
		return
	}
	opts, _ := json.Marshal(req.Options)
	var id string
	err := s.pool.QueryRow(r.Context(), `
		INSERT INTO polls (league_id, created_by, question, options, closes_at)
		VALUES ($1, $2, $3, $4::jsonb, $5) RETURNING id`,
		leagueID, uid, req.Question, string(opts), req.ClosesAt).Scan(&id)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"id": id})
}

type votePollReq struct {
	OptionID string `json:"option_id"`
}

func (s *Service) votePoll(w http.ResponseWriter, r *http.Request) {
	uid, _ := httpx.UserID(r.Context())
	id := chi.URLParam(r, "pollID")
	var req votePollReq
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err)
		return
	}
	_, err := s.pool.Exec(r.Context(), `
		INSERT INTO poll_votes (poll_id, user_id, option_id) VALUES ($1, $2, $3)
		ON CONFLICT (poll_id, user_id) DO UPDATE SET option_id = EXCLUDED.option_id, voted_at = now()`,
		id, uid, req.OptionID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}
