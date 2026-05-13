// Package player owns the shared player universe + sync.
package player

import (
	"context"
	"database/sql"
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
	"github.com/bulbousoars/lunarleague/apps/api/internal/scoring"
	"github.com/go-chi/chi/v5"
)

// DisplayNameP is SQL for a non-empty label when reading players as alias p.
// Upstreams sometimes omit full_name while first/last or provider id exist.
const DisplayNameP = `COALESCE(NULLIF(btrim(p.full_name), ''), NULLIF(btrim(concat_ws(' ', p.first_name, p.last_name)), ''), p.provider_player_id)`

// DisplayNameBare is the same logic without a table alias.
const DisplayNameBare = `COALESCE(NULLIF(btrim(full_name), ''), NULLIF(btrim(concat_ws(' ', first_name, last_name)), ''), provider_player_id)`

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
	Players              []player `json:"players"`
	Total                int      `json:"total"`
	Limit                int      `json:"limit"`
	Offset               int      `json:"offset"`
	StatColumns          []string `json:"stat_columns,omitempty"`
	AggregateSeason      int      `json:"aggregate_season,omitempty"`
	CurrentStatsSeason   *int     `json:"current_stats_season,omitempty"`
	CurrentStatsWeek     *int     `json:"current_stats_week,omitempty"`
}

type player struct {
	ID                string           `json:"id"`
	FullName          string           `json:"full_name"`
	Position          *string          `json:"position"`
	EligiblePositions []string         `json:"eligible_positions"`
	NFLTeam           *string          `json:"nfl_team"`
	Status            *string          `json:"status"`
	InjuryStatus      *string          `json:"injury_status"`
	HeadshotURL       *string          `json:"headshot_url"`
	JerseyNumber      *int             `json:"jersey_number,omitempty"`
	Age               *int             `json:"age,omitempty"`
	HeightInches      *int             `json:"height_inches,omitempty"`
	WeightLbs         *int             `json:"weight_lbs,omitempty"`
	College           *string          `json:"college,omitempty"`
	YearsExp          *int             `json:"years_exp,omitempty"`
	StatsSeason       *int             `json:"stats_season,omitempty"`
	StatsWeek         *int             `json:"stats_week,omitempty"`
	WeeklyStats       json.RawMessage  `json:"weekly_stats,omitempty"`
	SeasonTotals      map[string]float64 `json:"season_totals,omitempty"`
	SeasonWeeks       int              `json:"season_weeks,omitempty"`
	SeasonWeeklyAvg   map[string]float64 `json:"season_weekly_avg,omitempty"`
}

