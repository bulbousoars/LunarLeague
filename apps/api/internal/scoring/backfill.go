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

// BackfillRegularSeasonStats pulls stat blobs for NFL/NBA (weekly) and MLB
// (daily YYYYMMDD slates) for the current API season and the prior year.
// PollLiveStats only touches a limited number of recent windows per tick; this
// backfill runs on a slow cadence so season totals match providers.
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
	if err := s.backfillMLBSeasonStats(ctx, dp, cy); err != nil {
		return err
	}
	return nil
}

func (s *Service) backfillMLBSeasonStats(ctx context.Context, dp provider.DataProvider, calendarYear int) error {
	eff := dataprovider.ForSport(dp, "mlb")
	if eff == nil {
		return nil
	}
	sp, err := sport.FindByCode(ctx, s.pool, "mlb")
	if err != nil {
		return nil
	}
	ps := provider.Sport{ID: sp.ID, Code: "mlb"}
	primary := schedule.SeasonForSport("mlb", calendarYear)
	seasons := []int{primary, primary - 1}
	seen := map[int]struct{}{}
	now := time.Now().UTC()
	for _, season := range seasons {
		if season < 2000 {
			continue
		}
		if _, ok := seen[season]; ok {
			continue
		}
		seen[season] = struct{}{}
		start := time.Date(season, 3, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(season, 11, 10, 0, 0, 0, 0, time.UTC)
		if season == now.Year() && end.After(now) {
			end = now
		}
		for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
			week := d.Year()*10000 + int(d.Month())*100 + d.Day()
			stats, err := eff.SyncWeekStats(ctx, ps, season, week)
			if err != nil {
				slog.Warn("mlb stats backfill day", "season", season, "week", week, "err", err)
				continue
			}
			if len(stats) == 0 {
				continue
			}
			if err := s.upsertStatLines(ctx, sp.ID, "mlb", eff.Name(), season, week, stats); err != nil {
				return err
			}
			time.Sleep(120 * time.Millisecond)
		}
	}
	return nil
}
