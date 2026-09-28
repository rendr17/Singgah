package planner

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// Synthetic network — two parallel corridors plus a walk link:
//
//	R1 rail:  A → B → D          (08:00 trip, arrival 08:25)
//	R2 bus:   A → C → D          (08:05 trip, arrival 08:40 — the corridor alternative)
//	walk:     B ⇄ C, 300 m
//
// The fastest path rides R1 direct; excluding R1 must surface R2.
var (
	uA = pgtype.UUID{Bytes: [16]byte{1}, Valid: true}
	uB = pgtype.UUID{Bytes: [16]byte{2}, Valid: true}
	uC = pgtype.UUID{Bytes: [16]byte{3}, Valid: true}
	uD = pgtype.UUID{Bytes: [16]byte{4}, Valid: true}
	uE = pgtype.UUID{Bytes: [16]byte{5}, Valid: true} // freq line stop
	uF = pgtype.UUID{Bytes: [16]byte{6}, Valid: true}

	r1 = pgtype.UUID{Bytes: [16]byte{101}, Valid: true}
	r2 = pgtype.UUID{Bytes: [16]byte{102}, Valid: true}
	r3 = pgtype.UUID{Bytes: [16]byte{103}, Valid: true} // headway line E→F
)

const dayMaskDaily = 127

func sec(h, m int) int32 { return int32(h*3600 + m*60) }

func testEngine() *Engine {
	loc, _ := time.LoadLocation("Asia/Jakarta")
	svc := serviceRec{
		dayMask: dayMaskDaily,
		start:   time.Date(2026, 1, 1, 0, 0, 0, 0, loc),
		end:     time.Date(2027, 12, 31, 0, 0, 0, 0, loc),
	}
	dep3 := func(a, b, c int32) []int32 { return []int32{a, b, c} }
	arrEq := func(d []int32) []int32 { return append([]int32(nil), d...) }
	e := &Engine{
		loc: loc,
		stops: map[pgtype.UUID]Stop{
			uA: {ID: uA, EntityID: "A", Name: "Alpha"},
			uB: {ID: uB, EntityID: "B", Name: "Beta"},
			uC: {ID: uC, EntityID: "C", Name: "Gamma"},
			uD: {ID: uD, EntityID: "D", Name: "Delta"},
			uE: {ID: uE, EntityID: "E", Name: "Echo"},
			uF: {ID: uF, EntityID: "F", Name: "Foxtrot"},
		},
		edges: map[pgtype.UUID][]walkEdge{
			uB: {{from: uB, to: uC, distM: 300, secs: 240}},
			uC: {{from: uC, to: uB, distM: 300, secs: 240}},
		},
		edgesIn: map[pgtype.UUID][]walkEdge{
			uC: {{from: uB, to: uC, distM: 300, secs: 240}},
			uB: {{from: uC, to: uB, distM: 300, secs: 240}},
		},
	}
	mk := func(id byte, routeID pgtype.UUID, key, mode string, stops []pgtype.UUID, dep, arr []int32, freq *freqRec) {
		e.trips = append(e.trips, tripRec{
			id:  pgtype.UUID{Bytes: [16]byte{id}, Valid: true},
			key: key, routeID: routeID, routeKey: key, lineName: key,
			mode: mode, service: svc, stops: stops,
			dep: dep, arr: arr, derived: make([]bool, len(stops)), freq: freq,
		})
	}
	mk(1, r1, "R1", "rail", []pgtype.UUID{uA, uB, uD}, dep3(sec(8, 0), sec(8, 10), sec(8, 20)), arrEq(dep3(sec(8, 0), sec(8, 10), sec(8, 20))), nil)
	mk(2, r1, "R1", "rail", []pgtype.UUID{uA, uB, uD}, dep3(sec(9, 0), sec(9, 10), sec(9, 20)), arrEq(dep3(sec(9, 0), sec(9, 10), sec(9, 20))), nil)
	mk(3, r2, "R2", "bus", []pgtype.UUID{uA, uC, uD}, dep3(sec(8, 5), sec(8, 15), sec(8, 30)), arrEq(dep3(sec(8, 5), sec(8, 15), sec(8, 30))), nil)
	// Headway service E→F every 10 min 06:00–08:00 — instance times are derived.
	mk(4, r3, "R3", "subway", []pgtype.UUID{uE, uF},
		[]int32{sec(6, 0), sec(6, 5)}, []int32{sec(6, 0), sec(6, 5)},
		&freqRec{start: sec(6, 0), end: sec(8, 0), headway: 600})
	return e
}

func monday(h, m int) int64 {
	return time.Date(2026, 9, 28, h, m, 0, 0, testEngine().loc).Unix() // 2026-09-28 = Monday
}

func TestPlanDepartAtFastest(t *testing.T) {
	e := testEngine()
	q := Query{From: uA, To: uD, DepartAt: ptrTime(monday(8, 0)), MaxWalkM: -1, MaxTransfers: -1}
	its, err := e.Plan(q)
	if err != nil {
		t.Fatal(err)
	}
	if len(its) == 0 {
		t.Fatal("no itinerary")
	}
	prim := its[0]
	if prim.Label != "fastest" {
		t.Errorf("primary label=%q", prim.Label)
	}
	if len(prim.Legs) != 1 || prim.Legs[0].Kind != "ride" || prim.Legs[0].RouteKey != "R1" {
		t.Fatalf("primary legs=%+v", prim.Legs)
	}
	if prim.Depart != monday(8, 0) || prim.Arrive != monday(8, 20) {
		t.Fatalf("times dep=%d arr=%d, want 08:00→08:20", prim.Depart, prim.Arrive)
	}
	if prim.Transfers != 0 {
		t.Errorf("transfers=%d", prim.Transfers)
	}

	// Excluding R1 must surface the R2 corridor as an alternative.
	if len(its) < 2 {
		t.Fatal("no alternative found")
	}
	alt := its[1]
	if alt.Legs[0].RouteKey != "R2" {
		t.Fatalf("alt route=%s, want R2", alt.Legs[0].RouteKey)
	}
	if alt.Arrive != monday(8, 30) {
		t.Errorf("alt arrive=%d, want 08:30", alt.Arrive)
	}
}

