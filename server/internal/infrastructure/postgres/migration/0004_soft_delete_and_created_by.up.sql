ALTER TABLE users ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE workspaces ADD COLUMN IF NOT EXISTS created_at timestamptz;
ALTER TABLE workspaces ADD COLUMN IF NOT EXISTS created_by text;
ALTER TABLE workspaces ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
