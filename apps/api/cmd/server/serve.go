package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/bulbousoars/lunarleague/apps/api/internal/config"
	"github.com/bulbousoars/lunarleague/apps/api/internal/db"
	"github.com/bulbousoars/lunarleague/apps/api/internal/notify"
	"github.com/bulbousoars/lunarleague/apps/api/internal/provider"
	"github.com/bulbousoars/lunarleague/apps/api/internal/provider/sleeper"
	"github.com/bulbousoars/lunarleague/apps/api/internal/router"
	"github.com/bulbousoars/lunarleague/apps/api/internal/ws"
	"github.com/redis/go-redis/v9"
)

func runServe(ctx context.Context, cfg *config.Config) {
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("db connect", "err", err)
		return
	}
	defer pool.Close()

	rdbOpt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		slog.Error("redis url", "err", err)
		return
	}
	rdb := redis.NewClient(rdbOpt)
	defer rdb.Close()

	mailer := notify.NewSMTPMailer(cfg.SMTP)

	var dp provider.DataProvider
	switch cfg.DataProvider {
	case "sleeper":
		dp = sleeper.New()
	default:
		slog.Error("data provider not yet implemented", "provider", cfg.DataProvider)
		return
	}

	hub := ws.NewHub(rdb)
	go hub.Run(ctx)

	deps := &router.Deps{
		Cfg:      cfg,
		DB:       pool,
		Redis:    rdb,
		Mailer:   mailer,
		Provider: dp,
		Hub:      hub,
	}

	handler := router.New(deps)

	addr := fmt.Sprintf("%s:%d", cfg.HTTPBind, cfg.HTTPPort)
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	go func() {
		slog.Info("api listening", "addr", addr, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("listen", "err", err)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down api")
	shutCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutCtx)
}
