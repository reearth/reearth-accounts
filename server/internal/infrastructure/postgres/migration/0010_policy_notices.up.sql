CREATE TABLE IF NOT EXISTS policy_notices (
    id            text PRIMARY KEY,
    type          text NOT NULL,
    version       text NOT NULL,
    audience      text NOT NULL,
    bump          text NOT NULL,
    requested_by  text NOT NULL,
    created_at    timestamptz NOT NULL,
    fanout_cursor text NOT NULL DEFAULT '',
    fanout_end    text NOT NULL DEFAULT '',
    lease_token   text,
    lease_until   timestamptz,
    CONSTRAINT policy_notices_type_version_audience UNIQUE (type, version, audience)
);
CREATE INDEX IF NOT EXISTS policy_notices_fanout ON policy_notices (created_at) WHERE fanout_end = '';
CREATE INDEX IF NOT EXISTS mail_outbox_notice_state ON mail_outbox ((subject->>'notice'), state) WHERE subject ? 'notice';
