-- One record per user, policy type and version: the evidence of consent and
-- the consent state. Inserting an existing key does nothing.
CREATE TABLE IF NOT EXISTS policy_acceptances (
    user_id     text NOT NULL,
    type        text NOT NULL,
    version     text NOT NULL,
    accepted_at timestamptz NOT NULL,
    origin      text NOT NULL DEFAULT '',
    user_agent  text NOT NULL DEFAULT '',
    ip          text NOT NULL DEFAULT '',
    PRIMARY KEY (user_id, type, version)
);
