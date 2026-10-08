-- name: ListTrails :many
-- Trails are curated editorial definitions. Each stop resolves through its
-- canonical places.id; intended_place_name is editorial metadata only. The first
-- stop must link to the start transit anchor, and the last to the end anchor;
-- intermediate stops are reached by ordered walking segments, not station trips.
SELECT
    t.id,
    t.slug,
    t.title,
    t.description,
    t.theme,
    start_stop.id AS start_stop_id,
    start_stop.name AS start_stop_name,
    st_y(start_stop.location::geometry) AS start_lat,
    st_x(start_stop.location::geometry) AS start_lon,
    end_stop.id AS end_stop_id,
    end_stop.name AS end_stop_name,
    st_y(end_stop.location::geometry) AS end_lat,
    st_x(end_stop.location::geometry) AS end_lon,
    count(ts.sequence)::bigint AS expected_stop_count,
    count(available.id)::bigint AS available_stop_count,
    count(available.end_access_id)::bigint AS end_access_count,
    t.budget_min_idr,
    t.budget_max_idr
FROM trails t
JOIN providers start_provider ON start_provider.code = t.start_provider_code
JOIN stops start_stop ON start_stop.provider_id = start_provider.id
    AND start_stop.provider_entity_id = t.start_provider_entity_id
    AND start_stop.removed_at IS NULL
JOIN providers end_provider ON end_provider.code = t.end_provider_code
JOIN stops end_stop ON end_stop.provider_id = end_provider.id
    AND end_stop.provider_entity_id = t.end_provider_entity_id
    AND end_stop.removed_at IS NULL
LEFT JOIN trail_stops ts ON ts.trail_id = t.id
LEFT JOIN LATERAL (
    SELECT
        p.id,
        CASE WHEN ts.sequence = (
            SELECT max(last_ts.sequence) FROM trail_stops last_ts WHERE last_ts.trail_id = t.id
        ) THEN p.id END AS end_access_id
    FROM places p
    LEFT JOIN place_transit_access start_access
        ON start_access.place_id = p.id AND start_access.stop_id = start_stop.id
    LEFT JOIN place_transit_access end_access
        ON end_access.place_id = p.id AND end_access.stop_id = end_stop.id
    WHERE p.id = ts.place_id
        AND p.editorial_status <> 'hidden'
        AND (ts.sequence > 1 OR start_access.place_id IS NOT NULL)
        AND (
            ts.sequence < (SELECT max(last_ts.sequence) FROM trail_stops last_ts WHERE last_ts.trail_id = t.id)
            OR end_access.place_id IS NOT NULL
        )
    ORDER BY
        CASE WHEN ts.sequence = 1 THEN start_access.walk_distance_m END NULLS LAST,
        CASE WHEN ts.sequence = (SELECT max(last_ts.sequence) FROM trail_stops last_ts WHERE last_ts.trail_id = t.id) THEN end_access.walk_distance_m END NULLS LAST,
        (p.editorial_status = 'curated') DESC, p.name, p.id
    LIMIT 1
) available ON true
WHERE t.status = 'published'
GROUP BY t.id, start_stop.id, end_stop.id
ORDER BY t.title;

-- name: GetTrail :one
SELECT
    t.id,
    t.slug,
    t.title,
    t.description,
    t.theme,
    start_stop.id AS start_stop_id,
    start_stop.name AS start_stop_name,
    st_y(start_stop.location::geometry) AS start_lat,
    st_x(start_stop.location::geometry) AS start_lon,
    end_stop.id AS end_stop_id,
    end_stop.name AS end_stop_name,
    st_y(end_stop.location::geometry) AS end_lat,
    st_x(end_stop.location::geometry) AS end_lon,
    count(ts.sequence)::bigint AS expected_stop_count,
    count(available.id)::bigint AS available_stop_count,
    count(available.end_access_id)::bigint AS end_access_count,
    t.budget_min_idr,
    t.budget_max_idr
FROM trails t
JOIN providers start_provider ON start_provider.code = t.start_provider_code
JOIN stops start_stop ON start_stop.provider_id = start_provider.id
    AND start_stop.provider_entity_id = t.start_provider_entity_id
    AND start_stop.removed_at IS NULL
JOIN providers end_provider ON end_provider.code = t.end_provider_code
JOIN stops end_stop ON end_stop.provider_id = end_provider.id
    AND end_stop.provider_entity_id = t.end_provider_entity_id
    AND end_stop.removed_at IS NULL
