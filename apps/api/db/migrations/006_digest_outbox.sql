-- +goose Up
-- +goose StatementBegin

-- Outbox row per scheduled digest (weekly recap, draft reminder, etc.).
CREATE TABLE digest_outbox (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    league_id    uuid REFERENCES leagues(id) ON DELETE CASCADE,
    kind         text NOT NULL,                  -- 'weekly_recap'|'draft_reminder'|'waiver_results'
    scheduled_at timestamptz NOT NULL,
    sent_at      timestamptz,
    payload      jsonb NOT NULL DEFAULT '{}',
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX digest_outbox_due_idx ON digest_outbox (scheduled_at) WHERE sent_at IS NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS digest_outbox;
-- +goose StatementEnd
