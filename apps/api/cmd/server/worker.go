package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/bulbousoars/lunarleague/apps/api/internal/config"
	"github.com/bulbousoars/lunarleague/apps/api/internal/db"
	"github.com/bulbousoars/lunarleague/apps/api/internal/notify"
	"github.com/bulbousoars/lunarleague/apps/api/internal/player"
	"github.com/bulbousoars/lunarleague/apps/api/internal/schedule"
	"github.com/bulbousoars/lunarleague/apps/api/internal/scoring"
	"github.com/bulbousoars/lunarleague/apps/api/internal/waivers"
)

// runWorker runs scheduled background jobs:
//   - Player universe sync (daily)
//   - Injury / news sync (hourly)
//   - Schedule refresh (6h) for live stat polling
//   - Live stat poll (30s when games are in flight)
//   - Waiver processing (default Wed 03:00 ET)
//   - Email digests (hourly tick)
//
// Implementation note: this uses a simple in-process ticker scheduler. River is
// declared in go.mod for future migration once we need durable cross-replica jobs.
func runWorker(ctx context.Context, cfg *config.Config) {
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("db connect", "err", err)
		return
	}
	defer pool.Close()

	mailer := notify.NewSMTPMailer(cfg.SMTP)
	notify.LogSMTPStartup(cfg.SMTP)
	go func() {
		pctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		notify.LogSMTPReachability(pctx, cfg.SMTP)
	}()

	dp, err := newDataProvider(cfg)
	if err != nil {
		slog.Error("data provider", "err", err)
		return
	}

	playerSvc := player.NewService(pool)
	scoringSvc := scoring.NewService(pool)
	waiverSvc := waivers.NewService(pool)

	jobs := []job{
		{name: "player-sync", every: 24 * time.Hour, fn: func(ctx context.Context) error {
			return playerSvc.SyncFromProvider(ctx, dp)
		}},
		{name: "injury-sync", every: 1 * time.Hour, fn: func(ctx context.Context) error {
			return playerSvc.SyncInjuriesFromProvider(ctx, dp)
		}},
		{name: "schedule-sync", every: 6 * time.Hour, fn: func(ctx context.Context) error {
			return schedule.SyncFromProviders(ctx, pool, dp)
		}},
		{name: "live-stats", every: 30 * time.Second, fn: func(ctx context.Context) error {
			return scoringSvc.PollLiveStats(ctx, dp)
		}},
		{name: "waiver-processor", every: 5 * time.Minute, fn: func(ctx context.Context) error {
			return waiverSvc.ProcessDue(ctx)
		}},
		{name: "email-digests", every: 1 * time.Hour, fn: func(ctx context.Context) error {
			return notify.SendDueDigests(ctx, pool, mailer, cfg.PublicWebURL)
		}},
	}

	slog.Info("worker started", "jobs", len(jobs))

	for i := range jobs {
		j := jobs[i]
		go runJobLoop(ctx, j)
	}

	<-ctx.Done()
	slog.Info("worker shutting down")
}

type job struct {
	name  string
	every time.Duration
	fn    func(context.Context) error
}

func runJobLoop(ctx context.Context, j job) {
	t := time.NewTicker(j.every)
	defer t.Stop()
	// Run once on startup (after a short delay so the API has time to migrate).
	time.Sleep(5 * time.Second)
	tick(ctx, j)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			tick(ctx, j)
		}
	}
}

func tick(ctx context.Context, j job) {
	jctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	start := time.Now()
	err := j.fn(jctx)
	if err != nil {
		slog.Warn("job failed", "name", j.name, "err", err, "dur", time.Since(start))
		return
	}
	slog.Debug("job ok", "name", j.name, "dur", time.Since(start))
}
