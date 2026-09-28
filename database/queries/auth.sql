-- name: CreateUser :one
-- Anonymous-first identity (ADR-010): a bare row is the whole account.
INSERT INTO users DEFAULT VALUES RETURNING *;

-- name: CreateAuthSession :one
-- token_hash is SHA-256 of the opaque client token — the raw token is never stored.
INSERT INTO auth_sessions (user_id, token_hash, expires_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetAuthSessionByTokenHash :one
SELECT id, user_id, token_hash, created_at, expires_at, revoked_at
FROM auth_sessions
WHERE token_hash = $1
	AND revoked_at IS NULL
	AND expires_at > now();

-- name: TouchUserLastSeen :exec
-- Refreshed on authenticated requests; basis for future inactive cleanup.
-- Throttled to hourly so a busy session doesn't turn into write amplification.
UPDATE users SET last_seen_at = now()
WHERE id = $1 AND last_seen_at < now() - interval '1 hour';

-- name: RevokeAuthSession :exec
UPDATE auth_sessions SET revoked_at = now() WHERE token_hash = $1;
