// Package sportsdataio implements provider.DataProvider against SportsData.io
// v3 JSON feeds (requires SPORTSDATAIO_API_KEY and an appropriate subscription).
package sportsdataio

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/bulbousoars/lunarleague/apps/api/internal/provider"
)

const host = "https://api.sportsdata.io/v3"

type Provider struct {
	key    string
	client *http.Client
}

func New(apiKey string) *Provider {
	return &Provider{
		key:    strings.TrimSpace(apiKey),
		client: &http.Client{Timeout: 45 * time.Second},
	}
}

func (p *Provider) Name() string { return "sportsdataio" }

func (p *Provider) leaguePath(sport provider.Sport) (string, error) {
	switch sport.Code {
	case "nfl":
		return "nfl", nil
	case "nba":
		return "nba", nil
	case "mlb":
		return "mlb", nil
	default:
		return "", fmt.Errorf("sportsdataio: unsupported sport %q", sport.Code)
	}
}

func (p *Provider) withKey(path string) (string, error) {
	if p.key == "" {
		return "", errors.New("sportsdataio: missing API key")
	}
	u, err := url.Parse(path)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("key", p.key)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (p *Provider) get(ctx context.Context, fullURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "LunarLeague/0.1 (+https://github.com/bulbousoars/LunarLeague)")
	res, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusTooManyRequests {
		return nil, errors.New("sportsdataio: rate limited")
	}
	if res.StatusCode >= 400 {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return nil, fmt.Errorf("sportsdataio: HTTP %d: %s", res.StatusCode, string(b))
	}
	return io.ReadAll(io.LimitReader(res.Body, 64*1024*1024))
}

// --- Players ---

type sdPlayer struct {
	PlayerID   int    `json:"PlayerID"`
	FirstName  string `json:"FirstName"`
	LastName   string `json:"LastName"`
	Name       string `json:"Name"`
	Position   string `json:"Position"`
	Team       string `json:"Team"`
	Status     string `json:"Status"`
	InjuryBodyPart   string `json:"InjuryBodyPart"`
	InjuryStartDate string `json:"InjuryStartDate"`
	InjuryNotes      string `json:"InjuryNotes"`
}

func (p *Provider) SyncPlayers(ctx context.Context, sport provider.Sport) ([]provider.Player, error) {
	lg, err := p.leaguePath(sport)
	if err != nil {
		return nil, err
	}
	u, err := p.withKey(fmt.Sprintf("%s/%s/scores/json/Players", host, lg))
	if err != nil {
		return nil, err
	}
	body, err := p.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var raw []sdPlayer
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("sportsdataio players decode: %w", err)
	}
	out := make([]provider.Player, 0, len(raw))
	for _, r := range raw {
		fn := strings.TrimSpace(r.FirstName)
		ln := strings.TrimSpace(r.LastName)
		full := strings.TrimSpace(r.Name)
		if full == "" {
			full = strings.TrimSpace(fn + " " + ln)
		}
		pos := strings.TrimSpace(r.Position)
		elig := []string{}
		if pos != "" {
			elig = append(elig, pos)
		}
		if full == "" {
			full = strconv.Itoa(r.PlayerID)
		}
		out = append(out, provider.Player{
			ProviderPlayerID:  strconv.Itoa(r.PlayerID),
			FullName:          full,
			FirstName:         fn,
			LastName:          ln,
			Position:          pos,
			EligiblePositions: elig,
			NFLTeam:           strings.TrimSpace(r.Team),
			Status:            strings.TrimSpace(r.Status),
			InjuryStatus:      strings.TrimSpace(r.Status),
			InjuryBodyPart:    strings.TrimSpace(r.InjuryBodyPart),
			InjuryNotes:       strings.TrimSpace(r.InjuryNotes),
		})
	}
	return out, nil
}

// --- Stats ---

func (p *Provider) SyncWeekStats(ctx context.Context, sport provider.Sport, season, week int) ([]provider.StatLine, error) {
	lg, err := p.leaguePath(sport)
	if err != nil {
		return nil, err
	}
	u, err := p.withKey(fmt.Sprintf("%s/%s/stats/json/PlayerGameStatsByWeek/%d/%d", host, lg, season, week))
	if err != nil {
		return nil, err
	}
	body, err := p.get(ctx, u)
	if err != nil {
		return nil, err
	}
	switch sport.Code {
	case "nfl":
		return decodeNFLPlayerGames(body)
	case "nba":
		return decodeNBAPlayerGames(body)
	case "mlb":
		return decodeMLBPlayerGames(body)
	default:
		return nil, nil
	}
}

