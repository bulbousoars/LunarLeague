package player

import (
	"strings"
	"time"

	"github.com/bulbousoars/lunarleague/apps/api/internal/schedule"
)

// DefaultAggregateSeason is the provider season year used for YTD totals when
// the client omits aggregate_season (mirrors schedule.SeasonForSport).
func DefaultAggregateSeason(sport string) int {
	return schedule.SeasonForSport(strings.ToLower(strings.TrimSpace(sport)), time.Now().Year())
}
