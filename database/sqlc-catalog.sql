-- Declared schema for sqlc's static analysis only — NEVER executed.
-- Mirrors the DDL in migrations/ (goose Down sections make migrations/
-- unusable as a sqlc schema source: sqlc would apply the DROPs too).
-- Keep this file in sync whenever a migration adds/alters tables or when a
-- query needs an extension function signature.
CREATE TYPE geometry;
CREATE TYPE geography;

CREATE FUNCTION postgis_version() RETURNS text AS
$$ SELECT ''::text $$ LANGUAGE sql;
CREATE FUNCTION gen_random_uuid() RETURNS uuid AS
$$ SELECT NULL::uuid $$ LANGUAGE sql;
CREATE FUNCTION st_makepoint(float8, float8) RETURNS geometry AS
$$ SELECT NULL::geometry $$ LANGUAGE sql;
CREATE FUNCTION st_setsrid(geometry, int4) RETURNS geometry AS
$$ SELECT NULL::geometry $$ LANGUAGE sql;
CREATE FUNCTION st_dwithin(geography, geography, float8) RETURNS bool AS
$$ SELECT true $$ LANGUAGE sql;
CREATE FUNCTION st_distance(geography, geography) RETURNS float8 AS
$$ SELECT 0::float8 $$ LANGUAGE sql;
CREATE FUNCTION st_x(geometry) RETURNS float8 AS
$$ SELECT 0::float8 $$ LANGUAGE sql;
CREATE FUNCTION st_y(geometry) RETURNS float8 AS
$$ SELECT 0::float8 $$ LANGUAGE sql;
CREATE FUNCTION wgs84_point(float8, float8) RETURNS geography AS
$$ SELECT NULL::geography $$ LANGUAGE sql;
CREATE FUNCTION st_geomfromtext(text, int4) RETURNS geometry AS
$$ SELECT NULL::geometry $$ LANGUAGE sql;
CREATE FUNCTION st_asgeojson(geometry) RETURNS text AS
$$ SELECT ''::text $$ LANGUAGE sql;
CREATE FUNCTION st_linelocatepoint(geometry, geometry) RETURNS float8 AS
$$ SELECT 0::float8 $$ LANGUAGE sql;
CREATE FUNCTION st_linesubstring(geometry, float8, float8) RETURNS geometry AS
$$ SELECT NULL::geometry $$ LANGUAGE sql;
CREATE FUNCTION st_distance(geometry, geometry) RETURNS float8 AS
$$ SELECT 0::float8 $$ LANGUAGE sql;
CREATE FUNCTION unnest(float8[], float8[]) RETURNS TABLE (lon float8, lat float8) AS
$$ SELECT 0::float8, 0::float8 $$ LANGUAGE sql;

CREATE TABLE providers (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	code text NOT NULL UNIQUE,
	name text NOT NULL,
	source_url text,
	terms_url text,
	license_name text,
	attribution_text text,
	allowed_use text,
	refresh_cadence text,
	retention_policy text,
	owner text,
	fallback_provider_id uuid REFERENCES providers (id),
	known_limitations text,
	is_active boolean NOT NULL DEFAULT true,
	last_success_at timestamptz,
	last_attempt_at timestamptz
);

CREATE TABLE agencies (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	provider_id uuid NOT NULL REFERENCES providers (id),
	provider_entity_id text NOT NULL,
	code text,
	name text NOT NULL,
	timezone text NOT NULL,
	fetched_at timestamptz,
	source_updated_at timestamptz,
	UNIQUE (provider_id, provider_entity_id)
);

CREATE TABLE stops (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	provider_id uuid NOT NULL REFERENCES providers (id),
	provider_entity_id text NOT NULL,
	parent_station_id uuid REFERENCES stops (id),
	kind text NOT NULL,
	code text,
	name text NOT NULL,
	location geography NOT NULL,
	metadata jsonb NOT NULL DEFAULT '{}',
	fetched_at timestamptz,
	source_updated_at timestamptz,
	removed_at timestamptz,
	UNIQUE (provider_id, provider_entity_id)
);

CREATE TABLE routes (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	agency_id uuid REFERENCES agencies (id),
	provider_id uuid NOT NULL REFERENCES providers (id),
	provider_entity_id text NOT NULL,
	short_name text,
	long_name text,
	mode text NOT NULL,
	color text,
	text_color text,
	fetched_at timestamptz,
	source_updated_at timestamptz,
	removed_at timestamptz,
	UNIQUE (provider_id, provider_entity_id)
);

CREATE TABLE transfers (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	from_stop_id uuid NOT NULL REFERENCES stops (id),
	to_stop_id uuid NOT NULL REFERENCES stops (id),
	walk_distance_m integer,
	min_transfer_seconds integer,
	geometry geometry,
	accessibility jsonb NOT NULL DEFAULT '{}',
	fare_context jsonb NOT NULL DEFAULT '{}',
	fetched_at timestamptz,
	UNIQUE (from_stop_id, to_stop_id)
);

