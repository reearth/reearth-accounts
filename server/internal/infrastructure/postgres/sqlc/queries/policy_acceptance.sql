-- name: PolicyAcceptanceFindByUser :many
SELECT * FROM policy_acceptances WHERE user_id = $1;

-- name: PolicyAcceptanceInsert :exec
INSERT INTO policy_acceptances (user_id, type, version, accepted_at, origin, user_agent, ip)
SELECT unnest(sqlc.arg(user_ids)::text[]), unnest(sqlc.arg(types)::text[]), unnest(sqlc.arg(versions)::text[]),
    unnest(sqlc.arg(accepted_ats)::timestamptz[]), unnest(sqlc.arg(origins)::text[]),
    unnest(sqlc.arg(user_agents)::text[]), unnest(sqlc.arg(ips)::text[])
ON CONFLICT (user_id, type, version) DO NOTHING;

-- name: PolicyAcceptanceRemoveByUser :exec
DELETE FROM policy_acceptances WHERE user_id = $1;
