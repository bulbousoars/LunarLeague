// Package scoring is the sport-agnostic stat -> fantasy-points engine.
//
// Rules are stored as a flat JSON map of stat-key -> points-per-unit, plus
// optional bonus thresholds. Examples:
//
//	{
//	  "pass_yd": 0.04,
//	  "pass_td": 4,
//	  "pass_int": -2,
//	  "rush_yd": 0.1,
//	  "rec": 1.0,                       // PPR
//	  "rec_yd": 0.1,
//	  "rec_td": 6,
//	  "fum_lost": -2,
//	  "bonus_pass_yd_300": 3,           // bonus_<stat>_<threshold> = points
//	  "bonus_rush_yd_100": 3,
//	  "bonus_rec_yd_100": 3
//	}
package scoring

import (
	"context"
	"encoding/json"
	"errors"
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

func NewService(pool *db.DB) *Service {
	return &Service{pool: pool}
}

func (s *Service) Mount(r chi.Router) {
	r.Get("/leagues/{leagueID}/scoring/preview", s.preview)
}

// preview computes fantasy points for a single arbitrary stat line under a
// league's current rules, useful for the rules editor UI.
type previewReq struct {
	Stats map[string]float64 `json:"stats"`
}

func (s *Service) preview(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "leagueID")
	var raw []byte
	err := s.pool.QueryRow(r.Context(),
		`SELECT rules FROM scoring_rules WHERE league_id = $1`, id).Scan(&raw)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, err)
		return
	}
	rules, err := parseRules(raw)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	var req previewReq
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err)
		return
	}
	pts := Compute(rules, req.Stats)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"points": pts})
}

// PollLiveStats is invoked by the worker on a periodic tick. It pulls the
// latest stat lines from the configured provider for every sport that has
// games marked scheduled or in progress, then upserts player_stats.
func (s *Service) PollLiveStats(ctx context.Context, dp provider.DataProvider) error {
	if dp == nil {
		return errors.New("no data provider")
	}
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT ON (g.sport_id, g.season, g.week)
			g.sport_id, sp.code, g.season, g.week
		FROM games g
		JOIN sports sp ON sp.id = g.sport_id
		WHERE g.status IN ('in_progress','scheduled')
		ORDER BY g.sport_id, g.season, g.week, g.kickoff_at
		LIMIT 32`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var sportID int
		var sportCode string
		var season, week int
		if err := rows.Scan(&sportID, &sportCode, &season, &week); err != nil {
			return err
		}
		eff := dataprovider.ForSport(dp, sportCode)
		if eff == nil {
			continue
		}
		stats, err := eff.SyncWeekStats(ctx, provider.Sport{ID: sportID, Code: sportCode}, season, week)
		if err != nil {
			return err
		}
		for _, sl := range stats {
			body, _ := json.Marshal(sl.Stats)
			_, err := s.pool.Exec(ctx, `
				INSERT INTO player_stats (sport_id, season, week, player_id, stats, is_final)
				SELECT $1, $2, $3, p.id, $5::jsonb, $6
				FROM players p
				WHERE p.sport_id = $1 AND p.provider_player_id = $4 AND p.provider = $7
				ON CONFLICT (sport_id, season, week, player_id) DO UPDATE
					SET stats = EXCLUDED.stats, is_final = EXCLUDED.is_final, updated_at = now()`,
				sportID, season, week, sl.ProviderPlayerID, string(body), sl.IsFinal, eff.Name())
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// --- Rules + compute ---

type Rules map[string]float64

func parseRules(raw []byte) (Rules, error) {
	r := Rules{}
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, err
	}
	return r, nil
}

// Compute applies the rules to a stat line.
func Compute(rules Rules, stats map[string]float64) float64 {
	var pts float64
	for k, v := range stats {
		if rate, ok := rules[k]; ok {
			pts += v * rate
		}
	}
	// Threshold bonuses. Keys are "bonus_<stat>_<thr>".
	for key, val := range rules {
		if !strings.HasPrefix(key, "bonus_") {
			continue
		}
		rest := strings.TrimPrefix(key, "bonus_")
		i := strings.LastIndexByte(rest, '_')
		if i < 0 {
			continue
		}
		stat := rest[:i]
		thr, err := strconv.ParseFloat(rest[i+1:], 64)
		if err != nil {
			continue
		}
		if stats[stat] >= thr {
			pts += val
		}
	}
	// Round to 2 decimals.
	return float64(int(pts*100)) / 100.0
}

// DefaultRules returns sane sport-specific defaults that the league commissioner
// can later tweak via the UI.
func DefaultRules(sport string) Rules {
	switch sport {
	case "nfl":
		return Rules{
			"pass_yd":           0.04,
			"pass_td":           4,
			"pass_int":          -2,
			"pass_2pt":          2,
			"rush_yd":           0.1,
			"rush_td":           6,
			"rush_2pt":          2,
			"rec":               0.5, // half-PPR; commissioner can tweak
			"rec_yd":            0.1,
			"rec_td":            6,
			"rec_2pt":           2,
			"fum_lost":          -2,
			"def_int":           2,
			"def_fr":            2,
			"def_sack":          1,
			"def_td":            6,
			"def_safe":          2,
			"def_block_kick":    2,
			"st_td":             6,
			"fgm_0_19":          3,
			"fgm_20_29":         3,
			"fgm_30_39":         3,
			"fgm_40_49":         4,
			"fgm_50p":           5,
			"fgmiss":            -1,
			"xpm":               1,
			"xpmiss":            -1,
			"bonus_pass_yd_300": 0,
			"bonus_rush_yd_100": 0,
			"bonus_rec_yd_100":  0,
		}
	case "nba":
		return Rules{
			"pts": 1, "reb": 1.2, "ast": 1.5, "stl": 3, "blk": 3, "to": -1, "fg3m": 0.5,
		}
	case "mlb":
		return Rules{
			"hit": 1, "run": 1, "rbi": 1, "hr": 4, "sb": 2, "bb": 1, "k": -0.5,
			"win": 5, "loss": -3, "qs": 3, "sv": 5, "ip": 3, "er": -2, "k_p": 1,
		}
	}
	return Rules{}
}
