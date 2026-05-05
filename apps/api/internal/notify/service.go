package notify

import (
	"context"
	"net/http"

	"github.com/bulbousoars/lunarleague/apps/api/internal/db"
	"github.com/bulbousoars/lunarleague/apps/api/internal/httpx"
	"github.com/go-chi/chi/v5"
)

type Service struct {
	pool   *db.DB
	mailer Mailer
}

func NewService(pool *db.DB, mailer Mailer) *Service {
	return &Service{pool: pool, mailer: mailer}
}

func (s *Service) Mount(r chi.Router) {
	r.Get("/notifications", s.list)
	r.Post("/notifications/{id}/read", s.markRead)
	r.Post("/push/subscribe", s.subscribe)
	r.Post("/push/unsubscribe", s.unsubscribe)
}

type notification struct {
	ID         string  `json:"id"`
	Type       string  `json:"type"`
	Title      string  `json:"title"`
	Body       *string `json:"body,omitempty"`
	DeepLink   *string `json:"deep_link,omitempty"`
	LeagueID   *string `json:"league_id,omitempty"`
	ReadAt     *string `json:"read_at,omitempty"`
	CreatedAt  string  `json:"created_at"`
}

func (s *Service) list(w http.ResponseWriter, r *http.Request) {
	uid, err := httpx.UserID(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, err)
		return
	}
	rows, err := s.pool.Query(r.Context(), `
		SELECT id, type, title, body, deep_link, league_id, read_at, created_at
		FROM notifications
		WHERE user_id = $1
		ORDER BY created_at DESC LIMIT 100`, uid)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()
	var out []notification
	for rows.Next() {
		var n notification
		var leagueID *string
		var readAt *string
		var created string
		if err := rows.Scan(&n.ID, &n.Type, &n.Title, &n.Body, &n.DeepLink, &leagueID, &readAt, &created); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, err)
			return
		}
		n.LeagueID = leagueID
		n.ReadAt = readAt
		n.CreatedAt = created
		out = append(out, n)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"notifications": out})
}

func (s *Service) markRead(w http.ResponseWriter, r *http.Request) {
	uid, err := httpx.UserID(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, err)
		return
	}
	id := chi.URLParam(r, "id")
	_, err = s.pool.Exec(r.Context(),
		`UPDATE notifications SET read_at = now() WHERE id = $1 AND user_id = $2`, id, uid)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

type subscribeRequest struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

func (s *Service) subscribe(w http.ResponseWriter, r *http.Request) {
	uid, err := httpx.UserID(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, err)
		return
	}
	var req subscribeRequest
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err)
		return
	}
	_, err = s.pool.Exec(r.Context(), `
		INSERT INTO push_subscriptions (user_id, endpoint, p256dh, auth)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (endpoint) DO UPDATE SET user_id = EXCLUDED.user_id`,
		uid, req.Endpoint, req.Keys.P256dh, req.Keys.Auth)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"ok": true})
}

func (s *Service) unsubscribe(w http.ResponseWriter, r *http.Request) {
	uid, err := httpx.UserID(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, err)
		return
	}
	var req struct {
		Endpoint string `json:"endpoint"`
	}
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err)
		return
	}
	_, err = s.pool.Exec(r.Context(),
		`DELETE FROM push_subscriptions WHERE user_id = $1 AND endpoint = $2`,
		uid, req.Endpoint)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// Insert is a helper for other packages to enqueue a notification.
func Insert(ctx context.Context, pool *db.DB, userID string, leagueID *string, kind, title, body, deepLink string) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO notifications (user_id, league_id, type, title, body, deep_link)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		userID, leagueID, kind, title, nilStr(body), nilStr(deepLink))
	return err
}

func nilStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
