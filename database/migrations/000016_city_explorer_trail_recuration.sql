-- +goose Up
-- Re-curate against exact loaded OSM records. Mapping uses provider entity IDs;
-- no name-pattern matching is used to publish a trail.
UPDATE trails
SET status = 'draft'
WHERE slug IN ('cikini-90-menit', 'blok-m-sore-sore', 'glodok-lapar-mata', 'museum-hopping');

UPDATE trails
SET slug = 'cikini-seni-sejarah',
    title = 'Cikini: seni & sejarah',
    description = 'Dari kompleks seni Taman Ismail Marzuki ke Monumen Kudatuli. Kunjungan dan jam akses belum terverifikasi; hormati aturan setempat.',
    theme = 'Budaya & sejarah'
WHERE slug = 'cikini-90-menit';

UPDATE trails
SET title = 'Blok M sore-sore',
    description = 'Ruang hijau, tempat kreatif, dan pilihan belanja dekat simpul Blok M.',
    theme = 'Kota & kreatif'
WHERE slug = 'blok-m-sore-sore';

UPDATE trails
SET title = 'Glodok lapar mata',
    description = 'Dari vihara di Glodok ke pasar setempat, lalu lanjut kuliner di Mangga Besar. Hormati kegiatan ibadah; jam buka pasar dan tenant belum tersedia di katalog.',
    theme = 'Budaya & kuliner'
WHERE slug = 'glodok-lapar-mata';

UPDATE trails
SET title = 'Museum hopping Kota Tua',
    description = 'Tiga museum di kawasan Kota Tua dengan akses KRL Jakarta Kota. Jam buka dan tiket perlu dicek ke pengelola.',
    theme = 'Budaya & sejarah'
WHERE slug = 'museum-hopping';

INSERT INTO trail_stops (trail_id, sequence, intended_place_name, stay_minutes, notes)
SELECT id, 3, 'Cita Rasa Restu', 30, 'Pilihan kuliner; jam buka dan ketersediaan tenant belum tersedia di katalog.'
FROM trails
WHERE slug = 'glodok-lapar-mata'
  AND NOT EXISTS (
      SELECT 1 FROM trail_stops ts WHERE ts.trail_id = trails.id AND ts.sequence = 3
  );

-- Clear old mappings first so a missing exact provider ID leaves the route draft
-- instead of silently retaining a semantically different place.
UPDATE trail_stops ts
SET place_id = NULL
FROM trails t
WHERE ts.trail_id = t.id
  AND t.slug IN ('cikini-seni-sejarah', 'blok-m-sore-sore', 'glodok-lapar-mata', 'museum-hopping');

UPDATE trail_stops ts
SET intended_place_name = mapping.place_name,
    stay_minutes = mapping.stay_minutes,
    notes = mapping.notes
FROM trails t
JOIN (VALUES
    ('cikini-seni-sejarah', 1, 'Taman Ismail Marzuki', 35, 'Mulai di kompleks seni; cek agenda dan akses pengunjung.'),
    ('cikini-seni-sejarah', 2, 'Monumen Kudatuli', 10, 'Titik memorial; hormati aturan dan kegiatan setempat.'),
    ('blok-m-sore-sore', 1, 'Taman Literasi Martha Tiahahu', 25, 'Ruang hijau dekat akses MRT.'),
    ('blok-m-sore-sore', 2, 'RUCI Art Space', 40, 'Ruang seni; agenda dan jam kunjungan perlu dicek ke pengelola.'),
    ('blok-m-sore-sore', 3, 'Blok M Square', 30, 'Pilihan belanja; cek jam operasional sebelum berangkat.'),
    ('glodok-lapar-mata', 1, 'Vihara Dharma Sakti', 25, 'Titik kunjungan budaya; cek aturan pengunjung dan hormati kegiatan ibadah.'),
    ('glodok-lapar-mata', 2, 'Pasar Glodok', 35, 'Pasar; tenant dan jam operasi bisa berbeda.'),
    ('glodok-lapar-mata', 3, 'Cita Rasa Restu', 30, 'Pilihan kuliner; jam buka dan ketersediaan tenant belum tersedia di katalog.'),
    ('museum-hopping', 1, 'Museum Sejarah Jakarta', 40, 'Jam operasional dan tiket perlu dicek ke pengelola.'),
    ('museum-hopping', 2, 'Wayang Museum', 35, 'Nama pada katalog sumber; jam operasional dan tiket perlu dicek ke pengelola.'),
    ('museum-hopping', 3, 'Museum Bank Indonesia', 40, 'Jam operasional dan tiket perlu dicek ke pengelola.')
) AS mapping(slug, sequence, place_name, stay_minutes, notes) ON mapping.slug = t.slug
WHERE ts.trail_id = t.id
  AND ts.sequence = mapping.sequence;

