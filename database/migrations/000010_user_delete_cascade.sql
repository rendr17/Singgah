-- +goose Up
-- Account deletion path (docs/26, ADR-010): deleting a user must actually
-- remove their owned data. visit_events/journal_entries user FKs defaulted
-- to NO ACTION, which made user deletion impossible — cascade them.
-- journal_entries.visit_event_id SET NULL so a (hypothetical) visit removal
-- doesn't drag the journal text with it; user deletion still wipes both.

ALTER TABLE visit_events
	DROP CONSTRAINT visit_events_user_id_fkey,
	ADD CONSTRAINT visit_events_user_id_fkey
		FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE;

ALTER TABLE journal_entries
	DROP CONSTRAINT journal_entries_user_id_fkey,
	ADD CONSTRAINT journal_entries_user_id_fkey
		FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE;

ALTER TABLE journal_entries
	DROP CONSTRAINT journal_entries_visit_event_id_fkey,
	ADD CONSTRAINT journal_entries_visit_event_id_fkey
		FOREIGN KEY (visit_event_id) REFERENCES visit_events (id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE journal_entries
	DROP CONSTRAINT journal_entries_visit_event_id_fkey,
	ADD CONSTRAINT journal_entries_visit_event_id_fkey
		FOREIGN KEY (visit_event_id) REFERENCES visit_events (id);

ALTER TABLE journal_entries
	DROP CONSTRAINT journal_entries_user_id_fkey,
	ADD CONSTRAINT journal_entries_user_id_fkey
		FOREIGN KEY (user_id) REFERENCES users (id);

ALTER TABLE visit_events
	DROP CONSTRAINT visit_events_user_id_fkey,
	ADD CONSTRAINT visit_events_user_id_fkey
		FOREIGN KEY (user_id) REFERENCES users (id);
