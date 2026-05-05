// LunarLeague server binary. Subcommands: serve, worker, migrate, seed.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/bulbousoars/lunarleague/apps/api/internal/config"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		// `migrate up` may be invoked before SECRET_KEY is set in CI; allow it to proceed
		// with a partial config.
		if cmd != "migrate" {
			fmt.Fprintf(os.Stderr, "config: %v\n", err)
			os.Exit(2)
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch cmd {
	case "serve":
		runServe(ctx, cfg)
	case "worker":
		runWorker(ctx, cfg)
	case "migrate":
		runMigrate(ctx, args)
	case "seed":
		runSeed(ctx, cfg)
	case "version":
		fmt.Println("LunarLeague 0.1.0")
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `lunarleague - self-hosted fantasy sports platform

Usage:
  lunarleague serve          Run the HTTP + WebSocket API
  lunarleague worker         Run the background job worker
  lunarleague migrate up     Apply pending DB migrations
  lunarleague migrate down   Roll back the latest DB migration
  lunarleague seed           Seed sports + (optional) demo league
  lunarleague version        Print version`)
}