UPDATE trail_stops ts
SET place_id = p.id
FROM trails t
JOIN (VALUES
    ('cikini-seni-sejarah', 1, 'way:28971307'),
    ('cikini-seni-sejarah', 2, 'node:14137192296'),
    ('blok-m-sore-sore', 1, 'node:10277068519'),
    ('blok-m-sore-sore', 2, 'way:659808481'),
    ('blok-m-sore-sore', 3, 'way:516775426'),
    ('glodok-lapar-mata', 1, 'way:1331227709'),
    ('glodok-lapar-mata', 2, 'way:492256169'),
    ('glodok-lapar-mata', 3, 'node:5479457121'),
    ('museum-hopping', 1, 'node:4243812179'),
    ('museum-hopping', 2, 'node:359852056'),
    ('museum-hopping', 3, 'relation:7226303')
) AS mapping(slug, sequence, provider_entity_id) ON mapping.slug = t.slug
JOIN providers source ON source.code = 'osm'
JOIN places p ON p.provider_id = source.id AND p.provider_entity_id = mapping.provider_entity_id
WHERE ts.trail_id = t.id
  AND ts.sequence = mapping.sequence;

-- Publish only when all exact stops resolve and both active transit endpoints
-- have a non-negative, computed access estimate. Otherwise the route stays draft.
UPDATE trails t
SET status = 'published'
WHERE t.slug IN ('cikini-seni-sejarah', 'blok-m-sore-sore', 'glodok-lapar-mata', 'museum-hopping')
  AND (SELECT count(*) FROM trail_stops ts WHERE ts.trail_id = t.id) =
      CASE WHEN t.slug = 'cikini-seni-sejarah' THEN 2 ELSE 3 END
  AND (SELECT min(ts.sequence) FROM trail_stops ts WHERE ts.trail_id = t.id) = 1
  AND (SELECT max(ts.sequence) FROM trail_stops ts WHERE ts.trail_id = t.id) =
      CASE WHEN t.slug = 'cikini-seni-sejarah' THEN 2 ELSE 3 END
  AND (SELECT count(DISTINCT ts.place_id) FROM trail_stops ts WHERE ts.trail_id = t.id) =
      CASE WHEN t.slug = 'cikini-seni-sejarah' THEN 2 ELSE 3 END
  AND NOT EXISTS (
      SELECT 1
      FROM trail_stops ts
      LEFT JOIN places p ON p.id = ts.place_id
      WHERE ts.trail_id = t.id
        AND (p.id IS NULL OR p.location IS NULL OR p.editorial_status = 'hidden')
  )
  AND EXISTS (
      SELECT 1
      FROM trail_stops first_stop
      JOIN providers start_provider ON start_provider.code = t.start_provider_code
      JOIN stops start_stop ON start_stop.provider_id = start_provider.id
          AND start_stop.provider_entity_id = t.start_provider_entity_id
          AND start_stop.removed_at IS NULL
      JOIN place_transit_access access ON access.place_id = first_stop.place_id
          AND access.stop_id = start_stop.id
          AND access.walk_distance_m >= 0
          AND access.computed_at IS NOT NULL
      WHERE first_stop.trail_id = t.id AND first_stop.sequence = 1
  )
  AND EXISTS (
      SELECT 1
      FROM trail_stops last_stop
      JOIN providers end_provider ON end_provider.code = t.end_provider_code
      JOIN stops end_stop ON end_stop.provider_id = end_provider.id
          AND end_stop.provider_entity_id = t.end_provider_entity_id
          AND end_stop.removed_at IS NULL
      JOIN place_transit_access access ON access.place_id = last_stop.place_id
          AND access.stop_id = end_stop.id
          AND access.walk_distance_m >= 0
          AND access.computed_at IS NOT NULL
      WHERE last_stop.trail_id = t.id
        AND last_stop.sequence = (SELECT max(ts.sequence) FROM trail_stops ts WHERE ts.trail_id = t.id)
  );

