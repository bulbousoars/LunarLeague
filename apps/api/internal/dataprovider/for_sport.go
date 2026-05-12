// Package dataprovider picks the upstream feed for a sport when the primary
// DATA_PROVIDER does not cover every league (e.g. Sleeper has no MLB).
package dataprovider

import (
	"github.com/bulbousoars/lunarleague/apps/api/internal/provider"
	"github.com/bulbousoars/lunarleague/apps/api/internal/provider/mlbstatsapi"
)

// ForSport returns the DataProvider implementation that should serve the
// given sport for this deployment. Primary is always non-nil when called
// from the worker or HTTP server bootstrap.
func ForSport(primary provider.DataProvider, sportCode string) provider.DataProvider {
	if primary == nil {
		return nil
	}
	switch primary.Name() {
	case "sleeper":
		if sportCode == "mlb" {
			return mlbstatsapi.New()
		}
		return primary
	case "mlbstatsapi":
		if sportCode != "mlb" {
			return nil
		}
		return primary
	default:
		return primary
	}
}
