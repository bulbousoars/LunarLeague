// Package league owns league CRUD, settings, scoring rules, and member
// management. It's intentionally the *largest* domain package because almost
// every other service flows through a league.
package league

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/bulbousoars/lunarleague/apps/api/internal/db"
	"github.com/bulbousoars/lunarleague/apps/api/internal/httpx"
	"github.com/bulbousoars/lunarleague/apps/api/internal/notify"
	"github.com/bulbousoars/lunarleague/apps/api/internal/player"
	"github.com/bulbousoars/lunarleague/apps/api/internal/provider"
	"github.com/bulbousoars/lunarleague/apps/api/internal/scoring"
	"github.com/bulbousoars/lunarleague/apps/api/internal/sport"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

type Service struct {
	pool      *db.DB
	mailer    notify.Mailer
	publicURL string
	dp        provider.DataProvider
	players   *player.Service
}

func NewService(pool *db.DB, mailer notify.Mailer, publicURL string, dp provider.DataProvider, players *player.Service) *Service {
	pub := strings.TrimSpace(publicURL)
	return &Service{pool: pool, mailer: mailer, publicURL: pub, dp: dp, players: players}
}

func (s *Service) Mount(r chi.Router) {
	r.Get("/leagues", s.list)
	r.Post("/leagues", s.create)
	r.Get("/leagues/{leagueID}", s.get)
	r.Patch("/leagues/{leagueID}", s.update)

	r.Get("/leagues/{leagueID}/settings", s.getSettings)
	r.Patch("/leagues/{leagueID}/settings", s.updateSettings)

	r.Get("/leagues/{leagueID}/scoring", s.getScoring)
	r.Patch("/leagues/{leagueID}/scoring", s.updateScoring)

	r.Get("/leagues/{leagueID}/members", s.listMembers)
	r.Post("/leagues/{leagueID}/join", s.join)
	r.Post("/leagues/{leagueID}/seed-sports", s.seedSports)

	r.Get("/leagues/{leagueID}/teams", s.listTeams)
	r.Post("/leagues/{leagueID}/teams", s.createTeam)
	r.Patch("/leagues/{leagueID}/teams/{teamID}", s.updateTeam)
	r.Post("/leagues/{leagueID}/teams/{teamID}/claim", s.claimTeam)
}

// --- Models ---

type League struct {
	ID           string `json:"id"`
	SportID      int    `json:"sport_id"`
	SportCode    string `json:"sport_code"`
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	Season       int    `json:"season"`
	LeagueFormat string `json:"league_format"`
	DraftFormat  string `json:"draft_format"`
	TeamCount    int    `json:"team_count"`
	InviteCode   string `json:"invite_code,omitempty"`
	Status       string `json:"status"`
	CreatedBy    string `json:"created_by"`
}

type Settings struct {
	RosterSlots       map[string]int  `json:"roster_slots"`
	WaiverType        string          `json:"waiver_type"`
	WaiverBudget      int             `json:"waiver_budget"`
	WaiverRunDOW      int             `json:"waiver_run_dow"`
	WaiverRunHour     int             `json:"waiver_run_hour"`
	TradeDeadlineWeek *int            `json:"trade_deadline_week"`
	PlayoffStartWeek  int             `json:"playoff_start_week"`
	PlayoffTeamCount  int             `json:"playoff_team_count"`
	KeeperCount       int             `json:"keeper_count"`
	AuctionBudget     *int            `json:"auction_budget,omitempty"`
	ScheduleType      string          `json:"schedule_type"`
	PublicVisible     bool            `json:"public_visible"`
}

type Team struct {
	ID            string  `json:"id"`
	LeagueID      string  `json:"league_id"`
	OwnerID       *string `json:"owner_id"`
	Name          string  `json:"name"`
	Abbreviation  string  `json:"abbreviation"`
	LogoURL       *string `json:"logo_url,omitempty"`
	Motto         *string `json:"motto,omitempty"`
	WaiverPosition *int   `json:"waiver_position,omitempty"`
	WaiverBudget   *int   `json:"waiver_budget,omitempty"`
	AuctionBudget  *int   `json:"auction_budget,omitempty"`
	RecordWins     int    `json:"record_wins"`
	RecordLosses   int    `json:"record_losses"`
	RecordTies     int    `json:"record_ties"`
	PointsFor      string `json:"points_for"`
	PointsAgainst  string `json:"points_against"`
}

