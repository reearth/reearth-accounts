CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- users
CREATE TABLE users (
    id               text PRIMARY KEY,
    name             text NOT NULL,
    alias            text NOT NULL DEFAULT '',
    email            text NOT NULL,
    workspace        text NOT NULL,
    password         bytea,
    subs             text[] NOT NULL DEFAULT '{}',
    latest_logout_at timestamptz,
    metadata         jsonb NOT NULL DEFAULT '{}',
    verification     jsonb,
    password_reset   jsonb,
    team             text,
    lang             text,
    theme            text,
    updated_at       timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_email_ci_uniq ON users (lower(email));
CREATE UNIQUE INDEX users_alias_ci_uniq ON users (lower(alias)) WHERE alias <> '';
CREATE INDEX users_subs_gin ON users USING gin (subs);
CREATE INDEX users_name_trgm ON users USING gin (name gin_trgm_ops);
CREATE INDEX users_email_trgm ON users USING gin (email gin_trgm_ops);

-- workspaces
CREATE TABLE workspaces (
    id           text PRIMARY KEY,
    name         text NOT NULL,
    alias        text NOT NULL DEFAULT '',
    email        text NOT NULL DEFAULT '',
    personal     boolean NOT NULL DEFAULT false,
    policy       text,
    members_hash text NOT NULL DEFAULT '',
    metadata     jsonb NOT NULL DEFAULT '{}',
    updated_at   timestamptz NOT NULL DEFAULT now()
);
-- Composite alias+members_hash unique index, mirroring the FINAL Mongo state
-- (alias_members_hash_case_insensitive_unique). Verified against
-- internal/infrastructure/mongo/migration/260122110001_replace_workspace_alias_members_index.go.
CREATE UNIQUE INDEX workspaces_alias_members_ci_uniq
    ON workspaces (lower(alias), members_hash) WHERE alias <> '';
CREATE INDEX workspaces_alias_trgm ON workspaces USING gin (alias gin_trgm_ops);

CREATE TABLE workspace_members (
    workspace_id text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id      text NOT NULL,
    role         text NOT NULL,
    invited_by   text NOT NULL DEFAULT '',
    disabled     boolean NOT NULL DEFAULT false,
    PRIMARY KEY (workspace_id, user_id)
);
CREATE INDEX workspace_members_user_idx ON workspace_members (user_id);

CREATE TABLE workspace_integrations (
    workspace_id   text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    integration_id text NOT NULL,
    role           text NOT NULL,
    invited_by     text NOT NULL DEFAULT '',
    disabled       boolean NOT NULL DEFAULT false,
    PRIMARY KEY (workspace_id, integration_id)
);
CREATE INDEX workspace_integrations_integration_idx ON workspace_integrations (integration_id);

-- roles
CREATE TABLE roles (
    id   text PRIMARY KEY,
    name text NOT NULL
);
CREATE UNIQUE INDEX roles_name_uniq ON roles (name);

-- permittables
CREATE TABLE permittables (
    id         text PRIMARY KEY,
    user_id    text NOT NULL UNIQUE,
    role_ids   text[] NOT NULL DEFAULT '{}',
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE permittable_workspace_roles (
    permittable_id text NOT NULL REFERENCES permittables(id) ON DELETE CASCADE,
    workspace_id   text NOT NULL,
    role_id        text NOT NULL,
    PRIMARY KEY (permittable_id, workspace_id, role_id)
);

-- config (single row)
CREATE TABLE config (
    id             integer PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    migration      bigint NOT NULL DEFAULT 0,
    auth_cert      text NOT NULL DEFAULT '',
    auth_key       text NOT NULL DEFAULT '',
    default_policy text
);