func queryBool(q string) bool {
	v := strings.TrimSpace(strings.ToLower(q))
	return v == "1" || v == "true" || v == "yes"
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
	hasTeam := queryBool(q.Get("has_team"))
	includeStats := queryBool(q.Get("include_stats"))
	// Season for YTD + per-week averages (defaults 2025; override with aggregate_season).
	aggSeason := 2025
	if includeStats {
		if v, err := strconv.Atoi(q.Get("aggregate_season")); err == nil && v >= 1990 && v <= 2100 {
			aggSeason = v
		}
	}

	limit := 50
	if v, _ := strconv.Atoi(q.Get("limit")); v > 0 && v <= 500 {
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
		pat := "%" + search + "%"
		conds = append(conds, fmt.Sprintf(`(p.full_name ILIKE $%d OR concat_ws(' ', p.first_name, p.last_name) ILIKE $%d OR p.provider_player_id ILIKE $%d)`, idx, idx, idx))
		args = append(args, pat)
		idx++
	}
	if hasTeam {
		conds = append(conds, `p.nfl_team IS NOT NULL AND btrim(p.nfl_team) <> '' AND lower(btrim(p.nfl_team)) NOT IN ('fa','n/a','--')`)
	}
	whereSQL := strings.Join(conds, " AND ")

	var total int
	countSQL := fmt.Sprintf(`SELECT count(*)::int FROM players p JOIN sports sp ON sp.id = p.sport_id WHERE %s`, whereSQL)
	if err := s.pool.QueryRow(r.Context(), countSQL, args...).Scan(&total); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}

	filterArgs := append([]any{}, args...)
	idxAfter := idx
	statsJoin := ""
	statsSelect := `, NULL::int, NULL::int, '{}'::jsonb`
	statsSeason, statsWeek := 0, 0
	statsResolved := false
	if includeStats {
		if qs, err := strconv.Atoi(q.Get("season")); err == nil && qs > 0 {
			if qw, err := strconv.Atoi(q.Get("week")); err == nil && qw >= 0 {
				statsSeason, statsWeek = qs, qw
				statsResolved = true
			}
		}
		if !statsResolved {
			err := s.pool.QueryRow(r.Context(), `
				SELECT ps.season, ps.week
				FROM player_stats ps
				JOIN sports sp2 ON sp2.id = ps.sport_id
				WHERE sp2.code = $1 AND ps.season = $2
				ORDER BY ps.week DESC
				LIMIT 1`, sport, aggSeason).Scan(&statsSeason, &statsWeek)
			if err != nil {
				err = s.pool.QueryRow(r.Context(), `
					SELECT ps.season, ps.week
					FROM player_stats ps
					JOIN sports sp2 ON sp2.id = ps.sport_id
					WHERE sp2.code = $1
					ORDER BY ps.season DESC, ps.week DESC
					LIMIT 1`, sport).Scan(&statsSeason, &statsWeek)
			}
			if err == nil {
				statsResolved = true
			}
		}
		if statsResolved {
			statsJoin = fmt.Sprintf(`
				LEFT JOIN player_stats ps ON ps.player_id = p.id AND ps.sport_id = p.sport_id
					AND ps.season = $%d AND ps.week = $%d`, idxAfter, idxAfter+1)
			filterArgs = append(filterArgs, statsSeason, statsWeek)
			idxAfter += 2
			statsSelect = `, ps.season, ps.week, COALESCE(ps.stats, '{}'::jsonb)`
		}
	}

	limitArg := idxAfter
	offsetArg := idxAfter + 1
	dataArgs := append(filterArgs, limit, offset)

	selectCols := fmt.Sprintf(`p.id, %s, p.position, p.eligible_positions, p.nfl_team,
		p.status, p.injury_status, p.headshot_url,
		p.jersey_number, p.age, p.height_inches, p.weight_lbs, p.college, p.years_exp%s`,
		DisplayNameP, statsSelect)

	query := fmt.Sprintf(`
		SELECT %s
		FROM players p
		JOIN sports sp ON sp.id = p.sport_id
		%s
		WHERE %s
		ORDER BY %s
		LIMIT $%d OFFSET $%d`,
		selectCols,
		statsJoin,
		whereSQL,
		DisplayNameP,
		limitArg,
		offsetArg)

	rows, err := s.pool.Query(r.Context(), query, dataArgs...)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()
	out := []player{}
	for rows.Next() {
		var p player
		var jn, ag, hi, wl, ye sql.NullInt64
		var col sql.NullString
		var ss, sw sql.NullInt64
		var statsBlob []byte
		if err := rows.Scan(&p.ID, &p.FullName, &p.Position, &p.EligiblePositions,
			&p.NFLTeam, &p.Status, &p.InjuryStatus, &p.HeadshotURL,
			&jn, &ag, &hi, &wl, &col, &ye,
			&ss, &sw, &statsBlob); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, err)
			return
		}
		if jn.Valid {
			v := int(jn.Int64)
			p.JerseyNumber = &v
		}
		if ag.Valid {
			v := int(ag.Int64)
			p.Age = &v
		}
		if hi.Valid {
			v := int(hi.Int64)
			p.HeightInches = &v
		}
		if wl.Valid {
			v := int(wl.Int64)
			p.WeightLbs = &v
		}
		if col.Valid && strings.TrimSpace(col.String) != "" {
			c := strings.TrimSpace(col.String)
			p.College = &c
		}
		if ye.Valid {
			v := int(ye.Int64)
			p.YearsExp = &v
		}
		if ss.Valid {
			v := int(ss.Int64)
			p.StatsSeason = &v
		}
		if sw.Valid {
			v := int(sw.Int64)
			p.StatsWeek = &v
		}
		if len(statsBlob) > 2 && string(statsBlob) != "null" && string(statsBlob) != "{}" {
			p.WeeklyStats = json.RawMessage(statsBlob)
		}
		out = append(out, p)
	}

	resp := listResp{Players: out, Total: total, Limit: limit, Offset: offset}
	if includeStats {
		resp.StatColumns = scoring.DisplayStatKeys(sport)
		resp.AggregateSeason = aggSeason
		if statsResolved {
			ss := statsSeason
			sw := statsWeek
			resp.CurrentStatsSeason = &ss
			resp.CurrentStatsWeek = &sw
		}
		if len(out) > 0 {
			ids := make([]string, len(out))
			for i := range out {
				ids[i] = out[i].ID
			}
			aggs, err := aggregateSeasonPlayerStats(r.Context(), s.pool, sport, aggSeason, ids)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, err)
				return
			}
			for i := range out {
				if a, ok := aggs[out[i].ID]; ok && a.Weeks > 0 {
					out[i].SeasonTotals = a.Totals
					out[i].SeasonWeeks = a.Weeks
					out[i].SeasonWeeklyAvg = a.Avg
				}
			}
		}
		resp.Players = out
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (s *Service) get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "playerID")
	q := r.URL.Query()
	includeStats := queryBool(q.Get("include_stats"))

	if !includeStats {
		var p player
		var jn, ag, hi, wl, ye sql.NullInt64
		var col sql.NullString
		err := s.pool.QueryRow(r.Context(), fmt.Sprintf(`
			SELECT id, %s, position, eligible_positions, nfl_team, status, injury_status, headshot_url,
				jersey_number, age, height_inches, weight_lbs, college, years_exp
			FROM players WHERE id = $1`, DisplayNameBare), id).
			Scan(&p.ID, &p.FullName, &p.Position, &p.EligiblePositions, &p.NFLTeam, &p.Status, &p.InjuryStatus, &p.HeadshotURL,
				&jn, &ag, &hi, &wl, &col, &ye)
		if err != nil {
			httpx.WriteError(w, http.StatusNotFound, err)
			return
		}
		fillProfileInts(&p, jn, ag, hi, wl, ye, col)
		httpx.WriteJSON(w, http.StatusOK, p)
		return
	}

	statsSeason, statsWeek := 0, 0
	statsResolved := false
	if qs, err := strconv.Atoi(q.Get("season")); err == nil && qs > 0 {
		if qw, err := strconv.Atoi(q.Get("week")); err == nil && qw >= 0 {
			statsSeason, statsWeek = qs, qw
			statsResolved = true
		}
	}
	if !statsResolved {
		err := s.pool.QueryRow(r.Context(), `
			SELECT ps.season, ps.week
			FROM player_stats ps
			JOIN players p0 ON p0.id = ps.player_id
			JOIN sports sp2 ON sp2.id = p0.sport_id
			WHERE p0.id = $1
			ORDER BY ps.season DESC, ps.week DESC
			LIMIT 1`, id).Scan(&statsSeason, &statsWeek)
		if err == nil {
			statsResolved = true
		}
	}

	var p player
	var jn, ag, hi, wl, ye sql.NullInt64
	var col sql.NullString
	var ss, sw sql.NullInt64
	var statsBlob []byte

	if !statsResolved {
		err := s.pool.QueryRow(r.Context(), fmt.Sprintf(`
			SELECT p.id, %s, p.position, p.eligible_positions, p.nfl_team, p.status, p.injury_status, p.headshot_url,
				p.jersey_number, p.age, p.height_inches, p.weight_lbs, p.college, p.years_exp,
				NULL::int, NULL::int, '{}'::jsonb
			FROM players p WHERE p.id = $1`, DisplayNameP), id).
			Scan(&p.ID, &p.FullName, &p.Position, &p.EligiblePositions, &p.NFLTeam, &p.Status, &p.InjuryStatus, &p.HeadshotURL,
				&jn, &ag, &hi, &wl, &col, &ye, &ss, &sw, &statsBlob)
		if err != nil {
			httpx.WriteError(w, http.StatusNotFound, err)
			return
		}
		fillProfileInts(&p, jn, ag, hi, wl, ye, col)
		httpx.WriteJSON(w, http.StatusOK, p)
		return
	}

	err := s.pool.QueryRow(r.Context(), fmt.Sprintf(`
		SELECT p.id, %s, p.position, p.eligible_positions, p.nfl_team, p.status, p.injury_status, p.headshot_url,
			p.jersey_number, p.age, p.height_inches, p.weight_lbs, p.college, p.years_exp,
			ps.season, ps.week, COALESCE(ps.stats, '{}'::jsonb)
		FROM players p
		LEFT JOIN player_stats ps ON ps.player_id = p.id AND ps.sport_id = p.sport_id
			AND ps.season = $2 AND ps.week = $3
		WHERE p.id = $1`, DisplayNameP), id, statsSeason, statsWeek).
		Scan(&p.ID, &p.FullName, &p.Position, &p.EligiblePositions, &p.NFLTeam, &p.Status, &p.InjuryStatus, &p.HeadshotURL,
			&jn, &ag, &hi, &wl, &col, &ye, &ss, &sw, &statsBlob)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, err)
		return
	}
	fillProfileInts(&p, jn, ag, hi, wl, ye, col)
	if ss.Valid {
		v := int(ss.Int64)
		p.StatsSeason = &v
	}
	if sw.Valid {
		v := int(sw.Int64)
		p.StatsWeek = &v
	}
	if len(statsBlob) > 2 && string(statsBlob) != "null" && string(statsBlob) != "{}" {
		p.WeeklyStats = json.RawMessage(statsBlob)
	}
	httpx.WriteJSON(w, http.StatusOK, p)
}

