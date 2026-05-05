-- +goose Up
-- +goose StatementBegin

-- League chat / message board / trash talk feed. channel = 'main'|'trades'|<draft_id>.
CREATE TABLE messages (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    league_id   uuid NOT NULL REFERENCES leagues(id) ON DELETE CASCADE,
    channel     text NOT NULL DEFAULT 'main',
    user_id     uuid REFERENCES users(id) ON DELETE SET NULL,
    body        text NOT NULL,
    -- references can deep-link a player or team
    refs        jsonb NOT NULL DEFAULT '{}',
    edited_at   timestamptz,
    deleted_at  timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX messages_league_chan_idx ON messages (league_id, channel, created_at DESC);

CREATE TABLE message_reactions (
    message_id  uuid NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    emoji       text NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (message_id, user_id, emoji)
);

CREATE TABLE polls (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    league_id   uuid NOT NULL REFERENCES leagues(id) ON DELETE CASCADE,
    created_by  uuid REFERENCES users(id) ON DELETE SET NULL,
    question    text NOT NULL,
    options     jsonb NOT NULL DEFAULT '[]',     -- [{id, label}]
    closes_at   timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE poll_votes (
    poll_id     uuid NOT NULL REFERENCES polls(id) ON DELETE CASCADE,
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    option_id   text NOT NULL,
    voted_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (poll_id, user_id)
);

-- In-app notifications + email digest queue.
CREATE TABLE notifications (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    league_id   uuid REFERENCES leagues(id) ON DELETE CASCADE,
    type        text NOT NULL,                   -- 'trade_proposed','waiver_won','draft_started', etc.
    title       text NOT NULL,
    body        text,
    deep_link   text,
    read_at     timestamptz,
    sent_email_at timestamptz,
    sent_push_at  timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX notifications_user_idx ON notifications (user_id, created_at DESC);

-- Web Push subscriptions per user.
CREATE TABLE push_subscriptions (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    endpoint    text NOT NULL UNIQUE,
    p256dh      text NOT NULL,
    auth        text NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);

-- Per-user notification preferences. Sparse rows; defaults assumed if absent.
-- A null league_id means "applies to all leagues" (the global default).
CREATE TABLE notification_prefs (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    league_id      uuid REFERENCES leagues(id) ON DELETE CASCADE,
    type           text NOT NULL,
    via_email      boolean NOT NULL DEFAULT true,
    via_push       boolean NOT NULL DEFAULT true
);
CREATE UNIQUE INDEX notification_prefs_user_league_type_idx
    ON notification_prefs (user_id, COALESCE(league_id, '00000000-0000-0000-0000-000000000000'::uuid), type);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS notification_prefs;
DROP TABLE IF EXISTS push_subscriptions;
DROP TABLE IF EXISTS notifications;
DROP TABLE IF EXISTS poll_votes;
DROP TABLE IF EXISTS polls;
DROP TABLE IF EXISTS message_reactions;
DROP TABLE IF EXISTS messages;
-- +goose StatementEnd
