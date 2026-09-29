-- +goose Up
-- City Explorer places (docs/52, sketch docs/32, decision ADR-011). Every
-- place carries provider provenance — provider_id/provider_entity_id are NOT
-- NULL because ADR-011 makes provenance the load-bearing contract for license
-- attribution and source freshness, not an optional enrichment.
--
-- primary_category is the canonical doc/14 launch set (Indonesian slugs to
-- match UI copy directly) + 'other': imported POIs that fit no canonical
-- category stay honest as 'other' instead of being forced into a wrong
-- bucket or silently dropped.
--
-- editorial_status separates raw import from curated truth per docs/13 —
-- 'hidden' exists so bad/unsafe imports can be retracted without deleting
-- provenance.

CREATE TABLE places (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	provider_id uuid NOT NULL REFERENCES providers (id),
	provider_entity_id text NOT NULL,
	name text NOT NULL,
	primary_category text NOT NULL CHECK (primary_category IN ('makan', 'ngopi', 'hiburan', 'taman', 'budaya', 'belanja', 'other')),
	location geography (point, 4326) NOT NULL,
	price_band smallint,
	accessibility jsonb NOT NULL DEFAULT '{}',
	source_payload jsonb NOT NULL DEFAULT '{}',
	source_updated_at timestamptz,
	editorial_status text NOT NULL DEFAULT 'unreviewed' CHECK (editorial_status IN ('unreviewed', 'curated', 'hidden')),
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	UNIQUE (provider_id, provider_entity_id)
);
CREATE INDEX places_location_gix ON places USING gist (location);
CREATE INDEX places_name_trgm ON places USING gin (name extensions.gin_trgm_ops);
CREATE INDEX places_category_idx ON places (primary_category);

-- place_transit_access is computed at ingest/analysis time, never at request
-- time (ADR-011): walk_seconds is a straight-line × detour-factor estimate
-- until a real foot router exists; geometry stays NULL for estimates so a
-- non-NULL geometry later unambiguously means a real walked path.
CREATE TABLE place_transit_access (
	place_id uuid NOT NULL REFERENCES places (id) ON DELETE CASCADE,
	stop_id uuid NOT NULL REFERENCES stops (id) ON DELETE CASCADE,
	walk_distance_m integer NOT NULL CHECK (walk_distance_m >= 0),
	walk_seconds integer CHECK (walk_seconds IS NULL OR walk_seconds >= 0),
	geometry geometry (linestring, 4326),
	computed_at timestamptz NOT NULL,
	PRIMARY KEY (place_id, stop_id)
);
CREATE INDEX place_transit_access_stop_idx ON place_transit_access (stop_id);

-- +goose Down
DROP TABLE place_transit_access;
DROP TABLE places;
