-- +goose Up
-- First collections (docs/51 task, docs/42 "progress by collection"). A
-- collection is a named, ordered set of catalog stops that passport progress
-- can be measured against. `kind` encodes the docs/13 editorial distinction
-- up front — 'algorithmic' sets may arrive later (City Explorer) and must not
-- masquerade as curated.
--
-- Seed membership is drawn from real corridor stop sets; the curation is in
-- choosing which sets deserve passport status, titles, and descriptions.

CREATE TABLE collections (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	slug text NOT NULL UNIQUE,
	title text NOT NULL,
	description text,
	kind text NOT NULL DEFAULT 'curated' CHECK (kind IN ('curated', 'algorithmic')),
	status text NOT NULL DEFAULT 'published' CHECK (status IN ('draft', 'published')),
	created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE collection_items (
	collection_id uuid NOT NULL REFERENCES collections (id) ON DELETE CASCADE,
	sequence smallint NOT NULL,
	stop_id uuid NOT NULL REFERENCES stops (id),
	PRIMARY KEY (collection_id, sequence),
	UNIQUE (collection_id, stop_id)
);

CREATE INDEX collection_items_stop_idx ON collection_items (stop_id);

INSERT INTO collections (slug, title, description) VALUES
	('mrt-jakarta', 'MRT Jakarta', 'Semua stasiun lintas Lebak Bulus–Bundaran HI.'),
	('krl-jabodetabek', 'KRL Commuter Jabodetabek', 'Seluruh stasiun Commuter Line di Jabodetabek.'),
	('lrt-jabodebek', 'LRT Jabodebek', 'Stasiun LRT Jabodebek lintas Dukuh Atas–Jatimulya dan Harjamukti.'),
	('tj-koridor-1', 'TransJakarta Koridor 1', 'Halte koridor pertama TransJakarta, Blok M–Kota.');

-- Deterministic ordering: stop name within each collection. Membership comes
-- from route_stops on the seeded corridor keys; a stop on multiple corridors
-- of the same collection appears once.
WITH seed AS (
	SELECT
		c.id AS collection_id,
		rs.stop_id,
		row_number() OVER (
			PARTITION BY c.slug
			ORDER BY min(s.name)
		) AS seq
	FROM collections c
	JOIN routes r ON r.provider_entity_id = ANY (
		CASE c.slug
			WHEN 'mrt-jakarta' THEN ARRAY['MRTJ:M']
			WHEN 'krl-jabodetabek' THEN ARRAY['KCI:A','KCI:B','KCI:C','KCI:R','KCI:T','KCI:TP']
			WHEN 'lrt-jabodebek' THEN ARRAY['LRTJBDB:BK','LRTJBDB:CB']
			WHEN 'tj-koridor-1' THEN ARRAY['TJ:1']
		END
	)
	JOIN route_stops rs ON rs.route_id = r.id
	JOIN stops s ON s.id = rs.stop_id
	WHERE s.removed_at IS NULL
	GROUP BY c.id, c.slug, rs.stop_id
)
INSERT INTO collection_items (collection_id, sequence, stop_id)
SELECT collection_id, seq, stop_id FROM seed;

-- +goose Down
DROP TABLE collection_items;
DROP TABLE collections;
