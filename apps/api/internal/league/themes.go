package league

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/bulbousoars/lunarleague/apps/api/internal/httpx"
	"github.com/bulbousoars/lunarleague/apps/api/internal/themes"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var errNotThemeBall = errors.New("league is not Theme Ball")

func requireThemeBall(scheduleType string) error {
	if scheduleType != "theme_ball" {
		return errNotThemeBall
	}
	return nil
}

func (s *Service) mountThemes(r chi.Router) {
	r.Get("/meta/theme-catalog", s.themeCatalog)
	r.Get("/leagues/{leagueID}/theme-modifiers", s.getThemeModifiers)
	r.Patch("/leagues/{leagueID}/theme-modifiers", s.patchThemeModifiers)
	r.Get("/leagues/{leagueID}/theme-votes", s.listThemeVotes)
	r.Post("/leagues/{leagueID}/theme-votes", s.createThemeVote)
	r.Post("/leagues/{leagueID}/theme-votes/{voteID}/ballot", s.castThemeBallot)
	r.Post("/leagues/{leagueID}/theme-votes/{voteID}/cancel", s.cancelThemeVote)
}

func (s *Service) themeCatalog(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"themes": themes.Catalog})
}

type themeModifiersResp struct {
	ScheduleType   string        `json:"schedule_type"`
	Modifiers      themes.Config `json:"modifiers"`
	EnabledCount   int           `json:"enabled_count"`
}

func (s *Service) getThemeModifiers(w http.ResponseWriter, r *http.Request) {
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
	st, raw, err := s.loadThemeModifiers(r.Context(), leagueID)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, err)
		return
	}
	if err := requireThemeBall(st); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err)
		return
	}
	cfg, err := themes.ParseConfig(raw)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, themeModifiersResp{
		ScheduleType: st,
		Modifiers:    cfg,
		EnabledCount: len(cfg.EnabledSlugs()),
	})
}

type patchThemeModifiersReq struct {
	Modifiers themes.Config `json:"modifiers"`
}

func (s *Service) patchThemeModifiers(w http.ResponseWriter, r *http.Request) {
	uid, err := httpx.UserID(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, err)
		return
	}
	leagueID := chi.URLParam(r, "leagueID")
	if !s.isCommish(r.Context(), leagueID, uid) {
		httpx.WriteError(w, http.StatusForbidden, errors.New("commissioner only"))
		return
	}
	st, raw, err := s.loadThemeModifiers(r.Context(), leagueID)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, err)
		return
	}
	if err := requireThemeBall(st); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err)
		return
	}
	var req patchThemeModifiersReq
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err)
		return
	}
	cfg, err := themes.ParseConfig(raw)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	before := cfg
	if err := cfg.MergePatch(req.Modifiers); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.saveThemeModifiers(r.Context(), leagueID, uid, before, cfg, "commissioner", nil); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "enabled_count": len(cfg.EnabledSlugs())})
}

func (s *Service) loadThemeModifiers(ctx context.Context, leagueID string) (scheduleType string, raw []byte, err error) {
	err = s.pool.QueryRow(ctx, `
		SELECT schedule_type, theme_modifiers
		FROM league_settings WHERE league_id = $1`, leagueID).Scan(&scheduleType, &raw)
	return scheduleType, raw, err
}