// --- Handlers ---

type createReq struct {
	Name         string `json:"name"`
	SportCode    string `json:"sport_code"`
	Season       int    `json:"season"`
	LeagueFormat string `json:"league_format"`
	DraftFormat  string `json:"draft_format"`
	TeamCount    int    `json:"team_count"`
}

func (s *Service) create(w http.ResponseWriter, r *http.Request) {
	uid, err := httpx.UserID(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, err)
		return
	}
	var req createReq
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err)
		return
	}
	if err := validateCreate(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err)
		return
	}

	var sportID int
	err = s.pool.QueryRow(r.Context(),
		`SELECT id FROM sports WHERE code = $1`, req.SportCode).Scan(&sportID)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, http.StatusBadRequest, fmt.Errorf("unknown sport %q", req.SportCode))
		return
	}
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}

	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	defer tx.Rollback(r.Context())

	slug := slugify(req.Name)
	invite := newInviteCode()

	var league League
	err = tx.QueryRow(r.Context(), `
		INSERT INTO leagues (sport_id, name, slug, season, league_format, draft_format, team_count, invite_code, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, sport_id, name, slug, season, league_format, draft_format, team_count, invite_code, status, created_by`,
		sportID, req.Name, slug, req.Season, req.LeagueFormat, req.DraftFormat, req.TeamCount, invite, uid).
		Scan(&league.ID, &league.SportID, &league.Name, &league.Slug, &league.Season,
			&league.LeagueFormat, &league.DraftFormat, &league.TeamCount, &league.InviteCode,
			&league.Status, &league.CreatedBy)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	league.SportCode = req.SportCode

	// Default settings.
	defaultSlots := defaultRosterSlots(req.SportCode, req.LeagueFormat)
	slotsJSON, _ := json.Marshal(defaultSlots)
	auctionBudget := (*int)(nil)
	if req.DraftFormat == "auction" {
		v := 200
		auctionBudget = &v
	}
	_, err = tx.Exec(r.Context(), `
		INSERT INTO league_settings (league_id, roster_slots, auction_budget)
		VALUES ($1, $2::jsonb, $3)`,
		league.ID, string(slotsJSON), auctionBudget)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}

	// Default scoring rules.
	rules := scoring.DefaultRules(req.SportCode)
	rulesJSON, _ := json.Marshal(rules)
	if _, err := tx.Exec(r.Context(),
		`INSERT INTO scoring_rules (league_id, rules) VALUES ($1, $2::jsonb)`,
		league.ID, string(rulesJSON)); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}

	// Add commissioner membership + an unclaimed team for them to take.
	if _, err := tx.Exec(r.Context(),
		`INSERT INTO league_members (league_id, user_id, role) VALUES ($1, $2, 'commissioner')`,
		league.ID, uid); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}

	for i := 1; i <= req.TeamCount; i++ {
		name := fmt.Sprintf("Team %d", i)
		abbr := fmt.Sprintf("T%d", i)
		ownerID := any(nil)
		if i == 1 {
			ownerID = uid
		}
		_, err := tx.Exec(r.Context(), `
			INSERT INTO teams (league_id, owner_id, name, abbreviation, waiver_position, waiver_budget, auction_budget)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			league.ID, ownerID, name, abbr, i, 100, auctionBudget)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, err)
			return
		}
	}

	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}

	var commissionerEmail string
	if err := s.pool.QueryRow(r.Context(),
		`SELECT email FROM users WHERE id = $1`, uid).Scan(&commissionerEmail); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		slog.Error("load commissioner email after league create", "err", err)
	}
	if commissionerEmail != "" && s.mailer != nil {
		base := strings.TrimRight(s.publicURL, "/")
		setupURL := fmt.Sprintf("%s/leagues/%s/setup", base, league.ID)
		inviteURL := fmt.Sprintf("%s/leagues/%s/join?code=%s", base, league.ID, league.InviteCode)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := s.mailer.SendLeagueCreated(ctx, commissionerEmail, league.Name, setupURL, inviteURL); err != nil {
				slog.Error("league created email delivery failed", "err", err)
			}
		}()
	}

	httpx.WriteJSON(w, http.StatusCreated, league)
}

func (s *Service) list(w http.ResponseWriter, r *http.Request) {
	uid, err := httpx.UserID(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, err)
		return
	}
	rows, err := s.pool.Query(r.Context(), `
		SELECT l.id, l.sport_id, sp.code, l.name, l.slug, l.season, l.league_format,
		       l.draft_format, l.team_count, l.status, l.created_by
		FROM leagues l
		JOIN sports sp ON sp.id = l.sport_id
		JOIN league_members m ON m.league_id = l.id
		WHERE m.user_id = $1
		ORDER BY l.created_at DESC`, uid)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()
	leagues := []League{}
	for rows.Next() {
		var l League
		if err := rows.Scan(&l.ID, &l.SportID, &l.SportCode, &l.Name, &l.Slug, &l.Season,
			&l.LeagueFormat, &l.DraftFormat, &l.TeamCount, &l.Status, &l.CreatedBy); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, err)
			return
		}
		leagues = append(leagues, l)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"leagues": leagues})
}

func (s *Service) get(w http.ResponseWriter, r *http.Request) {
	uid, err := httpx.UserID(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, err)
		return
	}
	id := chi.URLParam(r, "leagueID")
	if !s.canSee(r.Context(), id, uid) {
		httpx.WriteError(w, http.StatusForbidden, errors.New("not a member"))
		return
	}
	var l League
	err = s.pool.QueryRow(r.Context(), `
		SELECT l.id, l.sport_id, sp.code, l.name, l.slug, l.season, l.league_format,
		       l.draft_format, l.team_count, l.invite_code, l.status, l.created_by
		FROM leagues l JOIN sports sp ON sp.id = l.sport_id
		WHERE l.id = $1`, id).
		Scan(&l.ID, &l.SportID, &l.SportCode, &l.Name, &l.Slug, &l.Season,
			&l.LeagueFormat, &l.DraftFormat, &l.TeamCount, &l.InviteCode, &l.Status, &l.CreatedBy)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, l)
}

type updateLeagueReq struct {
	Name   *string `json:"name,omitempty"`
	Status *string `json:"status,omitempty"`
}

func (s *Service) update(w http.ResponseWriter, r *http.Request) {
	uid, _ := httpx.UserID(r.Context())
	id := chi.URLParam(r, "leagueID")
	if !s.isCommish(r.Context(), id, uid) {
		httpx.WriteError(w, http.StatusForbidden, errors.New("commissioner only"))
		return
	}
	var req updateLeagueReq
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err)
		return
	}
	_, err := s.pool.Exec(r.Context(), `
		UPDATE leagues SET name = COALESCE($2,name), status = COALESCE($3,status), updated_at = now()
		WHERE id = $1`, id, req.Name, req.Status)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Service) getSettings(w http.ResponseWriter, r *http.Request) {
	uid, _ := httpx.UserID(r.Context())
	id := chi.URLParam(r, "leagueID")
	if !s.canSee(r.Context(), id, uid) {
		httpx.WriteError(w, http.StatusForbidden, errors.New("not a member"))
		return
	}
	var (
		set  Settings
		raw  []byte
		auct *int
		td   *int
	)
	err := s.pool.QueryRow(r.Context(), `
		SELECT roster_slots, waiver_type, waiver_budget, waiver_run_dow, waiver_run_hour,
		       trade_deadline_week, playoff_start_week, playoff_team_count, keeper_count,
		       auction_budget, schedule_type, public_visible
		FROM league_settings WHERE league_id = $1`, id).
		Scan(&raw, &set.WaiverType, &set.WaiverBudget, &set.WaiverRunDOW, &set.WaiverRunHour,
			&td, &set.PlayoffStartWeek, &set.PlayoffTeamCount, &set.KeeperCount,
			&auct, &set.ScheduleType, &set.PublicVisible)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, err)
		return
	}
	_ = json.Unmarshal(raw, &set.RosterSlots)
	set.AuctionBudget = auct
	set.TradeDeadlineWeek = td
	httpx.WriteJSON(w, http.StatusOK, set)
}

func (s *Service) updateSettings(w http.ResponseWriter, r *http.Request) {
	uid, _ := httpx.UserID(r.Context())
	id := chi.URLParam(r, "leagueID")
	if !s.isCommish(r.Context(), id, uid) {
		httpx.WriteError(w, http.StatusForbidden, errors.New("commissioner only"))
		return
	}
	var set Settings
	if err := httpx.ReadJSON(r, &set); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err)
		return
	}
	rosterJSON, _ := json.Marshal(set.RosterSlots)
	_, err := s.pool.Exec(r.Context(), `
		UPDATE league_settings SET
			roster_slots = $2::jsonb,
			waiver_type = $3, waiver_budget = $4,
			waiver_run_dow = $5, waiver_run_hour = $6,
			trade_deadline_week = $7,
			playoff_start_week = $8, playoff_team_count = $9,
			keeper_count = $10, auction_budget = $11,
			schedule_type = $12, public_visible = $13,
			updated_at = now()
		WHERE league_id = $1`,
		id, string(rosterJSON), set.WaiverType, set.WaiverBudget,
		set.WaiverRunDOW, set.WaiverRunHour, set.TradeDeadlineWeek,
		set.PlayoffStartWeek, set.PlayoffTeamCount, set.KeeperCount,
		set.AuctionBudget, set.ScheduleType, set.PublicVisible)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Service) getScoring(w http.ResponseWriter, r *http.Request) {
	uid, _ := httpx.UserID(r.Context())
	id := chi.URLParam(r, "leagueID")
	if !s.canSee(r.Context(), id, uid) {
		httpx.WriteError(w, http.StatusForbidden, errors.New("not a member"))
		return
	}
	var raw []byte
	err := s.pool.QueryRow(r.Context(),
		`SELECT rules FROM scoring_rules WHERE league_id = $1`, id).Scan(&raw)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, err)
		return
	}
	var rules map[string]any
	_ = json.Unmarshal(raw, &rules)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"rules": rules})
}

func (s *Service) updateScoring(w http.ResponseWriter, r *http.Request) {
	uid, _ := httpx.UserID(r.Context())
	id := chi.URLParam(r, "leagueID")
	if !s.isCommish(r.Context(), id, uid) {
		httpx.WriteError(w, http.StatusForbidden, errors.New("commissioner only"))
		return
	}
	var req struct {
		Rules map[string]any `json:"rules"`
	}
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err)
		return
	}
	body, _ := json.Marshal(req.Rules)
	_, err := s.pool.Exec(r.Context(), `
		UPDATE scoring_rules SET rules = $2::jsonb, updated_at = now()
		WHERE league_id = $1`, id, string(body))
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Service) listMembers(w http.ResponseWriter, r *http.Request) {
	uid, _ := httpx.UserID(r.Context())
	id := chi.URLParam(r, "leagueID")
	if !s.canSee(r.Context(), id, uid) {
		httpx.WriteError(w, http.StatusForbidden, errors.New("not a member"))
		return
	}
	rows, err := s.pool.Query(r.Context(), `
		SELECT u.id, u.email, u.display_name, u.avatar_url, m.role, m.joined_at
		FROM league_members m JOIN users u ON u.id = m.user_id
		WHERE m.league_id = $1`, id)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()
	type row struct {
		ID, Email, DisplayName, Role string
		AvatarURL                    *string
		JoinedAt                     string
	}
	out := []map[string]any{}
	for rows.Next() {
		var rr row
		if err := rows.Scan(&rr.ID, &rr.Email, &rr.DisplayName, &rr.AvatarURL, &rr.Role, &rr.JoinedAt); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, err)
			return
		}
		out = append(out, map[string]any{
			"id": rr.ID, "email": rr.Email, "display_name": rr.DisplayName,
			"avatar_url": rr.AvatarURL, "role": rr.Role, "joined_at": rr.JoinedAt,
		})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"members": out})
}

type joinReq struct {
	InviteCode string `json:"invite_code"`
}

func (s *Service) join(w http.ResponseWriter, r *http.Request) {
	uid, _ := httpx.UserID(r.Context())
	id := chi.URLParam(r, "leagueID")
	var req joinReq
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err)
		return
	}
	var stored string
	err := s.pool.QueryRow(r.Context(),
		`SELECT invite_code FROM leagues WHERE id = $1`, id).Scan(&stored)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, err)
		return
	}
	if !strings.EqualFold(stored, req.InviteCode) {
		httpx.WriteError(w, http.StatusForbidden, errors.New("invalid invite"))
		return
	}
	_, err = s.pool.Exec(r.Context(),
		`INSERT INTO league_members (league_id, user_id) VALUES ($1, $2)
		 ON CONFLICT DO NOTHING`, id, uid)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Service) listTeams(w http.ResponseWriter, r *http.Request) {
	uid, _ := httpx.UserID(r.Context())
	id := chi.URLParam(r, "leagueID")
	if !s.canSee(r.Context(), id, uid) {
		httpx.WriteError(w, http.StatusForbidden, errors.New("not a member"))
		return
	}
	rows, err := s.pool.Query(r.Context(), `
		SELECT id, league_id, owner_id, name, abbreviation, logo_url, motto,
		       waiver_position, waiver_budget, auction_budget,
		       record_wins, record_losses, record_ties,
		       points_for::text, points_against::text
		FROM teams WHERE league_id = $1
		ORDER BY abbreviation`, id)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()
	teams := []Team{}
	for rows.Next() {
		var t Team
		if err := rows.Scan(&t.ID, &t.LeagueID, &t.OwnerID, &t.Name, &t.Abbreviation,
			&t.LogoURL, &t.Motto, &t.WaiverPosition, &t.WaiverBudget, &t.AuctionBudget,
			&t.RecordWins, &t.RecordLosses, &t.RecordTies, &t.PointsFor, &t.PointsAgainst); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, err)
			return
		}
		teams = append(teams, t)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"teams": teams})
}

func (s *Service) createTeam(w http.ResponseWriter, r *http.Request) {
	httpx.WriteError(w, http.StatusNotImplemented, errors.New("teams are auto-created at league creation"))
}

type updateTeamReq struct {
	Name         *string `json:"name,omitempty"`
	Abbreviation *string `json:"abbreviation,omitempty"`
	LogoURL      *string `json:"logo_url,omitempty"`
	Motto        *string `json:"motto,omitempty"`
}

func (s *Service) updateTeam(w http.ResponseWriter, r *http.Request) {
	uid, _ := httpx.UserID(r.Context())
	leagueID := chi.URLParam(r, "leagueID")
	teamID := chi.URLParam(r, "teamID")

	// Either the team owner or a commissioner can edit.
	var ownerID *string
	err := s.pool.QueryRow(r.Context(),
		`SELECT owner_id FROM teams WHERE id = $1 AND league_id = $2`, teamID, leagueID).
		Scan(&ownerID)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, err)
		return
	}
	if !((ownerID != nil && *ownerID == uid) || s.isCommish(r.Context(), leagueID, uid)) {
		httpx.WriteError(w, http.StatusForbidden, errors.New("not allowed"))
		return
	}

	var req updateTeamReq
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err)
		return
	}
	_, err = s.pool.Exec(r.Context(), `
		UPDATE teams SET
			name = COALESCE($3, name),
			abbreviation = COALESCE($4, abbreviation),
			logo_url = COALESCE($5, logo_url),
			motto = COALESCE($6, motto),
			updated_at = now()
		WHERE id = $1 AND league_id = $2`,
		teamID, leagueID, req.Name, req.Abbreviation, req.LogoURL, req.Motto)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Service) claimTeam(w http.ResponseWriter, r *http.Request) {
	uid, _ := httpx.UserID(r.Context())
	leagueID := chi.URLParam(r, "leagueID")
	teamID := chi.URLParam(r, "teamID")
	if !s.canSee(r.Context(), leagueID, uid) {
		httpx.WriteError(w, http.StatusForbidden, errors.New("not a member"))
		return
	}
	res, err := s.pool.Exec(r.Context(), `
		UPDATE teams SET owner_id = $1, updated_at = now()
		WHERE id = $2 AND league_id = $3 AND owner_id IS NULL`,
		uid, teamID, leagueID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	if res.RowsAffected() == 0 {
		httpx.WriteError(w, http.StatusConflict, errors.New("team already claimed"))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// seedSports runs sport.Seed (same as `lunarleague seed`). Allowed for site
// admins or the league commissioner — typical self-host setup never sets is_admin.
func (s *Service) seedSports(w http.ResponseWriter, r *http.Request) {
	uid, err := httpx.UserID(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, err)
		return
	}
	leagueID := chi.URLParam(r, "leagueID")
	if !s.canSee(r.Context(), leagueID, uid) {
		httpx.WriteError(w, http.StatusForbidden, errors.New("not a member"))
		return
	}
	if !s.isCommish(r.Context(), leagueID, uid) && !s.isAdmin(r.Context(), uid) {
		httpx.WriteError(w, http.StatusForbidden, errors.New("commissioner or admin only"))
		return
	}
	if err := sport.Seed(r.Context(), s.pool); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// SyncLeaguePlayers runs a one-off player universe sync for this league's sport
// (NFL, NBA, or MLB). Same permission model as seedSports. Registered from the
// HTTP router with an extended request timeout because provider pulls can be slow.
func (s *Service) SyncLeaguePlayers(w http.ResponseWriter, r *http.Request) {
	uid, err := httpx.UserID(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, err)
		return
	}
	leagueID := chi.URLParam(r, "leagueID")
	if !s.canSee(r.Context(), leagueID, uid) {
		httpx.WriteError(w, http.StatusForbidden, errors.New("not a member"))
		return
	}
	if !s.isCommish(r.Context(), leagueID, uid) && !s.isAdmin(r.Context(), uid) {
		httpx.WriteError(w, http.StatusForbidden, errors.New("commissioner or admin only"))
		return
	}
	if s.dp == nil || s.players == nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, errors.New("player sync not configured"))
		return
	}
	var sportCode string
	err = s.pool.QueryRow(r.Context(), `
		SELECT lower(sp.code) FROM leagues l
		JOIN sports sp ON sp.id = l.sport_id
		WHERE l.id = $1`, leagueID).Scan(&sportCode)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, http.StatusNotFound, errors.New("league not found"))
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	n, err := s.players.SyncFromProviderForSport(r.Context(), s.dp, sportCode)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "sport_code": sportCode, "count": n})
}

func (s *Service) isAdmin(ctx context.Context, uid string) bool {
	var admin bool
	_ = s.pool.QueryRow(ctx, `SELECT is_admin FROM users WHERE id = $1`, uid).Scan(&admin)
	return admin
}

// --- helpers ---

func (s *Service) canSee(ctx context.Context, leagueID, uid string) bool {
	var n int
	_ = s.pool.QueryRow(ctx,
		`SELECT 1 FROM league_members WHERE league_id = $1 AND user_id = $2`,
		leagueID, uid).Scan(&n)
	return n == 1
}

func (s *Service) isCommish(ctx context.Context, leagueID, uid string) bool {
	var role string
	_ = s.pool.QueryRow(ctx,
		`SELECT role FROM league_members WHERE league_id = $1 AND user_id = $2`,
		leagueID, uid).Scan(&role)
	return role == "commissioner"
}

func validateCreate(req *createReq) error {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > 80 {
		return errors.New("name required (1-80 chars)")
	}
	if req.SportCode == "" {
		req.SportCode = "nfl"
	}
	if req.Season < 2024 || req.Season > 2100 {
		return errors.New("invalid season")
	}
	switch req.LeagueFormat {
	case "redraft", "keeper", "dynasty":
	case "":
		req.LeagueFormat = "redraft"
	default:
		return errors.New("invalid league_format")
	}
	switch req.DraftFormat {
	case "snake", "auction":
	case "":
		req.DraftFormat = "snake"
	default:
		return errors.New("invalid draft_format")
	}
	if req.TeamCount < 4 || req.TeamCount > 20 || req.TeamCount%2 != 0 {
		return errors.New("team_count must be even and 4..20")
	}
	return nil
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteByte('-')
		}
	}
	out := b.String()
	for strings.Contains(out, "--") {
		out = strings.ReplaceAll(out, "--", "-")
	}
	out = strings.Trim(out, "-")
	if out == "" {
		out = "league"
	}
	if len(out) > 60 {
		out = out[:60]
	}
	return out
}

func newInviteCode() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b))
}

func defaultRosterSlots(sport, format string) map[string]int {
	switch sport {
	case "nfl":
		return map[string]int{
			"QB": 1, "RB": 2, "WR": 2, "TE": 1, "FLEX": 1, "DST": 1, "K": 1, "BN": 6, "IR": 1,
		}
	case "nba":
		return map[string]int{
			"PG": 1, "SG": 1, "SF": 1, "PF": 1, "C": 1, "G": 1, "F": 1, "UTIL": 3, "BN": 3, "IR": 2,
		}
	case "mlb":
		return map[string]int{
			"C": 1, "1B": 1, "2B": 1, "3B": 1, "SS": 1, "OF": 3, "UTIL": 1,
			"SP": 2, "RP": 2, "P": 2, "BN": 5, "IL": 2,
		}
	}
	_ = format
	return map[string]int{"BN": 12}
}
