// Package player owns the shared player universe + sync.
package player

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/bulbousoars/lunarleague/apps/api/internal/dataprovider"
	"github.com/bulbousoars/lunarleague/apps/api/internal/db"
	"github.com/bulbousoars/lunarleague/apps/api/internal/httpx"
	"github.com/bulbousoars/lunarleague/apps/api/internal/provider"
	"github.com/go-chi/chi/v5"
)

type Service struct {
	pool *db.DB
}

func NewService(pool *db.DB) *Service { return &Service{pool: pool} }

func (s *Service) Mount(r chi.Router) {
	r.Get("/players", s.list)
	r.Get("/players/{playerID}", s.get)
	r.Get("/players/trending", s.trending)
}

type listResp struct {
	Players []player `json:"players"`
	Total   int      `json:"total"`
}

type player struct {
	ID                string   `json:"id"`
	FullName          string   `json:"full_name"`
	Position          *string  `json:"position"`
	EligiblePositions []string `json:"eligible_positions"`
	NFLTeam           *string  `json:"nfl_team"`
	Status            *string  `json:"status"`
	InjuryStatus      *string  `json:"injury_status"`
	HeadshotURL       *string  `json:"headshot_url"`
}

func (s *Service) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	sport := strings.ToLower(q.Get("sport"))
	if sport == "" {
		sport = "nfl"
	}
	pos := q.Get("position")
	team := q.Get("team")
	search := q.Get("q")
	limit := 50
	if v, _ := strconv.Atoi(q.Get("limit")); v > 0 && v <= 200 {
		limit = v
	}
	offset := 0
	if v, _ := strconv.Atoi(q.Get("offset")); v >= 0 {
		offset = v
	}

	conds := []string{"sp.code = $1"}
	args := []any{sport}
	idx := 2
	if pos != "" {
		conds = append(conds, fmt.Sprintf("$%d = ANY(p.eligible_positions)", idx))
		args = append(args, pos)
		idx++
	}
	if team != "" {
		conds = append(conds, fmt.Sprintf("p.nfl_team = $%d", idx))
		args = append(args, team)
		idx++
	}
	if search != "" {
		conds = append(conds, fmt.Sprintf("p.full_name ILIKE $%d", idx))
		args = append(args, "%"+search+"%")
		idx++
	}
	args = append(args, limit, offset)

	query := fmt.Sprintf(`
		SELECT p.id, p.full_name, p.position, p.eligible_positions, p.nfl_team,
		       p.status, p.injury_status, p.headshot_url
		FROM players p JOIN sports sp ON sp.id = p.sport_id
		WHERE %s
		ORDER BY p.full_name
		LIMIT $%d OFFSET $%d`, strings.Join(conds, " AND "), idx, idx+1)

	rows, err := s.pool.Query(r.Context(), query, args...)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()
	out := []player{}
	for rows.Next() {
		var p player
		if err := rows.Scan(&p.ID, &p.FullName, &p.Position, &p.EligiblePositions,
			&p.NFLTeam, &p.Status, &p.InjuryStatus, &p.HeadshotURL); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, err)
			return
		}
		out = append(out, p)
	}
	httpx.WriteJSON(w, http.StatusOK, listResp{Players: out, Total: len(out)})
}

func (s *Service) get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "playerID")
	var p player
	err := s.pool.QueryRow(r.Context(), `
		SELECT id, full_name, position, eligible_positions, nfl_team, status, injury_status, headshot_url
		FROM players WHERE id = $1`, id).
		Scan(&p.ID, &p.FullName, &p.Position, &p.EligiblePositions, &p.NFLTeam, &p.Status, &p.InjuryStatus, &p.HeadshotURL)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, p)
}

func (s *Service) trending(w http.ResponseWriter, r *http.Request) {
	// Computed on the fly from a hypothetical trending_players table; for MVP
	// just surface the top recently-added rosters.
	rows, err := s.pool.Query(r.Context(), `
		SELECT p.id, p.full_name, p.position, p.nfl_team
		FROM rosters r
		JOIN players p ON p.id = r.player_id
		WHERE r.acquired_at > now() - interval '24 hours'
		GROUP BY p.id, p.full_name, p.position, p.nfl_team
		ORDER BY count(*) DESC LIMIT 25`)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()
	type t struct {
		ID, Name string
		Pos, Tm  *string
	}
	out := []t{}
	for rows.Next() {
		var x t
		if err := rows.Scan(&x.ID, &x.Name, &x.Pos, &x.Tm); err != nil {
			continue
		}
		out = append(out, x)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"trending": out})
}