func decodeNFLPlayerGames(body []byte) ([]provider.StatLine, error) {
	rows, err := statsRowsFromJSON(body)
	if err != nil {
		return nil, err
	}
	out := make([]provider.StatLine, 0, len(rows))
	for _, row := range rows {
		pid := anyToInt(row["PlayerID"])
		if pid == 0 {
			continue
		}
		stats := map[string]float64{}
		add := func(k string, keys ...string) {
			for _, alt := range keys {
				if v, ok := num(row[alt]); ok {
					stats[k] += v
					return
				}
			}
		}
		add("pass_yd", "PassingYards", "PassingYardage")
		add("pass_td", "PassingTouchdowns")
		add("pass_int", "PassingInterceptions")
		add("pass_2pt", "TwoPointConversionPasses")
		add("rush_yd", "RushingYards")
		add("rush_td", "RushingTouchdowns")
		add("rush_2pt", "TwoPointConversionRuns")
		add("rec", "Receptions")
		add("rec_yd", "ReceivingYards")
		add("rec_td", "ReceivingTouchdowns")
		add("rec_2pt", "TwoPointConversionReceptions")
		add("fum_lost", "FumblesLost")
		add("fgm_0_19", "FieldGoalsMade0to19")
		add("fgm_20_29", "FieldGoalsMade20to29")
		add("fgm_30_39", "FieldGoalsMade30to39")
		add("fgm_40_49", "FieldGoalsMade40to49")
		add("fgm_50p", "FieldGoalsMade50Plus")
		add("fgmiss", "FieldGoalsMissed", "FieldGoalsHadBlocked")
		add("xpm", "ExtraPointsMade")
		add("xpmiss", "ExtraPointsMissed", "ExtraPointsHadBlocked")
		add("def_int", "Interceptions")
		add("def_fr", "FumblesRecovered")
		add("def_sack", "Sacks")
		add("def_td", "DefensiveTouchdowns")
		add("def_safe", "Safeties")
		add("def_block_kick", "BlockedKicks", "BlockedKickReturns")
		add("st_td", "SpecialTeamsTouchdowns", "ReturnTouchdowns")
		out = append(out, provider.StatLine{
			ProviderPlayerID: strconv.Itoa(pid),
			Stats:            stats,
			IsFinal:          false,
		})
	}
	return out, nil
}

func statsRowsFromJSON(body []byte) ([]map[string]any, error) {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return nil, nil
	}
	var arr []map[string]any
	if err := json.Unmarshal(body, &arr); err == nil {
		return arr, nil
	}
	var obj map[string]map[string]any
	if err := json.Unmarshal(body, &obj); err == nil {
		rows := make([]map[string]any, 0, len(obj))
		for _, v := range obj {
			rows = append(rows, v)
		}
		return rows, nil
	}
	return nil, fmt.Errorf("sportsdataio stats: unexpected JSON shape")
}

func decodeNBAPlayerGames(body []byte) ([]provider.StatLine, error) {
	rows, err := statsRowsFromJSON(body)
	if err != nil {
		return nil, err
	}
	out := make([]provider.StatLine, 0, len(rows))
	for _, row := range rows {
		pid := anyToInt(row["PlayerID"])
		if pid == 0 {
			continue
		}
		stats := map[string]float64{}
		add := func(k string, keys ...string) {
			for _, alt := range keys {
				if v, ok := num(row[alt]); ok {
					stats[k] += v
					return
				}
			}
		}
		add("pts", "Points")
		add("reb", "Rebounds")
		add("ast", "Assists")
		add("stl", "Steals")
		add("blk", "BlockedShots", "Blocks")
		add("to", "Turnovers")
		add("fg3m", "ThreePointersMade")
		out = append(out, provider.StatLine{
			ProviderPlayerID: strconv.Itoa(pid),
			Stats:            stats,
			IsFinal:          false,
		})
	}
	return out, nil
}

func decodeMLBPlayerGames(body []byte) ([]provider.StatLine, error) {
	rows, err := statsRowsFromJSON(body)
	if err != nil {
		return nil, err
	}
	out := make([]provider.StatLine, 0, len(rows))
	for _, row := range rows {
		pid := anyToInt(row["PlayerID"])
		if pid == 0 {
			continue
		}
		stats := map[string]float64{}
		add := func(k string, keys ...string) {
			for _, alt := range keys {
				if v, ok := num(row[alt]); ok {
					stats[k] += v
					return
				}
			}
		}
		add("hit", "Hits")
		add("run", "Runs")
		add("rbi", "RunsBattedIn")
		add("hr", "HomeRuns")
		add("sb", "StolenBases")
		add("bb", "Walks")
		add("k", "Strikeouts")
		if v, ok := row["InningsPitchedDecimal"]; ok {
			if f, ok2 := num(v); ok2 {
				stats["ip"] += f
			}
		}
		if v, ok := row["InningsPitched"]; ok {
			if f, ok2 := num(v); ok2 {
				stats["ip"] += f
			} else if s, ok2 := v.(string); ok2 {
				stats["ip"] += parseBaseballIPDisplay(s)
			}
		}
		add("er", "EarnedRuns")
		add("k_p", "PitchingStrikeouts", "PitchingKs")
		add("win", "Wins")
		add("loss", "Losses")
		add("sv", "Saves")
		add("qs", "QualityStarts")
		out = append(out, provider.StatLine{
			ProviderPlayerID: strconv.Itoa(pid),
			Stats:            stats,
			IsFinal:          false,
		})
	}
	return out, nil
}

