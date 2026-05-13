package scoring

import (
	"context"
	"encoding/json"

	"github.com/bulbousoars/lunarleague/apps/api/internal/provider"
	"github.com/bulbousoars/lunarleague/apps/api/internal/statsnorm"
)

func (s *Service) upsertStatLines(
	ctx context.Context,
	sportID int,
	sportCode, providerName string,
	season, week int,
	lines []provider.StatLine,
) error {
	for _, sl := range lines {
		normalized := statsnorm.NormalizeStatMap(sportCode, providerName, sl.Stats)
		body, err := json.Marshal(normalized)
		if err != nil {
			return err
		}
		_, err = s.pool.Exec(ctx, `
			INSERT INTO player_stats (sport_id, season, week, player_id, stats, is_final)
			SELECT $1, $2, $3, p.id, $5::jsonb, $6
			FROM players p
			WHERE p.sport_id = $1 AND p.provider_player_id = $4 AND p.provider = $7
			ON CONFLICT (sport_id, season, week, player_id) DO UPDATE
				SET stats = EXCLUDED.stats, is_final = EXCLUDED.is_final, updated_at = now()`,
			sportID, season, week, sl.ProviderPlayerID, string(body), sl.IsFinal, providerName)
		if err != nil {
			return err
		}
	}
	return nil
}
