-- Canonical transit catalog queries. Ingest uses upsert on
-- (provider_id, provider_entity_id): re-ingesting a provider entity updates
-- the row and keeps the canonical UUID stable (docs/33 identity + idempotency).

-- name: UpsertProvider :one
INSERT INTO providers (
	code,
	name,
	source_url,
	terms_url,
	license_name,
	attribution_text,
	allowed_use,
	refresh_cadence,
	retention_policy,
	owner,
	known_limitations
) VALUES (
	$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
)
ON CONFLICT (code) DO UPDATE SET
	name = excluded.name,
	source_url = excluded.source_url,
	terms_url = excluded.terms_url,
	license_name = excluded.license_name,
	attribution_text = excluded.attribution_text,
	allowed_use = excluded.allowed_use,
	refresh_cadence = excluded.refresh_cadence,
	retention_policy = excluded.retention_policy,
	owner = excluded.owner,
	known_limitations = excluded.known_limitations
RETURNING *;

-- name: UpsertAgency :one
INSERT INTO agencies (
	provider_id,
	provider_entity_id,
	code,
	name,
	timezone,
	fetched_at,
	source_updated_at
) VALUES (
	$1, $2, $3, $4, $5, $6, $7
)
ON CONFLICT (provider_id, provider_entity_id) DO UPDATE SET
	code = excluded.code,
	name = excluded.name,
	timezone = excluded.timezone,
	fetched_at = excluded.fetched_at,
	source_updated_at = excluded.source_updated_at
RETURNING *;

-- name: UpsertStop :one
INSERT INTO stops (
	provider_id,
	provider_entity_id,
	parent_station_id,
	kind,
	code,
	name,
	location,
	metadata,
	fetched_at,
	source_updated_at
) VALUES (
	$1,
	$2,
	$3,
	$4,
	$5,
	$6,
	wgs84_point($7, $8),
	$9,
	$10,
	$11
)
ON CONFLICT (provider_id, provider_entity_id) DO UPDATE SET
	parent_station_id = excluded.parent_station_id,
	kind = excluded.kind,
	code = excluded.code,
	name = excluded.name,
	location = excluded.location,
	metadata = excluded.metadata,
	fetched_at = excluded.fetched_at,
	source_updated_at = excluded.source_updated_at,
	removed_at = NULL
RETURNING *;

