package themes

import (
	"testing"

	"github.com/bulbousoars/lunarleague/apps/api/internal/scoring"
	leaguethemes "github.com/bulbousoars/lunarleague/apps/api/internal/themes"
)

func TestFranchiseStackBoostsStartersOnWinningNFLTeam(t *testing.T) {
	rules := scoring.Rules{"rush_yd": 0.1, "rush_td": 6}
	cfg := leaguethemes.DefaultConfig()
	cfg["franchise_stack_win"] = leaguethemes.Entry{Enabled: true, Multiplier: 1.06, MinStarters: 3}

	ctx := &WeekContext{
		SportCode:    "nfl",
		ScheduleType: "theme_ball",
		Config:       cfg,
		Rules:        rules,
		NFLTeamWon:   map[string]bool{"KC": true},
		Teams: map[string]*Team{
			"t1": {
				ID: "t1",
				Starters: []Player{
					{ID: "p1", TeamID: "t1", NFLTeam: "KC", Stats: map[string]float64{"rush_yd": 100, "rush_td": 1}},
					{ID: "p2", TeamID: "t1", NFLTeam: "KC", Stats: map[string]float64{"rush_yd": 10}},
					{ID: "p3", TeamID: "t1", NFLTeam: "KC", Stats: map[string]float64{"rush_yd": 10}},
				},
			},
		},
	}

	base := scoring.Compute(rules, map[string]float64{"rush_yd": 100, "rush_td": 1})
	got := ScorePlayer(ctx, ctx.Teams["t1"].Starters[0])
	want := scoring.Compute(rules, map[string]float64{"rush_yd": 106, "rush_td": 1.06})
	if got.Points < want-0.01 || got.Points > want+0.01 {
		t.Fatalf("got %v want ~%v (base %v)", got.Points, want, base)
	}
}

func TestHeaviestTeamBoostsRushTDForRankOne(t *testing.T) {
	rules := scoring.Rules{"rush_td": 6}
	cfg := leaguethemes.DefaultConfig()
	cfg["heaviest_team"] = leaguethemes.Entry{Enabled: true, Strength: 1}

	heavy := 250
	light := 180
	ctx := &WeekContext{
		SportCode:    "nfl",
		ScheduleType: "theme_ball",
		Config:       cfg,
		Rules:        rules,
		Teams: map[string]*Team{
			"heavy": {
				ID: "heavy",
				Starters: []Player{
					{ID: "h1", TeamID: "heavy", WeightLbs: &heavy, Stats: map[string]float64{"rush_td": 1}},
				},
			},
			"light": {
				ID: "light",
				Starters: []Player{
					{ID: "l1", TeamID: "light", WeightLbs: &light, Stats: map[string]float64{"rush_td": 1}},
				},
			},
		},
	}

	gotHeavy := ScorePlayer(ctx, ctx.Teams["heavy"].Starters[0]).Points
	gotLight := ScorePlayer(ctx, ctx.Teams["light"].Starters[0]).Points
	if gotHeavy <= gotLight {
		t.Fatalf("heavy team should score more on rush_td theme: heavy=%v light=%v", gotHeavy, gotLight)
	}
}
