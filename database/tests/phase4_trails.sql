-- Phase 4 trail acceptance: exact catalog mappings + complete transit anchors.
DO $$
DECLARE
    published_count integer;
BEGIN
    SELECT count(*) INTO published_count
    FROM trails
    WHERE slug IN ('cikini-seni-sejarah', 'blok-m-sore-sore', 'glodok-lapar-mata', 'museum-hopping')
      AND status = 'published';

    IF published_count <> 4 THEN
        RAISE EXCEPTION 'expected 4 published Phase 4 trails, got %', published_count;
    END IF;

    IF EXISTS (
        SELECT 1
        FROM (VALUES
            ('cikini-seni-sejarah', 1, 'Taman Ismail Marzuki', 'way:28971307'),
            ('cikini-seni-sejarah', 2, 'Monumen Kudatuli', 'node:14137192296'),
            ('blok-m-sore-sore', 1, 'Taman Literasi Martha Tiahahu', 'node:10277068519'),
            ('blok-m-sore-sore', 2, 'RUCI Art Space', 'way:659808481'),
            ('blok-m-sore-sore', 3, 'Blok M Square', 'way:516775426'),
            ('glodok-lapar-mata', 1, 'Vihara Dharma Sakti', 'way:1331227709'),
            ('glodok-lapar-mata', 2, 'Pasar Glodok', 'way:492256169'),
            ('glodok-lapar-mata', 3, 'Cita Rasa Restu', 'node:5479457121'),
            ('museum-hopping', 1, 'Museum Sejarah Jakarta', 'node:4243812179'),
            ('museum-hopping', 2, 'Wayang Museum', 'node:359852056'),
            ('museum-hopping', 3, 'Museum Bank Indonesia', 'relation:7226303')
        ) AS expected(slug, sequence, place_name, osm_entity_id)
        LEFT JOIN trails t ON t.slug = expected.slug
        LEFT JOIN trail_stops ts ON ts.trail_id = t.id AND ts.sequence = expected.sequence
        LEFT JOIN places p ON p.id = ts.place_id
        LEFT JOIN providers source ON source.id = p.provider_id
        WHERE t.status IS DISTINCT FROM 'published'
           OR ts.intended_place_name IS DISTINCT FROM expected.place_name
           OR p.name IS DISTINCT FROM expected.place_name
           OR source.code IS DISTINCT FROM 'osm'
           OR p.provider_entity_id IS DISTINCT FROM expected.osm_entity_id
           OR p.editorial_status = 'hidden'
    ) THEN
        RAISE EXCEPTION 'a published trail stop is not mapped to its exact expected OSM catalog record';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM (VALUES
            ('cikini-seni-sejarah', 2),
            ('blok-m-sore-sore', 3),
            ('glodok-lapar-mata', 3),
            ('museum-hopping', 3)
        ) AS expected(slug, stop_count)
        JOIN trails t ON t.slug = expected.slug
        WHERE (SELECT count(*) FROM trail_stops ts WHERE ts.trail_id = t.id) <> expected.stop_count
           OR (SELECT min(sequence) FROM trail_stops ts WHERE ts.trail_id = t.id) <> 1
           OR (SELECT max(sequence) FROM trail_stops ts WHERE ts.trail_id = t.id) <> expected.stop_count
           OR (SELECT count(DISTINCT place_id) FROM trail_stops ts WHERE ts.trail_id = t.id) <> expected.stop_count
    ) THEN
        RAISE EXCEPTION 'a published trail has an incomplete, gapped, or duplicate stop sequence';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM trails t
        LEFT JOIN providers start_provider ON start_provider.code = t.start_provider_code
        LEFT JOIN stops start_stop ON start_stop.provider_id = start_provider.id
            AND start_stop.provider_entity_id = t.start_provider_entity_id
            AND start_stop.removed_at IS NULL
        LEFT JOIN providers end_provider ON end_provider.code = t.end_provider_code
        LEFT JOIN stops end_stop ON end_stop.provider_id = end_provider.id
            AND end_stop.provider_entity_id = t.end_provider_entity_id
            AND end_stop.removed_at IS NULL
        LEFT JOIN trail_stops first_stop ON first_stop.trail_id = t.id AND first_stop.sequence = 1
        LEFT JOIN trail_stops last_stop ON last_stop.trail_id = t.id
            AND last_stop.sequence = (SELECT max(ts.sequence) FROM trail_stops ts WHERE ts.trail_id = t.id)
        LEFT JOIN place_transit_access start_access
            ON start_access.place_id = first_stop.place_id AND start_access.stop_id = start_stop.id
        LEFT JOIN place_transit_access end_access
            ON end_access.place_id = last_stop.place_id AND end_access.stop_id = end_stop.id
        WHERE t.slug IN ('cikini-seni-sejarah', 'blok-m-sore-sore', 'glodok-lapar-mata', 'museum-hopping')
          AND (
              start_stop.id IS NULL OR end_stop.id IS NULL
              OR first_stop.place_id IS NULL OR last_stop.place_id IS NULL
              OR start_access.walk_distance_m IS NULL OR start_access.walk_distance_m < 0
              OR start_access.computed_at IS NULL
              OR end_access.walk_distance_m IS NULL OR end_access.walk_distance_m < 0
              OR end_access.computed_at IS NULL
          )
    ) THEN
        RAISE EXCEPTION 'a published trail is missing a valid start or end transit access record';
    END IF;

    IF EXISTS (
        WITH ordered AS (
            SELECT
                t.id AS trail_id,
                ts.sequence,
                p.location,
                lag(p.location) OVER (PARTITION BY t.id ORDER BY ts.sequence) AS previous_location,
                row_number() OVER (PARTITION BY t.id ORDER BY ts.sequence) AS position,
                count(*) OVER (PARTITION BY t.id) AS stop_count
            FROM trails t
            JOIN trail_stops ts ON ts.trail_id = t.id
            LEFT JOIN places p ON p.id = ts.place_id
            WHERE t.slug IN ('cikini-seni-sejarah', 'blok-m-sore-sore', 'glodok-lapar-mata', 'museum-hopping')
        )
        SELECT 1
        FROM ordered
        WHERE location IS NULL
           OR (position > 1 AND (previous_location IS NULL OR ST_Distance(previous_location, location) < 0))
    ) THEN
        RAISE EXCEPTION 'a published trail has an invalid walking segment';
    END IF;
