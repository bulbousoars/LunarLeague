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
	"github.com/bulbousoars/lunarleague/apps/api/internal/router"
	"github.com/bulbousoars/lunarleague/apps/api/internal/ws"
	"github.com/redis/go-redis/v9"
)

func runServe(ctx context.Context, cfg *config.Config) error {
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("db connect", "err", err)
		return fmt.Errorf("db connect: %w", err)
	}
	defer pool.Close()

	rdbOpt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		slog.Error("redis url", "err", err)
		return fmt.Errorf("redis url: %w", err)
	}
	rdb := redis.NewClient(rdbOpt)
	defer rdb.Close()

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
		return fmt.Errorf("data provider: %w", err)
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
	return nil
}
