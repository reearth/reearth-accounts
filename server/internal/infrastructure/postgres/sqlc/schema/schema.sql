-- sqlc schema mirror (codegen only; runtime DDL lives in ../../migration).
-- Table definitions only (indexes omitted; sqlc infers types from columns and
-- inline constraints, and uses PK/UNIQUE columns for ON CONFLICT resolution).

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

CREATE TABLE workspace_members (
    workspace_id text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id      text NOT NULL,
    role         text NOT NULL,
    invited_by   text NOT NULL DEFAULT '',
    disabled     boolean NOT NULL DEFAULT false,
    PRIMARY KEY (workspace_id, user_id)
);

CREATE TABLE workspace_integrations (
    workspace_id   text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    integration_id text NOT NULL,
    role           text NOT NULL,
    invited_by     text NOT NULL DEFAULT '',
    disabled       boolean NOT NULL DEFAULT false,
    PRIMARY KEY (workspace_id, integration_id)
);

CREATE TABLE admin_users (
    id          text PRIMARY KEY,
    email       text NOT NULL,
    name        text NOT NULL,
    picture_url text NOT NULL DEFAULT '',
    role        text NOT NULL DEFAULT '',
    status      text NOT NULL,
    approved_by text NOT NULL DEFAULT '',
    approved_at timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE roles (
    id   text PRIMARY KEY,
    name text NOT NULL
);

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

CREATE TABLE config (
    id             integer PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    migration      bigint NOT NULL DEFAULT 0,
    auth_cert      text NOT NULL DEFAULT '',
    auth_key       text NOT NULL DEFAULT '',
    default_policy text
);
