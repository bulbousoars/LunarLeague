package main

import (
	"fmt"

	"github.com/bulbousoars/lunarleague/apps/api/internal/config"
	"github.com/bulbousoars/lunarleague/apps/api/internal/provider"
	"github.com/bulbousoars/lunarleague/apps/api/internal/provider/mlbstatsapi"
	"github.com/bulbousoars/lunarleague/apps/api/internal/provider/sleeper"
	"github.com/bulbousoars/lunarleague/apps/api/internal/provider/sportsdataio"
)

func newDataProvider(cfg *config.Config) (provider.DataProvider, error) {
	switch cfg.DataProvider {
	case "sleeper":
		return sleeper.New(), nil
	case "sportsdataio":
		return sportsdataio.New(cfg.SportsDataIOAPIKey), nil
	case "mlbstatsapi":
		return mlbstatsapi.New(), nil
	default:
		return nil, fmt.Errorf("unknown DATA_PROVIDER %q", cfg.DataProvider)
	}
}
