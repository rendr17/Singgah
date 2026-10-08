-- +goose Up
-- The loaded OSM catalog was audited before exposing these seed trails.
-- Store exact, unique catalog labels for existing intended places; unresolved
-- or semantically mismatched stops remain drafts until editorial recuration.
UPDATE trail_stops ts
SET place_name_pattern = mapping.verified_name
FROM trails t
JOIN (VALUES
    ('cikini-90-menit', 1, '%Taman Ismail Marzuki%', 'Taman Ismail Marzuki'),
    ('cikini-90-menit', 2, '%Perpustakaan Jakarta%', 'Perpustakaan Jakarta Pusat'),
    ('blok-m-sore-sore', 1, '%Taman Literasi%', 'Taman Literasi Martha Tiahahu'),
    ('blok-m-sore-sore', 3, '%Blok M Square%', 'Blok M Square'),
    ('glodok-lapar-mata', 1, '%Vihara Dharma Bhakti%', 'Vihara Dharma Bhakti'),
    ('museum-hopping', 3, '%Museum Bank Indonesia%', 'Museum Bank Indonesia')
) AS mapping(slug, sequence, old_pattern, verified_name) ON mapping.slug = t.slug
WHERE ts.trail_id = t.id
    AND ts.sequence = mapping.sequence
    AND ts.place_name_pattern = mapping.old_pattern;

-- None of the four seed trails currently has a complete, catalog-verified
-- sequence with both transit endpoints. Do not expose them as published routes.
UPDATE trails
SET status = 'draft'
WHERE slug IN ('cikini-90-menit', 'blok-m-sore-sore', 'glodok-lapar-mata', 'museum-hopping')
    AND status = 'published';

-- +goose Down
UPDATE trails
SET status = 'published'
WHERE slug IN ('cikini-90-menit', 'blok-m-sore-sore', 'glodok-lapar-mata', 'museum-hopping')
    AND status = 'draft';

UPDATE trail_stops ts
SET place_name_pattern = mapping.old_pattern
FROM trails t
JOIN (VALUES
    ('cikini-90-menit', 1, 'Taman Ismail Marzuki', '%Taman Ismail Marzuki%'),
    ('cikini-90-menit', 2, 'Perpustakaan Jakarta Pusat', '%Perpustakaan Jakarta%'),
    ('blok-m-sore-sore', 1, 'Taman Literasi Martha Tiahahu', '%Taman Literasi%'),
    ('blok-m-sore-sore', 3, 'Blok M Square', '%Blok M Square%'),
    ('glodok-lapar-mata', 1, 'Vihara Dharma Bhakti', '%Vihara Dharma Bhakti%'),
    ('museum-hopping', 3, 'Museum Bank Indonesia', '%Museum Bank Indonesia%')
) AS mapping(slug, sequence, verified_name, old_pattern) ON mapping.slug = t.slug
WHERE ts.trail_id = t.id
    AND ts.sequence = mapping.sequence
    AND ts.place_name_pattern = mapping.verified_name;
