-- +goose Up
-- +goose StatementBegin
-- Enforce at most one open vote per league (race-safe vs app-level count check).
-- If duplicates exist from a race before this migration, keep the newest open vote.

UPDATE theme_votes v
SET status = 'cancelled', closed_at = now()
WHERE v.status = 'open'
  AND v.id NOT IN (
    SELECT DISTINCT ON (league_id) id
    FROM theme_votes
    WHERE status = 'open'
    ORDER BY league_id, created_at DESC
  );

-- 007 must stay non-unique (theme_votes_league_open_idx). Some deploys briefly had a
-- unique index in 007; drop either name before creating the canonical partial unique index.
DROP INDEX IF EXISTS theme_votes_league_open_idx;
DROP INDEX IF EXISTS theme_votes_one_open_per_league;
CREATE UNIQUE INDEX theme_votes_one_open_per_league
    ON theme_votes (league_id) WHERE status = 'open';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS theme_votes_one_open_per_league;
CREATE INDEX IF NOT EXISTS theme_votes_league_open_idx ON theme_votes (league_id) WHERE status = 'open';

-- +goose StatementEnd
