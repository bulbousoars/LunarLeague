package themes

import (
	"strings"

	leaguethemes "github.com/bulbousoars/lunarleague/apps/api/internal/themes"
)

var birdTeams = map[string]struct{}{
	"PHI": {}, "ATL": {}, "BAL": {}, "ARI": {}, "SEA": {},
}

func applyPrime87(entry leaguethemes.Entry, p Player, m Multipliers, bd *Breakdown) {
	if p.Jersey == nil || *p.Jersey != 87 {
		return
	}
	mult := 1.25
	if entry.Strength > 0 {
		mult = 1 + (mult-1)*entry.Strength
	}
	m.multStat("rec_td", mult)
	appendEffect(bd, p.ID, Effect{Slug: "prime_87", Stat: "rec_td", Mult: mult})
}

func applyVeteranFloor(ctx *WeekContext, entry leaguethemes.Entry, p Player, flat *float64, bd *Breakdown) {
	team := ctx.Teams[p.TeamID]
	if team == nil {
		return
	}
	winner := topTeamByMetric(ctx, teamAvgYearsExp)
	if winner != p.TeamID {
		return
	}
	bonus := 0.5
	if entry.Strength > 0 {
		bonus *= entry.Strength
	}
	rec := p.Stats["rec"]
	if rec == 0 {
		return
	}
	*flat += rec * bonus
	appendEffect(bd, p.ID, Effect{Slug: "veteran_floor", Flat: rec * bonus, Note: "veteran PPR cushion"})
}

func teamAvgYearsExp(t *Team) (float64, bool) {
	var sum float64
	var n int
	for _, p := range t.Starters {
		if p.YearsExp == nil {
			continue
		}
		sum += float64(*p.YearsExp)
		n++
	}
	if n == 0 {
		return 0, false
	}
	return sum / float64(n), true
}

func topTeamByMetric(ctx *WeekContext, metric teamMetric) string {
	var bestID string
	var best float64
	okAny := false
	for id, t := range ctx.Teams {
		v, ok := metric(t)
		if !ok {
			continue
		}
		if !okAny || v > best {
			best = v
			bestID = id
			okAny = true
		}
	}
	return bestID
}

func applyBirdCaucus(ctx *WeekContext, entry leaguethemes.Entry, p Player, m Multipliers, bd *Breakdown) {
	winner := topTeamByBirdCount(ctx)
	if winner != p.TeamID {
		return
	}
	strength := entry.Strength
	if strength == 0 {
		strength = 1
	}
	mult := 1 + (1.15-1)*strength
	for _, stat := range []string{"def_td", "def_int"} {
		m.multStat(stat, mult)
	}
	appendEffect(bd, p.ID, Effect{Slug: "bird_caucus", Stat: "def_td/def_int", Mult: mult})
}

func topTeamByBirdCount(ctx *WeekContext) string {
	type scored struct {
		id    string
		count int
	}
	var list []scored
	for id, t := range ctx.Teams {
		n := 0
		for _, p := range t.Starters {
			if _, ok := birdTeams[strings.ToUpper(p.NFLTeam)]; ok {
				n++
			}
		}
		list = append(list, scored{id, n})
	}
	if len(list) == 0 {
		return ""
	}
	sortScored := list
	for i := 0; i < len(sortScored); i++ {
		for j := i + 1; j < len(sortScored); j++ {
			if sortScored[j].count > sortScored[i].count {
				sortScored[i], sortScored[j] = sortScored[j], sortScored[i]
			}
		}
	}
	return sortScored[0].id
}

func appendEffect(bd *Breakdown, playerID string, e Effect) {
	if *bd == nil {
		*bd = Breakdown{}
	}
	(*bd)[playerID] = append((*bd)[playerID], e)
}
