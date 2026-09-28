-- +goose Up
-- Canonical scheduled service (docs/31 Trip/StopTime). Times are integer
-- seconds since service-day midnight in the agency timezone — the GTFS
-- convention, so values >= 86400 are after-midnight trips still belonging
-- to the same service date (docs/33: provider service dates stay explicit).
-- Schedule rows are ingest-owned junction data: each run replaces them
-- atomically per schedule provider, never tombstoned (docs/33).

CREATE TABLE services (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	provider_id uuid NOT NULL REFERENCES providers (id),
	-- GTFS service_id / commute day-set key — the source's own identifier.
	provider_entity_id text NOT NULL,
	-- bit 0 = Monday .. bit 6 = Sunday.
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

CREATE INDEX trips_route_idx ON trips (route_id);
CREATE INDEX trips_service_idx ON trips (service_id);

-- derived marks a time the source never published — e.g. a terminus arrival
-- estimated from the reverse segment, or stop times grouped by heuristic
-- chaining. Planner output must treat derived rows as ESTIMATED (docs/9).
CREATE TABLE stop_times (
	trip_id uuid NOT NULL REFERENCES trips (id) ON DELETE CASCADE,
	stop_id uuid NOT NULL REFERENCES stops (id),
	seq integer NOT NULL,
	arrival_seconds integer NOT NULL,
	departure_seconds integer NOT NULL,
	derived boolean NOT NULL DEFAULT false,
	PRIMARY KEY (trip_id, seq)
);

CREATE INDEX stop_times_stop_idx ON stop_times (stop_id);

-- exact_times = false means the trip's stop_times are a template and buses
-- run every headway_seconds inside the window — an ESTIMATED-frequency
-- service, not concrete departures. The planner must not present expanded
-- template times as SCHEDULED (docs/9 transit truth).
CREATE TABLE frequencies (
	trip_id uuid NOT NULL REFERENCES trips (id) ON DELETE CASCADE,
	start_seconds integer NOT NULL,
	end_seconds integer NOT NULL,
	headway_seconds integer NOT NULL CHECK (headway_seconds > 0),
	exact_times boolean NOT NULL DEFAULT false,
	PRIMARY KEY (trip_id, start_seconds)
);

-- +goose Down
DROP TABLE frequencies;
DROP TABLE stop_times;
DROP TABLE trips;
DROP TABLE services;
