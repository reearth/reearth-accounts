-- name: WorkspaceUpsert :exec
INSERT INTO workspaces (id, name, alias, email, personal, policy, members_hash, metadata, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
ON CONFLICT (id) DO UPDATE SET
  name=EXCLUDED.name, alias=EXCLUDED.alias, email=EXCLUDED.email, personal=EXCLUDED.personal,
  policy=EXCLUDED.policy, members_hash=EXCLUDED.members_hash, metadata=EXCLUDED.metadata, updated_at=EXCLUDED.updated_at;

-- name: WorkspaceFindByID :one
SELECT * FROM workspaces WHERE id = $1;

-- name: WorkspaceFindByIDs :many
SELECT * FROM workspaces WHERE id = ANY($1::text[]) ORDER BY id;

-- name: WorkspaceFindByName :one
SELECT * FROM workspaces WHERE name = $1;

-- name: WorkspaceFindByAlias :one
SELECT * FROM workspaces WHERE lower(alias) = lower($1) AND alias <> '';

-- name: WorkspaceFindByAliases :many
SELECT * FROM workspaces WHERE lower(alias) = ANY($1::text[]) ORDER BY id;

-- name: WorkspaceDelete :exec
DELETE FROM workspaces WHERE id = $1;

-- name: WorkspaceMembersDeleteByWorkspace :exec
DELETE FROM workspace_members WHERE workspace_id = $1;

-- name: WorkspaceMemberInsert :exec
INSERT INTO workspace_members (workspace_id, user_id, role, invited_by, disabled) VALUES ($1,$2,$3,$4,$5);

-- name: WorkspaceMembersByWorkspaceIDs :many
SELECT * FROM workspace_members WHERE workspace_id = ANY($1::text[]);

-- name: WorkspaceIntegrationsDeleteByWorkspace :exec
DELETE FROM workspace_integrations WHERE workspace_id = $1;

-- name: WorkspaceIntegrationInsert :exec
INSERT INTO workspace_integrations (workspace_id, integration_id, role, invited_by, disabled) VALUES ($1,$2,$3,$4,$5);

-- name: WorkspaceIntegrationsByWorkspaceIDs :many
SELECT * FROM workspace_integrations WHERE workspace_id = ANY($1::text[]);

-- name: WorkspaceIDsByUser :many
SELECT DISTINCT workspace_id FROM workspace_members WHERE user_id = $1;

-- name: WorkspaceIDsByIntegration :many
SELECT DISTINCT workspace_id FROM workspace_integrations WHERE integration_id = $1;

-- name: WorkspaceIDsByIntegrations :many
SELECT DISTINCT workspace_id FROM workspace_integrations WHERE integration_id = ANY($1::text[]);
