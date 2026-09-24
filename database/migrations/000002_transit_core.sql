-- +goose Up
-- Canonical transit catalog (docs/32_DATABASE_SCHEMA.md core block).
-- IDs are internal UUIDs; provider identity maps through
-- (provider_id, provider_entity_id), never as the primary key (docs/33).
--
-- Schema qualification: geography/geometry stay unqualified — postgis lives in
-- public locally and in extensions on Supabase, both on the search_path.
-- pg_trgm opclasses are the exception: extensions is NOT on the local
-- search_path, so they must be written extensions.gin_trgm_ops (valid on
-- Supabase too, where pg_trgm also installs into extensions).

-- PostGIS silently COERCES out-of-range coordinates into [-180,-90,180,90]
-- (lat -91 becomes -89). Broken provider payloads must fail loudly instead,
-- so all ingest goes through this function, which validates before casting.
-- +goose StatementBegin
CREATE FUNCTION wgs84_point(lon float8, lat float8) RETURNS geography AS
$$
BEGIN
	IF lon < -180 OR lon > 180 OR lat < -90 OR lat > 90 THEN
		RAISE EXCEPTION 'coordinate out of range: lon=%, lat=%', lon, lat;
	END IF;
	RETURN st_setsrid(st_makepoint(lon, lat), 4326)::geography;
END;
$$ LANGUAGE plpgsql IMMUTABLE;
-- +goose StatementEnd

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
	last_success_at timestamptz
);

CREATE TABLE agencies (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	provider_id uuid NOT NULL REFERENCES providers (id),
	provider_entity_id text NOT NULL,
	code text,
	name text NOT NULL,
	-- GTFS agency_timezone is mandatory; provider service dates depend on it.
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
	kind text NOT NULL CHECK (kind IN ('station', 'stop', 'platform', 'entrance')),
	code text,
	name text NOT NULL,
	location geography (point, 4326) NOT NULL,
	metadata jsonb NOT NULL DEFAULT '{}',
	fetched_at timestamptz,
	source_updated_at timestamptz,
	UNIQUE (provider_id, provider_entity_id)
);

CREATE INDEX stops_location_gix ON stops USING gist (location);
CREATE INDEX stops_name_trgm ON stops USING gin (name extensions.gin_trgm_ops);
CREATE INDEX stops_parent_station_idx ON stops (parent_station_id);
CREATE INDEX stops_provider_idx ON stops (provider_id);

CREATE TABLE routes (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	agency_id uuid REFERENCES agencies (id),
	provider_id uuid NOT NULL REFERENCES providers (id),
	provider_entity_id text NOT NULL,
	short_name text,
	long_name text,
	-- Canonical vehicle mode; adapters map GTFS route_type onto this set.
	mode text NOT NULL CHECK (mode IN ('rail', 'subway', 'tram', 'bus', 'ferry', 'other')),
	color text,
	text_color text,
	fetched_at timestamptz,
	source_updated_at timestamptz,
	UNIQUE (provider_id, provider_entity_id)
);

CREATE INDEX routes_agency_idx ON routes (agency_id);
CREATE INDEX routes_provider_idx ON routes (provider_id);

CREATE TABLE transfers (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	from_stop_id uuid NOT NULL REFERENCES stops (id),
	to_stop_id uuid NOT NULL REFERENCES stops (id),
	walk_distance_m integer,
	min_transfer_seconds integer,
	geometry geometry (linestring, 4326),
	accessibility jsonb NOT NULL DEFAULT '{}',
	fare_context jsonb NOT NULL DEFAULT '{}',
	fetched_at timestamptz,
	UNIQUE (from_stop_id, to_stop_id)
);

CREATE INDEX transfers_from_stop_idx ON transfers (from_stop_id);
CREATE INDEX transfers_to_stop_idx ON transfers (to_stop_id);

-- +goose Down
DROP FUNCTION wgs84_point;
DROP TABLE transfers;
DROP TABLE routes;
DROP TABLE stops;
DROP TABLE agencies;
DROP TABLE providers;
