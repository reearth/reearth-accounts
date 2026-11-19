-- One head per policy type: the decision state that one compare-and-set on
-- revision replaces per webhook payload.
CREATE TABLE IF NOT EXISTS policy_types (
    type                  text PRIMARY KEY,
    revision              bigint NOT NULL,
    applied_at            timestamptz NOT NULL,
    upcoming_version      text,
    upcoming_effective_at timestamptz,
    withdrawn             jsonb NOT NULL DEFAULT '[]'
);

-- One record per policy version. Never deleted.
CREATE TABLE IF NOT EXISTS policy_versions (
    type         text NOT NULL REFERENCES policy_types(type),
    version      text NOT NULL,
    effective_at timestamptz NOT NULL,
    required     boolean NOT NULL,
    title        jsonb NOT NULL,
    url          jsonb NOT NULL DEFAULT '{}',
    stamp        timestamptz NOT NULL,
    PRIMARY KEY (type, version)
);
