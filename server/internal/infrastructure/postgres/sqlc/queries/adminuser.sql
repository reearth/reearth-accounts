-- name: AdminUserUpsert :exec
INSERT INTO admin_users (id, email, name, picture_url, status, approved_by, approved_at, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
ON CONFLICT (id) DO UPDATE SET
    email=EXCLUDED.email,
    name=EXCLUDED.name,
    picture_url=EXCLUDED.picture_url,
    status=EXCLUDED.status,
    approved_by=EXCLUDED.approved_by,
    approved_at=EXCLUDED.approved_at,
    updated_at=EXCLUDED.updated_at;

-- name: AdminUserFindByID :one
SELECT * FROM admin_users WHERE id = $1;

-- name: AdminUserFindByEmail :one
SELECT * FROM admin_users WHERE lower(email) = lower($1) LIMIT 1;

-- name: AdminUserFindByIDs :many
SELECT * FROM admin_users WHERE id = ANY($1::text[]) ORDER BY id;