func TestPlanArriveBy(t *testing.T) {
	e := testEngine()
	// arrive no later than 08:50 → the backward search maximizes the
	// departure: R2 dep 08:05 → arr 08:30 beats R1's 08:00 → 08:20.
	q := Query{From: uA, To: uD, ArriveBy: ptrTime(monday(8, 50)), MaxWalkM: -1, MaxTransfers: -1}
	its, err := e.Plan(q)
	if err != nil {
		t.Fatal(err)
	}
	if len(its) == 0 {
		t.Fatal("no itinerary")
	}
	prim := its[0]
	if prim.Depart != monday(8, 5) || prim.Arrive != monday(8, 30) {
		t.Fatalf("dep=%d arr=%d, want latest departure 08:05→08:30", prim.Depart, prim.Arrive)
	}
	if prim.Legs[0].RouteKey != "R2" {
		t.Fatalf("route=%s, want R2 (latest feasible departure)", prim.Legs[0].RouteKey)
	}
}

func TestPlanWalkTransfer(t *testing.T) {
	e := testEngine()
	// B→C walk link: A→B on R1 (08:10), walk 4 min → C 08:14, but R2's C
	// departure is 08:15 — inside minChangeSec only if the boarding
	// penalty is skipped; it isn't after a ride, so this path fails and
	// the direct ride stays.
	q := Query{From: uB, To: uC, DepartAt: ptrTime(monday(8, 0)), MaxWalkM: -1, MaxTransfers: -1}
	its, _ := e.Plan(q)
	if len(its) != 1 || len(its[0].Legs) != 1 || its[0].Legs[0].Kind != "walk" {
		t.Fatalf("B→C should be a single walk leg: %+v", its)
	}
	if its[0].Legs[0].DistM != 300 {
		t.Errorf("walk dist=%d", its[0].Legs[0].DistM)
	}

	// maxWalkM=100 kills the only path.
	q.MaxWalkM = 100
	its, _ = e.Plan(q)
	if len(its) != 0 {
		t.Fatalf("maxWalkM=100 should yield no path, got %+v", its)
	}
}

func TestPlanModesFilter(t *testing.T) {
	e := testEngine()
	q := Query{From: uA, To: uD, DepartAt: ptrTime(monday(8, 0)),
		Modes: map[string]bool{"bus": true}, MaxWalkM: -1, MaxTransfers: -1}
	its, _ := e.Plan(q)
	if len(its) == 0 || its[0].Legs[0].RouteKey != "R2" {
		t.Fatalf("mode=bus should ride R2: %+v", its)
	}
	q.Modes = map[string]bool{"ferry": true}
	if its, _ := e.Plan(q); len(its) != 0 {
		t.Fatalf("mode=ferry should yield nothing, got %+v", its)
	}
}

func TestPlanFrequencyEstimated(t *testing.T) {
	e := testEngine()
	q := Query{From: uE, To: uF, DepartAt: ptrTime(monday(7, 3)), MaxWalkM: -1, MaxTransfers: -1}
	its, _ := e.Plan(q)
	if len(its) != 1 {
		t.Fatalf("freq line should produce an itinerary: %+v", its)
	}
	l := its[0].Legs[0]
	// headway 600: next dep after 07:03 = 07:10
	if l.Dep != monday(7, 10) || l.Arr != monday(7, 15) {
		t.Fatalf("freq dep=%d arr=%d, want 07:10→07:15", l.Dep, l.Arr)
	}
	if !its[0].Estimated {
		t.Error("headway itinerary must be marked Estimated")
	}
}

func TestPlanStepFree(t *testing.T) {
	e := testEngine()
	// Mark only A..D as step-free (elevator) — R2's boarding still fine.
	e.stops[uA] = Stop{ID: uA, EntityID: "A", Name: "Alpha", StepFree: true}
	e.stops[uC] = Stop{ID: uC, EntityID: "C", Name: "Gamma", StepFree: true}
	e.stops[uD] = Stop{ID: uD, EntityID: "D", Name: "Delta", StepFree: true}
	q := Query{From: uA, To: uD, DepartAt: ptrTime(monday(8, 0)),
		StepFree: true, MaxWalkM: -1, MaxTransfers: -1}
	its, _ := e.Plan(q)
	if len(its) == 0 {
		t.Fatal("step-free path should exist via R2")
	}
	// B is not step-free — R1 (which passes B) is allowed to *pass through*
	// B but boarding/alighting there isn't needed on the direct ride.
	found := false
	for _, it := range its {
		if it.Legs[0].RouteKey == "R2" {
			found = true
		}
	}
	if !found {
		t.Error("R2 itinerary should be present under stepFree")
	}
}

func ptrTime(u int64) *time.Time { t := time.Unix(u, 0); return &t }