-- name: UpsertRoute :one
INSERT INTO routes (
	agency_id,
	provider_id,
	provider_entity_id,
	short_name,
	long_name,
	mode,
	color,
	text_color,
	fetched_at,
	source_updated_at
) VALUES (
	$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
ON CONFLICT (provider_id, provider_entity_id) DO UPDATE SET
	agency_id = excluded.agency_id,
	short_name = excluded.short_name,
	long_name = excluded.long_name,
	mode = excluded.mode,
	color = excluded.color,
	text_color = excluded.text_color,
	fetched_at = excluded.fetched_at,
	source_updated_at = excluded.source_updated_at,
	removed_at = NULL
RETURNING *;

-- name: UpsertTransfer :one
INSERT INTO transfers (
	from_stop_id,
	to_stop_id,
	walk_distance_m,
	min_transfer_seconds,
	geometry,
	accessibility,
	fare_context,
	fetched_at
) VALUES (
	$1, $2, $3, $4, $5, $6, $7, $8
)
ON CONFLICT (from_stop_id, to_stop_id) DO UPDATE SET
	walk_distance_m = excluded.walk_distance_m,
	min_transfer_seconds = excluded.min_transfer_seconds,
	geometry = excluded.geometry,
	accessibility = excluded.accessibility,
	fare_context = excluded.fare_context,
	fetched_at = excluded.fetched_at
RETURNING *;

-- name: TouchProviderLastAttempt :exec
-- Stamps the START of an ingest run — committed outside the ingest
-- transaction so a failed run still shows up on the health surface.
UPDATE providers
SET last_attempt_at = now()
WHERE code = $1;

-- name: TouchProviderLastSuccess :exec
-- Stamps a completed ingest run on the provider registry row.
UPDATE providers
SET last_success_at = now()
WHERE code = $1;

-- name: MarkRemovedStops :execrows
-- Tombstone every stop of this provider that the latest run did NOT
-- successfully upsert: dropped upstream OR rejected during normalize (its
-- last-ingestable data is stale either way, so it leaves the live catalog).
UPDATE stops
SET removed_at = now()
WHERE
	provider_id = $1
	AND removed_at IS NULL
	AND NOT (provider_entity_id = ANY(sqlc.arg(entity_ids)::text[]));

-- name: MarkRemovedRoutes :execrows
UPDATE routes
SET removed_at = now()
WHERE
	provider_id = $1
	AND removed_at IS NULL
	AND NOT (provider_entity_id = ANY(sqlc.arg(entity_ids)::text[]));

-- name: GetStop :one
-- location is returned as lon/lat floats — callers never handle raw geography.
SELECT
	s.id,
	s.parent_station_id,
	s.kind,
	s.code,
	s.name,
	st_x(s.location::geometry) AS lon,
	st_y(s.location::geometry) AS lat,
	s.metadata,
	s.fetched_at,
	s.source_updated_at,
	s.provider_entity_id,
	p.code AS provider_code
FROM stops s
JOIN providers p ON p.id = s.provider_id
WHERE s.id = $1 AND s.removed_at IS NULL;

-- name: ListStops :many
-- Unfiltered stop list — the catalog's reference set is small enough that a
-- plain bounded list beats a fake "match everything" search.
SELECT
	s.id,
	s.kind,
	s.code,
	s.name,
	st_x(s.location::geometry) AS lon,
	st_y(s.location::geometry) AS lat,
	p.code AS provider_code,
	coalesce(s.metadata->>'operator', '')::text AS operator
FROM stops s
JOIN providers p ON p.id = s.provider_id
WHERE s.removed_at IS NULL
ORDER BY s.name
LIMIT $1;

-- name: ListStopsInBBox :many
-- Stops inside a WGS84 envelope (minLon,minLat,maxLon,maxLat) — the map's
-- viewport-scoped fetch. The geography GiST index serves the && predicate.
-- $5 is the same '%'-wrapped pattern as SearchStops: '%%' disables the text
-- filter; otherwise name, code, and official_name alias all match.
SELECT
	s.id,
	s.kind,
	s.code,
	s.name,
	st_x(s.location::geometry) AS lon,
	st_y(s.location::geometry) AS lat,
	p.code AS provider_code,
	coalesce(s.metadata->>'operator', '')::text AS operator
FROM stops s
JOIN providers p ON p.id = s.provider_id
WHERE
	s.removed_at IS NULL
	AND s.location && st_makeenvelope($1::float8, $2::float8, $3::float8, $4::float8, 4326)::geography
	AND (
		s.name ILIKE $5
		OR s.code ILIKE $5
		OR s.metadata->>'official_name' ILIKE $5
	)
ORDER BY s.name
LIMIT $6;

-- name: SearchStops :many
-- Text search across display name, station code, and the operator's official
-- name (kept in metadata). The gin_trgm index accelerates the ILIKE patterns.
-- Rows are ranked by the provider's footfall score when present.
SELECT
	s.id,
	s.kind,
	s.code,
	s.name,
	st_x(s.location::geometry) AS lon,
	st_y(s.location::geometry) AS lat,
	p.code AS provider_code,
	coalesce(s.metadata->>'operator', '')::text AS operator
FROM stops s
JOIN providers p ON p.id = s.provider_id
WHERE
	s.removed_at IS NULL
	AND (
		s.name ILIKE $1
		OR s.code ILIKE $1
		OR s.metadata->>'official_name' ILIKE $1
	)
-- metadata->>'score' is only cast when it looks numeric — a non-numeric
-- provider value must not turn ordering into a 500.
ORDER BY (CASE WHEN s.metadata->>'score' ~ '^-?[0-9]+(\.[0-9]+)?$'
	THEN (s.metadata->>'score')::float8 END) DESC NULLS LAST, s.name
LIMIT $2;

-- name: ListRoutesServingStop :many
-- Lines serving a stop: the stop's provider line keys (metadata.lines) match
-- routes.provider_entity_id as ingested ("{operator}:{lineCode}").
SELECT
	r.id,
	r.short_name,
	r.long_name,
	r.mode,
	r.color,
	a.code AS agency_code,
	a.name AS agency_name
FROM stops s
JOIN routes r ON r.provider_id = s.provider_id
JOIN agencies a ON a.id = r.agency_id
WHERE
	s.id = $1
	AND s.removed_at IS NULL
	AND r.removed_at IS NULL
	AND EXISTS (
		SELECT 1
		FROM jsonb_array_elements_text(s.metadata->'lines') AS line_key
		WHERE line_key = r.provider_entity_id
	)
ORDER BY r.short_name;

-- name: ListTransfersFromStop :many
SELECT
	t.id,
	t.to_stop_id,
	s2.name AS to_stop_name,
	s2.code AS to_stop_code,
	st_x(s2.location::geometry) AS to_lon,
	st_y(s2.location::geometry) AS to_lat,
	t.walk_distance_m,
	COALESCE(t.accessibility->>'notes', '')::text AS notes
FROM transfers t
JOIN stops s2 ON s2.id = t.to_stop_id
WHERE t.from_stop_id = $1 AND s2.removed_at IS NULL
ORDER BY s2.name;

-- name: ListRoutes :many
-- Optional name/code filter; returns every route when $1 is empty.
SELECT
	r.id,
	r.short_name,
	r.long_name,
	r.mode,
	r.color,
	a.code AS agency_code,
	a.name AS agency_name,
	p.code AS provider_code
FROM routes r
LEFT JOIN agencies a ON a.id = r.agency_id
JOIN providers p ON p.id = r.provider_id
WHERE
	r.removed_at IS NULL
	AND (
		$1::text = ''
		OR r.short_name ILIKE '%' || $1 || '%'
		OR r.long_name ILIKE '%' || $1 || '%'
	)
ORDER BY r.short_name NULLS LAST, r.long_name
LIMIT $2;

-- name: GetRoute :one
SELECT
	r.id,
	r.agency_id,
	r.provider_id,
	r.provider_entity_id,
	r.short_name,
	r.long_name,
	r.mode,
	r.color,
	r.text_color,
	r.fetched_at,
	r.source_updated_at,
	a.code AS agency_code,
	a.name AS agency_name,
	p.code AS provider_code
FROM routes r
LEFT JOIN agencies a ON a.id = r.agency_id
JOIN providers p ON p.id = r.provider_id
WHERE r.id = $1 AND r.removed_at IS NULL;

-- name: ListStopsOnRoute :many
-- Real provider order via route_stops.seq — the relation ingest replaces
-- atomically per route. Falls back to nothing when the route has no
-- ingested sequence (station list then honestly empty, not faked).
SELECT
	s.id,
	s.kind,
	s.code,
	s.name,
	st_x(s.location::geometry) AS lon,
	st_y(s.location::geometry) AS lat,
	rs.seq,
	rs.station_number,
	rs.segment_kind
FROM route_stops rs
JOIN stops s ON s.id = rs.stop_id
WHERE rs.route_id = $1 AND s.removed_at IS NULL
ORDER BY rs.seq;

-- name: ListRouteAlternatives :many
-- Routes that also carry the leg's endpoints — boardable alternatives to the
-- provider's chosen line ("bisa juga naik koridor 2"). seq is flattened
-- segment order, not travel direction, so both orderings are reported; a
-- stop appearing twice on a route yields several pairs and the caller keeps
-- the min-hops one (rows arrive ordered by ABS(seq diff)). Alternatives stay
-- inside the chosen route's provider — different providers may not share
-- deduplicated stops.
SELECT
	r.id,
	r.provider_entity_id,
	r.short_name,
	r.long_name,
	ra.seq AS seq_from,
	rb.seq AS seq_to
FROM route_stops ra
JOIN route_stops rb ON rb.route_id = ra.route_id
JOIN routes r ON r.id = ra.route_id
WHERE
	ra.stop_id = $1
	AND rb.stop_id = $2
	AND r.id <> $3
	AND r.provider_id = (SELECT provider_id FROM routes WHERE id = $3)
	AND r.removed_at IS NULL
ORDER BY r.provider_entity_id, ABS(ra.seq - rb.seq);

-- name: ListRouteStopSlice :many
-- Stops between two seq positions on a route, seq-ascending — the caller
-- reverses when the ride runs the other way. Used to describe a corridor
-- alternative the provider didn't pick.
SELECT
	s.id,
	s.name,
	rs.seq,
	st_x(s.location::geometry) AS lon,
	st_y(s.location::geometry) AS lat
FROM route_stops rs
JOIN stops s ON s.id = rs.stop_id
WHERE rs.route_id = $1 AND rs.seq BETWEEN $2 AND $3 AND s.removed_at IS NULL
ORDER BY rs.seq;

-- name: DeleteRouteStops :exec
-- Route sequence is replaced atomically inside the ingest transaction:
-- delete-then-insert keeps re-ingest idempotent with no stale positions.
DELETE FROM route_stops
WHERE route_id = $1;

-- name: InsertRouteStop :exec
INSERT INTO route_stops (route_id, stop_id, seq, segment_kind, station_number)
VALUES ($1, $2, $3, $4, $5);

-- name: ListStopIDsByProviderEntityIDs :many
-- Reverse mapping for journey responses: provider station ids -> canonical
-- UUIDs, one query for every stop reference in a leg.
SELECT s.id, s.provider_entity_id
FROM stops s
JOIN providers p ON p.id = s.provider_id
WHERE p.code = $1 AND s.provider_entity_id = ANY(sqlc.arg(entity_ids)::text[]) AND s.removed_at IS NULL;

-- name: GetRouteByProviderEntityID :one
SELECT r.id
FROM routes r
JOIN providers p ON p.id = r.provider_id
WHERE p.code = $1 AND r.provider_entity_id = $2 AND r.removed_at IS NULL;

-- name: ListStopsWithin :many
-- Nearest stops to a WGS84 point within radius_m, by real distance in meters.
SELECT
	id,
	parent_station_id,
	kind,
	code,
	name,
	st_x(location::geometry) AS lon,
	st_y(location::geometry) AS lat,
	st_distance(location, wgs84_point($1, $2)) AS distance_m
FROM stops
WHERE removed_at IS NULL AND st_dwithin(location, wgs84_point($1, $2), $3)
ORDER BY distance_m
LIMIT $4;

-- name: ListProviders :many
-- Provider registry as the health surface: last_success_at is the freshest
-- ingest stamp; NULL means registered but never successfully ingested.
SELECT
	code,
	name,
	license_name,
	attribution_text,
	allowed_use,
	refresh_cadence,
	owner,
	known_limitations,
	is_active,
	last_success_at,
	last_attempt_at
FROM providers
ORDER BY code;

-- name: UpsertRouteShape :one
-- Shape ingest replaces by (source, source_shape_id) — a republished GTFS
-- feed updates geometry in place. Shape arrives as WKT LINESTRING.
INSERT INTO route_shapes (
	route_id,
	direction_id,
	shape,
	source,
	source_shape_id,
	fetched_at
) VALUES (
	$1, $2, st_geomfromtext($3, 4326), $4, $5, $6
)
ON CONFLICT (source, source_shape_id) DO UPDATE SET
	route_id = excluded.route_id,
	direction_id = excluded.direction_id,
	shape = excluded.shape,
	fetched_at = excluded.fetched_at
RETURNING id;

-- name: ListStopCoords :many
-- lon/lat for a set of canonical stop ids — journey legs resolve provider
-- refs to UUIDs first, then need coordinates for shape slicing.
SELECT
	id,
	st_x(location::geometry) AS lon,
	st_y(location::geometry) AS lat
FROM stops
WHERE id = ANY(sqlc.arg(ids)::uuid[]);

-- name: SliceRouteShape :one
-- Cut a route's shape between a leg's endpoints, then pick the candidate
-- whose CUT hugs the leg's whole stop sequence. Endpoint snap alone can't
-- tell same-termini variants apart: on TJ:7F a 60 km pattern beat the true
-- shape by ~2 m at the termini while running 600+ m from the listed halte.
-- A best cut still beyond max_snap_m from a listed stop is worse than no
-- shape — no row, and the client draws the honest stop-to-stop polyline.
WITH pts AS (
	SELECT st_setsrid(st_makepoint(u.lon, u.lat), 4326) AS g
	FROM unnest(sqlc.arg('lons')::float8[], sqlc.arg('lats')::float8[]) AS u(lon, lat)
),
cand AS (
	SELECT
		st_linesubstring(
			shape,
			least(
				st_linelocatepoint(shape, st_setsrid(st_makepoint(sqlc.arg('from_lon')::float8, sqlc.arg('from_lat')::float8), 4326)),
				st_linelocatepoint(shape, st_setsrid(st_makepoint(sqlc.arg('to_lon')::float8, sqlc.arg('to_lat')::float8), 4326))
			),
			greatest(
				st_linelocatepoint(shape, st_setsrid(st_makepoint(sqlc.arg('from_lon')::float8, sqlc.arg('from_lat')::float8), 4326)),
				st_linelocatepoint(shape, st_setsrid(st_makepoint(sqlc.arg('to_lon')::float8, sqlc.arg('to_lat')::float8), 4326))
			)
		) AS slice
	FROM route_shapes
	WHERE route_id = sqlc.arg('route_id')::uuid
),
scored AS (
	SELECT
		c.slice,
		sum(st_distance(c.slice::geography, p.g::geography)) AS snap_m,
		max(st_distance(c.slice::geography, p.g::geography)) AS worst_m
	FROM cand c
	LEFT JOIN pts p ON true
	GROUP BY c.slice
)
SELECT st_asgeojson(slice) AS geometry
FROM scored
WHERE worst_m IS NULL OR worst_m <= sqlc.arg('max_snap_m')::float8
ORDER BY snap_m NULLS LAST
LIMIT 1;

-- name: ListRouteLinesInBBox :many
-- One drawable geometry per route for the integrated network map. Real
-- ingested path geometry wins (route_shapes merged across directions and
-- patterns into a MultiLineString); routes with no shape fall back to
-- polylines through their ordered stops — one linestring per segment_kind
-- so trunk and branches don't zigzag across each other. geom_source tells
-- the client which it got. $1..$4 is the WGS84 viewport envelope; a route
-- is returned when its geometry touches it.
WITH shape_geoms AS (
	SELECT
		route_id,
		st_collect(shape) AS geom
	FROM route_shapes
	GROUP BY route_id
),
stop_geoms AS (
	SELECT route_id, st_collect(geom) AS geom
	FROM (
		SELECT
			rs.route_id,
			st_makeline(s.location::geometry ORDER BY rs.seq) AS geom
		FROM route_stops rs
		JOIN stops s ON s.id = rs.stop_id AND s.removed_at IS NULL
		GROUP BY rs.route_id, rs.segment_kind
		HAVING count(*) > 1
	) seg
	GROUP BY route_id
)
SELECT
	r.id,
	r.short_name,
	r.long_name,
	r.mode,
	r.color,
	a.name AS agency_name,
	CASE WHEN sg.geom IS NOT NULL THEN 'shape' ELSE 'stops' END AS geom_source,
	st_asgeojson(COALESCE(sg.geom, stg.geom)) AS geometry
FROM routes r
LEFT JOIN agencies a ON a.id = r.agency_id
LEFT JOIN shape_geoms sg ON sg.route_id = r.id
LEFT JOIN stop_geoms stg ON stg.route_id = r.id
WHERE
	r.removed_at IS NULL
	AND COALESCE(sg.geom, stg.geom) IS NOT NULL
	AND st_intersects(
		COALESCE(sg.geom, stg.geom),
		st_makeenvelope($1::float8, $2::float8, $3::float8, $4::float8, 4326)
	)
ORDER BY r.mode, r.short_name;
