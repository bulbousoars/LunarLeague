-- +goose Up
-- +goose StatementBegin

CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS citext;

-- Sports lookup. Multi-sport from day 1 — every player / league / stat is
-- scoped to a sport so adding NBA and MLB later is purely additive.
CREATE TABLE sports (
    id          smallserial PRIMARY KEY,
    code        text NOT NULL UNIQUE,    -- 'nfl', 'nba', 'mlb'
    name        text NOT NULL,
    season_type text NOT NULL,            -- 'weekly' (NFL) | 'daily' (NBA, MLB)
    created_at  timestamptz NOT NULL DEFAULT now()
);

-- Users are global across leagues. A user can belong to many leagues.
CREATE TABLE users (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email         citext NOT NULL UNIQUE,
    display_name  text NOT NULL,
    avatar_url    text,
    timezone      text NOT NULL DEFAULT 'America/New_York',
    is_admin      boolean NOT NULL DEFAULT false,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

-- Sessions are server-issued opaque tokens. Magic-link auth produces a session
-- on success.
CREATE TABLE sessions (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash    bytea NOT NULL UNIQUE,
    user_agent    text,
    ip            inet,
    created_at    timestamptz NOT NULL DEFAULT now(),
    expires_at    timestamptz NOT NULL,
    revoked_at    timestamptz
);
CREATE INDEX sessions_user_idx ON sessions (user_id);
CREATE INDEX sessions_expires_idx ON sessions (expires_at);

-- Single-use magic-link tokens.
CREATE TABLE magic_links (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email       citext NOT NULL,
    token_hash  bytea NOT NULL UNIQUE,
    expires_at  timestamptz NOT NULL,
    consumed_at timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now()
);

-- A league is the tenant boundary for almost all gameplay tables.
CREATE TABLE leagues (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    sport_id        smallint NOT NULL REFERENCES sports(id),
    name            text NOT NULL,
    slug            text NOT NULL,
    season          int NOT NULL,
    league_format   text NOT NULL,                -- 'redraft' | 'keeper' | 'dynasty'
    draft_format    text NOT NULL DEFAULT 'snake', -- 'snake' | 'auction'
    team_count      int NOT NULL DEFAULT 12,
    invite_code     text NOT NULL UNIQUE,
    status          text NOT NULL DEFAULT 'setup', -- 'setup'|'drafting'|'in_season'|'playoffs'|'complete'
    created_by      uuid NOT NULL REFERENCES users(id),
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (slug, season)
);
CREATE INDEX leagues_sport_idx ON leagues (sport_id, season);

-- League settings are heavy and change frequently — kept in their own row to
-- avoid wide updates on the main row.
CREATE TABLE league_settings (
    league_id           uuid PRIMARY KEY REFERENCES leagues(id) ON DELETE CASCADE,
    -- roster slots, e.g. {"QB":1,"RB":2,"WR":2,"TE":1,"FLEX":1,"DST":1,"K":1,"BN":7,"IR":1}
    roster_slots        jsonb NOT NULL,
    -- waivers: 'faab' | 'rolling' | 'reverse_standings'
    waiver_type         text NOT NULL DEFAULT 'faab',
    waiver_budget       int NOT NULL DEFAULT 100,
    waiver_run_dow      smallint NOT NULL DEFAULT 3, -- 0=Sun..6=Sat (3=Wed default)
    waiver_run_hour     smallint NOT NULL DEFAULT 3, -- 0..23 league-time
    trade_deadline_week int,
    playoff_start_week  int NOT NULL DEFAULT 15,
    playoff_team_count  int NOT NULL DEFAULT 6,
    keeper_count        int NOT NULL DEFAULT 0,
    auction_budget      int,
    schedule_type       text NOT NULL DEFAULT 'h2h_points', -- 'h2h_points' | 'h2h_categories' | 'rotisserie'
    public_visible      boolean NOT NULL DEFAULT false,
    updated_at          timestamptz NOT NULL DEFAULT now()
);

-- Scoring rules are JSONB keyed by stat category. Sport-agnostic.
CREATE TABLE scoring_rules (
    league_id   uuid PRIMARY KEY REFERENCES leagues(id) ON DELETE CASCADE,
    rules       jsonb NOT NULL,
    updated_at  timestamptz NOT NULL DEFAULT now()
);

-- A team is one user's franchise inside a league. Co-managers handled later.
CREATE TABLE teams (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    league_id     uuid NOT NULL REFERENCES leagues(id) ON DELETE CASCADE,
    owner_id      uuid REFERENCES users(id) ON DELETE SET NULL, -- null = unclaimed
    name          text NOT NULL,
    abbreviation  text NOT NULL,
    logo_url      text,
    motto         text,
    waiver_position int,
    waiver_budget   int,
    auction_budget  int,
    record_wins   int NOT NULL DEFAULT 0,
    record_losses int NOT NULL DEFAULT 0,
    record_ties   int NOT NULL DEFAULT 0,
    points_for    numeric(10,2) NOT NULL DEFAULT 0,
    points_against numeric(10,2) NOT NULL DEFAULT 0,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX teams_league_idx ON teams (league_id);
CREATE UNIQUE INDEX teams_league_owner_idx ON teams (league_id, owner_id) WHERE owner_id IS NOT NULL;

-- League membership for invite/role-tracking. owner_id on teams is the cheap
-- check; this is the source of truth for *who can see* the league.
CREATE TABLE league_members (
    league_id   uuid NOT NULL REFERENCES leagues(id) ON DELETE CASCADE,
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role        text NOT NULL DEFAULT 'manager', -- 'commissioner' | 'manager' | 'observer'
    joined_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (league_id, user_id)
);
CREATE INDEX league_members_user_idx ON league_members (user_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS league_members;
DROP TABLE IF EXISTS teams;
DROP TABLE IF EXISTS scoring_rules;
DROP TABLE IF EXISTS league_settings;
DROP TABLE IF EXISTS leagues;
DROP TABLE IF EXISTS magic_links;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS sports;
-- +goose StatementEnd
