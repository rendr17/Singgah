// Package gtfs parses the three GTFS Schedule files needed for route shapes:
// routes.txt, trips.txt, shapes.txt. Everything else in the feed is ignored —
// the commute provider remains the schedule/fare source; GTFS contributes
// path geometry only (docs/35_DATA_SOURCES.md).
package gtfs

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// OpenFeed reads a GTFS zip from an http(s) URL or a local file path —
// shared by the shape and schedule ingest commands.
func OpenFeed(spec string) (*zip.Reader, error) {
	if strings.HasPrefix(spec, "http://") || strings.HasPrefix(spec, "https://") {
		// Bound the download: the in-process refresher shares this path and a
		// hung feed must not block the loop past its 30-minute run budget.
		resp, err := (&http.Client{Timeout: 2 * time.Minute}).Get(spec) //nolint:gosec — operator-provided URL is the point
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("feed download: HTTP %d", resp.StatusCode)
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return zip.NewReader(bytes.NewReader(body), int64(len(body)))
	}
	body, err := os.ReadFile(spec)
	if err != nil {
		return nil, err
	}
	return zip.NewReader(bytes.NewReader(body), int64(len(body)))
}

// Point is one vertex of a shape, WGS84.
type Point struct {
	Lon float64
	Lat float64
}

// RouteShape is one drawable path for a route — one row of route_shapes.
// DirectionID is GTFS direction_id (-1 when the feed omits it); several
// shapes per route are expected (directions and trip patterns).
type RouteShape struct {
	RouteShortName string
	DirectionID    int
	ShapeID        string
	Points         []Point
}

// Stats reports parse quality — malformed rows are counted loudly instead of
// failing a 200k-row feed or silently dropping geometry.
type Stats struct {
	Shapes       int
	RouteShapes  int
	SkippedRows  int
	OrphanShape  int // in shapes.txt but no trip references it — ignored by design
	UnboundShape int // trips reference it but shapes.txt has no geometry
}

// WKT renders the shape as a LINESTRING for st_geomfromtext.
func (s RouteShape) WKT() string {
	var b strings.Builder
	b.WriteString("LINESTRING(")
	for i, p := range s.Points {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, "%v %v", p.Lon, p.Lat)
	}
	b.WriteByte(')')
	return b.String()
}

// ParseFeed reads a GTFS zip and returns one RouteShape per shape_id bound to
// a route through trips.txt, ordered deterministically. Orphan shapes (no
// trip reference) are dropped — they belong to no route and could never be
// selected.
func ParseFeed(zr *zip.Reader) ([]RouteShape, Stats, error) {
	var stats Stats

	routeNames, err := csvMap(zr, "routes.txt", "route_id", "route_short_name")
	if err != nil {
		return nil, stats, err
	}

	// shape_id -> (route short name, direction_id); first trip wins.
	type binding struct {
		shortName string
		direction int
	}
	shapeToRoute := map[string]binding{}
	if err := eachCSV(zr, "trips.txt", func(row map[string]string) error {
		shapeID := row["shape_id"]
		if shapeID == "" {
			return nil
		}
		if _, seen := shapeToRoute[shapeID]; seen {
			return nil
		}
		dir := -1
		if v := row["direction_id"]; v != "" {
			d, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("trips.txt: bad direction_id %q", v)
			}
			dir = d
		}
		shapeToRoute[shapeID] = binding{shortName: routeNames[row["route_id"]], direction: dir}
		return nil
	}); err != nil {
		return nil, stats, err
	}

	// shape_id -> seq-ordered points.
	type seqPoint struct {
		seq int
		pt  Point
	}
	raw := map[string][]seqPoint{}
	if err := eachCSV(zr, "shapes.txt", func(row map[string]string) error {
		id := row["shape_id"]
		lat, err1 := strconv.ParseFloat(row["shape_pt_lat"], 64)
		lon, err2 := strconv.ParseFloat(row["shape_pt_lon"], 64)
		seqN, err3 := strconv.Atoi(row["shape_pt_sequence"])
		if id == "" || err1 != nil || err2 != nil || err3 != nil {
			stats.SkippedRows++
			return nil
		}
		if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
			stats.SkippedRows++
			return nil
		}
		raw[id] = append(raw[id], seqPoint{seq: seqN, pt: Point{Lon: lon, Lat: lat}})
		return nil
	}); err != nil {
		return nil, stats, err
	}
	stats.Shapes = len(raw)

	matched := 0
	out := make([]RouteShape, 0, len(shapeToRoute))
	for shapeID, b := range shapeToRoute {
		pts, ok := raw[shapeID]
		if !ok {
			stats.UnboundShape++
			continue
		}
		matched++
		if b.shortName == "" {
			// Trip references a route_id missing from routes.txt — the feed
			// is inconsistent; skip loudly rather than guess the route.
			stats.SkippedRows++
			continue
		}
		sort.Slice(pts, func(i, j int) bool { return pts[i].seq < pts[j].seq })
		shape := RouteShape{
			RouteShortName: b.shortName,
			DirectionID:    b.direction,
			ShapeID:        shapeID,
			Points:         make([]Point, len(pts)),
		}
		for i, sp := range pts {
			shape.Points[i] = sp.pt
		}
		out = append(out, shape)
	}
	stats.OrphanShape = stats.Shapes - matched
	sort.Slice(out, func(i, j int) bool { return out[i].ShapeID < out[j].ShapeID })
	stats.RouteShapes = len(out)
	return out, stats, nil
}

// csvMap streams one GTFS file and returns key->value column pairs.
func csvMap(zr *zip.Reader, name, keyCol, valCol string) (map[string]string, error) {
	out := map[string]string{}
	err := eachCSV(zr, name, func(row map[string]string) error {
		out[row[keyCol]] = row[valCol]
		return nil
	})
	return out, err
}

// eachCSV streams a GTFS file as header-keyed rows — memory stays O(row)
// because a full shapes.txt parse in RAM is how ingest jobs die on laptops.
func eachCSV(zr *zip.Reader, name string, fn func(row map[string]string) error) error {
	f, err := openFile(zr, name)
	if err != nil {
		return err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.ReuseRecord = true
	hdr, err := r.Read()
	if err != nil {
		return fmt.Errorf("%s: header: %w", name, err)
	}
	// ReuseRecord recycles the row buffer — header must be copied or every
	// subsequent Read overwrites the column names.
	header := append([]string(nil), hdr...)
	row := make(map[string]string, len(header))
	for {
		rec, err := r.Read()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		for i, col := range header {
			row[col] = rec[i]
		}
		if err := fn(row); err != nil {
			return err
		}
	}
}

func openFile(zr *zip.Reader, name string) (io.ReadCloser, error) {
	for _, f := range zr.File {
		if f.Name == name {
			return f.Open()
		}
	}
	return nil, fmt.Errorf("gtfs: %s missing from feed", name)
}
