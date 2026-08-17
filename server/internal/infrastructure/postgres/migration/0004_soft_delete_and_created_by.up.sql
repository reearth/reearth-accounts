ALTER TABLE users ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
-- Nullable from the start: historical rows predating these columns have no real
-- creation time to backfill, so NULL (unknown) is used rather than a fabricated
-- now(). New rows set it explicitly at creation (see workspace.Init /
-- interactor.Workspace.Create).
ALTER TABLE users ADD COLUMN IF NOT EXISTS created_at timestamptz;
ALTER TABLE workspaces ADD COLUMN IF NOT EXISTS created_at timestamptz;
ALTER TABLE workspaces ADD COLUMN IF NOT EXISTS created_by text;
ALTER TABLE workspaces ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
