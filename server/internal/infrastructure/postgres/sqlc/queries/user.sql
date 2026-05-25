-- name: UserInsert :exec
INSERT INTO users (id, name, alias, email, workspace, password, subs, latest_logout_at, metadata, verification, password_reset, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12);

-- name: UserUpsert :exec
INSERT INTO users (id, name, alias, email, workspace, password, subs, latest_logout_at, metadata, verification, password_reset, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
ON CONFLICT (id) DO UPDATE SET
  name=EXCLUDED.name, alias=EXCLUDED.alias, email=EXCLUDED.email, workspace=EXCLUDED.workspace,
  password=EXCLUDED.password, subs=EXCLUDED.subs, latest_logout_at=EXCLUDED.latest_logout_at,
  metadata=EXCLUDED.metadata, verification=EXCLUDED.verification, password_reset=EXCLUDED.password_reset,
  updated_at=EXCLUDED.updated_at;

-- name: UserFindByID :one
SELECT * FROM users WHERE id = $1;

-- name: UserFindByIDs :many
SELECT * FROM users WHERE id = ANY($1::text[]);

-- name: UserFindAll :many
SELECT * FROM users ORDER BY id;

-- name: UserFindByEmail :one
-- Exact (case-sensitive) match, mirroring the Mongo backend.
SELECT * FROM users WHERE email = $1;

-- name: UserFindByName :one
SELECT * FROM users WHERE name = $1;

-- name: UserFindByAlias :one
-- Exact (case-sensitive) match, mirroring the Mongo backend.
SELECT * FROM users WHERE alias = $1;

-- name: UserFindBySub :one
SELECT * FROM users WHERE subs @> ARRAY[$1::text] LIMIT 1;

-- name: UserFindByNameOrEmail :one
-- Exact (case-sensitive) match on name OR email, mirroring the Mongo backend.
SELECT * FROM users WHERE name = $1 OR email = $1 LIMIT 1;

-- name: UserFindByVerification :one
SELECT * FROM users WHERE verification ->> 'code' = $1::text LIMIT 1;

-- name: UserFindByPasswordResetRequest :one
SELECT * FROM users WHERE password_reset ->> 'token' = $1::text LIMIT 1;

-- name: UserDelete :exec
DELETE FROM users WHERE id = $1;
