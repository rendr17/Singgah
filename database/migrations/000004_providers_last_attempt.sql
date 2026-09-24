-- +goose Up
-- Provider health honesty: last_success_at only records wins, so a provider
-- whose ingest keeps failing looked identical to one never run. Attempts are
-- stamped before the ingest transaction so a failed run still leaves a mark.
ALTER TABLE providers
	ADD COLUMN last_attempt_at timestamptz;

-- +goose Down
ALTER TABLE providers DROP COLUMN last_attempt_at;
