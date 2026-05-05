package main

import (
	"context"
	"log/slog"

	"github.com/bulbousoars/lunarleague/apps/api/internal/config"
	"github.com/bulbousoars/lunarleague/apps/api/internal/db"
	"github.com/bulbousoars/lunarleague/apps/api/internal/sport"
)

func runSeed(ctx context.Context, cfg *config.Config) {
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("db connect", "err", err)
		return
	}
	defer pool.Close()

	if err := sport.Seed(ctx, pool); err != nil {
		slog.Error("seed sports", "err", err)
		return
	}
	slog.Info("seed complete")
}
