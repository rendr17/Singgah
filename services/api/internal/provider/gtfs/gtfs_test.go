package gtfs

import (
	"archive/zip"
	"bytes"
	"testing"
)

// feedZip builds a minimal in-memory GTFS zip from per-file CSV content.
func feedZip(t *testing.T, files map[string]string) *zip.Reader {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	return zr
}

const routesCSV = `route_id,route_short_name,route_type
R1,6A,3
R2,1,3
`

const tripsCSV = `trip_id,route_id,service_id,direction_id,shape_id
t1,R1,SH,0,shpA
t2,R1,SH,1,shpB
t3,R2,SH,0,shpC
t4,RGONE,SH,0,shpDangling
`

const shapesCSV = `shape_id,shape_pt_sequence,shape_pt_lat,shape_pt_lon
shpA,2,-6.2,106.82
shpA,0,-6.1,106.80
shpA,1,-6.15,106.81
shpB,0,-6.9,106.9
shpB,1,-6.91,106.91
shpOrphan,0,-6.0,106.0
shpOrphan,1,-6.01,106.01
`

func TestParseFeedJoinsShapesToRoutes(t *testing.T) {
	zr := feedZip(t, map[string]string{
		"routes.txt": routesCSV,
		"trips.txt":  tripsCSV,
		"shapes.txt": shapesCSV,
	})
	shapes, stats, err := ParseFeed(zr)
	if err != nil {
		t.Fatal(err)
	}
	if len(shapes) != 2 {
		t.Fatalf("got %d route shapes, want 2 (shpDangling has no trip)", len(shapes))
	}
	byID := map[string]RouteShape{}
	for _, s := range shapes {
		byID[s.ShapeID] = s
	}
	a := byID["shpA"]
	if a.RouteShortName != "6A" || a.DirectionID != 0 {
		t.Fatalf("shpA bound to %+v, want route 6A dir 0", a)
	}
	// Points must be ordered by shape_pt_sequence, not file order.
	want := []Point{{106.80, -6.1}, {106.81, -6.15}, {106.82, -6.2}}
	if len(a.Points) != len(want) {
		t.Fatalf("shpA has %d points, want %d", len(a.Points), len(want))
	}
	for i, p := range a.Points {
		if p != want[i] {
			t.Fatalf("point %d = %+v, want %+v", i, p, want[i])
		}
	}
	// shpC has a trip but no shapes.txt rows; shpDangling's route is missing
	// from routes.txt — both count as unbound.
	if stats.UnboundShape != 2 {
		t.Fatalf("UnboundShape = %d, want 2", stats.UnboundShape)
	}
	if stats.OrphanShape != 1 {
		t.Fatalf("OrphanShape = %d, want 1 (shpOrphan)", stats.OrphanShape)
	}
}

func TestParseFeedCountsOrphanShapes(t *testing.T) {
	zr := feedZip(t, map[string]string{
		"routes.txt": routesCSV,
		"trips.txt": `trip_id,route_id,service_id,direction_id,shape_id
t1,R1,SH,0,shpA
`,
		"shapes.txt": shapesCSV,
	})
	_, stats, err := ParseFeed(zr)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Shapes != 3 {
		t.Fatalf("Shapes = %d, want 3", stats.Shapes)
	}
	if stats.OrphanShape != 2 {
		t.Fatalf("OrphanShape = %d, want 2 (shpB, shpOrphan)", stats.OrphanShape)
	}
}

func TestParseFeedMissingFile(t *testing.T) {
	zr := feedZip(t, map[string]string{"routes.txt": routesCSV})
	if _, _, err := ParseFeed(zr); err == nil {
		t.Fatal("missing trips.txt must error, not silently produce nothing")
	}
}

func TestWKT(t *testing.T) {
	s := RouteShape{Points: []Point{{106.8, -6.1}, {106.9, -6.2}}}
	if got := s.WKT(); got != "LINESTRING(106.8 -6.1,106.9 -6.2)" {
		t.Fatalf("WKT = %s", got)
	}
}
