-- name: UpsertPlace :one
-- Import upsert keyed on (provider_id, provider_entity_id). editorial_status
-- is deliberately NOT updated on conflict — a re-import must not erase
-- curation (docs/13 editorial distinction, ADR-011).
INSERT INTO places (
	provider_id, provider_entity_id, name, primary_category, location,
	price_band, accessibility, source_payload, source_updated_at
) VALUES (
	$1, $2, $3, $4, wgs84_point($5, $6), $7, $8, $9, $10
)
ON CONFLICT (provider_id, provider_entity_id) DO UPDATE SET
	name = excluded.name,
	primary_category = excluded.primary_category,
	location = excluded.location,
	price_band = excluded.price_band,
	accessibility = excluded.accessibility,
	source_payload = excluded.source_payload,
	source_updated_at = excluded.source_updated_at,
	updated_at = now()
RETURNING id;

-- name: ReplacePlaceTransitAccess :exec
-- Recompute per place: drop stale rows (stop moved out of range or
-- tombstoned), caller re-inserts the fresh set inside the same tx.
DELETE FROM place_transit_access WHERE place_id = $1;

-- name: InsertPlaceTransitAccess :exec
INSERT INTO place_transit_access (place_id, stop_id, walk_distance_m, walk_seconds, computed_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (place_id, stop_id) DO UPDATE SET
	walk_distance_m = excluded.walk_distance_m,
	walk_seconds = excluded.walk_seconds,
	computed_at = excluded.computed_at;

-- name: ListStopsWithinRadiusOfPoint :many
-- Offline access-linkage computation: live stops within radius of a place.
SELECT s.id, s.name, st_distance(s.location, wgs84_point($1, $2))::int AS distance_m
FROM stops s
WHERE s.removed_at IS NULL
	AND st_dwithin(s.location, wgs84_point($1, $2), $3)
ORDER BY distance_m;

-- name: ListPlacesNearStop :many
-- Request-path listing for GET /places?near_stop_id= (ADR-011 ranking):
-- walk distance primary, curated editorial boost, stable name tiebreak.
-- 'hidden' rows never surface.
SELECT p.id, p.name, p.primary_category, p.price_band, p.editorial_status,
	pta.walk_distance_m, pta.walk_seconds,
	st_y(p.location::geometry) AS lat, st_x(p.location::geometry) AS lon
FROM place_transit_access pta
JOIN places p ON p.id = pta.place_id
WHERE pta.stop_id = $1
	AND p.editorial_status <> 'hidden'
ORDER BY pta.walk_distance_m ASC, (p.editorial_status = 'curated') DESC, p.name ASC
LIMIT $2;