-- +goose Down
UPDATE trails
SET status = 'draft',
    slug = CASE WHEN slug = 'cikini-seni-sejarah' THEN 'cikini-90-menit' ELSE slug END,
    title = CASE
        WHEN slug = 'cikini-seni-sejarah' THEN 'Cikini 90 menit'
        WHEN slug = 'blok-m-sore-sore' THEN 'Blok M sore-sore'
        WHEN slug = 'glodok-lapar-mata' THEN 'Glodok lapar mata'
        WHEN slug = 'museum-hopping' THEN 'Museum hopping Kota Tua'
    END,
    description = CASE
        WHEN slug = 'cikini-seni-sejarah' THEN 'Jeda singkat untuk menikmati ruang seni dan membaca di sekitar Cikini.'
        WHEN slug = 'blok-m-sore-sore' THEN 'Ruang hijau, tempat kreatif, dan pilihan belanja dekat simpul Blok M.'
        WHEN slug = 'glodok-lapar-mata' THEN 'Jelajah kawasan Glodok: tempat ibadah, pasar, dan jejak kawasan Pecinan.'
        WHEN slug = 'museum-hopping' THEN 'Tiga museum di kawasan Kota Tua dengan akses KRL Jakarta Kota.'
    END,
    theme = CASE
        WHEN slug = 'cikini-seni-sejarah' THEN 'Budaya & santai'
        WHEN slug = 'blok-m-sore-sore' THEN 'Kota & kreatif'
        WHEN slug = 'glodok-lapar-mata' THEN 'Budaya & kuliner'
        WHEN slug = 'museum-hopping' THEN 'Budaya & sejarah'
    END
WHERE slug IN ('cikini-seni-sejarah', 'blok-m-sore-sore', 'glodok-lapar-mata', 'museum-hopping');

DELETE FROM trail_stops ts
USING trails t
WHERE ts.trail_id = t.id AND t.slug = 'glodok-lapar-mata' AND ts.sequence = 3;

UPDATE trail_stops ts
SET place_id = NULL
FROM trails t
WHERE ts.trail_id = t.id
  AND t.slug IN ('cikini-90-menit', 'blok-m-sore-sore', 'glodok-lapar-mata', 'museum-hopping');

UPDATE trail_stops ts
SET intended_place_name = original.place_name,
    stay_minutes = original.stay_minutes,
    notes = original.notes
FROM trails t
JOIN (VALUES
    ('cikini-90-menit', 1, 'Taman Ismail Marzuki', 35, 'Mulai dari kompleks seni dan ruang publik.'),
    ('cikini-90-menit', 2, 'Perpustakaan Jakarta Pusat', 30, 'Waktu kunjungan mengikuti jam operasional lokasi.'),
    ('blok-m-sore-sore', 1, 'Taman Literasi Martha Tiahahu', 25, 'Ruang hijau dekat akses MRT.'),
    ('blok-m-sore-sore', 2, 'M Bloc', 40, 'Area kreatif; tenant dan jam buka dapat berubah.'),
    ('blok-m-sore-sore', 3, 'Blok M Square', 30, 'Pilihan belanja; cek jam operasional sebelum berangkat.'),
    ('glodok-lapar-mata', 1, 'Vihara Dharma Bhakti', 25, 'Hormati kegiatan ibadah dan aturan pengunjung.'),
    ('glodok-lapar-mata', 2, 'Petak Sembilan', 35, 'Pasar dan tenant dapat memiliki jam berbeda.'),
    ('museum-hopping', 1, 'Museum Fatahillah', 40, 'Jam buka dan tiket perlu dicek ke pengelola.'),
    ('museum-hopping', 2, 'Museum Wayang', 35, 'Jam buka dan tiket perlu dicek ke pengelola.'),
    ('museum-hopping', 3, 'Museum Bank Indonesia', 40, 'Jam buka dan tiket perlu dicek ke pengelola.')
) AS original(slug, sequence, place_name, stay_minutes, notes) ON original.slug = t.slug
WHERE ts.trail_id = t.id AND ts.sequence = original.sequence;

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
) AS original(slug, sequence, provider_entity_id) ON original.slug = t.slug
JOIN providers source ON source.code = 'osm'
JOIN places p ON p.provider_id = source.id AND p.provider_entity_id = original.provider_entity_id
WHERE ts.trail_id = t.id AND ts.sequence = original.sequence;
