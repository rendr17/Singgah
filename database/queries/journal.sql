-- name: CreateJournalEntry :one
INSERT INTO journal_entries (user_id, stop_id, visit_event_id, body)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetJournalEntry :one
-- user_id is part of the lookup — ownership is a WHERE clause, never implied.
SELECT * FROM journal_entries WHERE id = $1 AND user_id = $2;

-- name: ListJournalEntries :many
SELECT * FROM journal_entries
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2;

-- name: UpdateJournalEntry :one
-- Version check happens in the handler: GetJournalEntry first, compare
-- baseUpdatedAt, then this unconditional write (docs/24: last-write with
-- version check).
UPDATE journal_entries SET body = $3, updated_at = now()
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: DeleteJournalEntry :execrows
DELETE FROM journal_entries WHERE id = $1 AND user_id = $2;

-- name: GetVisitEventOwner :one
-- A journal may link a visit — but only the owner's own visit events.
SELECT user_id FROM visit_events WHERE id = $1;
