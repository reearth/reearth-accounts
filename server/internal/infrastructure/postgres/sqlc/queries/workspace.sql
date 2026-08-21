-- name: WorkspaceUpsert :exec
INSERT INTO workspaces (id, name, alias, email, personal, policy, members_hash, metadata, created_at, created_by, updated_at, deleted_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
ON CONFLICT (id) DO UPDATE SET
  name=EXCLUDED.name, alias=EXCLUDED.alias, email=EXCLUDED.email, personal=EXCLUDED.personal,
  policy=EXCLUDED.policy, members_hash=EXCLUDED.members_hash, metadata=EXCLUDED.metadata, created_by=EXCLUDED.created_by,
  updated_at=EXCLUDED.updated_at, deleted_at=EXCLUDED.deleted_at;

-- name: WorkspaceFindByID :one
SELECT * FROM workspaces WHERE id = $1;

-- name: WorkspaceFindByIDs :many
SELECT * FROM workspaces WHERE id = ANY($1::text[]) ORDER BY id;

-- name: WorkspaceFindByName :one
SELECT * FROM workspaces WHERE name = $1;

-- name: WorkspaceFindByAlias :one
SELECT * FROM workspaces WHERE lower(alias) = lower($1) AND alias <> '';

-- name: WorkspaceFindByAliases :many
-- Case-insensitive, matching the case-insensitive unique alias index.
SELECT * FROM workspaces WHERE lower(alias) = ANY($1::text[]) ORDER BY id;

-- name: WorkspaceIDsAll :many
-- Cross-tenant listing (no readable-workspace filter): pass '' for no keyword
-- filter, and one of 'all'/'active'/'deleted' for the status filter.
-- Set exclude_personal=true to omit per-user personal workspaces (used by /api/workspaces/all).
SELECT id FROM workspaces
WHERE (NOT sqlc.arg(exclude_personal)::boolean OR personal = false)
  AND (sqlc.arg(keyword)::text = '' OR name ILIKE '%' || sqlc.arg(keyword)::text || '%' OR alias ILIKE '%' || sqlc.arg(keyword)::text || '%')
  AND (sqlc.arg(status)::text = 'all' OR (sqlc.arg(status)::text = 'active' AND deleted_at IS NULL) OR (sqlc.arg(status)::text = 'deleted' AND deleted_at IS NOT NULL))
ORDER BY id;

-- name: WorkspaceDelete :exec
DELETE FROM workspaces WHERE id = $1;

-- name: WorkspaceMembersDeleteByWorkspace :exec
DELETE FROM workspace_members WHERE workspace_id = $1;

-- name: WorkspaceMemberInsert :exec
INSERT INTO workspace_members (workspace_id, user_id, role, invited_by, disabled) VALUES ($1,$2,$3,$4,$5);

-- name: WorkspaceMembersInsertBulk :exec
-- One round-trip for any number of members, instead of one
-- WorkspaceMemberInsert per member. jsonb_to_recordset takes a single
-- parameter (rather than one array per column, as a multi-arg unnest would),
-- which sqlc's static catalog can type-check without a live database.
INSERT INTO workspace_members (workspace_id, user_id, role, invited_by, disabled)
SELECT workspace_id, user_id, role, invited_by, disabled
  FROM jsonb_to_recordset($1::jsonb)
  AS t(workspace_id text, user_id text, role text, invited_by text, disabled bool);

-- name: WorkspaceMembersByWorkspaceIDs :many
SELECT * FROM workspace_members WHERE workspace_id = ANY($1::text[]);

-- name: WorkspaceIntegrationsDeleteByWorkspace :exec
DELETE FROM workspace_integrations WHERE workspace_id = $1;

-- name: WorkspaceIntegrationInsert :exec
INSERT INTO workspace_integrations (workspace_id, integration_id, role, invited_by, disabled) VALUES ($1,$2,$3,$4,$5);

-- name: WorkspaceIntegrationsInsertBulk :exec
-- One round-trip for any number of integration members, instead of one
-- WorkspaceIntegrationInsert per integration. See WorkspaceMembersInsertBulk
-- for why jsonb_to_recordset is used over a multi-arg unnest.
INSERT INTO workspace_integrations (workspace_id, integration_id, role, invited_by, disabled)
SELECT workspace_id, integration_id, role, invited_by, disabled
  FROM jsonb_to_recordset($1::jsonb)
  AS t(workspace_id text, integration_id text, role text, invited_by text, disabled bool);

-- name: WorkspaceIntegrationsByWorkspaceIDs :many
SELECT * FROM workspace_integrations WHERE workspace_id = ANY($1::text[]);

-- name: WorkspaceIDsByUser :many
SELECT DISTINCT workspace_id FROM workspace_members WHERE user_id = $1;

-- name: WorkspaceIDsByIntegration :many
SELECT DISTINCT workspace_id FROM workspace_integrations WHERE integration_id = $1;

-- name: WorkspaceIDsByIntegrations :many
SELECT DISTINCT workspace_id FROM workspace_integrations WHERE integration_id = ANY($1::text[]);
