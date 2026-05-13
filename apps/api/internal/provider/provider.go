// Package provider defines the abstraction over upstream sports data feeds.
//
// Supported path today: Sleeper (free, NFL+NBA) plus MLB Stats API for MLB when
// DATA_PROVIDER=sleeper. SportsData.io is scaffolded and deferred; see
// docs/DATA_PROVIDERS.md and docs/ROADMAP.md. Implementations must be safe for
// concurrent use; the worker invokes them from multiple goroutines.
package provider

import "context"

type Sport struct {
	ID   int
	Code string // 'nfl' | 'nba' | 'mlb'
}

type Player struct {
	ProviderPlayerID  string
	FullName          string
	FirstName         string
	LastName          string
	Position          string
	EligiblePositions []string
	NFLTeam           string
	JerseyNumber      *int
	Status            string
	InjuryStatus      string
	InjuryBodyPart    string
	InjuryNotes       string
	Age               *int
	HeightInches      *int
	WeightLbs         *int
	College           string
	YearsExp          *int
	HeadshotURL       string
	Extra             map[string]any
}

type StatLine struct {
	ProviderPlayerID string
	Stats            map[string]float64
	IsFinal          bool
}

type InjuryUpdate struct {
	ProviderPlayerID string
	Status           string
	BodyPart         string
	Notes            string
}

type Game struct {
	ProviderGameID string
	Season         int
	Week           int
	HomeTeam       string
	AwayTeam       string
	KickoffISO     string
	Status         string
	HomeScore      *int
	AwayScore      *int
}

type TrendingPlayer struct {
	ProviderPlayerID string
	Count            int
	Direction        string // 'add' | 'drop'
}

type DataProvider interface {
	Name() string
	SyncPlayers(ctx context.Context, sport Sport) ([]Player, error)
	SyncWeekStats(ctx context.Context, sport Sport, season int, week int) ([]StatLine, error)
	SyncInjuries(ctx context.Context, sport Sport) ([]InjuryUpdate, error)
	SyncSchedule(ctx context.Context, sport Sport, season int) ([]Game, error)
	SyncTrending(ctx context.Context, sport Sport) ([]TrendingPlayer, error)
}
