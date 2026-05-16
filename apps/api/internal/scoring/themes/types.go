package themes

import (
	"github.com/bulbousoars/lunarleague/apps/api/internal/scoring"
	leaguethemes "github.com/bulbousoars/lunarleague/apps/api/internal/themes"
)

// Player holds one starter's attributes and weekly stats for theme evaluation.
type Player struct {
	ID        string
	TeamID    string
	Stats     map[string]float64
	Position  string
	NFLTeam   string
	WeightLbs *int
	HeightIn  *int
	Jersey    *int
	YearsExp  *int
	FirstName string
	LastName  string
}

// Team groups starters for one fantasy franchise in a week.
type Team struct {
	ID       string
	Starters []Player
}

// WeekContext is the league-week snapshot used by theme calculators.
type WeekContext struct {
	LeagueID     string
	Season       int
	Week         int
	SportCode    string
	ScheduleType string
	Config       leaguethemes.Config
	Rules        scoring.Rules
	Teams        map[string]*Team
	// NFLTeamWon maps nfl_team code -> whether that franchise won a final game this week.
	NFLTeamWon map[string]bool
}

// Breakdown records which themes affected a player (for UI transparency).
type Breakdown map[string][]Effect

// Effect is one applied theme touch on a player.
type Effect struct {
	Slug string  `json:"slug"`
	Stat string  `json:"stat,omitempty"`
	Mult float64 `json:"mult,omitempty"`
	Flat float64 `json:"flat,omitempty"`
	Note string  `json:"note,omitempty"`
}

// Multipliers accumulates per-stat and all-stat scaling for one player.
type Multipliers struct {
	All   float64
	Stats map[string]float64
}

func (m *Multipliers) scale(stat string) float64 {
	all := m.All
	if all == 0 {
		all = 1
	}
	if m.Stats == nil {
		return all
	}
	s := m.Stats[stat]
	if s == 0 {
		s = 1
	}
	return all * s
}

func (m *Multipliers) multAll(f float64) {
	if m.All == 0 {
		m.All = 1
	}
	m.All *= f
}

func (m *Multipliers) multStat(stat string, f float64) {
	if m.Stats == nil {
		m.Stats = map[string]float64{}
	}
	cur := m.Stats[stat]
	if cur == 0 {
		cur = 1
	}
	m.Stats[stat] = cur * f
}
