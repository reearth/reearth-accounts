-- name: PermittableUpsert :one
INSERT INTO permittables (id, user_id, role_ids, updated_at)
VALUES ($1,$2,$3,$4)
ON CONFLICT (user_id) DO UPDATE SET role_ids=EXCLUDED.role_ids, updated_at=EXCLUDED.updated_at
RETURNING id;

-- name: PermittableFindByUserID :one
SELECT * FROM permittables WHERE user_id = $1;

-- name: PermittableFindByUserIDs :many
SELECT * FROM permittables WHERE user_id = ANY($1::text[]) ORDER BY id;

-- name: PermittableFindByRoleID :many
SELECT * FROM permittables WHERE role_ids @> ARRAY[$1::text] ORDER BY id;

-- name: PermittableWorkspaceRolesDeleteByPermittable :exec
DELETE FROM permittable_workspace_roles WHERE permittable_id = $1;

-- name: PermittableWorkspaceRoleInsert :exec
INSERT INTO permittable_workspace_roles (permittable_id, workspace_id, role_id) VALUES ($1,$2,$3);

-- name: PermittableWorkspaceRolesByPermittableIDs :many
SELECT * FROM permittable_workspace_roles WHERE permittable_id = ANY($1::text[]);
