-- +goose Up
-- +goose StatementBegin

-- Player universe is shared across leagues. Every player is scoped to a sport.
-- provider_player_id is the upstream key (e.g. Sleeper player_id) so we can
-- re-sync without churning our internal IDs.
CREATE TABLE players (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    sport_id            smallint NOT NULL REFERENCES sports(id),
    provider            text NOT NULL,           -- 'sleeper', 'sportsdataio'
    provider_player_id  text NOT NULL,
    full_name           text NOT NULL,
    first_name          text,
    last_name           text,
    position            text,                    -- 'QB','RB','WR','TE','K','DEF', etc.
    eligible_positions  text[] NOT NULL DEFAULT '{}',
    nfl_team            text,                    -- 'KC', 'DAL', 'FA', etc.
    jersey_number       int,
    status              text,                    -- 'Active','Injured Reserve','Out', etc.
    injury_status       text,
    injury_body_part    text,
    injury_notes        text,
    age                 int,
    height_inches       int,
    weight_lbs          int,
    college             text,
    years_exp           int,
    headshot_url        text,
    extra               jsonb NOT NULL DEFAULT '{}',
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    UNIQUE (sport_id, provider, provider_player_id)
);
CREATE INDEX players_sport_pos_idx ON players (sport_id, position);
CREATE INDEX players_team_idx ON players (sport_id, nfl_team);
CREATE INDEX players_name_idx ON players (sport_id, lower(full_name));

-- NFL games (used for scoring + schedule).
CREATE TABLE games (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    sport_id        smallint NOT NULL REFERENCES sports(id),
    season          int NOT NULL,
    week            int NOT NULL,
    home_team       text NOT NULL,
    away_team       text NOT NULL,
    kickoff_at      timestamptz NOT NULL,
    status          text NOT NULL DEFAULT 'scheduled',  -- 'scheduled'|'in_progress'|'final'|'postponed'
    home_score      int,
    away_score      int,
    provider_game_id text NOT NULL,
    provider        text NOT NULL,
    UNIQUE (sport_id, provider, provider_game_id)
);
CREATE INDEX games_week_idx ON games (sport_id, season, week);

-- Per-player stat lines per game (or per week for sleeper-style cumulative weekly).
-- We store the raw stat map and a precomputed fantasy_points; scoring rules
-- can be reapplied at read time when rules change without re-syncing data.
CREATE TABLE player_stats (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    sport_id        smallint NOT NULL REFERENCES sports(id),
    season          int NOT NULL,
    week            int NOT NULL,                  -- 0 for offseason / aggregate
    player_id       uuid NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    game_id         uuid REFERENCES games(id) ON DELETE SET NULL,
    stats           jsonb NOT NULL DEFAULT '{}',   -- canonical_v1 keys (e.g. {"pass_yd":312,"pass_td":2,"pass_int":1}); see internal/statsnorm
    is_final        boolean NOT NULL DEFAULT false,
    updated_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (sport_id, season, week, player_id)
);
CREATE INDEX player_stats_lookup_idx ON player_stats (sport_id, season, week);
CREATE INDEX player_stats_player_idx ON player_stats (player_id, season);

-- Player projections (also from provider). Used during draft + lineup assist.
CREATE TABLE player_projections (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    sport_id    smallint NOT NULL REFERENCES sports(id),
    season      int NOT NULL,
    week        int NOT NULL,                  -- 0 for season-long
    player_id   uuid NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    stats       jsonb NOT NULL DEFAULT '{}',
    points      numeric(8,2),                  -- if provider gives a single-number proj
    source      text NOT NULL,                  -- 'sleeper', 'sportsdataio', 'consensus'
    updated_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (sport_id, season, week, player_id, source)
);
CREATE INDEX player_projections_lookup_idx ON player_projections (sport_id, season, week);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS player_projections;
DROP TABLE IF EXISTS player_stats;
DROP TABLE IF EXISTS games;
DROP TABLE IF EXISTS players;
-- +goose StatementEnd
