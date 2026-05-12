// Package sleeper is the free default DataProvider, backed by the Sleeper
// public API at https://api.sleeper.app/v1/.
//
// Sleeper is unauthenticated, generous on rate limits (~1000 req/min), and
// covers NFL + NBA player universes, weekly stats, projections, injuries,
// schedules, and trending adds/drops. It does not require an API key.
package sleeper

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/bulbousoars/lunarleague/apps/api/internal/provider"
)

const baseURL = "https://api.sleeper.app/v1"

type Provider struct {
	client *http.Client
}

func New() *Provider {
	return &Provider{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *Provider) Name() string { return "sleeper" }

// --- Players ---

// SyncPlayers fetches the full player universe. Sleeper exposes this at
// /players/{sport}; the response is a large object keyed by player_id. NFL
// is ~5 MiB so we don't call this often.
func (p *Provider) SyncPlayers(ctx context.Context, sport provider.Sport) ([]provider.Player, error) {
	if sport.Code != "nfl" && sport.Code != "nba" {
		return nil, fmt.Errorf("sleeper provider does not yet support sport %q", sport.Code)
	}
	url := fmt.Sprintf("%s/players/%s", baseURL, sport.Code)
	body, err := p.get(ctx, url)
	if err != nil {
		return nil, err
	}
	var raw map[string]rawPlayer
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	out := make([]provider.Player, 0, len(raw))
	for id, r := range raw {
		out = append(out, r.toPlayer(id, sport.Code))
	}
	return out, nil
}

type rawPlayer struct {
	FullName        string   `json:"full_name"`
	FirstName       string   `json:"first_name"`
	LastName        string   `json:"last_name"`
	Position        string   `json:"position"`
	FantasyPositions []string `json:"fantasy_positions"`
	Team            string   `json:"team"`
	Number          *int     `json:"number"`
	Status          string   `json:"status"`
	InjuryStatus    string   `json:"injury_status"`
	InjuryBodyPart  string   `json:"injury_body_part"`
	InjuryNotes     string   `json:"injury_notes"`
	Age             *int     `json:"age"`
	Height          string   `json:"height"`
	Weight          string   `json:"weight"`
	College         string   `json:"college"`
	YearsExp        *int     `json:"years_exp"`
}

func (r rawPlayer) toPlayer(id, sportCode string) provider.Player {
	heightIn := parseHeight(r.Height)
	weightLb := parseInt(r.Weight)
	return provider.Player{
		ProviderPlayerID:  id,
		FullName:          r.FullName,
		FirstName:         r.FirstName,
		LastName:          r.LastName,
		Position:          r.Position,
		EligiblePositions: r.FantasyPositions,
		NFLTeam:           r.Team,
		JerseyNumber:      r.Number,
		Status:            r.Status,
		InjuryStatus:      r.InjuryStatus,
		InjuryBodyPart:    r.InjuryBodyPart,
		InjuryNotes:       r.InjuryNotes,
		Age:               r.Age,
		HeightInches:      heightIn,
		WeightLbs:         weightLb,
		College:           r.College,
		YearsExp:          r.YearsExp,
		HeadshotURL:       fmt.Sprintf("https://sleepercdn.com/content/%s/players/thumb/%s.jpg", sportCode, id),
	}
}

// --- Stats ---

// SyncWeekStats: Sleeper exposes /stats/{sport}/regular/{season}/{week}.
func (p *Provider) SyncWeekStats(ctx context.Context, sport provider.Sport, season, week int) ([]provider.StatLine, error) {
	if sport.Code != "nfl" && sport.Code != "nba" {
		return nil, fmt.Errorf("sleeper: weekly stats not available for sport %q", sport.Code)
	}
	url := fmt.Sprintf("%s/stats/%s/regular/%d/%d", baseURL, sport.Code, season, week)
	body, err := p.get(ctx, url)
	if err != nil {
		return nil, err
	}
	var raw map[string]map[string]float64
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	out := make([]provider.StatLine, 0, len(raw))
	for id, stats := range raw {
		out = append(out, provider.StatLine{
			ProviderPlayerID: id,
			Stats:            stats,
			IsFinal:          true, // Sleeper's weekly endpoint is post-final
		})
	}
	return out, nil
}

// --- Injuries ---

// SyncInjuries: there's no dedicated endpoint, but the players blob includes
// injury_status. We surface that as updates by re-running SyncPlayers and
// projecting only the injury fields.
func (p *Provider) SyncInjuries(ctx context.Context, sport provider.Sport) ([]provider.InjuryUpdate, error) {
	players, err := p.SyncPlayers(ctx, sport)
	if err != nil {
		return nil, err
	}
	out := make([]provider.InjuryUpdate, 0)
	for _, pl := range players {
		if pl.InjuryStatus == "" && pl.InjuryBodyPart == "" {
			continue
		}
		out = append(out, provider.InjuryUpdate{
			ProviderPlayerID: pl.ProviderPlayerID,
			Status:           pl.InjuryStatus,
			BodyPart:         pl.InjuryBodyPart,
			Notes:            pl.InjuryNotes,
		})
	}
	return out, nil
}

// --- Schedule ---

// Sleeper exposes /schedule/{sport}/regular/{season}.
func (p *Provider) SyncSchedule(ctx context.Context, sport provider.Sport, season int) ([]provider.Game, error) {
	url := fmt.Sprintf("%s/schedule/%s/regular/%d", baseURL, sport.Code, season)
	body, err := p.get(ctx, url)
	if err != nil {
		return nil, err
	}
	type rawGame struct {
		Week   int    `json:"week"`
		Home   string `json:"home"`
		Away   string `json:"away"`
		Date   string `json:"date"`
		Time   string `json:"time"`
		Status string `json:"status"`
		HomeS  *int   `json:"home_score"`
		AwayS  *int   `json:"away_score"`
		ID     string `json:"game_id"`
	}
	var raw []rawGame
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	out := make([]provider.Game, 0, len(raw))
	for _, g := range raw {
		out = append(out, provider.Game{
			ProviderGameID: g.ID,
			Season:         season,
			Week:           g.Week,
			HomeTeam:       g.Home,
			AwayTeam:       g.Away,
			KickoffISO:     g.Date + "T" + g.Time + "Z",
			Status:         g.Status,
			HomeScore:      g.HomeS,
			AwayScore:      g.AwayS,
		})
	}
	return out, nil
}

// --- Trending ---

func (p *Provider) SyncTrending(ctx context.Context, sport provider.Sport) ([]provider.TrendingPlayer, error) {
	type entry struct {
		PlayerID string `json:"player_id"`
		Count    int    `json:"count"`
	}
	out := []provider.TrendingPlayer{}
	for _, dir := range []string{"add", "drop"} {
		url := fmt.Sprintf("%s/players/%s/trending/%s?lookback_hours=24&limit=50", baseURL, sport.Code, dir)
		body, err := p.get(ctx, url)
		if err != nil {
			continue
		}
		var raw []entry
		if err := json.Unmarshal(body, &raw); err != nil {
			continue
		}
		for _, e := range raw {
			out = append(out, provider.TrendingPlayer{
				ProviderPlayerID: e.PlayerID,
				Count:            e.Count,
				Direction:        dir,
			})
		}
	}
	return out, nil
}

// --- internals ---

func (p *Provider) get(ctx context.Context, url string) ([]byte, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "LunarLeague/0.1 (+https://github.com/bulbousoars/LunarLeague)")
	res, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusTooManyRequests {
		return nil, errors.New("sleeper: rate limited")
	}
	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("sleeper: HTTP %d", res.StatusCode)
	}
	return io.ReadAll(io.LimitReader(res.Body, 32*1024*1024))
}

func parseHeight(h string) *int {
	// Sleeper sends inches as a string sometimes, "6'2\"" other times. Best
	// effort.
	if h == "" {
		return nil
	}
	n := parseInt(h)
	if n != nil {
		return n
	}
	// Format like "72" or "6'2"
	feet, inches := 0, 0
	for i, r := range h {
		if r == '\'' {
			feet = atoi(h[:i])
		} else if r == '"' && feet > 0 {
			inches = atoi(h[len(string(h[0]))+1 : i])
			break
		}
	}
	if feet > 0 {
		v := feet*12 + inches
		return &v
	}
	return nil
}

func parseInt(s string) *int {
	if s == "" {
		return nil
	}
	n := atoi(s)
	if n == 0 {
		return nil
	}
	return &n
}

func atoi(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			continue
		}
		n = n*10 + int(r-'0')
	}
	return n
}
