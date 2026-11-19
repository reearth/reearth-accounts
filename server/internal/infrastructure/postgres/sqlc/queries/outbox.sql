-- name: OutboxClaimDue :one
UPDATE mail_outbox
SET state = 'sending', claim_token = sqlc.arg(token), claim_until = sqlc.arg(until),
    attempts = attempts + 1, updated_at = sqlc.arg(now)
WHERE key = (
    SELECT key FROM mail_outbox
    WHERE state = 'pending' AND next_attempt_at <= sqlc.arg(now)
    ORDER BY next_attempt_at
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: OutboxComplete :execrows
UPDATE mail_outbox
SET state = sqlc.arg(state), reason = sqlc.arg(reason),
    next_attempt_at = CASE WHEN sqlc.arg(state)::text = 'pending' THEN sqlc.arg(next_attempt_at) ELSE next_attempt_at END,
    ended_at = CASE WHEN sqlc.arg(state)::text = 'pending' THEN NULL ELSE sqlc.arg(now) END,
    claim_token = NULL, claim_until = NULL, updated_at = sqlc.arg(now)
WHERE key = sqlc.arg(key) AND state = 'sending' AND claim_token = sqlc.arg(token) AND claim_until > sqlc.arg(now);

-- name: OutboxCountByNotice :many
SELECT state, count(*)::integer AS n FROM mail_outbox
WHERE subject->>'notice' = sqlc.arg(notice)::text
GROUP BY state;

-- name: OutboxEnqueue :exec
INSERT INTO mail_outbox (key, kind, subject, recipient_user_id, recipient_address, state, attempts,
    next_attempt_at, reason, created_at, updated_at)
SELECT unnest(sqlc.arg(keys)::text[]), unnest(sqlc.arg(kinds)::text[]), unnest(sqlc.arg(subjects)::jsonb[]),
    NULLIF(unnest(sqlc.arg(user_ids)::text[]), ''), NULLIF(unnest(sqlc.arg(addresses)::text[]), ''),
    unnest(sqlc.arg(states)::text[]), unnest(sqlc.arg(attempts)::integer[]),
    unnest(sqlc.arg(next_attempt_ats)::timestamptz[]), unnest(sqlc.arg(reasons)::text[]),
    unnest(sqlc.arg(created_ats)::timestamptz[]), unnest(sqlc.arg(updated_ats)::timestamptz[])
ON CONFLICT (key) DO NOTHING;

-- name: OutboxFindByKey :one
SELECT * FROM mail_outbox WHERE key = $1;

-- name: OutboxRemoveByUser :exec
DELETE FROM mail_outbox WHERE recipient_user_id = $1;

-- name: OutboxExpireClaims :execrows
UPDATE mail_outbox
SET state = 'unknown', reason = 'claim_expired', ended_at = sqlc.arg(now), updated_at = sqlc.arg(now),
    claim_token = NULL, claim_until = NULL
WHERE state = 'sending' AND claim_until <= sqlc.arg(now);

-- name: OutboxFailStale :execrows
UPDATE mail_outbox
SET state = 'failed', reason = 'ttl_expired', ended_at = sqlc.arg(now), updated_at = sqlc.arg(now)
WHERE state = 'pending' AND created_at <= sqlc.arg(cutoff);

-- name: OutboxDeleteRetired :execrows
DELETE FROM mail_outbox WHERE ended_at <= $1;
