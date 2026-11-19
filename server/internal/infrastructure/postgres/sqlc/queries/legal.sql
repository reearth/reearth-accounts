-- name: PolicyTypeFind :one
SELECT * FROM policy_types WHERE type = $1;

-- name: PolicyTypeFindAll :many
SELECT * FROM policy_types ORDER BY type;

-- name: PolicyTypeInsert :execrows
INSERT INTO policy_types (type, revision, applied_at, upcoming_version, upcoming_effective_at, withdrawn)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (type) DO NOTHING;

-- name: PolicyTypeSwap :execrows
UPDATE policy_types
SET revision = sqlc.arg(revision), applied_at = sqlc.arg(applied_at),
    upcoming_version = sqlc.arg(upcoming_version), upcoming_effective_at = sqlc.arg(upcoming_effective_at),
    withdrawn = sqlc.arg(withdrawn)
WHERE type = sqlc.arg(type) AND revision = sqlc.arg(revision)::bigint - 1;

-- name: PolicyVersionFindAll :many
SELECT * FROM policy_versions ORDER BY type, version;

-- name: PolicyVersionUpsert :exec
-- A stored record is replaced only when it is not newer than this write and
-- has not taken effect.
INSERT INTO policy_versions (type, version, effective_at, required, title, url, stamp)
VALUES (sqlc.arg(type), sqlc.arg(version), sqlc.arg(effective_at), sqlc.arg(required), sqlc.arg(title), sqlc.arg(url), sqlc.arg(stamp))
ON CONFLICT (type, version) DO UPDATE SET
    effective_at = EXCLUDED.effective_at, required = EXCLUDED.required,
    title = EXCLUDED.title, url = EXCLUDED.url, stamp = EXCLUDED.stamp
WHERE policy_versions.stamp <= EXCLUDED.stamp AND policy_versions.effective_at > sqlc.arg(now)::timestamptz;
