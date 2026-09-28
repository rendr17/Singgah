-- Progress = distinct catalog stops the user has any visit event for,
-- over the stops the network actually serves (route_stops join). Low-
-- confidence visits count — passport progress is personal (docs/42).

-- name: PassportProgressTotal :one
SELECT
	count(DISTINCT rs.stop_id) AS total_stops,
	count(DISTINCT ve.stop_id) AS visited_stops
FROM routes r
JOIN route_stops rs ON rs.route_id = r.id
JOIN stops s ON s.id = rs.stop_id AND s.removed_at IS NULL
LEFT JOIN visit_events ve ON ve.stop_id = rs.stop_id AND ve.user_id = $1
WHERE r.removed_at IS NULL;

-- name: PassportProgressByMode :many
SELECT
	r.mode,
	count(DISTINCT rs.stop_id) AS total_stops,
	count(DISTINCT ve.stop_id) AS visited_stops
FROM routes r
JOIN route_stops rs ON rs.route_id = r.id
JOIN stops s ON s.id = rs.stop_id AND s.removed_at IS NULL
LEFT JOIN visit_events ve ON ve.stop_id = rs.stop_id AND ve.user_id = $1
WHERE r.removed_at IS NULL
GROUP BY r.mode
ORDER BY r.mode;

-- name: PassportProgressByRoute :many
SELECT
	r.id,
	r.provider_entity_id AS route_key,
	r.short_name,
	r.long_name,
	r.mode,
	r.color,
	count(DISTINCT rs.stop_id) AS total_stops,
	count(DISTINCT ve.stop_id) AS visited_stops
FROM routes r
JOIN route_stops rs ON rs.route_id = r.id
JOIN stops s ON s.id = rs.stop_id AND s.removed_at IS NULL
LEFT JOIN visit_events ve ON ve.stop_id = rs.stop_id AND ve.user_id = $1
WHERE r.removed_at IS NULL
GROUP BY r.id
ORDER BY r.mode, r.short_name;
