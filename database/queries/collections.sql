-- Collections are curated editorial sets (docs/13) — published rows only ever
-- reach the public API. Progress counts distinct visited stops inside the set.

-- name: ListCollections :many
SELECT
	c.id,
	c.slug,
	c.title,
	c.description,
	c.kind,
	count(ci.stop_id) AS item_count
FROM collections c
LEFT JOIN collection_items ci ON ci.collection_id = c.id
WHERE c.status = 'published'
GROUP BY c.id
ORDER BY c.title;

-- name: PassportProgressByCollection :many
SELECT
	c.id,
	c.slug,
	c.title,
	count(DISTINCT ci.stop_id) AS total_stops,
	count(DISTINCT ve.stop_id) AS visited_stops
FROM collections c
JOIN collection_items ci ON ci.collection_id = c.id
JOIN stops s ON s.id = ci.stop_id AND s.removed_at IS NULL
LEFT JOIN visit_events ve ON ve.stop_id = ci.stop_id AND ve.user_id = $1
WHERE c.status = 'published'
GROUP BY c.id
ORDER BY c.title;
