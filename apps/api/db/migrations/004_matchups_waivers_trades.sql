-- +goose Up
-- +goose StatementBegin

-- The schedule of who plays whom each week.
CREATE TABLE matchups (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    league_id   uuid NOT NULL REFERENCES leagues(id) ON DELETE CASCADE,
    season      int NOT NULL,
    week        int NOT NULL,
    home_team_id uuid NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    away_team_id uuid NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    home_score  numeric(8,2) NOT NULL DEFAULT 0,
    away_score  numeric(8,2) NOT NULL DEFAULT 0,
    home_projected numeric(8,2),
    away_projected numeric(8,2),
    is_playoff  boolean NOT NULL DEFAULT false,
    is_consolation boolean NOT NULL DEFAULT false,
    is_final    boolean NOT NULL DEFAULT false,
    UNIQUE (league_id, season, week, home_team_id, away_team_id)
);
CREATE INDEX matchups_lookup_idx ON matchups (league_id, season, week);

-- A waiver claim. Processed in batch by the worker on the league's schedule.
CREATE TABLE waiver_claims (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    league_id   uuid NOT NULL REFERENCES leagues(id) ON DELETE CASCADE,
    team_id     uuid NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    add_player_id  uuid NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    drop_player_id uuid REFERENCES players(id) ON DELETE SET NULL,
    bid_amount  int,                             -- FAAB only
    priority    int NOT NULL DEFAULT 1,           -- claim order within a team
    status      text NOT NULL DEFAULT 'pending', -- 'pending'|'won'|'lost'|'cancelled'|'failed'
    process_at  timestamptz NOT NULL,
    processed_at timestamptz,
    failure_reason text,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX waiver_claims_due_idx ON waiver_claims (league_id, process_at) WHERE status = 'pending';

-- A free-agent transaction (after waiver period clears).
CREATE TABLE transactions (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    league_id   uuid NOT NULL REFERENCES leagues(id) ON DELETE CASCADE,
    team_id     uuid REFERENCES teams(id) ON DELETE SET NULL,
    type        text NOT NULL,                   -- 'add'|'drop'|'add_drop'|'waiver_claim'|'trade'
    detail      jsonb NOT NULL DEFAULT '{}',
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX transactions_league_idx ON transactions (league_id, created_at DESC);

-- A trade between two (or more) teams.
CREATE TABLE trades (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    league_id       uuid NOT NULL REFERENCES leagues(id) ON DELETE CASCADE,
    proposer_team_id uuid NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    status          text NOT NULL DEFAULT 'proposed', -- 'proposed'|'accepted'|'rejected'|'cancelled'|'vetoed'|'executed'|'review'
    note            text,
    review_until    timestamptz,                  -- league-wide veto deadline
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX trades_league_idx ON trades (league_id, status);

-- A side of a trade. Each team's incoming + outgoing assets.
CREATE TABLE trade_assets (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    trade_id    uuid NOT NULL REFERENCES trades(id) ON DELETE CASCADE,
    from_team_id uuid NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    to_team_id   uuid NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    asset_type  text NOT NULL,                  -- 'player' | 'faab' | 'draft_pick'
    player_id   uuid REFERENCES players(id) ON DELETE SET NULL,
    faab_amount int,
    pick_round  int,
    pick_year   int
);

-- Trade votes (when the commissioner requires league-wide approval).
CREATE TABLE trade_votes (
    trade_id    uuid NOT NULL REFERENCES trades(id) ON DELETE CASCADE,
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    vote        text NOT NULL,                   -- 'approve' | 'veto'
    voted_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (trade_id, user_id)
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS trade_votes;
DROP TABLE IF EXISTS trade_assets;
DROP TABLE IF EXISTS trades;
DROP TABLE IF EXISTS transactions;
DROP TABLE IF EXISTS waiver_claims;
DROP TABLE IF EXISTS matchups;
-- +goose StatementEnd