// --- Sync (worker side) ---

func (s *Service) SyncFromProvider(ctx context.Context, dp provider.DataProvider) error {
	if dp == nil {
		return errors.New("no provider")
	}
	for _, code := range []string{"nfl", "nba", "mlb"} {
		var sportID int
		err := s.pool.QueryRow(ctx, `SELECT id FROM sports WHERE code = $1`, code).Scan(&sportID)
		if err != nil {
			continue
		}
		eff := dataprovider.ForSport(dp, code)
		if eff == nil {
			continue
		}
		players, err := eff.SyncPlayers(ctx, provider.Sport{ID: sportID, Code: code})
		if err != nil {
			return fmt.Errorf("%s players: %w", code, err)
		}
		if len(players) == 0 {
			continue
		}
		batch := 500
		for i := 0; i < len(players); i += batch {
			end := i + batch
			if end > len(players) {
				end = len(players)
			}
			if err := s.upsertBatch(ctx, sportID, eff.Name(), players[i:end]); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) upsertBatch(ctx context.Context, sportID int, providerName string, batch []provider.Player) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, p := range batch {
		extra, _ := json.Marshal(p.Extra)
		_, err := tx.Exec(ctx, `
			INSERT INTO players (sport_id, provider, provider_player_id, full_name, first_name, last_name,
				position, eligible_positions, nfl_team, jersey_number, status, injury_status,
				injury_body_part, injury_notes, age, height_inches, weight_lbs, college, years_exp, headshot_url, extra)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,COALESCE($21::jsonb,'{}'::jsonb))
			ON CONFLICT (sport_id, provider, provider_player_id) DO UPDATE SET
				full_name = EXCLUDED.full_name,
				first_name = EXCLUDED.first_name,
				last_name = EXCLUDED.last_name,
				position = EXCLUDED.position,
				eligible_positions = EXCLUDED.eligible_positions,
				nfl_team = EXCLUDED.nfl_team,
				jersey_number = EXCLUDED.jersey_number,
				status = EXCLUDED.status,
				injury_status = EXCLUDED.injury_status,
				injury_body_part = EXCLUDED.injury_body_part,
				injury_notes = EXCLUDED.injury_notes,
				age = EXCLUDED.age,
				height_inches = EXCLUDED.height_inches,
				weight_lbs = EXCLUDED.weight_lbs,
				college = EXCLUDED.college,
				years_exp = EXCLUDED.years_exp,
				headshot_url = EXCLUDED.headshot_url,
				updated_at = now()`,
			sportID, providerName, p.ProviderPlayerID, p.FullName, nilStr(p.FirstName), nilStr(p.LastName),
			nilStr(p.Position), p.EligiblePositions, nilStr(p.NFLTeam), p.JerseyNumber,
			nilStr(p.Status), nilStr(p.InjuryStatus), nilStr(p.InjuryBodyPart), nilStr(p.InjuryNotes),
			p.Age, p.HeightInches, p.WeightLbs, nilStr(p.College), p.YearsExp,
			nilStr(p.HeadshotURL), string(extra))
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Service) SyncInjuriesFromProvider(ctx context.Context, dp provider.DataProvider) error {
	if dp == nil {
		return nil
	}
	for _, code := range []string{"nfl", "nba", "mlb"} {
		var sportID int
		err := s.pool.QueryRow(ctx, `SELECT id FROM sports WHERE code = $1`, code).Scan(&sportID)
		if err != nil {
			continue
		}
		eff := dataprovider.ForSport(dp, code)
		if eff == nil {
			continue
		}
		updates, err := eff.SyncInjuries(ctx, provider.Sport{ID: sportID, Code: code})
		if err != nil {
			return err
		}
		for _, u := range updates {
			_, _ = s.pool.Exec(ctx, `
				UPDATE players SET
					injury_status = $4, injury_body_part = $5, injury_notes = $6, updated_at = now()
				WHERE sport_id = $1 AND provider = $2 AND provider_player_id = $3`,
				sportID, eff.Name(), u.ProviderPlayerID, nilStr(u.Status), nilStr(u.BodyPart), nilStr(u.Notes))
		}
	}
	return nil
}

func nilStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
