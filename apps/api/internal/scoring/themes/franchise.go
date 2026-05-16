package themes

import leaguethemes "github.com/bulbousoars/lunarleague/apps/api/internal/themes"

func applyFranchiseStack(ctx *WeekContext, entry leaguethemes.Entry, p Player, m Multipliers, bd *Breakdown) {
	if p.NFLTeam == "" {
		return
	}
	min := entry.MinStarters
	if min == 0 {
		min = 3
	}
	mult := entry.Multiplier
	if mult == 0 {
		mult = 1.06
	}
	if entry.Strength > 0 {
		mult = 1 + (mult-1)*entry.Strength
	}

	team := ctx.Teams[p.TeamID]
	if team == nil {
		return
	}
	count := 0
	for _, s := range team.Starters {
		if s.NFLTeam == p.NFLTeam {
			count++
		}
	}
	if count < min {
		return
	}
	if !ctx.NFLTeamWon[p.NFLTeam] {
		return
	}
	m.multAll(mult)
	appendEffect(bd, p.ID, Effect{
		Slug: "franchise_stack_win",
		Mult: mult,
		Note: p.NFLTeam + " stack win",
	})
}
