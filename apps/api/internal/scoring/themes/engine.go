package themes

import (
	"maps"

	leaguethemes "github.com/bulbousoars/lunarleague/apps/api/internal/themes"
	"github.com/bulbousoars/lunarleague/apps/api/internal/scoring"
	"github.com/bulbousoars/lunarleague/apps/api/internal/statsnorm"
)

// PlayerScore is fantasy points for one starter after themes.
type PlayerScore struct {
	Points    float64
	Breakdown Breakdown
	Adjusted  map[string]float64
}

// ScorePlayer applies enabled theme modifiers then base scoring rules.
func ScorePlayer(ctx *WeekContext, p Player) PlayerScore {
	stats := maps.Clone(p.Stats)
	if stats == nil {
		stats = map[string]float64{}
	}
	stats = statsnorm.NormalizeStatMap(ctx.SportCode, "", stats)

	var mult Multipliers
	flat := 0.0
	bd := Breakdown{}

	if ctx.ScheduleType == "theme_ball" {
		for _, slug := range ctx.Config.EnabledSlugs() {
			entry := ctx.Config[slug]
			applyTheme(ctx, slug, entry, p, &mult, &flat, &bd)
		}
	}

	adjusted := make(map[string]float64, len(stats))
	for k, v := range stats {
		adjusted[k] = v * mult.scale(k)
	}
	pts := scoring.Compute(ctx.Rules, adjusted) + flat

	if len(bd[p.ID]) > 0 || flat > 0 {
		// ensure player key exists when only flat bonus applied
		if _, ok := bd[p.ID]; !ok && flat > 0 {
			bd[p.ID] = nil
		}
	}

	return PlayerScore{
		Points:    pts,
		Breakdown: bd,
		Adjusted:  adjusted,
	}
}

func applyTheme(ctx *WeekContext, slug string, entry leaguethemes.Entry, p Player, m *Multipliers, flat *float64, bd *Breakdown) {
	switch slug {
	case "franchise_stack_win":
		applyFranchiseStack(ctx, entry, p, m, bd)
	case "heaviest_team":
		applyComparativeStat(ctx, entry, p, m, bd, slug, "rush_td", teamAvgWeight, true, 1.08, 0.92)
	case "tallest_team":
		applyComparativeStat(ctx, entry, p, m, bd, slug, "rec_td", teamAvgHeight, true, 0.95, 1.05)
	case "prime_87":
		applyPrime87(entry, p, m, bd)
	case "veteran_floor":
		applyVeteranFloor(ctx, entry, p, flat, bd)
	case "bird_caucus":
		applyBirdCaucus(ctx, entry, p, m, bd)
	}
}
