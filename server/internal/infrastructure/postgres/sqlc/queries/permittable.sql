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

-- name: PermittableWorkspaceRolesInsertBulk :exec
-- One round-trip for any number of workspace roles, instead of one
-- PermittableWorkspaceRoleInsert per role. See
-- WorkspaceMembersInsertBulk (workspace.sql) for why jsonb_to_recordset is
-- used over a multi-arg unnest.
INSERT INTO permittable_workspace_roles (permittable_id, workspace_id, role_id)
SELECT permittable_id, workspace_id, role_id
  FROM jsonb_to_recordset($1::jsonb)
  AS t(permittable_id text, workspace_id text, role_id text);

-- name: PermittableWorkspaceRolesByPermittableIDs :many
SELECT * FROM permittable_workspace_roles WHERE permittable_id = ANY($1::text[]);
