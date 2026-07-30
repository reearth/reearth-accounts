-- Nullable from the start: historical users predating this column have no
-- real creation time to backfill, so NULL (unknown) is used rather than a
-- fabricated now(). New users set it explicitly at creation (see
-- interactor.User.Signup/SignupOIDC).
ALTER TABLE users ADD COLUMN IF NOT EXISTS created_at timestamptz;
