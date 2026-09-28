-- name: CreateAnonymousSession :one
-- Atomic user+session in one statement: a separate CreateUser then
-- CreateAuthSession could leave an orphaned user row on failure.
-- token_hash is SHA-256 of the opaque client token — the raw token is never stored.
WITH u AS (
	INSERT INTO users DEFAULT VALUES RETURNING id
)
INSERT INTO auth_sessions (user_id, token_hash, expires_at)
SELECT id, $1, $2 FROM u
RETURNING auth_sessions.*;

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

-- name: DeleteUser :exec
-- Full account deletion (docs/26 delete path): cascades wipe sessions,
-- visit_events, and journal_entries for this user.
DELETE FROM users WHERE id = $1;
