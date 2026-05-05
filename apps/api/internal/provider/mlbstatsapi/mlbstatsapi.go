// Package mlbstatsapi is a DataProvider implementation backed by the official
// MLB Stats API at https://statsapi.mlb.com/api/v1/.
//
// Status: scaffolded. SyncPlayers and SyncSchedule are wired up; stats and
// injuries are stubs that return empty so leagues can be created and the
// player universe will populate, but live scoring is intentionally noop until
// we settle on a category schema for fantasy baseball (categories vs. points).
package mlbstatsapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/bulbousoars/lunarleague/apps/api/internal/provider"
)

const baseURL = "https://statsapi.mlb.com/api/v1"

type Provider struct {
	client *http.Client
}

func New() *Provider {
	return &Provider{client: &http.Client{Timeout: 30 * time.Second}}
}

func (p *Provider) Name() string { return "mlbstatsapi" }

// SyncPlayers calls /sports/1/players (sport_id=1 is MLB) and returns the
// active player roster.
func (p *Provider) SyncPlayers(ctx context.Context, sport provider.Sport) ([]provider.Player, error) {
	if sport.Code != "mlb" {
		return nil, errors.New("mlbstatsapi only supports MLB")
	}
	url := fmt.Sprintf("%s/sports/1/players?season=%d", baseURL, time.Now().Year())
	body, err := p.get(ctx, url)
	if err != nil {
		return nil, err
	}
	var raw struct {
		People []struct {
			ID                int    `json:"id"`
			FullName          string `json:"fullName"`
			FirstName         string `json:"firstName"`
			LastName          string `json:"lastName"`
			PrimaryPosition   struct {
				Abbreviation string `json:"abbreviation"`
			} `json:"primaryPosition"`
			CurrentTeam struct {
				Abbreviation string `json:"abbreviation"`
			} `json:"currentTeam"`
			PrimaryNumber string `json:"primaryNumber"`
			Active        bool   `json:"active"`
			Height        string `json:"height"`
			Weight        int    `json:"weight"`
		} `json:"people"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	out := make([]provider.Player, 0, len(raw.People))
	for _, p := range raw.People {
		number, _ := strconv.Atoi(p.PrimaryNumber)
		out = append(out, provider.Player{
			ProviderPlayerID:  strconv.Itoa(p.ID),
			FullName:          p.FullName,
			FirstName:         p.FirstName,
			LastName:          p.LastName,
			Position:          p.PrimaryPosition.Abbreviation,
			EligiblePositions: []string{p.PrimaryPosition.Abbreviation},
			NFLTeam:           p.CurrentTeam.Abbreviation,
			JerseyNumber:      &number,
			Status:            ifThen(p.Active, "Active", "Inactive"),
			WeightLbs:         &p.Weight,
		})
	}
	return out, nil
}

// SyncSchedule pulls the season schedule, returning all regular-season games
// flattened into provider.Game.
func (p *Provider) SyncSchedule(ctx context.Context, sport provider.Sport, season int) ([]provider.Game, error) {
	url := fmt.Sprintf("%s/schedule?sportId=1&season=%d&gameType=R", baseURL, season)
	body, err := p.get(ctx, url)
	if err != nil {
		return nil, err
	}
	var raw struct {
		Dates []struct {
			Games []struct {
				GamePk     int    `json:"gamePk"`
				GameDate   string `json:"gameDate"`
				DetailedState string `json:"status.detailedState"`
				Teams      struct {
					Home struct {
						Team struct {
							Abbr string `json:"abbreviation"`
						} `json:"team"`
						Score *int `json:"score"`
					} `json:"home"`
					Away struct {
						Team struct {
							Abbr string `json:"abbreviation"`
						} `json:"team"`
						Score *int `json:"score"`
					} `json:"away"`
				} `json:"teams"`
			} `json:"games"`
		} `json:"dates"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	out := []provider.Game{}
	for _, d := range raw.Dates {
		for _, g := range d.Games {
			out = append(out, provider.Game{
				ProviderGameID: strconv.Itoa(g.GamePk),
				Season:         season,
				HomeTeam:       g.Teams.Home.Team.Abbr,
				AwayTeam:       g.Teams.Away.Team.Abbr,
				KickoffISO:     g.GameDate,
				Status:         g.DetailedState,
				HomeScore:      g.Teams.Home.Score,
				AwayScore:      g.Teams.Away.Score,
			})
		}
	}
	return out, nil
}

// SyncWeekStats / SyncInjuries / SyncTrending: not yet implemented.
func (p *Provider) SyncWeekStats(_ context.Context, _ provider.Sport, _, _ int) ([]provider.StatLine, error) {
	return nil, nil
}

func (p *Provider) SyncInjuries(_ context.Context, _ provider.Sport) ([]provider.InjuryUpdate, error) {
	return nil, nil
}

func (p *Provider) SyncTrending(_ context.Context, _ provider.Sport) ([]provider.TrendingPlayer, error) {
	return nil, nil
}

func (p *Provider) get(ctx context.Context, url string) ([]byte, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "LunarLeague/0.1")
	res, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("mlbstatsapi: HTTP %d", res.StatusCode)
	}
	return io.ReadAll(io.LimitReader(res.Body, 64*1024*1024))
}

func ifThen(cond bool, t, f string) string {
	if cond {
		return t
	}
	return f
}
