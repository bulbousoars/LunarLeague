-- +goose Up
-- +goose StatementBegin

ALTER TABLE league_settings
    ADD COLUMN IF NOT EXISTS theme_modifiers jsonb NOT NULL DEFAULT '{}'::jsonb;

COMMENT ON COLUMN league_settings.theme_modifiers IS
    'Per-slug theme toggles for schedule_type theme_ball. Keys are theme slugs; values {"enabled":bool,...}';

CREATE TABLE theme_modifier_audit (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    league_id   uuid NOT NULL REFERENCES leagues(id) ON DELETE CASCADE,
    actor_id    uuid NOT NULL REFERENCES users(id),
    slug        text NOT NULL,
    old_enabled boolean,
    new_enabled boolean NOT NULL,
    source      text NOT NULL, -- commissioner | vote
    vote_id     uuid,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX theme_modifier_audit_league_idx ON theme_modifier_audit (league_id, created_at DESC);

CREATE TABLE theme_votes (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    league_id   uuid NOT NULL REFERENCES leagues(id) ON DELETE CASCADE,
    slug        text NOT NULL,
    action      text NOT NULL CHECK (action IN ('enable', 'disable')),
    opened_by   uuid NOT NULL REFERENCES users(id),
    opens_at    timestamptz NOT NULL DEFAULT now(),
    closes_at   timestamptz NOT NULL,
    status      text NOT NULL DEFAULT 'open'
        CHECK (status IN ('open', 'passed', 'failed', 'cancelled')),
    yes_count   int NOT NULL DEFAULT 0,
    no_count    int NOT NULL DEFAULT 0,
    created_at  timestamptz NOT NULL DEFAULT now(),
    closed_at   timestamptz
);
CREATE INDEX theme_votes_league_open_idx ON theme_votes (league_id) WHERE status = 'open';

CREATE TABLE theme_vote_ballots (
    vote_id     uuid NOT NULL REFERENCES theme_votes(id) ON DELETE CASCADE,
    team_id     uuid NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id     uuid NOT NULL REFERENCES users(id),
    yes         boolean NOT NULL,
    cast_at     timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (vote_id, team_id)
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS theme_vote_ballots;
DROP TABLE IF EXISTS theme_votes;
DROP TABLE IF EXISTS theme_modifier_audit;
ALTER TABLE league_settings DROP COLUMN IF EXISTS theme_modifiers;

-- +goose StatementEnd
