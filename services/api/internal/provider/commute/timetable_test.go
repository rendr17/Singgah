package commute

import (
	"strings"
	"testing"
)

// Timetable reconstruction against synthetic entries shaped like the real
// provider semantics verified 26 Sep 2026 — the cases mirror the operator
// differences BuildTrips exists to handle.

func entry(id, stationID, dep, arr, boundFor, line, trip string) TimetableEntry {
	var tn *string
	if trip != "" {
		tn = &trip
	}
	return TimetableEntry{
		ID:                 id,
		StationID:          stationID,
		TripNumber:         tn,
		EstimatedDeparture: dep,
		EstimatedArrival:   arr,
		BoundFor:           boundFor,
		LineCode:           line,
	}
}

var kciTopo = map[string]LineTopology{
	"KCI:C": {RouteKey: "KCI:C", Stops: []TopoStop{
		{EntityID: "KCI-SUD", Name: "Sudirman"},
		{EntityID: "KCI-THB", Name: "Tanah Abang"},
		{EntityID: "KCI-DU", Name: "Duri"},
		{EntityID: "KCI-AK", Name: "Angke"},
	}},
}

var mrtjTopo = map[string]LineTopology{
	"MRTJ:M": {RouteKey: "MRTJ:M", Stops: []TopoStop{
		{EntityID: "MRTJ-BHI", Name: "Bundaran HI"},
		{EntityID: "MRTJ-DKA", Name: "Dukuh Atas BNI"},
		{EntityID: "MRTJ-SEN", Name: "Senayan"},
		{EntityID: "MRTJ-LBB", Name: "Lebak Bulus"},
	}},
}

var lrtjTopo = map[string]LineTopology{
	"LRTJ:S": {RouteKey: "LRTJ:S", Stops: []TopoStop{
		{EntityID: "LRTJ-PGD", Name: "Kelapa Gading"},
		{EntityID: "LRTJ-BVU", Name: "Boulevard Utara Summarecon Mall"},
		{EntityID: "LRTJ-VEL", Name: "Velodrome"},
	}},
}

func findTrip(trips []ReconstructedTrip, key string) *ReconstructedTrip {
	for i := range trips {
		if trips[i].EntityKey == key {
			return &trips[i]
		}
	}
	return nil
}

func TestBuildTripsKCIChainsByTripNumber(t *testing.T) {
	// tripNumber is a real run id: one entry per served station, and
	// estimatedArrival is the published arrival at the boundFor terminus.
	res := BuildTrips([]TimetableEntry{
		entry("KCI-SUD-5023B", "KCI-SUD", "05:09:00", "05:28:00", "Angke", "C", "5023B"),
		entry("KCI-DU-5023B", "KCI-DU", "05:22:00", "05:28:00", "Angke", "C", "5023B"),
		entry("KCI-THB-5023B", "KCI-THB", "05:15:00", "05:28:00", "Angke", "C", "5023B"),
	}, kciTopo)
	if len(res.Rejections) != 0 {
		t.Fatalf("unexpected rejections: %v", res.Rejections)
	}
	if len(res.Trips) != 1 {
		t.Fatalf("trips=%d, want 1", len(res.Trips))
	}
	trip := res.Trips[0]
	got := []string{}
	for _, s := range trip.Stops {
		got = append(got, s.StopEntityID)
	}
	want := []string{"KCI-SUD", "KCI-THB", "KCI-DU", "KCI-AK"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("stops=%v, want %v", got, want)
		}
	}
	last := trip.Stops[len(trip.Stops)-1]
	if last.ArrivalSec != 5*3600+28*60 {
		t.Fatalf("terminus arrival=%d, want published 05:28", last.ArrivalSec)
	}
	if last.Derived {
		t.Error("published terminus arrival must not be marked derived")
	}
	if trip.Direction != 0 {
		t.Errorf("direction=%d, want 0 (seq ascending toward Angke)", trip.Direction)
	}
}

func TestBuildTripsMRTJDerivesTerminusFromReverse(t *testing.T) {
	// arr == dep means no published terminus arrival: inbound trips get a
	// derived terminus from the observed reverse-direction first segment.
	res := BuildTrips([]TimetableEntry{
		entry("MRTJ-BHI-1", "MRTJ-BHI", "06:00:00", "06:00:00", "Lebak Bulus Bank Syariah Indonesia", "M", "MRTJ-1001"),
		entry("MRTJ-DKA-1", "MRTJ-DKA", "06:03:00", "06:03:00", "Lebak Bulus Bank Syariah Indonesia", "M", "MRTJ-1001"),
		entry("MRTJ-SEN-1", "MRTJ-SEN", "06:06:00", "06:06:00", "Lebak Bulus Bank Syariah Indonesia", "M", "MRTJ-1001"),
		entry("MRTJ-LBB-2", "MRTJ-LBB", "06:10:00", "06:10:00", "Bundaran HI Bank Jakarta", "M", "MRTJ-1002"),
		entry("MRTJ-SEN-2", "MRTJ-SEN", "06:13:00", "06:13:00", "Bundaran HI Bank Jakarta", "M", "MRTJ-1002"),
		entry("MRTJ-DKA-2", "MRTJ-DKA", "06:16:00", "06:16:00", "Bundaran HI Bank Jakarta", "M", "MRTJ-1002"),
	}, mrtjTopo)
	if len(res.Trips) != 2 {
		t.Fatalf("trips=%d, want 2 (rejections: %v)", len(res.Trips), res.Rejections)
	}
	var south *ReconstructedTrip
	for i := range res.Trips {
		if res.Trips[i].Headsign == "Lebak Bulus Bank Syariah Indonesia" {
			south = &res.Trips[i]
		}
	}
	if south == nil {
		t.Fatal("no southbound trip")
	}
	last := south.Stops[len(south.Stops)-1]
	if last.StopEntityID != "MRTJ-LBB" || !last.Derived {
		t.Fatalf("last stop = %+v, want derived MRTJ-LBB", last)
	}
	// LBB->SEN first-segment median is 3 min → last published SEN 06:06
	// + 3 = 06:09 at LBB.
	if last.ArrivalSec != 6*3600+9*60 {
		t.Fatalf("derived terminus arrival=%d, want 06:09", last.ArrivalSec)
	}
	if !south.Derived {
		t.Error("trip with a derived stop must carry the flag")
	}
}

