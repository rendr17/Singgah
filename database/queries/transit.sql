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

-- name: GetStop :one
-- location is returned as lon/lat floats — callers never handle raw geography.
SELECT
	id,
	parent_station_id,
	kind,
	code,
	name,
	st_x(location::geometry) AS lon,
	st_y(location::geometry) AS lat,
	metadata,
	fetched_at,
	source_updated_at
FROM stops
WHERE id = $1;

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
