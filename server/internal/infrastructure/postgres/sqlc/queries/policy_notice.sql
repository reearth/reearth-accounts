-- name: PolicyNoticeAdvanceFanout :execrows
UPDATE policy_notices
SET fanout_cursor = sqlc.arg(cursor)::text, fanout_end = sqlc.arg(fanout_end)::text,
    lease_token = CASE WHEN sqlc.arg(fanout_end)::text = '' THEN lease_token ELSE NULL END,
    lease_until = CASE WHEN sqlc.arg(fanout_end)::text = '' THEN sqlc.arg(until)::timestamptz ELSE NULL END
WHERE id = sqlc.arg(id) AND lease_token = sqlc.arg(token)::text;

-- name: PolicyNoticeClaimFanout :one
UPDATE policy_notices
SET lease_token = sqlc.arg(token)::text, lease_until = sqlc.arg(until)::timestamptz
WHERE id = (
    SELECT id FROM policy_notices
    WHERE fanout_end = '' AND (lease_until IS NULL OR lease_until <= sqlc.arg(now)::timestamptz)
    ORDER BY created_at
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: PolicyNoticeCreate :one
INSERT INTO policy_notices (id, type, version, audience, bump, requested_by, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (type, version, audience) DO NOTHING
RETURNING *;

-- name: PolicyNoticeFindByRef :many
SELECT * FROM policy_notices WHERE type = $1 AND version = $2 ORDER BY created_at;

-- name: PolicyNoticeFindByVersionAudience :one
SELECT * FROM policy_notices WHERE type = $1 AND version = $2 AND audience = $3;

-- name: PolicyNoticeReopen :one
UPDATE policy_notices
SET fanout_cursor = '', fanout_end = '', lease_token = NULL, lease_until = NULL
WHERE type = $1 AND version = $2 AND audience = $3 AND fanout_end IN ('withdrawn', 'covered')
RETURNING *;

-- name: PolicyNoticeReleaseFanout :exec
UPDATE policy_notices SET lease_token = NULL, lease_until = NULL
WHERE id = sqlc.arg(id) AND lease_token = sqlc.arg(token)::text;

-- name: PolicyNoticeWithdraw :exec
UPDATE policy_notices SET fanout_end = 'withdrawn', lease_token = NULL, lease_until = NULL
WHERE id = $1 AND fanout_end IN ('', 'complete');
