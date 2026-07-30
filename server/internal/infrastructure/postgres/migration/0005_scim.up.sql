ALTER TABLE workspace_members ADD COLUMN IF NOT EXISTS external_id text NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS workspace_scim_configs (
    workspace_id       text PRIMARY KEY REFERENCES workspaces(id) ON DELETE CASCADE,
    enabled            boolean NOT NULL DEFAULT false,
    token_hash         text NOT NULL DEFAULT '',
    group_role_mapping jsonb NOT NULL DEFAULT '{}',
    updated_at         timestamptz NOT NULL DEFAULT now()
);
