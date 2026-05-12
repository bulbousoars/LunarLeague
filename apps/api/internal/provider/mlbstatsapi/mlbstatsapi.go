// Package mlbstatsapi is a DataProvider implementation backed by the official
// MLB Stats API at https://statsapi.mlb.com/api/v1/.
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
	return &Provider{client: &http.Client{Timeout: 45 * time.Second}}
}

func (p *Provider) Name() string { return "mlbstatsapi" }

func (p *Provider) SyncPlayers(ctx context.Context, sport provider.Sport) ([]provider.Player, error) {
	if sport.Code != "mlb" {
		return nil, errors.New("mlbstatsapi only supports MLB")
	}
	yr := time.Now().Year()
	url := fmt.Sprintf("%s/sports/1/players?season=%d", baseURL, yr)
	body, err := p.get(ctx, url)
	if err != nil {
		return nil, err
	}
	var raw struct {
		People []struct {
			ID              int    `json:"id"`
			FullName        string `json:"fullName"`
			FirstName       string `json:"firstName"`
			LastName        string `json:"lastName"`
			PrimaryPosition struct {
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
	for _, pl := range raw.People {
		number, _ := strconv.Atoi(pl.PrimaryNumber)
		out = append(out, provider.Player{
			ProviderPlayerID:  strconv.Itoa(pl.ID),
			FullName:          pl.FullName,
			FirstName:         pl.FirstName,
			LastName:          pl.LastName,
			Position:          pl.PrimaryPosition.Abbreviation,
			EligiblePositions: []string{pl.PrimaryPosition.Abbreviation},
			NFLTeam:           pl.CurrentTeam.Abbreviation,
			JerseyNumber:      &number,
			Status:            ifThen(pl.Active, "Active", "Inactive"),
			WeightLbs:         &pl.Weight,
		})
	}
	return out, nil
}

// SyncSchedule pulls the season schedule, returning all regular-season games
// flattened into provider.Game. Week is a YYYYMMDD slate key for daily sports.
func (p *Provider) SyncSchedule(ctx context.Context, sport provider.Sport, season int) ([]provider.Game, error) {
	if sport.Code != "mlb" {
		return nil, errors.New("mlbstatsapi only supports MLB")
	}
	url := fmt.Sprintf("%s/schedule?sportId=1&season=%d&gameType=R", baseURL, season)
	body, err := p.get(ctx, url)
	if err != nil {
		return nil, err
	}
	var raw struct {
		Dates []struct {
			Games []struct {
				GamePk   int    `json:"gamePk"`
				GameDate string `json:"gameDate"`
				Status   struct {
					DetailedState string `json:"detailedState"`
				} `json:"status"`
				Teams struct {
					Home struct {
						Team struct {
							Abbreviation string `json:"abbreviation"`
						} `json:"team"`
						Score *int `json:"score"`
					} `json:"home"`
					Away struct {
						Team struct {
							Abbreviation string `json:"abbreviation"`
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
			t, err := time.Parse(time.RFC3339, g.GameDate)
			if err != nil {
				continue
			}
			utc := t.UTC()
			weekKey := utc.Year()*10000 + int(utc.Month())*100 + utc.Day()
			out = append(out, provider.Game{
				ProviderGameID: strconv.Itoa(g.GamePk),
				Season:         season,
				Week:           weekKey,
				HomeTeam:       g.Teams.Home.Team.Abbreviation,
				AwayTeam:       g.Teams.Away.Team.Abbreviation,
				KickoffISO:     g.GameDate,
				Status:         g.Status.DetailedState,
				HomeScore:      g.Teams.Home.Score,
				AwayScore:      g.Teams.Away.Score,
			})
		}
	}
	return out, nil
}

func (p *Provider) SyncWeekStats(ctx context.Context, sport provider.Sport, season, week int) ([]provider.StatLine, error) {
	if sport.Code != "mlb" {
		return nil, nil
	}
	if week < 20000000 {
		return nil, nil
	}
	y := week / 10000
	md := week % 10000
	mo := md / 100
	da := md % 100
	date := fmt.Sprintf("%04d-%02d-%02d", y, mo, da)
	return p.statsForDate(ctx, date)
}

func (p *Provider) SyncInjuries(context.Context, provider.Sport) ([]provider.InjuryUpdate, error) {
	return nil, nil
}

func (p *Provider) SyncTrending(context.Context, provider.Sport) ([]provider.TrendingPlayer, error) {
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
