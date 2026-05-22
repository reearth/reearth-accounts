-- name: ConfigLoad :one
SELECT migration, auth_cert, auth_key, default_policy FROM config WHERE id = 1;

-- name: ConfigUpsert :exec
INSERT INTO config (id, migration, auth_cert, auth_key, default_policy)
VALUES (1, $1, $2, $3, $4)
ON CONFLICT (id) DO UPDATE SET
  migration=EXCLUDED.migration, auth_cert=EXCLUDED.auth_cert, auth_key=EXCLUDED.auth_key, default_policy=EXCLUDED.default_policy;

-- name: ConfigUpsertAuth :exec
INSERT INTO config (id, auth_cert, auth_key)
VALUES (1, $1, $2)
ON CONFLICT (id) DO UPDATE SET auth_cert=EXCLUDED.auth_cert, auth_key=EXCLUDED.auth_key;
