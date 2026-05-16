package scoring

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/bulbousoars/lunarleague/apps/api/internal/db"
	"github.com/bulbousoars/lunarleague/apps/api/internal/scoring/themes"
	leaguethemes "github.com/bulbousoars/lunarleague/apps/api/internal/themes"
	"github.com/jackc/pgx/v5"
)

type leagueWeek struct {
	LeagueID     string
	Season       int
	Week         int
	SportID      int
	SportCode    string
	ScheduleType string
	ThemeRaw     []byte
	RulesRaw     []byte
}

// ScoreActiveWeeks recomputes lineup points and matchup scores for league-weeks
// that have player stat lines available.
func (s *Service) ScoreActiveWeeks(ctx context.Context) error {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT l.id, m.season, m.week, l.sport_id, sp.code,
		       ls.schedule_type, ls.theme_modifiers, sr.rules
		FROM leagues l
		JOIN league_settings ls ON ls.league_id = l.id
		JOIN sports sp ON sp.id = l.sport_id
		JOIN matchups m ON m.league_id = l.id
		JOIN scoring_rules sr ON sr.league_id = l.id
		WHERE l.status IN ('in_season', 'playoffs', 'drafting')
		  AND EXISTS (
		    SELECT 1 FROM player_stats ps
		    WHERE ps.sport_id = l.sport_id
		      AND ps.season = m.season
		      AND ps.week = m.week
		  )`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var targets []leagueWeek
	for rows.Next() {
		var lw leagueWeek
		if err := rows.Scan(&lw.LeagueID, &lw.Season, &lw.Week, &lw.SportID, &lw.SportCode,
			&lw.ScheduleType, &lw.ThemeRaw, &lw.RulesRaw); err != nil {
			return err
		}
		targets = append(targets, lw)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, lw := range targets {
		if err := s.scoreLeagueWeek(ctx, lw); err != nil {
			return fmt.Errorf("score league %s week %d: %w", lw.LeagueID, lw.Week, err)
		}
	}
	return nil
}

func (s *Service) scoreLeagueWeek(ctx context.Context, lw leagueWeek) error {
	rules, err := parseRules(lw.RulesRaw)
	if err != nil {
		return err
	}
	themeCfg, err := leaguethemes.ParseConfig(lw.ThemeRaw)
	if err != nil {
		return err
	}

	wctx, err := s.buildWeekContext(ctx, lw, rules, themeCfg)
	if err != nil {
		return err
	}
	if len(wctx.Teams) == 0 {
		return nil
	}

	teamPoints := make(map[string]float64)
	teamBreakdown := make(map[string]themes.Breakdown)

	for teamID, team := range wctx.Teams {
		var total float64
		combined := themes.Breakdown{}
		for _, p := range team.Starters {
			ps := themes.ScorePlayer(wctx, p)
			total += ps.Points
			for pid, effects := range ps.Breakdown {
				combined[pid] = append(combined[pid], effects...)
			}
		}
		teamPoints[teamID] = round2(total)
		teamBreakdown[teamID] = combined
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for teamID, pts := range teamPoints {
		bdJSON, _ := json.Marshal(teamBreakdown[teamID])
		tag, err := tx.Exec(ctx, `
			UPDATE lineups SET points = $5, theme_breakdown = $6::jsonb
			WHERE team_id = $2 AND season = $3 AND week = $4`,
			lw.LeagueID, teamID, lw.Season, lw.Week, pts, string(bdJSON))
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			_, err = tx.Exec(ctx, `
				INSERT INTO lineups (league_id, team_id, season, week, points, theme_breakdown)
				VALUES ($1, $2, $3, $4, $5, $6::jsonb)`,
				lw.LeagueID, teamID, lw.Season, lw.Week, pts, string(bdJSON))
			if err != nil {
				return err
			}
		}
	}

	matchRows, err := tx.Query(ctx, `
		SELECT id, home_team_id, away_team_id FROM matchups
		WHERE league_id = $1 AND season = $2 AND week = $3`,
		lw.LeagueID, lw.Season, lw.Week)
	if err != nil {
		return err
	}
	defer matchRows.Close()

	for matchRows.Next() {
		var id, home, away string
		if err := matchRows.Scan(&id, &home, &away); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			UPDATE matchups SET home_score = $2, away_score = $3
			WHERE id = $1`,
			id, teamPoints[home], teamPoints[away])
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (s *Service) buildWeekContext(ctx context.Context, lw leagueWeek, rules Rules, themeCfg leaguethemes.Config) (*themes.WeekContext, error) {
	wctx := &themes.WeekContext{
		LeagueID:     lw.LeagueID,
		Season:       lw.Season,
		Week:         lw.Week,
		SportCode:    lw.SportCode,
		ScheduleType: lw.ScheduleType,
		Config:       themeCfg,
		Rules:        rules,
		Teams:        map[string]*themes.Team{},
		NFLTeamWon:   map[string]bool{},
	}

	if err := s.loadNFLWins(ctx, lw, wctx); err != nil {
		return nil, err
	}

	teamRows, err := s.pool.Query(ctx,
		`SELECT id FROM teams WHERE league_id = $1`, lw.LeagueID)
	if err != nil {
		return nil, err
	}
	defer teamRows.Close()

	for teamRows.Next() {
		var teamID string
		if err := teamRows.Scan(&teamID); err != nil {
			return nil, err
		}
		starters, err := s.loadStarters(ctx, lw, teamID)
		if err != nil {
			return nil, err
		}
		if len(starters) == 0 {
			continue
		}
		wctx.Teams[teamID] = &themes.Team{ID: teamID, Starters: starters}
	}
	return wctx, nil
}

