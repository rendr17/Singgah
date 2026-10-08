-- +goose Up
-- Phase 4 City Explorer: curated walking trails plus private save/visit state.
-- Trail place patterns are editorial lookup hints into the imported OSM catalog;
-- API responses always resolve them to canonical places.id values and retain
-- the matched place's provider attribution.

CREATE TABLE trails (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    slug text NOT NULL UNIQUE,
    title text NOT NULL,
    description text NOT NULL,
    theme text NOT NULL,
    start_provider_code text NOT NULL,
    start_provider_entity_id text NOT NULL,
    end_provider_code text NOT NULL,
    end_provider_entity_id text NOT NULL,
    budget_min_idr integer CHECK (budget_min_idr IS NULL OR budget_min_idr >= 0),
    budget_max_idr integer CHECK (budget_max_idr IS NULL OR budget_max_idr >= 0),
    status text NOT NULL DEFAULT 'published' CHECK (status IN ('draft', 'published')),
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (budget_min_idr IS NULL OR budget_max_idr IS NULL OR budget_min_idr <= budget_max_idr)
);

CREATE TABLE trail_stops (
    trail_id uuid NOT NULL REFERENCES trails (id) ON DELETE CASCADE,
    sequence smallint NOT NULL CHECK (sequence BETWEEN 1 AND 6),
    place_name_pattern text NOT NULL CHECK (length(place_name_pattern) > 2),
    stay_minutes smallint NOT NULL DEFAULT 0 CHECK (stay_minutes BETWEEN 0 AND 240),
    notes text NOT NULL DEFAULT '',
    PRIMARY KEY (trail_id, sequence)
);

CREATE TABLE saved_places (
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    place_id uuid NOT NULL REFERENCES places (id),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, place_id)
);
CREATE INDEX saved_places_user_created_idx ON saved_places (user_id, created_at DESC);

-- Manual visit attestations are append-only and idempotent for offline retries.
-- Unlike station check-ins, this does not request or persist device location.
CREATE TABLE place_visit_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    place_id uuid NOT NULL REFERENCES places (id),
    client_mutation_id uuid NOT NULL,
    observed_at timestamptz NOT NULL,
    validation_method text NOT NULL DEFAULT 'manual' CHECK (validation_method = 'manual'),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, client_mutation_id)
);
CREATE INDEX place_visit_events_user_observed_idx ON place_visit_events (user_id, observed_at DESC);

INSERT INTO trails (
    slug, title, description, theme,
    start_provider_code, start_provider_entity_id,
    end_provider_code, end_provider_entity_id
) VALUES
    ('cikini-90-menit', 'Cikini 90 menit', 'Jeda singkat untuk menikmati ruang seni dan membaca di sekitar Cikini.', 'Budaya & santai', 'commute', 'KCI-CKI', 'commute', 'KCI-CKI'),
    ('blok-m-sore-sore', 'Blok M sore-sore', 'Ruang hijau, tempat kreatif, dan pilihan belanja dekat simpul Blok M.', 'Kota & kreatif', 'commute', 'MRTJ-BLM', 'commute', 'TJ-H00014P'),
    ('glodok-lapar-mata', 'Glodok lapar mata', 'Jelajah kawasan Glodok: tempat ibadah, pasar, dan jejak kawasan Pecinan.', 'Budaya & kuliner', 'commute', 'TJ-H00068P', 'commute', 'KCI-MGB'),
    ('museum-hopping', 'Museum hopping Kota Tua', 'Tiga museum di kawasan Kota Tua dengan akses KRL Jakarta Kota.', 'Budaya & sejarah', 'commute', 'KCI-JAKK', 'commute', 'KCI-JAKK');

INSERT INTO trail_stops (trail_id, sequence, place_name_pattern, stay_minutes, notes)
SELECT t.id, s.sequence, s.place_name_pattern, s.stay_minutes, s.notes
FROM trails t
JOIN (VALUES
    ('cikini-90-menit', 1, '%Taman Ismail Marzuki%', 35, 'Mulai dari kompleks seni dan ruang publik.'),
    ('cikini-90-menit', 2, '%Perpustakaan Jakarta%', 30, 'Waktu kunjungan mengikuti jam operasional lokasi.'),
    ('blok-m-sore-sore', 1, '%Taman Literasi%', 25, 'Ruang hijau dekat akses MRT.'),
    ('blok-m-sore-sore', 2, '%M Bloc%', 40, 'Area kreatif; tenant dan jam buka dapat berubah.'),
    ('blok-m-sore-sore', 3, '%Blok M Square%', 30, 'Pilihan belanja; cek jam operasional sebelum berangkat.'),
    ('glodok-lapar-mata', 1, '%Vihara Dharma Bhakti%', 25, 'Hormati kegiatan ibadah dan aturan pengunjung.'),
    ('glodok-lapar-mata', 2, '%Petak Sembilan%', 35, 'Pasar dan tenant dapat memiliki jam berbeda.'),
    ('museum-hopping', 1, '%Museum Fatahillah%', 40, 'Jam buka dan tiket perlu dicek ke pengelola.'),
    ('museum-hopping', 2, '%Museum Wayang%', 35, 'Jam buka dan tiket perlu dicek ke pengelola.'),
    ('museum-hopping', 3, '%Museum Bank Indonesia%', 40, 'Jam buka dan tiket perlu dicek ke pengelola.')
) AS s(slug, sequence, place_name_pattern, stay_minutes, notes) ON s.slug = t.slug;

-- +goose Down
DROP TABLE place_visit_events;
DROP TABLE saved_places;
DROP TABLE trail_stops;
DROP TABLE trails;
