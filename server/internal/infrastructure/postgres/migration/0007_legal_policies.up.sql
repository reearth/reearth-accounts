CREATE TABLE IF NOT EXISTS policy_types (
    type            text PRIMARY KEY,
    revision        bigint NOT NULL,
    applied_at      timestamptz NOT NULL,
    applied_version text NOT NULL,
    applied_digest  bytea NOT NULL,
    versions        jsonb NOT NULL DEFAULT '[]'
);

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
