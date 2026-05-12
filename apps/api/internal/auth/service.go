// Package auth implements passwordless magic-link authentication.
//
// Flow:
//   1. POST /v1/auth/magic-link {email}
//      - Insert magic_links row with hash(token) and 15-min expiry
//      - Mail user a link to PUBLIC_WEB_URL/auth/callback?token=...
//   2. POST /v1/auth/verify {token}
//      - Hash, find row, mark consumed, ensure user exists, mint session
//      - Return session token (set as cookie + JSON)
//   3. Subsequent requests carry Authorization: Bearer <session_token>
//      OR a cookie called "lunarleague_session"
//
// Tokens are 32 random bytes, base64url-encoded. Stored as SHA-256 hashes so a
// DB leak doesn't grant login.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/bulbousoars/lunarleague/apps/api/internal/db"
	"github.com/bulbousoars/lunarleague/apps/api/internal/httpx"
	"github.com/bulbousoars/lunarleague/apps/api/internal/notify"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

const (
	magicLinkTTL = 15 * time.Minute
	sessionTTL   = 30 * 24 * time.Hour
	cookieName   = "lunarleague_session"
)

type Service struct {
	pool           *db.DB
	mailer         notify.Mailer
	publicURL      string
	secret         []byte
	cookieSecure   bool
}

func NewService(pool *db.DB, mailer notify.Mailer, publicURL string, secret []byte) *Service {
	pub := strings.TrimSpace(publicURL)
	cookieSecure := strings.HasPrefix(strings.ToLower(pub), "https://")
	return &Service{
		pool: pool, mailer: mailer, publicURL: publicURL, secret: secret,
		cookieSecure: cookieSecure,
	}
}

func (s *Service) Mount(r chi.Router) {
	r.Post("/magic-link", s.requestMagicLink)
	r.Post("/verify", s.verify)
	r.Post("/logout", s.logout)
}

type magicLinkRequest struct {
	Email string `json:"email"`
}

func (s *Service) requestMagicLink(w http.ResponseWriter, r *http.Request) {
	var req magicLinkRequest
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err)
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if !looksLikeEmail(email) {
		httpx.WriteError(w, http.StatusBadRequest, errors.New("valid email required"))
		return
	}

	token, hash, err := newToken()
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	expiresAt := time.Now().Add(magicLinkTTL)
	_, err = s.pool.Exec(r.Context(),
		`INSERT INTO magic_links (email, token_hash, expires_at) VALUES ($1, $2, $3)`,
		email, hash, expiresAt)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}

	link := fmt.Sprintf("%s/auth/callback?token=%s", strings.TrimRight(s.publicURL, "/"), token)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.mailer.SendMagicLink(ctx, email, link); err != nil {
			slog.Error("magic link email delivery failed", "err", err)
		}
	}()

	// Always 202 to prevent email enumeration.
	httpx.WriteJSON(w, http.StatusAccepted, map[string]any{
		"message": "If that email exists or is allowed to sign up, a sign-in link is on its way.",
	})
}

type verifyRequest struct {
	Token       string `json:"token"`
	DisplayName string `json:"display_name,omitempty"`
}

type verifyResponse struct {
	SessionToken string `json:"session_token"`
	User         User   `json:"user"`
}

type User struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url,omitempty"`
	Timezone    string `json:"timezone"`
	IsAdmin     bool   `json:"is_admin"`
}

