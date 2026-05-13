package player

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/bulbousoars/lunarleague/apps/api/internal/db"
)

type aggRow struct {
	Totals map[string]float64
	Weeks  int
	Avg    map[string]float64
}

func addJSONStats(dst map[string]float64, raw []byte) {
	if len(raw) == 0 || string(raw) == "null" {
		return
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil || m == nil {
		return
	}
	for k, v := range m {
		f, ok := toFloatStat(v)
		if !ok {
			continue
		}
		dst[k] += f
	}
}

func toFloatStat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	case string:
		if strings.TrimSpace(x) == "" {
			return 0, false
		}
		f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func aggregateSeasonPlayerStats(ctx context.Context, pool *db.DB, sportCode string, season int, playerIDs []string) (map[string]aggRow, error) {
	if len(playerIDs) == 0 {
		return map[string]aggRow{}, nil
	}
	start := 3
	ph := make([]string, len(playerIDs))
	args := []any{strings.ToLower(strings.TrimSpace(sportCode)), season}
	for i, id := range playerIDs {
		ph[i] = fmt.Sprintf("$%d", start+i)
		args = append(args, id)
	}
	q := fmt.Sprintf(`
		SELECT ps.player_id::text, ps.stats
		FROM player_stats ps
		JOIN sports sp ON sp.id = ps.sport_id
		WHERE sp.code = $1 AND ps.season = $2 AND ps.week > 0
		  AND ps.player_id IN (%s)`, strings.Join(ph, ","))

	rows, err := pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]*aggRow)
	for rows.Next() {
		var pid string
		var statsRaw []byte
		if err := rows.Scan(&pid, &statsRaw); err != nil {
			return nil, err
		}
		ar := out[pid]
		if ar == nil {
			ar = &aggRow{Totals: make(map[string]float64)}
			out[pid] = ar
		}
		addJSONStats(ar.Totals, statsRaw)
		ar.Weeks++
	}

	res := make(map[string]aggRow, len(out))
	for pid, ar := range out {
		avg := make(map[string]float64)
		if ar.Weeks > 0 {
			for k, v := range ar.Totals {
				avg[k] = v / float64(ar.Weeks)
			}
		}
		res[pid] = aggRow{
			Totals: ar.Totals,
			Weeks:  ar.Weeks,
			Avg:    avg,
		}
	}
	return res, nil
}
