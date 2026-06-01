-- name: RoleUpsert :exec
INSERT INTO roles (id, name) VALUES ($1,$2)
ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name;

-- name: RoleFindByID :one
SELECT * FROM roles WHERE id = $1;

-- name: RoleFindByIDs :many
SELECT * FROM roles WHERE id = ANY($1::text[]) ORDER BY id;

-- name: RoleFindByName :one
SELECT * FROM roles WHERE name = $1;

-- name: RoleFindAll :many
SELECT * FROM roles ORDER BY id;

-- name: RoleDelete :exec
DELETE FROM roles WHERE id = $1;
