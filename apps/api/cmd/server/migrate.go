package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func runMigrate(ctx context.Context, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: migrate up|down|status|version|reset")
		os.Exit(2)
	}
	cmd := args[0]
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is required for migrations")
		os.Exit(2)
	}

	dsn := dbURL
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		slog.Error("open db", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		slog.Error("set dialect", "err", err)
		os.Exit(1)
	}

	dir := os.Getenv("MIGRATIONS_DIR")
	if dir == "" {
		dir = "/app/db/migrations"
	}
	// dev fallback when running from repo root
	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		dir = "db/migrations"
	}

	if err := goose.RunContext(ctx, cmd, db, dir, args[1:]...); err != nil {
		slog.Error("migrate", "cmd", cmd, "err", err)
		os.Exit(1)
	}
	slog.Info("migrate ok", "cmd", cmd)
}