END $$;

WITH ordered AS (
    SELECT
        t.id AS trail_id,
        t.slug,
        ts.sequence,
        ts.stay_minutes,
        p.id AS place_id,
        p.location,
        lag(p.location) OVER (PARTITION BY t.id ORDER BY ts.sequence) AS previous_location,
        row_number() OVER (PARTITION BY t.id ORDER BY ts.sequence) AS position,
        count(*) OVER (PARTITION BY t.id) AS stop_count,
        t.start_provider_code,
        t.start_provider_entity_id,
        t.end_provider_code,
        t.end_provider_entity_id
    FROM trails t
    JOIN trail_stops ts ON ts.trail_id = t.id
    JOIN places p ON p.id = ts.place_id
    WHERE t.slug IN ('cikini-seni-sejarah', 'blok-m-sore-sore', 'glodok-lapar-mata', 'museum-hopping')
), legs AS (
    SELECT
        ordered.*,
        CASE WHEN position = 1 THEN start_access.walk_distance_m
             ELSE round(ST_Distance(previous_location, ordered.location))::integer END AS leg_m,
        end_access.walk_distance_m AS end_access_m
    FROM ordered
    JOIN providers start_provider ON start_provider.code = ordered.start_provider_code
    JOIN stops start_stop ON start_stop.provider_id = start_provider.id
        AND start_stop.provider_entity_id = ordered.start_provider_entity_id
        AND start_stop.removed_at IS NULL
    JOIN providers end_provider ON end_provider.code = ordered.end_provider_code
    JOIN stops end_stop ON end_stop.provider_id = end_provider.id
        AND end_stop.provider_entity_id = ordered.end_provider_entity_id
        AND end_stop.removed_at IS NULL
    LEFT JOIN place_transit_access start_access ON start_access.place_id = ordered.place_id
        AND ordered.position = 1 AND start_access.stop_id = start_stop.id
    LEFT JOIN place_transit_access end_access ON end_access.place_id = ordered.place_id
        AND ordered.position = ordered.stop_count AND end_access.stop_id = end_stop.id
), totals AS (
    SELECT
        slug,
        sum(leg_m) + max(end_access_m) AS estimated_walk_distance_m,
        sum(stay_minutes) AS planned_stay_minutes
    FROM legs
    GROUP BY slug
)
SELECT
    slug,
    estimated_walk_distance_m,
    ceil(estimated_walk_distance_m * 1.3 / 80.0)::integer AS estimated_walking_minutes,
    planned_stay_minutes,
    ceil(estimated_walk_distance_m * 1.3 / 80.0)::integer + planned_stay_minutes AS estimated_total_minutes
FROM totals
ORDER BY slug;