func fillProfileInts(p *player, jn, ag, hi, wl, ye sql.NullInt64, col sql.NullString) {
	if jn.Valid {
		v := int(jn.Int64)
		p.JerseyNumber = &v
	}
	if ag.Valid {
		v := int(ag.Int64)
		p.Age = &v
	}
	if hi.Valid {
		v := int(hi.Int64)
		p.HeightInches = &v
	}
	if wl.Valid {
		v := int(wl.Int64)
		p.WeightLbs = &v
	}
	if col.Valid && strings.TrimSpace(col.String) != "" {
		c := strings.TrimSpace(col.String)
		p.College = &c
	}
	if ye.Valid {
		v := int(ye.Int64)
		p.YearsExp = &v
	}
}

func (s *Service) trending(w http.ResponseWriter, r *http.Request) {
	// Computed on the fly from a hypothetical trending_players table; for MVP
	// just surface the top recently-added rosters.
	rows, err := s.pool.Query(r.Context(), fmt.Sprintf(`
		SELECT p.id, %s, p.position, p.nfl_team
		FROM rosters r
		JOIN players p ON p.id = r.player_id
		WHERE r.acquired_at > now() - interval '24 hours'
		GROUP BY p.id, p.full_name, p.first_name, p.last_name, p.provider_player_id, p.position, p.nfl_team
		ORDER BY count(*) DESC LIMIT 25`, DisplayNameP))
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
		if _, err := s.fetchAndUpsertPlayers(ctx, sportID, code, eff); err != nil {
			return err
		}
	}
	return nil
}

