package themes

import (
	"math"
	"sort"

	leaguethemes "github.com/bulbousoars/lunarleague/apps/api/internal/themes"
)

type teamMetric func(*Team) (float64, bool)

func teamAvgWeight(t *Team) (float64, bool) {
	var sum float64
	var n int
	for _, p := range t.Starters {
		if p.WeightLbs == nil {
			continue
		}
		sum += float64(*p.WeightLbs)
		n++
	}
	if n == 0 {
		return 0, false
	}
	return sum / float64(n), true
}

func teamAvgHeight(t *Team) (float64, bool) {
	var sum float64
	var n int
	for _, p := range t.Starters {
		if p.HeightIn == nil {
			continue
		}
		sum += float64(*p.HeightIn)
		n++
	}
	if n == 0 {
		return 0, false
	}
	return sum / float64(n), true
}

func applyComparativeStat(
	ctx *WeekContext,
	entry leaguethemes.Entry,
	p Player,
	m Multipliers,
	bd *Breakdown,
	slug, stat string,
	metric teamMetric,
	higherBetter bool,
	topMult, bottomMult float64,
) {
	strength := entry.Strength
	if strength == 0 {
		strength = 1
	}
	top := 1 + (topMult-1)*strength
	bottom := 1 + (bottomMult-1)*strength

	type ranked struct {
		teamID string
		val    float64
	}
	var ranks []ranked
	for id, t := range ctx.Teams {
		v, ok := metric(t)
		if !ok {
			continue
		}
		ranks = append(ranks, ranked{id, v})
	}
	if len(ranks) < 2 {
		return
	}
	sort.Slice(ranks, func(i, j int) bool {
		if higherBetter {
			return ranks[i].val > ranks[j].val
		}
		return ranks[i].val < ranks[j].val
	})
	first := ranks[0].teamID
	last := ranks[len(ranks)-1].teamID

	var mult float64
	var note string
	switch p.TeamID {
	case first:
		mult = top
		note = "rank #1"
	case last:
		mult = bottom
		note = "rank last"
	default:
		return
	}
	if math.Abs(mult-1) < 1e-9 {
		return
	}
	m.multStat(stat, mult)
	appendEffect(bd, p.ID, Effect{Slug: slug, Stat: stat, Mult: mult, Note: note})
}