func (s *Service) saveThemeModifiers(ctx context.Context, leagueID, actorID string, before, after themes.Config, source string, voteID *string) error {
	blob, err := json.Marshal(after)
	if err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `
		UPDATE league_settings SET theme_modifiers = $2::jsonb, updated_at = now()
		WHERE league_id = $1`, leagueID, string(blob)); err != nil {
		return err
	}
	slugs := themes.SlugSet()
	for slug := range slugs {
		be, bok := before[slug]
		ae, aok := after[slug]
		if !bok || !aok || be.Enabled == ae.Enabled {
			continue
		}
		var oldPtr *bool
		if bok {
			v := be.Enabled
			oldPtr = &v
		}
		var vid any
		if voteID != nil {
			vid = *voteID
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO theme_modifier_audit (league_id, actor_id, slug, old_enabled, new_enabled, source, vote_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			leagueID, actorID, slug, oldPtr, ae.Enabled, source, vid); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// --- votes ---

type themeVote struct {
	ID        string    `json:"id"`
	Slug      string    `json:"slug"`
	Action    string    `json:"action"`
	Status    string    `json:"status"`
	OpensAt   time.Time `json:"opens_at"`
	ClosesAt  time.Time `json:"closes_at"`
	YesCount  int       `json:"yes_count"`
	NoCount   int       `json:"no_count"`
	OpenedBy  string    `json:"opened_by"`
	MyBallot  *bool     `json:"my_ballot,omitempty"`
	Eligible  int       `json:"eligible_owners"`
	NeedYes   int       `json:"need_yes_votes"`
}

func (s *Service) listThemeVotes(w http.ResponseWriter, r *http.Request) {
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
	_ = s.tallyExpiredVotes(r.Context(), leagueID)
	eligible, needYes, _ := s.voteThresholds(r.Context(), leagueID)
	rows, err := s.pool.Query(r.Context(), `
		SELECT id, slug, action, status, opens_at, closes_at, yes_count, no_count, opened_by
		FROM theme_votes WHERE league_id = $1
		ORDER BY created_at DESC LIMIT 20`, leagueID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()
	var out []themeVote
	for rows.Next() {
		var v themeVote
		if err := rows.Scan(&v.ID, &v.Slug, &v.Action, &v.Status, &v.OpensAt, &v.ClosesAt, &v.YesCount, &v.NoCount, &v.OpenedBy); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, err)
			return
		}
		v.Eligible = eligible
		v.NeedYes = needYes
		var ballot sql.NullBool
		_ = s.pool.QueryRow(r.Context(), `
			SELECT b.yes FROM theme_vote_ballots b
			JOIN teams t ON t.id = b.team_id
			WHERE b.vote_id = $1 AND t.owner_id = $2`, v.ID, uid).Scan(&ballot)
		if ballot.Valid {
			b := ballot.Bool
			v.MyBallot = &b
		}
		out = append(out, v)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"votes": out, "eligible_owners": eligible, "need_yes_votes": needYes})
}

type createThemeVoteReq struct {
	Slug   string `json:"slug"`
	Action string `json:"action"` // enable | disable
}

