// Package themes defines Theme Ball modifier catalog and configuration.
package themes

// Def describes one theme modifier for UI and scoring.
type Def struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Available   bool   `json:"available"` // false = show but cannot enable (e.g. weather v2)
}

// Catalog is the canonical list of 30 Theme Ball modifiers.
var Catalog = []Def{
	{Slug: "heaviest_team", Name: "Heaviest", Description: "#1 avg starter weight: rush TD boosted; lightest inverts.", Available: true},
	{Slug: "tallest_team", Name: "Tallest", Description: "#1 avg height: rec TD adjusted; shortest inverts.", Available: true},
	{Slug: "widest_spread", Name: "Widest", Description: "Largest weight spread on roster affects fumble scoring.", Available: true},
	{Slug: "lightest_skill", Name: "Lightest skill", Description: "Lightest RB/WR/TE group boosts receiving; heaviest boosts rushing.", Available: true},
	{Slug: "tower_te", Name: "Tower TE", Description: "Tallest TE room boosts TE receiving TDs.", Available: true},
	{Slug: "franchise_stack_win", Name: "Franchise stack", Description: "3+ starters on same NFL team: if that NFL team wins, those starters get a bonus.", Available: true},
	{Slug: "bird_caucus", Name: "Bird caucus", Description: "Most bird-mascot NFL teams boosts DEF TD and INT.", Available: true},
	{Slug: "underdog_market", Name: "Small market", Description: "Most small-market franchises boosts receptions.", Available: true},
	{Slug: "division_grudge", Name: "Division grudge", Description: "Starters facing a division opponent that week get a small boost.", Available: true},
	{Slug: "oldest_lineup", Name: "Oldest", Description: "Oldest starting lineup boosts DST/INT; youngest boosts rush yards.", Available: true},
	{Slug: "rookie_factory", Name: "Rookie factory", Description: "Most rookies: boom/bust on rec TD and fumbles.", Available: true},
	{Slug: "veteran_floor", Name: "Veteran floor", Description: "Most experienced roster gets PPR cushion on receptions.", Available: true},
	{Slug: "jersey_chaos", Name: "Jersey chaos", Description: "Highest avg jersey number hurts passing yards; lowest boosts rush TD.", Available: true},
	{Slug: "prime_87", Name: "Prime 87", Description: "Players wearing #87 get a rec TD multiplier.", Available: true},
	{Slug: "sec_speed", Name: "SEC speed", Description: "Most SEC alumni boosts rush yards.", Available: true},
	{Slug: "big_ten_grit", Name: "Big Ten grit", Description: "Most Big Ten alumni boosts receptions.", Available: true},
	{Slug: "ivy_accountant", Name: "Ivy accountant", Description: "Most Ivy alumni: lower PPR on catches, bonus on pass TD.", Available: true},
	{Slug: "long_names", Name: "Long names", Description: "Longest average last name boosts pass yards; shortest boosts rec yards.", Available: true},
	{Slug: "alliteration", Name: "Alliteration", Description: "3+ starters with matching initials: weekly coin-flip bonus or tax.", Available: true},
	{Slug: "rb_hoarder", Name: "RB hoarder", Description: "Most RBs rostered boosts rush TD; RB receptions slightly reduced.", Available: true},
	{Slug: "zero_rb", Name: "Zero-RB", Description: "Fewest RBs boosts WR rec TD; most RBs slightly reduces rush TD.", Available: true},
	{Slug: "kicker_chaos", Name: "Kicker chaos", Description: "2+ kickers rostered dampens other stats; 0 kickers boosts DEF/ST TD.", Available: true},
	{Slug: "bench_mob", Name: "Bench mob", Description: "Deep bench penalties/bonuses based on bench scoring.", Available: true},
	{Slug: "questionable_gambit", Name: "Questionable", Description: "Most Q tags: playing starters boosted; benched Q explosions taxed.", Available: true},
	{Slug: "iron_man", Name: "Iron man", Description: "Fewest injury designations reduces fumble penalty severity.", Available: true},
	{Slug: "bye_survivor", Name: "Bye survivor", Description: "Most byes: surviving starters get a boost.", Available: true},
	{Slug: "primetime_mayor", Name: "Primetime", Description: "Primetime-heavy rosters boost receptions; early-only boosts rush yards.", Available: true},
	{Slug: "weather_goblin", Name: "Weather goblin", Description: "Cold outdoor games boost rush TD (requires weather feed).", Available: false},
	{Slug: "wheel_of_stat", Name: "Wheel of stat", Description: "Random weekly stat boosted league-wide; last week's winner dampened on it.", Available: true},
	{Slug: "motto_or_tax", Name: "Motto", Description: "Teams without a motto get a small random stat tax until they set one.", Available: true},
}

// SlugSet returns all known slugs.
func SlugSet() map[string]struct{} {
	m := make(map[string]struct{}, len(Catalog))
	for _, d := range Catalog {
		m[d.Slug] = struct{}{}
	}
	return m
}
