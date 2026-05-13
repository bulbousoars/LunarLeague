// Package schedule upserts canonical games rows from each configured provider.
package schedule

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bulbousoars/lunarleague/apps/api/internal/dataprovider"
	"github.com/bulbousoars/lunarleague/apps/api/internal/db"
	"github.com/bulbousoars/lunarleague/apps/api/internal/provider"
	"github.com/bulbousoars/lunarleague/apps/api/internal/sport"
)

// SyncFromProviders pulls regular-season schedules for NFL, NBA, and MLB and
// merges them into the games table (used by live stat polling).
func SyncFromProviders(ctx context.Context, pool *db.DB, primary provider.DataProvider) error {
	if primary == nil {
		return fmt.Errorf("no data provider")
	}
	yr := time.Now().Year()
	for _, code := range []string{"nfl", "nba", "mlb"} {
		dp := dataprovider.ForSport(primary, code)
		if dp == nil {
			continue
		}
		sp, err := sport.FindByCode(ctx, pool, code)
		if err != nil {
			continue
		}
		games, err := dp.SyncSchedule(ctx, provider.Sport{ID: sp.ID, Code: code}, SeasonForSport(code, yr))
		if err != nil || len(games) == 0 {
			continue
		}
		for _, g := range games {
			st := mapGameStatus(g.Status)
			_, err := pool.Exec(ctx, `
				INSERT INTO games (sport_id, season, week, home_team, away_team, kickoff_at, status,
					home_score, away_score, provider_game_id, provider)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
				ON CONFLICT (sport_id, provider, provider_game_id) DO UPDATE SET
					week = EXCLUDED.week,
					home_team = EXCLUDED.home_team,
					away_team = EXCLUDED.away_team,
					kickoff_at = EXCLUDED.kickoff_at,
					status = EXCLUDED.status,
					home_score = EXCLUDED.home_score,
					away_score = EXCLUDED.away_score`,
				sp.ID, g.Season, g.Week, g.HomeTeam, g.AwayTeam, parseKickoff(g.KickoffISO), st,
				g.HomeScore, g.AwayScore, g.ProviderGameID, dp.Name(),
			)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// SeasonForSport maps calendar time to the provider season year (e.g. NFL
// early calendar year uses prior season).
func SeasonForSport(code string, calendarYear int) int {
	// Jan–Feb: NFL postseason still tags prior season in most APIs.
	if code == "nfl" {
		now := time.Now()
		if now.Month() <= time.February {
			return calendarYear - 1
		}
	}
	return calendarYear
}

func mapGameStatus(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	switch {
	case strings.Contains(s, "postpon") || strings.Contains(s, "canc") || strings.Contains(s, "ppd"):
		return "postponed"
	case strings.Contains(s, "final") || strings.Contains(s, "complete"):
		return "final"
	case strings.Contains(s, "progress") || strings.Contains(s, "live") || strings.Contains(s, "in progress"):
		return "in_progress"
	default:
		return "scheduled"
	}
}

func parseKickoff(iso string) time.Time {
	iso = strings.TrimSpace(iso)
	if iso == "" {
		return time.Now().UTC()
	}
	t, err := time.Parse(time.RFC3339, iso)
	if err == nil {
		return t.UTC()
	}
	t, err = time.Parse("2006-01-02T15:04:05Z", iso)
	if err == nil {
		return t.UTC()
	}
	t, err = time.Parse("2006-01-02", iso)
	if err == nil {
		return t.UTC()
	}
	return time.Now().UTC()
}
