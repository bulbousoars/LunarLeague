package mlbstatsapi

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/bulbousoars/lunarleague/apps/api/internal/provider"
)

// statsForDate aggregates batting/pitching counting stats for every player who
// appeared in games on the given calendar date (YYYY-MM-DD).
func (p *Provider) statsForDate(ctx context.Context, date string) ([]provider.StatLine, error) {
	url := fmt.Sprintf("%s/schedule?sportId=1&date=%s", baseURL, date)
	body, err := p.get(ctx, url)
	if err != nil {
		return nil, err
	}
	var sched struct {
		Dates []struct {
			Games []struct {
				GamePk int `json:"gamePk"`
			} `json:"games"`
		} `json:"dates"`
	}
	if err := json.Unmarshal(body, &sched); err != nil {
		return nil, fmt.Errorf("schedule decode: %w", err)
	}
	agg := map[string]map[string]float64{}
	for _, d := range sched.Dates {
		for _, g := range d.Games {
			if err := p.mergeBoxscore(ctx, g.GamePk, agg); err != nil {
				continue
			}
		}
	}
	out := make([]provider.StatLine, 0, len(agg))
	for pid, stats := range agg {
		out = append(out, provider.StatLine{
			ProviderPlayerID: pid,
			Stats:            stats,
			IsFinal:          false,
		})
	}
	return out, nil
}

func (p *Provider) mergeBoxscore(ctx context.Context, gamePk int, agg map[string]map[string]float64) error {
	if gamePk == 0 {
		return nil
	}
	url := fmt.Sprintf("%s/game/%d/boxscore", baseURL, gamePk)
	body, err := p.get(ctx, url)
	if err != nil {
		return err
	}
	var box struct {
		Decisions *struct {
			Winner *struct {
				ID int `json:"id"`
			} `json:"winner"`
			Loser *struct {
				ID int `json:"id"`
			} `json:"loser"`
			Save *struct {
				ID int `json:"id"`
			} `json:"save"`
		} `json:"decisions"`
		Teams struct {
			Home *teamSide `json:"home"`
			Away *teamSide `json:"away"`
		} `json:"teams"`
	}
	if err := json.Unmarshal(body, &box); err != nil {
		return err
	}
	if box.Decisions != nil {
		if box.Decisions.Winner != nil {
			addStat(agg, strconv.Itoa(box.Decisions.Winner.ID), "win", 1)
		}
		if box.Decisions.Loser != nil {
			addStat(agg, strconv.Itoa(box.Decisions.Loser.ID), "loss", 1)
		}
		if box.Decisions.Save != nil {
			addStat(agg, strconv.Itoa(box.Decisions.Save.ID), "sv", 1)
		}
	}
	if box.Teams.Home != nil {
		mergeTeamPlayers(agg, box.Teams.Home)
	}
	if box.Teams.Away != nil {
		mergeTeamPlayers(agg, box.Teams.Away)
	}
	return nil
}

type teamSide struct {
	Players map[string]playerBox `json:"players"`
}

type playerBox struct {
	Person struct {
		ID int `json:"id"`
	} `json:"person"`
	Stats struct {
		Batting  map[string]any `json:"batting"`
		Pitching map[string]any `json:"pitching"`
	} `json:"stats"`
}

func mergeTeamPlayers(agg map[string]map[string]float64, side *teamSide) {
	for _, pb := range side.Players {
		id := strconv.Itoa(pb.Person.ID)
		if id == "0" {
			continue
		}
		mergeBatting(agg, id, pb.Stats.Batting)
		mergePitching(agg, id, pb.Stats.Pitching)
	}
}

func mergeBatting(agg map[string]map[string]float64, id string, b map[string]any) {
	if len(b) == 0 {
		return
	}
	add := func(stat, key string) {
		if v, ok := floatFromAny(b[key]); ok {
			addStat(agg, id, stat, v)
		}
	}
	add("hit", "hits")
	add("run", "runs")
	add("rbi", "rbi")
	add("hr", "homeRuns")
	add("sb", "stolenBases")
	add("bb", "baseOnBalls")
	add("k", "strikeOuts")
}

func mergePitching(agg map[string]map[string]float64, id string, m map[string]any) {
	if len(m) == 0 {
		return
	}
	if v, ok := floatFromAny(m["inningsPitched"]); ok {
		addStat(agg, id, "ip", v)
	} else if s, ok := m["inningsPitched"].(string); ok {
		addStat(agg, id, "ip", parseIPString(s))
	}
	if v, ok := floatFromAny(m["earnedRuns"]); ok {
		addStat(agg, id, "er", v)
	}
	if v, ok := floatFromAny(m["strikeOuts"]); ok {
		addStat(agg, id, "k_p", v)
	}
}

func addStat(agg map[string]map[string]float64, id, key string, val float64) {
	if val == 0 {
		return
	}
	if agg[id] == nil {
		agg[id] = map[string]float64{}
	}
	agg[id][key] += val
}

func floatFromAny(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	default:
		return 0, false
	}
}

func parseIPString(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	dot := strings.IndexByte(s, '.')
	if dot < 0 {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0
		}
		return v
	}
	whole, err1 := strconv.Atoi(s[:dot])
	frac, err2 := strconv.Atoi(s[dot+1:])
	if err1 != nil || err2 != nil || frac < 0 || frac > 2 {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0
		}
		return v
	}
	outs := whole*3 + frac
	return float64(outs) / 3.0
}
