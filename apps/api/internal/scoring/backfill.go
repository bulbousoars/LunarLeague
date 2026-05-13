package scoring

import (
	"context"
	"log/slog"
	"time"

	"github.com/bulbousoars/lunarleague/apps/api/internal/dataprovider"
	"github.com/bulbousoars/lunarleague/apps/api/internal/provider"
	"github.com/bulbousoars/lunarleague/apps/api/internal/schedule"
	"github.com/bulbousoars/lunarleague/apps/api/internal/sport"
)

// BackfillRegularSeasonStats pulls weekly stat blobs for NFL/NBA for the current
// API season and the prior year (covers offseason / calendar edges). It is
// meant to run on a slow cadence (daily): PollLiveStats only touches a limited
// number of recent weeks per tick; this walks weeks 1–24 so season totals in
// the DB match Sleeper for aggregate_season.
func (s *Service) BackfillRegularSeasonStats(ctx context.Context, dp provider.DataProvider) error {
	if dp == nil {
		return nil
	}
	cy := time.Now().Year()
	for _, code := range []string{"nfl", "nba"} {
		eff := dataprovider.ForSport(dp, code)
		if eff == nil {
			continue
		}
		sp, err := sport.FindByCode(ctx, s.pool, code)
		if err != nil {
			continue
		}
		ps := provider.Sport{ID: sp.ID, Code: code}
		primary := schedule.SeasonForSport(code, cy)
		seasons := []int{primary, primary - 1}
		seen := map[int]struct{}{}
		for _, season := range seasons {
			if season < 2000 {
				continue
			}
			if _, ok := seen[season]; ok {
				continue
			}
			seen[season] = struct{}{}
			for week := 1; week <= 24; week++ {
				stats, err := eff.SyncWeekStats(ctx, ps, season, week)
				if err != nil {
					slog.Warn("stats backfill week", "sport", code, "season", season, "week", week, "err", err)
					continue
				}
				if len(stats) == 0 {
					continue
				}
				if err := s.upsertStatLines(ctx, sp.ID, code, eff.Name(), season, week, stats); err != nil {
					return err
				}
				time.Sleep(120 * time.Millisecond)
			}
		}
	}
	return nil
}