func anyToInt(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case json.Number:
		i, _ := t.Int64()
		return int(i)
	case string:
		i, _ := strconv.Atoi(strings.TrimSpace(t))
		return i
	default:
		return 0
	}
}

func num(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		return f, err == nil
	default:
		return 0, false
	}
}

// --- Injuries ---

func (p *Provider) SyncInjuries(ctx context.Context, sport provider.Sport) ([]provider.InjuryUpdate, error) {
	players, err := p.SyncPlayers(ctx, sport)
	if err != nil {
		return nil, err
	}
	out := make([]provider.InjuryUpdate, 0)
	for _, pl := range players {
		if pl.InjuryStatus == "" && pl.InjuryBodyPart == "" && pl.InjuryNotes == "" {
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

type rawSDScheduleGame struct {
	GlobalGameID int    `json:"GlobalGameID"`
	GameKey      string `json:"GameKey"`
	Season       int    `json:"Season"`
	Week         *int   `json:"Week"`
	HomeTeam     string `json:"HomeTeam"`
	AwayTeam     string `json:"AwayTeam"`
	DateTime     string `json:"DateTime"`
	Day          string `json:"Day"`
	Status       string `json:"Status"`
	HomeScore    *int   `json:"HomeScore"`
	AwayScore    *int   `json:"AwayScore"`
}

func (p *Provider) SyncSchedule(ctx context.Context, sport provider.Sport, season int) ([]provider.Game, error) {
	lg, err := p.leaguePath(sport)
	if err != nil {
		return nil, err
	}
	uPath := fmt.Sprintf("%s/%s/scores/json/Schedules/%d", host, lg, season)
	if sport.Code == "mlb" {
		uPath = fmt.Sprintf("%s/mlb/scores/json/Games/%d", host, season)
	}
	u, err := p.withKey(uPath)
	if err != nil {
		return nil, err
	}
	body, err := p.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var raw []rawSDScheduleGame
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("sportsdataio schedule decode: %w", err)
	}
	out := make([]provider.Game, 0, len(raw))
	for _, g := range raw {
		week := 0
		if g.Week != nil {
			week = *g.Week
		} else if g.Day != "" {
			week = dateKeyFromYMD(g.Day)
		}
		kick := g.DateTime
		if kick == "" && g.Day != "" {
			kick = g.Day + "T20:05:00Z"
		}
		gid := g.GameKey
		if gid == "" {
			gid = strconv.Itoa(g.GlobalGameID)
		}
		out = append(out, provider.Game{
			ProviderGameID: gid,
			Season:         season,
			Week:           week,
			HomeTeam:       g.HomeTeam,
			AwayTeam:       g.AwayTeam,
			KickoffISO:     kick,
			Status:         g.Status,
			HomeScore:      g.HomeScore,
			AwayScore:      g.AwayScore,
		})
	}
	return out, nil
}

func dateKeyFromYMD(day string) int {
	// day is YYYY-MM-DD
	t, err := time.Parse("2006-01-02", strings.TrimSpace(day))
	if err != nil {
		return 0
	}
	return t.Year()*10000 + int(t.Month())*100 + t.Day()
}

// parseBaseballIPDisplay converts MLB-style innings strings like "6.1" (6⅓ IP)
// into fractional innings as a float64.
func parseBaseballIPDisplay(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	dot := strings.IndexByte(s, '.')
	if dot < 0 {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0
		}
		return v
	}
	whole, err1 := strconv.Atoi(s[:dot])
	frac, err2 := strconv.Atoi(s[dot+1:])
	if err1 != nil || err2 != nil || frac < 0 || frac > 2 {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0
		}
		return v
	}
	outs := whole*3 + frac
	return float64(outs) / 3.0
}

// --- Trending ---

func (p *Provider) SyncTrending(context.Context, provider.Sport) ([]provider.TrendingPlayer, error) {
	return nil, nil
}
