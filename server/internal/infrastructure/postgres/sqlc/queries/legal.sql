-- name: PolicyTypeFind :one
SELECT * FROM policy_types WHERE type = $1;

-- name: PolicyTypeFindAll :many
SELECT * FROM policy_types ORDER BY type;

-- name: PolicyTypeInsert :execrows
INSERT INTO policy_types (type, revision, applied_at, applied_version, applied_digest, current, upcoming, withdrawn)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (type) DO NOTHING;

-- name: PolicyTypeSwap :execrows
UPDATE policy_types
SET revision = sqlc.arg(revision), applied_at = sqlc.arg(applied_at),
    applied_version = sqlc.arg(applied_version), applied_digest = sqlc.arg(applied_digest),
    current = sqlc.arg(current), upcoming = sqlc.arg(upcoming), withdrawn = sqlc.arg(withdrawn)
WHERE type = sqlc.arg(type) AND revision = sqlc.arg(revision)::bigint - 1;

-- name: PolicyVersionFindAll :many
SELECT * FROM policy_versions ORDER BY type, version;

-- name: PolicyVersionArchive :exec
-- A stored record is replaced only by a write from a head revision not
-- below its own.
INSERT INTO policy_versions (type, version, effective_at, required, title, url, revision)
VALUES (sqlc.arg(type), sqlc.arg(version), sqlc.arg(effective_at), sqlc.arg(required), sqlc.arg(title), sqlc.arg(url), sqlc.arg(revision))
ON CONFLICT (type, version) DO UPDATE SET
    effective_at = EXCLUDED.effective_at, required = EXCLUDED.required,
    title = EXCLUDED.title, url = EXCLUDED.url, revision = EXCLUDED.revision
WHERE policy_versions.revision <= EXCLUDED.revision;
