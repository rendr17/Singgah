-- +goose Up
-- Tombstone policy: a provider entity absent from a successful ingest run is
-- soft-deleted, never hard-deleted. Foreign keys stay valid (visit events
-- and journals will reference these rows), and an entity that reappears in
-- a later feed is resurrected by the normal upsert (removed_at -> NULL).
ALTER TABLE stops ADD COLUMN removed_at timestamptz;
ALTER TABLE routes ADD COLUMN removed_at timestamptz;

-- +goose Down
ALTER TABLE stops DROP COLUMN removed_at;
ALTER TABLE routes DROP COLUMN removed_at;
