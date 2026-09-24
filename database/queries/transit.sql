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
	source_updated_at = excluded.source_updated_at
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
	source_updated_at = excluded.source_updated_at
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

-- name: TouchProviderLastSuccess :exec
-- Stamps a completed ingest run on the provider registry row.
UPDATE providers
SET last_success_at = now()
WHERE code = $1;

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
	p.code AS provider_code
FROM stops s
JOIN providers p ON p.id = s.provider_id
WHERE s.id = $1;

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
	p.code AS provider_code
FROM stops s
JOIN providers p ON p.id = s.provider_id
WHERE
	s.name ILIKE $1
	OR s.code ILIKE $1
	OR s.metadata->>'official_name' ILIKE $1
ORDER BY (s.metadata->>'score')::float8 DESC NULLS LAST, s.name
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
	(t.accessibility->>'notes')::text AS notes
FROM transfers t
JOIN stops s2 ON s2.id = t.to_stop_id
WHERE t.from_stop_id = $1
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
WHERE $1::text = '' OR r.short_name ILIKE '%' || $1 || '%' OR r.long_name ILIKE '%' || $1 || '%'
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
WHERE r.id = $1;

-- name: ListStopsOnRoute :many
-- Stops whose provider line keys include this route's entity id. Ordering is
-- unknown until line-detail/trip ingest lands — sorted by name for now.
SELECT
	s.id,
	s.kind,
	s.code,
	s.name,
	st_x(s.location::geometry) AS lon,
	st_y(s.location::geometry) AS lat
FROM routes r
JOIN stops s ON s.provider_id = r.provider_id
WHERE
	r.id = $1
	AND EXISTS (
		SELECT 1
		FROM jsonb_array_elements_text(s.metadata->'lines') AS line_key
		WHERE line_key = r.provider_entity_id
	)
ORDER BY s.name;

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
WHERE st_dwithin(location, wgs84_point($1, $2), $3)
ORDER BY distance_m
LIMIT $4;
