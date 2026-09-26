package osm

import (
	"sort"
	"testing"
)

func p(lon, lat float64) Point { return Point{Lon: lon, Lat: lat} }

func TestStitchChainsUnorderedReversedWays(t *testing.T) {
	// Path 0→4 stored as three ways: one reversed, order shuffled.
	// Shared vertices carry identical coordinates (OSM shared nodes).
	ways := [][]Point{
		{p(3, 0), p(2, 0)}, // C→D stored backwards
		{p(2, 0), p(1, 0)}, // B→C stored backwards
		{p(0, 0), p(1, 0)}, // A→B
	}
	comps := Stitch(ways)
	if len(comps) != 1 {
		t.Fatalf("components = %d, want 1", len(comps))
	}
	c := comps[0]
	if len(c) != 4 {
		t.Fatalf("len = %d, want 4 (0..3)", len(c))
	}
	// Direction of the stitched component is arbitrary (either 0→4 or 4→0);
	// slicing is direction-agnostic, so assert continuity only.
	xs := make([]float64, len(c))
	for i, pt := range c {
		xs[i] = pt.Lon
	}
	sort.Float64s(xs)
	for i, x := range xs {
		if x != float64(i) {
			t.Fatalf("sorted xs = %v — not a continuous chain", xs)
		}
	}
	for i := 1; i < len(c); i++ {
		d := c[i].Lon - c[i-1].Lon
		if d*d != 1 {
			t.Fatalf("component not continuous at %d: %+v → %+v", i, c[i-1], c[i])
		}
	}
}

func TestStitchKeepsGapsAsComponents(t *testing.T) {
	ways := [][]Point{
		{p(0, 0), p(1, 0)},
		{p(5, 5), p(6, 5)}, // disconnected — must NOT be bridged
	}
	comps := Stitch(ways)
	if len(comps) != 2 {
		t.Fatalf("components = %d, want 2 — gaps are honest", len(comps))
	}
}

func TestStitchDropsDegenerateWays(t *testing.T) {
	comps := Stitch([][]Point{{p(0, 0)}, {p(0, 0), p(1, 0)}})
	if len(comps) != 1 || len(comps[0]) != 2 {
		t.Fatalf("got %+v", comps)
	}
}