// SyncFromProviderForSport pulls the player universe for a single sport from the
// configured data provider and upserts into the database. code must be nfl,
// nba, or mlb (case-insensitive). The sport row must exist (run seed first).
// Returns the number of players returned by the provider.
func (s *Service) SyncFromProviderForSport(ctx context.Context, dp provider.DataProvider, code string) (int, error) {
	if dp == nil {
		return 0, errors.New("no provider")
	}
	code = strings.ToLower(strings.TrimSpace(code))
	switch code {
	case "nfl", "nba", "mlb":
	default:
		return 0, fmt.Errorf("unsupported sport %q", code)
	}
	var sportID int
	err := s.pool.QueryRow(ctx, `SELECT id FROM sports WHERE code = $1`, code).Scan(&sportID)
	if err != nil {
		return 0, fmt.Errorf("sport %q not in database (run seed first): %w", code, err)
	}
	eff := dataprovider.ForSport(dp, code)
	if eff == nil {
		return 0, fmt.Errorf("no data provider for sport %s", code)
	}
	return s.fetchAndUpsertPlayers(ctx, sportID, code, eff)
}

func (s *Service) fetchAndUpsertPlayers(ctx context.Context, sportID int, code string, eff provider.DataProvider) (int, error) {
	players, err := eff.SyncPlayers(ctx, provider.Sport{ID: sportID, Code: code})
	if err != nil {
		return 0, fmt.Errorf("%s players: %w", code, err)
	}
	if len(players) == 0 {
		return 0, nil
	}
	batch := 500
	for i := 0; i < len(players); i += batch {
		end := i + batch
		if end > len(players) {
			end = len(players)
		}
		if err := s.upsertBatch(ctx, sportID, eff.Name(), players[i:end]); err != nil {
			return 0, err
		}
	}
	return len(players), nil
}

func (s *Service) upsertBatch(ctx context.Context, sportID int, providerName string, batch []provider.Player) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, p := range batch {
		extra, _ := json.Marshal(p.Extra)
		elig := p.EligiblePositions
		if elig == nil {
			elig = []string{}
		}
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
			nilStr(p.Position), elig, nilStr(p.NFLTeam), p.JerseyNumber,
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
