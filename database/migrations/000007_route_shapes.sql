-- +goose Up
-- Real path geometry for routes, per source shape (docs/23_ROUTING_MAP_GIS.md).
-- Rows are keyed by (source, source_shape_id) so a re-ingested GTFS feed
-- updates in place and multiple shapes per route (directions/patterns)
-- coexist — leg slicing picks the best snap at query time.
CREATE TABLE route_shapes (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	route_id uuid NOT NULL REFERENCES routes (id),
	direction_id smallint,
	shape geometry (linestring, 4326) NOT NULL,
	source text NOT NULL,
	source_shape_id text NOT NULL,
	fetched_at timestamptz,
	UNIQUE (source, source_shape_id)
);

CREATE INDEX route_shapes_route_idx ON route_shapes (route_id);
CREATE INDEX route_shapes_shape_gix ON route_shapes USING gist (shape);

-- +goose Down
DROP TABLE route_shapes;