LEFT JOIN trail_stops ts ON ts.trail_id = t.id
LEFT JOIN LATERAL (
    SELECT
        p.id,
        CASE WHEN ts.sequence = (
            SELECT max(last_ts.sequence) FROM trail_stops last_ts WHERE last_ts.trail_id = t.id
        ) THEN p.id END AS end_access_id
    FROM places p
    LEFT JOIN place_transit_access start_access
        ON start_access.place_id = p.id AND start_access.stop_id = start_stop.id
    LEFT JOIN place_transit_access end_access
        ON end_access.place_id = p.id AND end_access.stop_id = end_stop.id
    WHERE p.id = ts.place_id
        AND p.editorial_status <> 'hidden'
        AND (ts.sequence > 1 OR start_access.place_id IS NOT NULL)
        AND (
            ts.sequence < (SELECT max(last_ts.sequence) FROM trail_stops last_ts WHERE last_ts.trail_id = t.id)
            OR end_access.place_id IS NOT NULL
        )
    ORDER BY
        CASE WHEN ts.sequence = 1 THEN start_access.walk_distance_m END NULLS LAST,
        CASE WHEN ts.sequence = (SELECT max(last_ts.sequence) FROM trail_stops last_ts WHERE last_ts.trail_id = t.id) THEN end_access.walk_distance_m END NULLS LAST,
        (p.editorial_status = 'curated') DESC, p.name, p.id
    LIMIT 1
) available ON true
WHERE t.status = 'published' AND t.slug = $1
GROUP BY t.id, start_stop.id, end_stop.id;

-- name: ListTrailStops :many
-- Distances are straight-line PostGIS geography values. The caller labels them
-- as estimates; no turn-by-turn walking graph is implied by this relationship.
WITH matched AS (
    SELECT
        ts.sequence,
        ts.stay_minutes,
        ts.notes,
        p.id AS place_id,
        p.name,
        p.primary_category,
        p.price_band,
        p.editorial_status,
        st_y(p.location::geometry) AS lat,
        st_x(p.location::geometry) AS lon,
        p.location,
        p.accessibility,
        p.source_updated_at,
        source.code AS source_code,
        source.name AS source_name,
        source.source_url,
        source.terms_url,
        source.license_name,
        source.attribution_text,
        pta.walk_distance_m AS start_walk_distance_m,
        end_access.walk_distance_m AS end_walk_distance_m
    FROM trails t
    JOIN trail_stops ts ON ts.trail_id = t.id
    JOIN providers start_provider ON start_provider.code = t.start_provider_code
    JOIN stops start_stop ON start_stop.provider_id = start_provider.id
        AND start_stop.provider_entity_id = t.start_provider_entity_id
        AND start_stop.removed_at IS NULL
    JOIN providers end_provider ON end_provider.code = t.end_provider_code
    JOIN stops end_stop ON end_stop.provider_id = end_provider.id
        AND end_stop.provider_entity_id = t.end_provider_entity_id
        AND end_stop.removed_at IS NULL
    JOIN places p ON p.id = ts.place_id
        AND p.editorial_status <> 'hidden'
    LEFT JOIN place_transit_access pta ON pta.place_id = p.id AND pta.stop_id = start_stop.id
    LEFT JOIN place_transit_access end_access ON end_access.place_id = p.id AND end_access.stop_id = end_stop.id
    JOIN providers source ON source.id = p.provider_id
    WHERE t.status = 'published' AND t.slug = $1
        AND (ts.sequence > 1 OR pta.place_id IS NOT NULL)
        AND (
            ts.sequence < (SELECT max(last_ts.sequence) FROM trail_stops last_ts WHERE last_ts.trail_id = t.id)
            OR end_access.place_id IS NOT NULL
        )
), sequenced AS (
    SELECT
        matched.*,
        lag(location) OVER (ORDER BY sequence) AS previous_location
    FROM matched
)
SELECT
    sequenced.sequence,
    sequenced.stay_minutes,
    sequenced.notes,
    sequenced.place_id,
    sequenced.name,
    sequenced.primary_category,
    sequenced.price_band,
    sequenced.editorial_status,
    sequenced.lat,
    sequenced.lon,
    sequenced.accessibility,
    sequenced.source_updated_at,
    sequenced.source_code,
    sequenced.source_name,
    sequenced.source_url,
    sequenced.terms_url,
    sequenced.license_name,
    sequenced.attribution_text,
    coalesce(round(st_distance(sequenced.previous_location, sequenced.location))::integer, sequenced.start_walk_distance_m) AS walk_distance_from_previous_m,
    sequenced.end_walk_distance_m AS end_walk_distance_m
FROM sequenced
ORDER BY sequenced.sequence;
