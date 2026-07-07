-- admin_users
CREATE TABLE admin_users (
    id          text PRIMARY KEY,
    email       text NOT NULL,
    name        text NOT NULL,
    picture_url text NOT NULL DEFAULT '',
    status      text NOT NULL,
    approved_by text NOT NULL DEFAULT '',
    approved_at timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

-- case-insensitive unique email (mirrors the Mongo unique index)
CREATE UNIQUE INDEX admin_users_email_uniq ON admin_users (lower(email));

-- list pending/approved/rejected users in creation order
CREATE INDEX admin_users_status_created_at_idx ON admin_users (status, created_at);
