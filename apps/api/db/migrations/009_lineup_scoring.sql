-- +goose Up
-- +goose StatementBegin

ALTER TABLE lineups
    ADD COLUMN IF NOT EXISTS points numeric(8,2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS theme_breakdown jsonb NOT NULL DEFAULT '{}'::jsonb;

COMMENT ON COLUMN lineups.points IS 'Fantasy points for this team-week after base rules and theme modifiers.';
COMMENT ON COLUMN lineups.theme_breakdown IS 'Per-slug theme effects applied when points were computed.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE lineups
    DROP COLUMN IF EXISTS theme_breakdown,
    DROP COLUMN IF EXISTS points;

-- +goose StatementEnd