func (s *Service) verify(w http.ResponseWriter, r *http.Request) {
	var req verifyRequest
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err)
		return
	}
	if req.Token == "" {
		httpx.WriteError(w, http.StatusBadRequest, errors.New("token required"))
		return
	}

	hash := hashToken(req.Token)

	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	defer tx.Rollback(r.Context())

	var (
		mlID      string
		email     string
		expiresAt time.Time
		consumed  *time.Time
	)
	err = tx.QueryRow(r.Context(),
		`SELECT id, email, expires_at, consumed_at FROM magic_links WHERE token_hash = $1`, hash).
		Scan(&mlID, &email, &expiresAt, &consumed)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, http.StatusUnauthorized, errors.New("invalid or expired link"))
		return
	}
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	if consumed != nil {
		httpx.WriteError(w, http.StatusUnauthorized, errors.New("link already used"))
		return
	}
	if time.Now().After(expiresAt) {
		httpx.WriteError(w, http.StatusUnauthorized, errors.New("link expired"))
		return
	}

	if _, err := tx.Exec(r.Context(),
		`UPDATE magic_links SET consumed_at = now() WHERE id = $1`, mlID); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}

	// Upsert user.
	var (
		userID      string
		displayName string
		avatar      *string
		tz          string
		isAdmin     bool
	)
	dn := strings.TrimSpace(req.DisplayName)
	if dn == "" {
		dn = defaultDisplayName(email)
	}
	err = tx.QueryRow(r.Context(),
		`INSERT INTO users (email, display_name)
		 VALUES ($1, $2)
		 ON CONFLICT (email) DO UPDATE SET updated_at = now()
		 RETURNING id, display_name, avatar_url, timezone, is_admin`,
		email, dn).Scan(&userID, &displayName, &avatar, &tz, &isAdmin)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}

	// Mint session.
	sessTok, sessHash, err := newToken()
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	if _, err := tx.Exec(r.Context(),
		`INSERT INTO sessions (user_id, token_hash, user_agent, ip, expires_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		userID, sessHash, r.UserAgent(), nilIfEmpty(r.RemoteAddr), time.Now().Add(sessionTTL)); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}

	avURL := ""
	if avatar != nil {
		avURL = *avatar
	}
	s.setSessionCookie(w, sessTok)
	httpx.WriteJSON(w, http.StatusOK, verifyResponse{
		SessionToken: sessTok,
		User: User{
			ID: userID, Email: email, DisplayName: displayName,
			AvatarURL: avURL, Timezone: tz, IsAdmin: isAdmin,
		},
	})
}

func (s *Service) logout(w http.ResponseWriter, r *http.Request) {
	tok := readSessionToken(r)
	if tok != "" {
		_, _ = s.pool.Exec(r.Context(),
			`UPDATE sessions SET revoked_at = now() WHERE token_hash = $1 AND revoked_at IS NULL`,
			hashToken(tok))
	}
	s.clearSessionCookie(w)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// MeHandler returns the current authenticated user.
func (s *Service) MeHandler(w http.ResponseWriter, r *http.Request) {
	uid, err := httpx.UserID(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, err)
		return
	}
	u, err := s.LoadUser(r.Context(), uid)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, u)
}

type updateMeRequest struct {
	DisplayName *string `json:"display_name,omitempty"`
	AvatarURL   *string `json:"avatar_url,omitempty"`
	Timezone    *string `json:"timezone,omitempty"`
}

func (s *Service) UpdateMeHandler(w http.ResponseWriter, r *http.Request) {
	uid, err := httpx.UserID(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, err)
		return
	}
	var req updateMeRequest
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err)
		return
	}
	_, err = s.pool.Exec(r.Context(),
		`UPDATE users SET
			display_name = COALESCE($2, display_name),
			avatar_url   = COALESCE($3, avatar_url),
			timezone     = COALESCE($4, timezone),
			updated_at   = now()
		 WHERE id = $1`,
		uid, req.DisplayName, req.AvatarURL, req.Timezone)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	u, _ := s.LoadUser(r.Context(), uid)
	httpx.WriteJSON(w, http.StatusOK, u)
}

func (s *Service) LoadUser(ctx context.Context, uid string) (User, error) {
	var u User
	var avatar *string
	err := s.pool.QueryRow(ctx,
		`SELECT id, email, display_name, avatar_url, timezone, is_admin FROM users WHERE id = $1`, uid).
		Scan(&u.ID, &u.Email, &u.DisplayName, &avatar, &u.Timezone, &u.IsAdmin)
	if err != nil {
		return u, err
	}
	if avatar != nil {
		u.AvatarURL = *avatar
	}
	return u, nil
}

// Middleware extracts the session token from cookie or Authorization header
// and injects user id into the context. Returns 401 if missing/invalid.
func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok := readSessionToken(r)
		if tok == "" {
			httpx.WriteError(w, http.StatusUnauthorized, errors.New("no session"))
			return
		}
		uid, err := s.userIDFromToken(r.Context(), tok)
		if err != nil {
			httpx.WriteError(w, http.StatusUnauthorized, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(httpx.WithUserID(r.Context(), uid)))
	})
}

// RequireAdmin runs after Middleware and returns 403 unless the user is an admin.
func (s *Service) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, err := httpx.UserID(r.Context())
		if err != nil {
			httpx.WriteError(w, http.StatusUnauthorized, err)
			return
		}
		u, err := s.LoadUser(r.Context(), uid)
		if err != nil {
			httpx.WriteError(w, http.StatusForbidden, errors.New("user not found"))
			return
		}
		if !u.IsAdmin {
			httpx.WriteError(w, http.StatusForbidden, errors.New("admin only"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// UserIDFromRequest is exposed so the WS handler can authenticate connection
// upgrades (where middleware composition is awkward).
func (s *Service) UserIDFromRequest(r *http.Request) (string, error) {
	tok := readSessionToken(r)
	if tok == "" {
		return "", errors.New("no session")
	}
	return s.userIDFromToken(r.Context(), tok)
}

func (s *Service) userIDFromToken(ctx context.Context, tok string) (string, error) {
	hash := hashToken(tok)
	var (
		userID    string
		expiresAt time.Time
		revokedAt *time.Time
	)
	err := s.pool.QueryRow(ctx,
		`SELECT user_id, expires_at, revoked_at FROM sessions WHERE token_hash = $1`,
		hash).Scan(&userID, &expiresAt, &revokedAt)
	if err != nil {
		return "", errors.New("invalid session")
	}
	if revokedAt != nil {
		return "", errors.New("session revoked")
	}
	if time.Now().After(expiresAt) {
		return "", errors.New("session expired")
	}
	return userID, nil
}

// --- helpers ---

func newToken() (token string, hash []byte, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}
	tok := base64.RawURLEncoding.EncodeToString(b)
	h := sha256.Sum256([]byte(tok))
	return tok, h[:], nil
}

func hashToken(tok string) []byte {
	h := sha256.Sum256([]byte(tok))
	return h[:]
}

func looksLikeEmail(s string) bool {
	at := strings.IndexByte(s, '@')
	return at > 0 && at < len(s)-3 && strings.Contains(s[at+1:], ".")
}

func defaultDisplayName(email string) string {
	at := strings.IndexByte(email, '@')
	if at <= 0 {
		return email
	}
	return email[:at]
}

// nilIfEmpty returns a clean IPv4 host stripped of port, or nil. We
// intentionally drop IPv6/literal addresses to avoid inet parse errors —
// the IP column is purely informational and nullable.
func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	// Drop bracketed IPv6 forms.
	if strings.ContainsAny(s, "[]") {
		return nil
	}
	if i := strings.LastIndex(s, ":"); i > 0 {
		s = s[:i]
	}
	// Sanity: must be IPv4-ish (digits + dots).
	for _, r := range s {
		if (r < '0' || r > '9') && r != '.' {
			return nil
		}
	}
	if s == "" {
		return nil
	}
	return s
}

func (s *Service) setSessionCookie(w http.ResponseWriter, tok string) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    tok,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionTTL.Seconds()),
	})
}

func (s *Service) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func readSessionToken(r *http.Request) string {
	if c, err := r.Cookie(cookieName); err == nil && c.Value != "" {
		return c.Value
	}
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	return ""
}
