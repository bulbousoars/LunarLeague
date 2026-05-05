// Package sport owns the sports lookup table and its seeding.
package sport

import (
	"context"

	"github.com/bulbousoars/lunarleague/apps/api/internal/db"
)

type Sport struct {
	ID         int
	Code       string
	Name       string
	SeasonType string
}

var defaults = []Sport{
	{Code: "nfl", Name: "Football (NFL)", SeasonType: "weekly"},
	{Code: "nba", Name: "Basketball (NBA)", SeasonType: "daily"},
	{Code: "mlb", Name: "Baseball (MLB)", SeasonType: "daily"},
}

// Seed inserts the canonical sports if missing.
func Seed(ctx context.Context, pool *db.DB) error {
	for _, s := range defaults {
		_, err := pool.Exec(ctx, `
			INSERT INTO sports (code, name, season_type)
			VALUES ($1, $2, $3)
			ON CONFLICT (code) DO UPDATE SET name = EXCLUDED.name, season_type = EXCLUDED.season_type`,
			s.Code, s.Name, s.SeasonType)
		if err != nil {
			return err
		}
	}
	return nil
}

// FindByCode returns the sport id for a code, seeding the table on the fly if
// it's empty (handy for first-run dev).
func FindByCode(ctx context.Context, pool *db.DB, code string) (Sport, error) {
	var s Sport
	err := pool.QueryRow(ctx,
		`SELECT id, code, name, season_type FROM sports WHERE code = $1`, code).
		Scan(&s.ID, &s.Code, &s.Name, &s.SeasonType)
	return s, err
}