CREATE TABLE route_stops (
	route_id uuid NOT NULL REFERENCES routes (id),
	stop_id uuid NOT NULL REFERENCES stops (id),
	seq integer NOT NULL,
	segment_kind text,
	station_number text,
	PRIMARY KEY (route_id, seq)
);

CREATE TABLE route_shapes (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	route_id uuid NOT NULL REFERENCES routes (id),
	direction_id smallint,
	shape geometry NOT NULL,
	source text NOT NULL,
	source_shape_id text NOT NULL,
	fetched_at timestamptz,
	UNIQUE (source, source_shape_id)
);

CREATE TABLE services (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	provider_id uuid NOT NULL REFERENCES providers (id),
	provider_entity_id text NOT NULL,
	day_mask smallint NOT NULL,
	start_date date NOT NULL,
	end_date date NOT NULL,
	fetched_at timestamptz,
	UNIQUE (provider_id, provider_entity_id)
);

CREATE TABLE trips (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	route_id uuid NOT NULL REFERENCES routes (id),
	service_id uuid NOT NULL REFERENCES services (id),
	provider_id uuid NOT NULL REFERENCES providers (id),
	provider_entity_id text NOT NULL,
	headsign text,
	direction_id smallint,
	fetched_at timestamptz,
	UNIQUE (provider_id, provider_entity_id)
);

CREATE TABLE stop_times (
	trip_id uuid NOT NULL REFERENCES trips (id) ON DELETE CASCADE,
	stop_id uuid NOT NULL REFERENCES stops (id),
	seq integer NOT NULL,
	arrival_seconds integer NOT NULL,
	departure_seconds integer NOT NULL,
	derived boolean NOT NULL DEFAULT false,
	PRIMARY KEY (trip_id, seq)
);

CREATE TABLE frequencies (
	trip_id uuid NOT NULL REFERENCES trips (id) ON DELETE CASCADE,
	start_seconds integer NOT NULL,
	end_seconds integer NOT NULL,
	headway_seconds integer NOT NULL CHECK (headway_seconds > 0),
	exact_times boolean NOT NULL DEFAULT false,
	PRIMARY KEY (trip_id, start_seconds)
);

CREATE TABLE users (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	created_at timestamptz NOT NULL DEFAULT now(),
	last_seen_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE auth_sessions (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
	token_hash bytea NOT NULL UNIQUE,
	created_at timestamptz NOT NULL DEFAULT now(),
	expires_at timestamptz NOT NULL,
	revoked_at timestamptz
);
CREATE INDEX auth_sessions_user_ix ON auth_sessions (user_id);

CREATE TABLE visit_events (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	user_id uuid NOT NULL REFERENCES users (id),
	stop_id uuid NOT NULL REFERENCES stops (id),
	client_mutation_id uuid NOT NULL,
	observed_at timestamptz NOT NULL,
	validation_method text NOT NULL CHECK (validation_method IN ('geofence', 'manual', 'trip')),
	distance_m numeric,
	status text NOT NULL CHECK (status IN ('confirmed', 'low_confidence')),
	created_at timestamptz NOT NULL DEFAULT now(),
	UNIQUE (user_id, client_mutation_id)
);
CREATE INDEX visit_events_user_stop_ix ON visit_events (user_id, stop_id);

CREATE TABLE journal_entries (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	user_id uuid NOT NULL REFERENCES users (id),
	stop_id uuid REFERENCES stops (id),
	visit_event_id uuid REFERENCES visit_events (id),
	body text NOT NULL,
	visibility text NOT NULL DEFAULT 'private' CHECK (visibility IN ('private')),
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX journal_entries_user_ix ON journal_entries (user_id);

-- 000010: user-owned rows cascade on account deletion.
-- (FK actions aren't visible to sqlc; documented here for reviewers.)

-- 000011: curated passport collections.
CREATE TABLE collections (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	slug text NOT NULL UNIQUE,
	title text NOT NULL,
	description text,
	kind text NOT NULL DEFAULT 'curated' CHECK (kind IN ('curated', 'algorithmic')),
	status text NOT NULL DEFAULT 'published' CHECK (status IN ('draft', 'published')),
	created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE collection_items (
	collection_id uuid NOT NULL REFERENCES collections (id) ON DELETE CASCADE,
	sequence smallint NOT NULL,
	stop_id uuid NOT NULL REFERENCES stops (id),
	PRIMARY KEY (collection_id, sequence),
	UNIQUE (collection_id, stop_id)
);
CREATE INDEX collection_items_stop_idx ON collection_items (stop_id);

-- 000012: City Explorer places (ADR-011 — provenance NOT NULL is intentional).
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
