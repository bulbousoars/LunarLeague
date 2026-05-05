-- +goose Up
-- +goose StatementBegin

-- A team's current player roster. One row per player on a team. slot is the
-- *assigned* lineup slot for the current week ('QB','RB','BN','IR', etc.).
CREATE TABLE rosters (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    league_id     uuid NOT NULL REFERENCES leagues(id) ON DELETE CASCADE,
    team_id       uuid NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    player_id     uuid NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    slot          text NOT NULL DEFAULT 'BN',
    acquired_via  text NOT NULL,                 -- 'draft'|'waiver'|'free_agent'|'trade'|'keeper'
    acquired_at   timestamptz NOT NULL DEFAULT now(),
    keeper_round_cost int,                       -- if kept this season, the round it cost
    UNIQUE (league_id, player_id)                -- a player can only be on one team per league
);
CREATE INDEX rosters_team_idx ON rosters (team_id);
CREATE INDEX rosters_league_idx ON rosters (league_id);

-- Per-week locked lineups. Set when a slate locks; used for scoring.
CREATE TABLE lineups (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    league_id   uuid NOT NULL REFERENCES leagues(id) ON DELETE CASCADE,
    team_id     uuid NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    season      int NOT NULL,
    week        int NOT NULL,
    starters    jsonb NOT NULL DEFAULT '[]',     -- [{slot, player_id}]
    bench       jsonb NOT NULL DEFAULT '[]',
    locked_at   timestamptz,
    UNIQUE (team_id, season, week)
);

-- A draft is a single event tied to a league. Snake or auction.
CREATE TABLE drafts (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    league_id       uuid NOT NULL REFERENCES leagues(id) ON DELETE CASCADE,
    type            text NOT NULL,               -- 'snake' | 'auction'
    status          text NOT NULL DEFAULT 'pending', -- 'pending'|'in_progress'|'paused'|'complete'
    rounds          int NOT NULL,
    pick_seconds    int NOT NULL DEFAULT 90,
    nomination_seconds int NOT NULL DEFAULT 30,  -- auction-only
    bidding_seconds    int NOT NULL DEFAULT 15,  -- auction-only
    starts_at       timestamptz,
    started_at      timestamptz,
    completed_at    timestamptz,
    -- snake-style ordered list of team_ids
    draft_order     jsonb NOT NULL DEFAULT '[]',
    -- arbitrary, e.g. "auction_budget":200
    config          jsonb NOT NULL DEFAULT '{}',
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

-- One pick. For snake: pick_no monotonic 1..N. For auction: pick_no = order won.
-- price is non-null for auction picks.
CREATE TABLE draft_picks (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    draft_id    uuid NOT NULL REFERENCES drafts(id) ON DELETE CASCADE,
    pick_no     int NOT NULL,
    round       int NOT NULL,
    team_id     uuid NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    player_id   uuid REFERENCES players(id) ON DELETE SET NULL,
    price       int,                             -- auction-only
    is_keeper   boolean NOT NULL DEFAULT false,
    is_autopick boolean NOT NULL DEFAULT false,
    picked_at   timestamptz,
    UNIQUE (draft_id, pick_no)
);
CREATE INDEX draft_picks_team_idx ON draft_picks (team_id);

-- A team's pre-draft player queue.
CREATE TABLE draft_queues (
    draft_id    uuid NOT NULL REFERENCES drafts(id) ON DELETE CASCADE,
    team_id     uuid NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    player_id   uuid NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    rank        int NOT NULL,
    PRIMARY KEY (draft_id, team_id, player_id)
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS draft_queues;
DROP TABLE IF EXISTS draft_picks;
DROP TABLE IF EXISTS drafts;
DROP TABLE IF EXISTS lineups;
DROP TABLE IF EXISTS rosters;
-- +goose StatementEnd
