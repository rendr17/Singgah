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

-- name: GetPlaceDetail :one
-- Only places with a live nearby transit relation enter the public catalog.
SELECT p.id, p.name, p.primary_category, p.price_band, p.editorial_status,
	st_y(p.location::geometry) AS lat, st_x(p.location::geometry) AS lon,
	p.accessibility, p.source_updated_at,
	source.code AS source_code, source.name AS source_name,
	source.source_url, source.terms_url, source.license_name, source.attribution_text
FROM places p
JOIN providers source ON source.id = p.provider_id
WHERE p.id = $1
	AND p.editorial_status <> 'hidden'
	AND EXISTS (
		SELECT 1 FROM place_transit_access pta
		JOIN stops s ON s.id = pta.stop_id AND s.removed_at IS NULL
		WHERE pta.place_id = p.id
	);

-- name: ListPlaceTransitAccesses :many
SELECT s.id AS stop_id, s.name AS stop_name, s.kind AS stop_kind,
	pta.walk_distance_m, pta.walk_seconds, pta.computed_at
FROM place_transit_access pta
JOIN stops s ON s.id = pta.stop_id AND s.removed_at IS NULL
WHERE pta.place_id = $1
ORDER BY pta.walk_distance_m, s.name
LIMIT 5;

-- name: SavePlace :execrows
INSERT INTO saved_places (user_id, place_id)
VALUES ($1, $2)
ON CONFLICT (user_id, place_id) DO NOTHING;

-- name: UnsavePlace :execrows
DELETE FROM saved_places WHERE user_id = $1 AND place_id = $2;

-- name: GetPlacePersonalState :one
SELECT EXISTS (
	SELECT 1 FROM saved_places AS saved WHERE saved.user_id = user_row.id AND saved.place_id = $2
) AS saved, latest_visit.observed_at AS visited_at
FROM users AS user_row
LEFT JOIN LATERAL (
	SELECT event.observed_at FROM place_visit_events AS event
	WHERE event.user_id = user_row.id AND event.place_id = $2
	ORDER BY event.observed_at DESC, event.id DESC
	LIMIT 1
) latest_visit ON true
WHERE user_row.id = $1;

-- name: ListSavedPlaces :many
SELECT p.id, p.name, p.primary_category, p.price_band, p.editorial_status,
	st_y(p.location::geometry) AS lat, st_x(p.location::geometry) AS lon,
	saved.created_at AS saved_at,
	nearby.stop_id, nearby.stop_name, nearby.walk_distance_m, nearby.walk_seconds,
	source.code AS source_code, source.name AS source_name,
	source.source_url, source.terms_url, source.license_name, source.attribution_text,
	latest_visit.observed_at AS visited_at
FROM saved_places saved
JOIN places p ON p.id = saved.place_id AND p.editorial_status <> 'hidden'
JOIN providers source ON source.id = p.provider_id
JOIN LATERAL (
	SELECT pta.stop_id, s.name AS stop_name, pta.walk_distance_m, pta.walk_seconds
	FROM place_transit_access pta
	JOIN stops s ON s.id = pta.stop_id AND s.removed_at IS NULL
	WHERE pta.place_id = p.id
	ORDER BY pta.walk_distance_m, s.name
	LIMIT 1
) nearby ON true
LEFT JOIN LATERAL (
	SELECT v.observed_at FROM place_visit_events v
	WHERE v.user_id = saved.user_id AND v.place_id = p.id
	ORDER BY v.observed_at DESC, v.id DESC
	LIMIT 1
) latest_visit ON true
WHERE saved.user_id = $1
ORDER BY saved.created_at DESC, p.name
LIMIT $2;

-- name: RecordPlaceVisit :one
-- Replaying a client mutation returns its original row without creating a
-- second manual attestation. The handler rejects reuse for a different place.
WITH ins AS (
	INSERT INTO place_visit_events (user_id, place_id, client_mutation_id, observed_at)
	VALUES ($1, $2, $3, $4)
	ON CONFLICT (user_id, client_mutation_id) DO NOTHING
	RETURNING *
)
SELECT *, true AS was_inserted FROM ins
UNION ALL
SELECT event.*, false AS was_inserted FROM place_visit_events AS event
WHERE event.user_id = $1 AND event.client_mutation_id = $3
	AND NOT EXISTS (SELECT 1 FROM ins)
LIMIT 1;