func (s *Service) loadNFLWins(ctx context.Context, lw leagueWeek, wctx *themes.WeekContext) error {
	if lw.SportCode != "nfl" {
		return nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT home_team, away_team, home_score, away_score
		FROM games
		WHERE sport_id = $1 AND season = $2 AND week = $3
		  AND status = 'final'
		  AND home_score IS NOT NULL AND away_score IS NOT NULL`,
		lw.SportID, lw.Season, lw.Week)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var home, away string
		var hs, as int
		if err := rows.Scan(&home, &away, &hs, &as); err != nil {
			return err
		}
		if hs > as {
			wctx.NFLTeamWon[home] = true
		} else if as > hs {
			wctx.NFLTeamWon[away] = true
		}
	}
	return rows.Err()
}

func (s *Service) loadStarters(ctx context.Context, lw leagueWeek, teamID string) ([]themes.Player, error) {
	playerIDs, err := s.resolveStarterIDs(ctx, lw, teamID)
	if err != nil {
		return nil, err
	}
	if len(playerIDs) == 0 {
		return nil, nil
	}

	out := make([]themes.Player, 0, len(playerIDs))
	for _, pid := range playerIDs {
		var p themes.Player
		p.ID = pid
		p.TeamID = teamID
		var pos, nfl *string
		var weight, height, jersey, years *int
		var first, last *string
		err := s.pool.QueryRow(ctx, `
			SELECT position, nfl_team, weight_lbs, height_inches, jersey_number, years_exp,
			       first_name, last_name
			FROM players WHERE id = $1`, pid).Scan(
			&pos, &nfl, &weight, &height, &jersey, &years, &first, &last)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			return nil, err
		}
		if pos != nil {
			p.Position = *pos
		}
		if nfl != nil {
			p.NFLTeam = *nfl
		}
		p.WeightLbs = weight
		p.HeightIn = height
		p.Jersey = jersey
		p.YearsExp = years
		if first != nil {
			p.FirstName = *first
		}
		if last != nil {
			p.LastName = *last
		}

		var statsRaw []byte
		err = s.pool.QueryRow(ctx, `
			SELECT stats FROM player_stats
			WHERE sport_id = $1 AND season = $2 AND week = $3 AND player_id = $4`,
			lw.SportID, lw.Season, lw.Week, pid).Scan(&statsRaw)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		if len(statsRaw) > 0 {
			_ = json.Unmarshal(statsRaw, &p.Stats)
		}
		out = append(out, p)
	}
	return out, nil
}

func (s *Service) resolveStarterIDs(ctx context.Context, lw leagueWeek, teamID string) ([]string, error) {
	var startersRaw []byte
	err := s.pool.QueryRow(ctx, `
		SELECT starters FROM lineups
		WHERE team_id = $1 AND season = $2 AND week = $3`,
		teamID, lw.Season, lw.Week).Scan(&startersRaw)
	if err == nil && len(startersRaw) > 2 {
		var slots []struct {
			PlayerID string `json:"player_id"`
		}
		if json.Unmarshal(startersRaw, &slots) == nil {
			ids := make([]string, 0, len(slots))
			for _, sl := range slots {
				if sl.PlayerID != "" {
					ids = append(ids, sl.PlayerID)
				}
			}
			if len(ids) > 0 {
				return ids, nil
			}
		}
	}

	rows, err := s.pool.Query(ctx, `
		SELECT player_id FROM rosters
		WHERE team_id = $1 AND slot NOT IN ('BN', 'IR')`,
		teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