func TestBuildTripsLRTJChainsPerEntryIDs(t *testing.T) {
	// tripNumber embeds the station id — station-local, no run identity.
	// Entries chain by direction + spacing window; every stop lands
	// Derived, and the terminus is estimated from the chain's own pacing.
	res := BuildTrips([]TimetableEntry{
		entry("LRTJ-VEL-06:00-PGD-BOUND", "LRTJ-VEL", "06:00:00", "00:00:00", "Kelapa Gading", "S", "LRTJ-VEL-06:00-PGD-BOUND"),
		entry("LRTJ-VEL-06:10-PGD-BOUND", "LRTJ-VEL", "06:10:00", "00:00:00", "Kelapa Gading", "S", "LRTJ-VEL-06:10-PGD-BOUND"),
		entry("LRTJ-BVU-06:02-PGD-BOUND", "LRTJ-BVU", "06:02:00", "00:00:00", "Kelapa Gading", "S", "LRTJ-BVU-06:02-PGD-BOUND"),
		entry("LRTJ-BVU-06:12-PGD-BOUND", "LRTJ-BVU", "06:12:00", "00:00:00", "Kelapa Gading", "S", "LRTJ-BVU-06:12-PGD-BOUND"),
	}, lrtjTopo)
	if len(res.Trips) != 2 {
		t.Fatalf("trips=%d, want 2 (rejections: %v)", len(res.Trips), res.Rejections)
	}
	for _, trip := range res.Trips {
		if !trip.Derived {
			t.Errorf("chained trip must be derived")
		}
		// VEL->BVU observed + PGD estimated from the chain's own 2-min
		// pacing (terminus adjacent, marked derived).
		if len(trip.Stops) != 3 {
			t.Fatalf("stops=%v, want VEL+BVU+PGD", trip.Stops)
		}
		if trip.Stops[2].StopEntityID != "LRTJ-PGD" || !trip.Stops[2].Derived {
			t.Fatalf("terminus = %+v, want derived LRTJ-PGD", trip.Stops[2])
		}
		if trip.Direction != 1 {
			t.Errorf("direction=%d, want 1 (descending toward seq-0 Kelapa Gading)", trip.Direction)
		}
	}
	if res.Trips[0].Stops[0].DepartureSec != 6*3600 {
		t.Fatalf("first chain starts %d, want 06:00", res.Trips[0].Stops[0].DepartureSec)
	}
}

func TestBuildTripsRejectsDegenerateAndFlagsGaps(t *testing.T) {
	// A trip listed at only one station can't be ridden; an entry whose
	// station isn't on its line's topology is skipped, not fatal.
	res := BuildTrips([]TimetableEntry{
		entry("KCI-SUD-X", "KCI-SUD", "07:00:00", "07:00:00", "Angke", "C", "X"),
		entry("KCI-THB-Y", "KCI-THB", "07:10:00", "07:00:00", "Nowhere", "C", "Y"),   // boundFor unresolvable
		entry("KCI-ELSE-Y", "KCI-ELSE", "07:11:00", "07:00:00", "Nowhere", "C", "Y"), // off topology
		entry("KCI-DU-Y", "KCI-DU", "07:12:00", "07:00:00", "Nowhere", "C", "Y"),
	}, kciTopo)
	if len(res.Trips) != 1 {
		t.Fatalf("trips=%d, want 1", len(res.Trips))
	}
	if len(res.Rejections) != 1 || !strings.Contains(res.Rejections[0], "fewer than 2") {
		t.Fatalf("rejections=%v, want one 'fewer than 2'", res.Rejections)
	}
	if res.SkippedStops != 1 {
		t.Fatalf("skipped=%d, want 1", res.SkippedStops)
	}
	trip := res.Trips[0]
	// boundFor "Nowhere" never resolves — the trip ends at its last
	// published stop, honestly truncated.
	if len(trip.Stops) != 2 || trip.Stops[0].StopEntityID != "KCI-THB" || trip.Stops[1].StopEntityID != "KCI-DU" {
		t.Fatalf("stops=%v, want [KCI-THB KCI-DU]", trip.Stops)
	}
	if trip.Direction != -1 {
		t.Errorf("direction=%d, want -1 unresolved", trip.Direction)
	}
}

func TestResolveBoundFor(t *testing.T) {
	stops := []TopoStop{
		{EntityID: "A", Name: "Terminal 1"},
		{EntityID: "B", Name: "Lebak Bulus"},
		{EntityID: "C", Name: "Bundaran HI"},
	}
	cases := map[string]int{
		"Terminal 1":                         0,
		"Lebak Bulus Bank Syariah Indonesia": 1,
		"Bundaran HI Bank Jakarta":           2,
		"bundaran hi":                        2,
	}
	for in, want := range cases {
		got, ok := resolveBoundFor(in, stops)
		if !ok || got != want {
			t.Errorf("resolveBoundFor(%q) = %d,%v want %d,true", in, got, ok, want)
		}
	}
	if _, ok := resolveBoundFor("Gambir", stops); ok {
		t.Error("unknown destination must not resolve")
	}
}