func (s *Service) createThemeVote(w http.ResponseWriter, r *http.Request) {
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
	st, _, err := s.loadThemeModifiers(r.Context(), leagueID)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, err)
		return
	}
	if err := requireThemeBall(st); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err)
		return
	}
	var req createThemeVoteReq
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err)
		return
	}
	if _, ok := themes.SlugSet()[req.Slug]; !ok {
		httpx.WriteError(w, http.StatusBadRequest, errors.New("unknown theme slug"))
		return
	}
	if req.Action != "enable" && req.Action != "disable" {
		httpx.WriteError(w, http.StatusBadRequest, errors.New("action must be enable or disable"))
		return
	}
	var open int
	_ = s.pool.QueryRow(r.Context(), `
		SELECT count(*)::int FROM theme_votes WHERE league_id = $1 AND status = 'open'`, leagueID).Scan(&open)
	if open > 0 {
		httpx.WriteError(w, http.StatusConflict, errors.New("an open vote already exists"))
		return
	}
	closes := time.Now().UTC().Add(7 * 24 * time.Hour)
	var id string
	err = s.pool.QueryRow(r.Context(), `
		INSERT INTO theme_votes (league_id, slug, action, opened_by, closes_at)
		VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		leagueID, req.Slug, req.Action, uid, closes).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			httpx.WriteError(w, http.StatusConflict, errors.New("an open vote already exists"))
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"id": id, "closes_at": closes})
}

type castBallotReq struct {
	Yes bool `json:"yes"`
}

func (s *Service) castThemeBallot(w http.ResponseWriter, r *http.Request) {
	uid, err := httpx.UserID(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, err)
		return
	}
	leagueID := chi.URLParam(r, "leagueID")
	voteID := chi.URLParam(r, "voteID")
	if !s.canSee(r.Context(), leagueID, uid) {
		httpx.WriteError(w, http.StatusForbidden, errors.New("not a member"))
		return
	}
	var teamID string
	err = s.pool.QueryRow(r.Context(), `
		SELECT id FROM teams WHERE league_id = $1 AND owner_id = $2`, leagueID, uid).Scan(&teamID)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, http.StatusForbidden, errors.New("claim a team to vote"))
		return
	}
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	var req castBallotReq
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err)
		return
	}
	var status string
	var closesAt time.Time
	err = s.pool.QueryRow(r.Context(), `
		SELECT status, closes_at FROM theme_votes WHERE id = $1 AND league_id = $2`,
		voteID, leagueID).Scan(&status, &closesAt)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, err)
		return
	}
	if status != "open" {
		httpx.WriteError(w, http.StatusConflict, errors.New("vote is not open"))
		return
	}
	if time.Now().UTC().After(closesAt) {
		_ = s.tallyVote(r.Context(), leagueID, voteID)
		httpx.WriteError(w, http.StatusConflict, errors.New("vote closed; refresh results"))
		return
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	defer tx.Rollback(r.Context())
	var had bool
	_ = tx.QueryRow(r.Context(), `
		SELECT true FROM theme_vote_ballots WHERE vote_id = $1 AND team_id = $2`, voteID, teamID).Scan(&had)
	if had {
		_, err = tx.Exec(r.Context(), `
			UPDATE theme_vote_ballots SET yes = $3, cast_at = now()
			WHERE vote_id = $1 AND team_id = $2`, voteID, teamID, req.Yes)
	} else {
		_, err = tx.Exec(r.Context(), `
			INSERT INTO theme_vote_ballots (vote_id, team_id, user_id, yes)
			VALUES ($1, $2, $3, $4)`, voteID, teamID, uid, req.Yes)
	}
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	var yesC, noC int
	_ = tx.QueryRow(r.Context(), `
		SELECT count(*) FILTER (WHERE yes), count(*) FILTER (WHERE NOT yes)
		FROM theme_vote_ballots WHERE vote_id = $1`, voteID).Scan(&yesC, &noC)
	_, err = tx.Exec(r.Context(), `
		UPDATE theme_votes SET yes_count = $2, no_count = $3 WHERE id = $1`,
		voteID, yesC, noC)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "yes_count": yesC, "no_count": noC})
}

func (s *Service) cancelThemeVote(w http.ResponseWriter, r *http.Request) {
	uid, err := httpx.UserID(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, err)
		return
	}
	leagueID := chi.URLParam(r, "leagueID")
	voteID := chi.URLParam(r, "voteID")
	if !s.isCommish(r.Context(), leagueID, uid) {
		httpx.WriteError(w, http.StatusForbidden, errors.New("commissioner only"))
		return
	}
	res, err := s.pool.Exec(r.Context(), `
		UPDATE theme_votes SET status = 'cancelled', closed_at = now()
		WHERE id = $1 AND league_id = $2 AND status = 'open'`, voteID, leagueID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	if res.RowsAffected() == 0 {
		httpx.WriteError(w, http.StatusNotFound, errors.New("open vote not found"))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Service) voteThresholds(ctx context.Context, leagueID string) (eligible, needYes int, err error) {
	err = s.pool.QueryRow(ctx, `
		SELECT count(*)::int FROM teams WHERE league_id = $1 AND owner_id IS NOT NULL`, leagueID).Scan(&eligible)
	if err != nil {
		return 0, 0, err
	}
	needYes = eligible/2 + 1
	if eligible == 0 {
		needYes = 1
	}
	return eligible, needYes, nil
}

func (s *Service) tallyExpiredVotes(ctx context.Context, leagueID string) error {
	rows, err := s.pool.Query(ctx, `
		SELECT id FROM theme_votes
		WHERE league_id = $1 AND status = 'open' AND closes_at <= now()`, leagueID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		_ = s.tallyVote(ctx, leagueID, id)
	}
	return rows.Err()
}

func (s *Service) tallyVote(ctx context.Context, leagueID, voteID string) error {
	var slug, action, status string
	var openedBy string
	err := s.pool.QueryRow(ctx, `
		SELECT slug, action, status, opened_by::text FROM theme_votes
		WHERE id = $1 AND league_id = $2`, voteID, leagueID).Scan(&slug, &action, &status, &openedBy)
	if err != nil || status != "open" {
		return err
	}
	_, needYes, err := s.voteThresholds(ctx, leagueID)
	if err != nil {
		return err
	}
	var yesC int
	_ = s.pool.QueryRow(ctx, `SELECT yes_count FROM theme_votes WHERE id = $1`, voteID).Scan(&yesC)
	passed := yesC >= needYes
	newStatus := "failed"
	if passed {
		newStatus = "passed"
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE theme_votes SET status = $2, closed_at = now() WHERE id = $1`,
		voteID, newStatus)
	if err != nil {
		return err
	}
	if !passed {
		return nil
	}
	_, raw, err := s.loadThemeModifiers(ctx, leagueID)
	if err != nil {
		return err
	}
	cfg, err := themes.ParseConfig(raw)
	if err != nil {
		return err
	}
	before := cfg
	enable := action == "enable"
	patch := themes.Config{slug: {Enabled: enable}}
	if err := cfg.MergePatch(patch); err != nil {
		return fmt.Errorf("apply vote: %w", err)
	}
	vid := voteID
	return s.saveThemeModifiers(ctx, leagueID, openedBy, before, cfg, "vote", &vid)
}
