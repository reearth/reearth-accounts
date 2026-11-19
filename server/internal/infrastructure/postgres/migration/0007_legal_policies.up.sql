-- One head per policy type. It holds every decision about the type (the
-- current and upcoming versions with their content, the withdrawn versions,
-- the newest applied payload), so one compare-and-set on revision applies a
-- webhook payload.
CREATE TABLE IF NOT EXISTS policy_types (
    type            text PRIMARY KEY,
    revision        bigint NOT NULL,
    applied_at      timestamptz NOT NULL,
    applied_version text NOT NULL,
    applied_digest  bytea NOT NULL,
    current         jsonb,
    upcoming        jsonb,
    withdrawn       jsonb NOT NULL DEFAULT '[]'
);

-- One archived record per policy version, copied from the head after each
-- apply. Nothing reads it to decide. Never deleted.
CREATE TABLE IF NOT EXISTS policy_versions (
    type         text NOT NULL REFERENCES policy_types(type),
    version      text NOT NULL,
    effective_at timestamptz NOT NULL,
    required     boolean NOT NULL,
    title        jsonb NOT NULL,
    url          jsonb NOT NULL DEFAULT '{}',
    revision     bigint NOT NULL,
    PRIMARY KEY (type, version)
);
