-- Auth queries: Google OAuth whitelist + break-glass local admin (ADR-0004).

-- name: GetActiveApprovedUserByEmail :one
SELECT * FROM approved_users
WHERE email = lower(sqlc.arg(email)) AND is_active = 1;

-- name: GetApprovedUserByEmail :one
SELECT * FROM approved_users WHERE email = lower(sqlc.arg(email));

-- name: GetApprovedUser :one
SELECT * FROM approved_users WHERE id = sqlc.arg(id);

-- name: ListApprovedUsers :many
SELECT * FROM approved_users ORDER BY email;

-- name: CountApprovedUsers :one
SELECT COUNT(*) FROM approved_users;

-- name: CreateApprovedUser :one
INSERT INTO approved_users (email, display_name, role, provider, password_hash, invited_by)
VALUES (
    lower(sqlc.arg(email)),
    sqlc.narg(display_name),
    sqlc.arg(role),
    sqlc.arg(provider),
    sqlc.narg(password_hash),
    sqlc.narg(invited_by)
)
RETURNING *;

-- name: SetApprovedUserRole :exec
UPDATE approved_users
SET role = sqlc.arg(role),
    updated_at = strftime('%Y-%m-%dT%H:%M:%SZ','now')
WHERE id = sqlc.arg(id);

-- name: SetApprovedUserActive :exec
UPDATE approved_users
SET is_active = sqlc.arg(is_active),
    updated_at = strftime('%Y-%m-%dT%H:%M:%SZ','now')
WHERE id = sqlc.arg(id);

-- name: SetApprovedUserPassword :exec
UPDATE approved_users
SET password_hash = sqlc.narg(password_hash),
    updated_at = strftime('%Y-%m-%dT%H:%M:%SZ','now')
WHERE id = sqlc.arg(id);

-- name: TouchApprovedUserLogin :exec
UPDATE approved_users
SET last_login_at = strftime('%Y-%m-%dT%H:%M:%SZ','now'),
    updated_at = strftime('%Y-%m-%dT%H:%M:%SZ','now')
WHERE id = sqlc.arg(id);

-- name: CreateSession :exec
INSERT INTO sessions (token, user_id, data, expires_at)
VALUES (sqlc.arg(token), sqlc.narg(user_id), sqlc.narg(data), sqlc.arg(expires_at));

-- name: GetSession :one
SELECT * FROM sessions WHERE token = sqlc.arg(token);

-- name: DeleteSession :exec
DELETE FROM sessions WHERE token = sqlc.arg(token);

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions WHERE expires_at < sqlc.arg(now);
