-- name: RecordVisitEvent :one
-- Idempotent check-in (docs/24): replaying the same client_mutation_id
-- returns the already-recorded row instead of erroring. was_inserted tells
-- the caller which path happened so the API can answer 201 vs 200 honestly.
WITH ins AS (
	INSERT INTO visit_events (
		user_id, stop_id, client_mutation_id,
		observed_at, validation_method, distance_m, status
	)
	VALUES ($1, $2, $3, $4, $5, $6, $7)
	ON CONFLICT (user_id, client_mutation_id) DO NOTHING
	RETURNING *
)
SELECT *, true AS was_inserted FROM ins
UNION ALL
SELECT *, false AS was_inserted FROM visit_events
WHERE user_id = $1 AND client_mutation_id = $3
	AND NOT EXISTS (SELECT 1 FROM ins)
LIMIT 1;

-- name: StopDistanceM :one
-- Meters between a device fix and a catalog stop — the geofence check.
-- Location math stays in PostGIS, never in Go.
SELECT st_distance(s.location, st_setsrid(st_makepoint(sqlc.arg(lon), sqlc.arg(lat)), 4326)::geography)
FROM stops s
WHERE s.id = sqlc.arg(stop_id) AND s.removed_at IS NULL;

-- name: ListVisitEvents :many
-- Keyset pagination — offset would drift under appends (offline replays can
-- land mid-history). The handler fetches limit+1 to learn whether a next
-- page exists; cursor = (observed_at, id) of the last emitted row.
SELECT id, user_id, stop_id, client_mutation_id, observed_at, validation_method, distance_m, status, created_at
FROM visit_events
WHERE user_id = $1
	AND (sqlc.narg('before_at')::timestamptz IS NULL
		OR (observed_at, id) < (sqlc.narg('before_at'), sqlc.narg('before_id')::uuid))
ORDER BY observed_at DESC, id DESC
LIMIT sqlc.arg('limit');
