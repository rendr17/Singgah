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
