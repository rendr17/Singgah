-- +goose Up
-- Integrity checks found missing in the PR-6 audit. All are cheap row-local
-- invariants; the deeper "parent must be a station" rule needs a trigger and
-- stays deferred until ingest semantics settle.

-- A stop cannot be its own parent (self-loop in the station hierarchy).
ALTER TABLE stops
	ADD CONSTRAINT stops_no_self_parent CHECK (parent_station_id IS NULL OR parent_station_id <> id);

-- A transfer between a stop and itself is meaningless; distances/durations
-- are physical quantities and must not be negative.
ALTER TABLE transfers
	ADD CONSTRAINT transfers_distinct_stops CHECK (from_stop_id <> to_stop_id),
	ADD CONSTRAINT transfers_walk_distance_nonneg CHECK (walk_distance_m IS NULL OR walk_distance_m >= 0),
	ADD CONSTRAINT transfers_min_transfer_nonneg CHECK (min_transfer_seconds IS NULL OR min_transfer_seconds >= 0);

-- Direct self-fallback is a cycle; longer cycles (A→B→A) remain possible and
-- are guarded when fallback chains are actually traversed.
ALTER TABLE providers
	ADD CONSTRAINT providers_no_self_fallback CHECK (fallback_provider_id IS NULL OR fallback_provider_id <> id);

-- +goose Down
ALTER TABLE providers DROP CONSTRAINT providers_no_self_fallback;
ALTER TABLE transfers
	DROP CONSTRAINT transfers_distinct_stops,
	DROP CONSTRAINT transfers_walk_distance_nonneg,
	DROP CONSTRAINT transfers_min_transfer_nonneg;
ALTER TABLE stops DROP CONSTRAINT stops_no_self_parent;
