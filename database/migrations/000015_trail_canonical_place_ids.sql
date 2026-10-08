-- +goose Up
-- Resolve curated stops by canonical place ID, never by a fuzzy name match.
-- Keep the intended editorial name for stops missing from the loaded catalog.
ALTER TABLE trail_stops RENAME COLUMN place_name_pattern TO intended_place_name;
ALTER TABLE trail_stops ADD COLUMN place_id uuid REFERENCES places (id);

UPDATE trail_stops ts
SET intended_place_name = mapping.intended_name
FROM trails t
JOIN (VALUES
    ('cikini-90-menit', 1, 'Taman Ismail Marzuki', 'Taman Ismail Marzuki'),
    ('cikini-90-menit', 2, 'Perpustakaan Jakarta Pusat', 'Perpustakaan Jakarta Pusat'),
    ('blok-m-sore-sore', 1, 'Taman Literasi Martha Tiahahu', 'Taman Literasi Martha Tiahahu'),
    ('blok-m-sore-sore', 2, '%M Bloc%', 'M Bloc'),
    ('blok-m-sore-sore', 3, 'Blok M Square', 'Blok M Square'),
    ('glodok-lapar-mata', 1, 'Vihara Dharma Bhakti', 'Vihara Dharma Bhakti'),
    ('glodok-lapar-mata', 2, '%Petak Sembilan%', 'Petak Sembilan'),
    ('museum-hopping', 1, '%Museum Fatahillah%', 'Museum Fatahillah'),
    ('museum-hopping', 2, '%Museum Wayang%', 'Museum Wayang'),
    ('museum-hopping', 3, 'Museum Bank Indonesia', 'Museum Bank Indonesia')
) AS mapping(slug, sequence, old_name, intended_name) ON mapping.slug = t.slug
WHERE ts.trail_id = t.id
    AND ts.sequence = mapping.sequence
    AND ts.intended_place_name = mapping.old_name;

UPDATE trail_stops ts
SET place_id = p.id
FROM trails t
JOIN (VALUES
    ('cikini-90-menit', 1, 'way:28971307'),
    ('cikini-90-menit', 2, 'node:10277993113'),
    ('blok-m-sore-sore', 1, 'node:10277068519'),
    ('blok-m-sore-sore', 3, 'way:516775426'),
    ('glodok-lapar-mata', 1, 'way:521966067'),
    ('museum-hopping', 3, 'relation:7226303')
) AS mapping(slug, sequence, provider_entity_id) ON mapping.slug = t.slug
JOIN providers source ON source.code = 'osm'
JOIN places p ON p.provider_id = source.id AND p.provider_entity_id = mapping.provider_entity_id
WHERE ts.trail_id = t.id
    AND ts.sequence = mapping.sequence;

-- +goose Down
ALTER TABLE trail_stops DROP COLUMN place_id;
ALTER TABLE trail_stops RENAME COLUMN intended_place_name TO place_name_pattern;

UPDATE trail_stops ts
SET place_name_pattern = mapping.old_pattern
FROM trails t
JOIN (VALUES
    ('blok-m-sore-sore', 2, 'M Bloc', '%M Bloc%'),
    ('glodok-lapar-mata', 2, 'Petak Sembilan', '%Petak Sembilan%'),
    ('museum-hopping', 1, 'Museum Fatahillah', '%Museum Fatahillah%'),
    ('museum-hopping', 2, 'Museum Wayang', '%Museum Wayang%')
) AS mapping(slug, sequence, intended_name, old_pattern) ON mapping.slug = t.slug
WHERE ts.trail_id = t.id
    AND ts.sequence = mapping.sequence
    AND ts.place_name_pattern = mapping.intended_name;
