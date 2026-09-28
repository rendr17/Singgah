-- Scheduled service queries (docs/31 Trip/StopTime). Re-ingest replaces a
-- provider's trips atomically — services upsert so their UUIDs (and the
-- trips.service_id FK target) stay stable across runs.

-- name: UpsertService :one
INSERT INTO services (
	provider_id,
	provider_entity_id,
	day_mask,
	start_date,
	end_date,
	fetched_at
) VALUES (
	$1, $2, $3, $4, $5, $6
)
ON CONFLICT (provider_id, provider_entity_id) DO UPDATE SET
	day_mask = excluded.day_mask,
	start_date = excluded.start_date,
	end_date = excluded.end_date,
	fetched_at = excluded.fetched_at
RETURNING *;

-- name: DeleteProviderTrips :execrows
-- Trip rows are replaced wholesale each run; stop_times and frequencies
-- die with them by ON DELETE CASCADE — one statement, no orphans.
DELETE FROM trips
WHERE provider_id = $1;

-- name: InsertTrip :one
INSERT INTO trips (
	route_id,
	service_id,
	provider_id,
	provider_entity_id,
	headsign,
	direction_id,
	fetched_at
) VALUES (
	$1, $2, $3, $4, $5, $6, $7
)
RETURNING id;

-- name: InsertStopTime :exec
INSERT INTO stop_times (trip_id, stop_id, seq, arrival_seconds, departure_seconds, derived)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: InsertFrequency :exec
INSERT INTO frequencies (trip_id, start_seconds, end_seconds, headway_seconds, exact_times)
VALUES ($1, $2, $3, $4, $5);

-- name: GetProviderByCode :one
SELECT * FROM providers WHERE code = $1;

-- name: ListCatalogStops :many
-- Every live stop of a provider — the sweep's target list and the
-- timetable reconstruction's station reference.
SELECT
	s.id,
	s.provider_entity_id,
	s.code,
	s.name
FROM stops s
JOIN providers p ON p.id = s.provider_id
WHERE p.code = $1 AND s.removed_at IS NULL;

-- name: ListRouteStopTopology :many
-- Ordered (route, seq) stop topology for every route of a provider — the
-- reconstruction uses it to resolve boundFor termini and travel direction.
SELECT
	r.provider_entity_id AS route_key,
	s.provider_entity_id AS stop_key,
	s.name,
	rs.seq
FROM route_stops rs
JOIN routes r ON r.id = rs.route_id
JOIN stops s ON s.id = rs.stop_id
JOIN providers p ON p.id = r.provider_id
WHERE p.code = $1 AND r.removed_at IS NULL AND s.removed_at IS NULL
ORDER BY r.provider_entity_id, rs.seq;

-- name: ListStopCoordsByProvider :many
-- All of a provider's live stops with lon/lat — the geo-match reference set
-- for feed stops that have no catalog id and no parent_station.
SELECT
	s.id,
	s.provider_entity_id,
	st_x(s.location::geometry) AS lon,
	st_y(s.location::geometry) AS lat
FROM stops s
JOIN providers p ON p.id = s.provider_id
WHERE p.code = $1 AND s.removed_at IS NULL;

-- name: ListScheduleRows :many
-- The planner snapshot: one row per stop_time joined with trip, service
-- window, and route mode. Loaded wholesale (~35k rows) — the schedule is
-- static data, so the engine caches it and refreshes on a TTL.
SELECT
	t.id AS trip_id,
	t.provider_entity_id AS trip_key,
	t.headsign,
	t.direction_id,
	s.day_mask,
	s.start_date,
	s.end_date,
	r.id AS route_id,
	r.provider_entity_id AS route_key,
	r.short_name,
	r.long_name,
	r.mode,
	st.seq,
	st.stop_id,
	st.arrival_seconds,
	st.departure_seconds,
	st.derived
FROM trips t
JOIN services s ON s.id = t.service_id
JOIN routes r ON r.id = t.route_id
JOIN stop_times st ON st.trip_id = t.id
ORDER BY t.id, st.seq;

-- name: ListAllFrequencies :many
SELECT trip_id, start_seconds, end_seconds, headway_seconds, exact_times
FROM frequencies;

-- name: ListTransferEdges :many
SELECT from_stop_id, to_stop_id, walk_distance_m FROM transfers;

-- name: ListPlannerStops :many
-- Stop refs the planner needs: names for output, amenities for stepFree.
SELECT s.id, s.provider_entity_id, s.name, s.metadata
FROM stops s
WHERE s.removed_at IS NULL;
