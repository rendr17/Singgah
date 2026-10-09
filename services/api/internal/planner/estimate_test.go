package planner

import (
	"math"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// estimateEngine extends testEngine with coordinates — the shared fixture
// leaves them zero because routing never looks at geography.
func estimateEngine() *Engine {
	e := testEngine()
	coords := map[pgtype.UUID][2]float64{
		uA: {106.0, -6.0}, uB: {106.1, -6.1}, uC: {106.2, -6.2},
		uD: {106.3, -6.3}, uE: {106.4, -6.4}, uF: {106.5, -6.5},
	}
	for id, ll := range coords {
		s := e.stops[id]
		s.Lon, s.Lat = ll[0], ll[1]
		e.stops[id] = s
	}
	return e
}

func at(t *testing.T, e *Engine, h, m int) []EstimatedPosition {
	t.Helper()
	ts := time.Date(2026, 9, 28, h, m, 0, 0, e.loc) // monday fixture date
	return e.EstimatedPositions(ts, map[string]bool{"rail": true, "subway": true})
}

func TestEstimatedMidLegInterpolates(t *testing.T) {
	pos := at(t, estimateEngine(), 8, 15)
	if len(pos) != 1 {
		t.Fatalf("want 1 active rail trip at 08:15, got %+v", pos)
	}
	p := pos[0]
	if p.TripKey != "R1" || p.ID != "R1" || p.RouteID != r1 {
		t.Fatalf("identity = %+v", p)
	}
	// Trip 1 dwells at B 08:10 then reaches D at 08:20 — halfway puts it
	// on the straight line B→D.
	if d := math.Abs(p.Lon - 106.2); d > 1e-9 {
		t.Fatalf("lon = %v want ~106.2", p.Lon)
	}
	if d := math.Abs(p.Lat - (-6.2)); d > 1e-9 {
		t.Fatalf("lat = %v want ~-6.2", p.Lat)
	}
}

func TestEstimatedDwellLandsOnStop(t *testing.T) {
	pos := at(t, estimateEngine(), 8, 10)
	if len(pos) != 1 || pos[0].Lon != 106.1 || pos[0].Lat != -6.1 {
		t.Fatalf("dwell at B: %+v", pos)
	}
}

func TestEstimatedModeFilterExcludesBus(t *testing.T) {
	e := estimateEngine()
	ts := time.Date(2026, 9, 28, 8, 15, 0, 0, e.loc)
	// The bus trip is active 08:05–08:30 but 'bus' is not in the set —
	// buses only ever appear estimated if the caller widens the modes.
	pos := e.EstimatedPositions(ts, map[string]bool{"bus": true})
	if len(pos) != 1 || pos[0].TripKey != "R2" {
		t.Fatalf("bus-only estimate: %+v", pos)
	}
	pos = e.EstimatedPositions(ts, map[string]bool{"rail": true})
	for _, p := range pos {
		if p.Mode == "bus" {
			t.Fatalf("bus leaked into rail estimate: %+v", pos)
		}
	}
}

func TestEstimatedNoActiveTrips(t *testing.T) {
	if pos := at(t, estimateEngine(), 12, 0); len(pos) != 0 {
		t.Fatalf("midday should be quiet: %+v", pos)
	}
}

func TestEstimatedFrequencyExpandsInstances(t *testing.T) {
	e := estimateEngine()
	// 20-minute ride on a 10-minute headway → up to three instances out
	// at once (one arriving, one mid-leg, one just departed).
	e.trips[3].dep = []int32{sec(6, 0), sec(6, 20)}
	e.trips[3].arr = e.trips[3].dep
	pos := e.EstimatedPositions(
		time.Date(2026, 9, 28, 7, 10, 0, 0, e.loc), map[string]bool{"subway": true})
	ids := map[string]bool{}
	for _, p := range pos {
		ids[p.ID] = true
	}
	want := map[string]bool{"R3~5": true, "R3~6": true, "R3~7": true}
	if len(pos) != 3 || !ids["R3~5"] || !ids["R3~6"] || !ids["R3~7"] {
		t.Fatalf("07:10 wants instances %v, got %+v", want, pos)
	}
	// Window end bounds the first departure: last instance leaves 08:00,
	// so at 08:03 it alone is still out.
	e.trips[3].dep = []int32{sec(6, 0), sec(6, 5)}
	e.trips[3].arr = e.trips[3].dep
	pos = e.EstimatedPositions(
		time.Date(2026, 9, 28, 8, 3, 0, 0, e.loc), map[string]bool{"subway": true})
	if len(pos) != 1 || pos[0].ID != "R3~12" {
		t.Fatalf("last instance: %+v", pos)
	}
}

func TestEstimatedPostMidnightServiceDay(t *testing.T) {
	// A trip departing 25:30 belongs to yesterday's service day — moving
	// the base back a day must surface it after midnight.
	e := estimateEngine()
	e.trips[0].dep = []int32{sec(25, 30), sec(25, 40), sec(25, 50)}
	e.trips[0].arr = e.trips[0].dep
	pos := e.EstimatedPositions(
		time.Date(2026, 9, 29, 1, 40, 0, 0, e.loc), // tue 01:40 = mon 25:40
		map[string]bool{"rail": true})
	if len(pos) != 1 || pos[0].Lon != 106.1 {
		t.Fatalf("overnight trip: %+v", pos)
	}
}
