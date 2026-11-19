CREATE TABLE IF NOT EXISTS mail_outbox (
    key               text PRIMARY KEY,
    kind              text NOT NULL,
    subject           jsonb NOT NULL DEFAULT '{}',
    recipient_user_id text,
    recipient_address text,
    state             text NOT NULL,
    attempts          integer NOT NULL DEFAULT 0,
    next_attempt_at   timestamptz NOT NULL,
    claim_token       text,
    claim_until       timestamptz,
    reason            text NOT NULL DEFAULT '',
    created_at        timestamptz NOT NULL,
    updated_at        timestamptz NOT NULL,
    ended_at          timestamptz,
    CONSTRAINT mail_outbox_one_recipient CHECK ((recipient_user_id IS NULL) <> (recipient_address IS NULL))
);
CREATE INDEX IF NOT EXISTS mail_outbox_state_next_attempt ON mail_outbox (state, next_attempt_at);
CREATE INDEX IF NOT EXISTS mail_outbox_state_claim_until ON mail_outbox (state, claim_until);
CREATE INDEX IF NOT EXISTS mail_outbox_state_created ON mail_outbox (state, created_at);
CREATE INDEX IF NOT EXISTS mail_outbox_ended ON mail_outbox (ended_at) WHERE ended_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS mail_outbox_recipient_user ON mail_outbox (recipient_user_id) WHERE recipient_user_id IS NOT NULL;
