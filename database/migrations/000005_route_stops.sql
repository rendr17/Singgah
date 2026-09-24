-- +goose Up
-- Ordered stop sequence per route — the canonical "this line serves these
-- stops in this order" relation. Providers declare segment order; seq is the
-- flattened position across all segments of the route.
CREATE TABLE route_stops (
	route_id uuid NOT NULL REFERENCES routes (id),
	stop_id uuid NOT NULL REFERENCES stops (id),
	seq integer NOT NULL,
	-- Provider segment kind (TRUNK | BRANCH | upstream value) — branches keep
	-- their kind so a UI can group later without re-deriving topology.
	segment_kind text,
	-- Operator-facing number when the source publishes one (M01, C07…).
	station_number text,
	PRIMARY KEY (route_id, seq)
);

CREATE INDEX route_stops_stop_idx ON route_stops (stop_id);

-- +goose Down
DROP TABLE route_stops;
