package journey

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	generated "singgah/services/api/db/generated"
	"singgah/services/api/internal/planner"
	"singgah/services/api/internal/provider/commute"
)

type fakeStore struct {
	stops        map[string]generated.GetStopRow
	getStopErr   error
	uuidRows     []generated.ListStopIDsByProviderEntityIDsRow
	uuidErr      error
	coordRows    []generated.ListStopCoordsRow
	coordErr     error
	shape        string
	shapeErr     error
	altRows      []generated.ListRouteAlternativesRow
	altSlices    map[string][]generated.ListRouteStopSliceRow // route id -> stops
	colorRows    []generated.ListRouteColorsRow
	colorErr     error
	gotEntityIDs []string
	gotSlice     generated.SliceRouteShapeParams
	gotSlices    []generated.SliceRouteShapeParams
}

func (f *fakeStore) GetStop(_ context.Context, id pgtype.UUID) (generated.GetStopRow, error) {
	if f.getStopErr != nil {
		return generated.GetStopRow{}, f.getStopErr
	}
	row, ok := f.stops[id.String()]
	if !ok {
		return generated.GetStopRow{}, pgx.ErrNoRows
	}
	return row, nil
}
func (f *fakeStore) ListStopIDsByProviderEntityIDs(_ context.Context, arg generated.ListStopIDsByProviderEntityIDsParams) ([]generated.ListStopIDsByProviderEntityIDsRow, error) {
	f.gotEntityIDs = arg.EntityIds
	return f.uuidRows, f.uuidErr
}
func (f *fakeStore) ListStopCoords(_ context.Context, _ []pgtype.UUID) ([]generated.ListStopCoordsRow, error) {
	return f.coordRows, f.coordErr
}
func (f *fakeStore) SliceRouteShape(_ context.Context, arg generated.SliceRouteShapeParams) (string, error) {
	f.gotSlice = arg
	f.gotSlices = append(f.gotSlices, arg)
	return f.shape, f.shapeErr
}
func (f *fakeStore) ListRouteAlternatives(_ context.Context, _ generated.ListRouteAlternativesParams) ([]generated.ListRouteAlternativesRow, error) {
	return f.altRows, nil
}
func (f *fakeStore) ListRouteStopSlice(_ context.Context, arg generated.ListRouteStopSliceParams) ([]generated.ListRouteStopSliceRow, error) {
	return slices.Clone(f.altSlices[arg.RouteID.String()]), nil // handler reverses in place
}
func (f *fakeStore) ListRouteColors(_ context.Context, _ []pgtype.UUID) ([]generated.ListRouteColorsRow, error) {
	return f.colorRows, f.colorErr
}

type fakePlanner struct {
	plan  *commute.FarePlan
	err   error
	gotAt *time.Time
}

func (f *fakePlanner) Fares(_ context.Context, _, _ string, at *time.Time) (*commute.FarePlan, error) {
	f.gotAt = at
	return f.plan, f.err
}

// fakeLoader feeds planner.Load a synthetic network — the handler then
// plans against the real engine, not a mock of it.
type fakeLoader struct {
	rows  []generated.ListScheduleRowsRow
	freqs []generated.Frequency
	edges []generated.ListTransferEdgesRow
	stops []generated.ListPlannerStopsRow
}

func (f *fakeLoader) ListScheduleRows(context.Context) ([]generated.ListScheduleRowsRow, error) {
	return f.rows, nil
}
func (f *fakeLoader) ListAllFrequencies(context.Context) ([]generated.Frequency, error) {
	return f.freqs, nil
}
func (f *fakeLoader) ListTransferEdges(context.Context) ([]generated.ListTransferEdgesRow, error) {
	return f.edges, nil
}
func (f *fakeLoader) ListPlannerStops(context.Context) ([]generated.ListPlannerStopsRow, error) {
	return f.stops, nil
}

func mustUUID(t *testing.T, s string) pgtype.UUID {
	t.Helper()
	var id pgtype.UUID
	if err := id.Scan(s); err != nil {
		t.Fatal(err)
	}
	return id
}

const (
	fromUUID = "b7f4b2a0-1f3d-4e5a-9c6b-2a1d3e4f5a6b" // A "Sudirman"
	toUUID   = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee" // B "Lebak Bulus"
	cUUID    = "cccccccc-1111-2222-3333-444444444444" // C "Dukuh Atas"
	dUUID    = "dddddddd-dddd-dddd-dddd-dddddddddddd" // D "Karet"
	routeR1  = "11111111-2222-3333-4444-555555555555" // rail line
	routeR2  = "66666666-7777-8888-9999-000000000000" // subway line
	routeR3  = "77777777-8888-9999-aaaa-bbbbbbbbbbbb" // bus line
	tripT1   = "11111111-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	tripT2   = "22222222-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	tripT3   = "33333333-cccc-4ccc-8ccc-cccccccccccc"
	tripT4   = "44444444-dddd-4ddd-8ddd-dddddddddddd"
	tripT5   = "55555555-eeee-4eee-8eee-eeeeeeeeeeee"
	tripT6   = "66666666-ffff-4fff-8fff-ffffffffffff"
	tripT7   = "77777777-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	testDay  = "2026-09-28T08%3A00%3A00%2B07%3A00" // Monday 08:00 WIB, URL-encoded
)

// schedRow is one stop_time in the synthetic network.
func schedRow(tripID, routeID, routeKey, mode, headsign, key string, stopID pgtype.UUID, seq int32, arr, dep int32, derived bool) generated.ListScheduleRowsRow {
	return generated.ListScheduleRowsRow{
		TripID: mustUUID2(tripID), TripKey: key,
		Headsign:  pgtype.Text{String: headsign, Valid: headsign != ""},
		DayMask:   127,
		StartDate: pgtype.Date{Time: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true},
		EndDate:   pgtype.Date{Time: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true},
		RouteID:   mustUUID2(routeID), RouteKey: routeKey, Mode: mode,
		ShortName: pgtype.Text{String: "X", Valid: true},
		Seq:       seq, StopID: stopID,
		ArrivalSeconds: arr, DepartureSeconds: dep, Derived: derived,
	}
}

func mustUUID2(s string) pgtype.UUID {
	var id pgtype.UUID
	if err := id.Scan(s); err != nil {
		panic(err)
	}
	return id
}

func stopRow(id, name string, elevator bool) generated.ListPlannerStopsRow {
	meta := []byte("{}")
	if elevator {
		meta = []byte(`{"amenities":[{"type":"ELEVATOR_PAID"}]}`)
	}
	return generated.ListPlannerStopsRow{
		ID: mustUUID2(id), ProviderEntityID: "OP-" + name[:3], Name: name, Metadata: meta,
	}
}

// testEngine builds the fixture network:
//   - T1/T4 rail R1: A 08:00→B 08:30 and A 08:20→B 08:50 (direct rides)
//   - T2/T3 subway R2: C 08:05→B 08:40 and C 09:00→B 09:35
//   - T5 bus R3: B 09:10→D 09:30 — makes A→D require a ride transfer
//   - walk edge A→C 100 m (~80 s) — both directions
//
// extraRows let tests park serving trips on corridor-alternative routes —
// legAlternatives now verifies a real trip rides the leg's endpoints.
func testEngine(t *testing.T, extraRows ...generated.ListScheduleRowsRow) *planner.Engine {
	t.Helper()
	A, B, C, D := mustUUID2(fromUUID), mustUUID2(toUUID), mustUUID2(cUUID), mustUUID2(dUUID)
	loader := &fakeLoader{
		rows: append([]generated.ListScheduleRowsRow{
			schedRow(tripT1, routeR1, "KCI:C", "rail", "Lebak Bulus", "T1", A, 1, 8*3600, 8*3600, false),
			schedRow(tripT1, routeR1, "KCI:C", "rail", "Lebak Bulus", "T1", B, 2, 8*3600+1800, 8*3600+1800, false),
			schedRow(tripT4, routeR1, "KCI:C", "rail", "Lebak Bulus", "T4", A, 1, 8*3600+1200, 8*3600+1200, false),
			schedRow(tripT4, routeR1, "KCI:C", "rail", "Lebak Bulus", "T4", B, 2, 8*3600+3000, 8*3600+3000, false),
			schedRow(tripT2, routeR2, "MRTJ:M", "subway", "Lebak Bulus", "T2", C, 1, 8*3600+300, 8*3600+300, false),
			schedRow(tripT2, routeR2, "MRTJ:M", "subway", "Lebak Bulus", "T2", B, 2, 8*3600+2400, 8*3600+2400, false),
			schedRow(tripT3, routeR2, "MRTJ:M", "subway", "Lebak Bulus", "T3", C, 1, 9*3600, 9*3600, false),
			schedRow(tripT3, routeR2, "MRTJ:M", "subway", "Lebak Bulus", "T3", B, 2, 9*3600+2100, 9*3600+2100, false),
			schedRow(tripT5, routeR3, "TJB:9", "bus", "Karet", "T5", B, 1, 9*3600+600, 9*3600+600, false),
			schedRow(tripT5, routeR3, "TJB:9", "bus", "Karet", "T5", D, 2, 9*3600+1800, 9*3600+1800, false),
		}, extraRows...),
		edges: []generated.ListTransferEdgesRow{
			{FromStopID: A, ToStopID: C, WalkDistanceM: pgtype.Int4{Int32: 100, Valid: true}},
			{FromStopID: C, ToStopID: A, WalkDistanceM: pgtype.Int4{Int32: 100, Valid: true}},
		},
		stops: []generated.ListPlannerStopsRow{
			stopRow(fromUUID, "Sudirman", false),
			stopRow(toUUID, "Lebak Bulus", true),
			stopRow(cUUID, "Dukuh Atas", false),
			stopRow(dUUID, "Karet", true),
		},
	}
	eng, err := planner.Load(context.Background(), loader)
	if err != nil {
		t.Fatalf("engine load: %v", err)
	}
	return eng
}

func serve(t *testing.T, store Store, eng *planner.Engine, fares FarePlanner, target string) *httptest.ResponseRecorder {
	t.Helper()
	h := NewHandler(store, planner.NewEngineSource(
		func(context.Context) (*planner.Engine, error) { return eng, nil }, time.Hour), fares)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	r.ServeHTTP(rec, req)
	return rec
}

func baseStore(t *testing.T) *fakeStore {
	t.Helper()
	return &fakeStore{
		stops: map[string]generated.GetStopRow{
			fromUUID: {ID: mustUUID(t, fromUUID), Name: "Sudirman", ProviderCode: "commute", ProviderEntityID: "KCI-SUD"},
			toUUID:   {ID: mustUUID(t, toUUID), Name: "Lebak Bulus", ProviderCode: "commute", ProviderEntityID: "MRTJ-LBB"},
			dUUID:    {ID: mustUUID(t, dUUID), Name: "Karet", ProviderCode: "commute", ProviderEntityID: "TJB-KAR"},
		},
	}
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response not JSON: %v\n%s", err, rec.Body.String())
	}
	return body
}

const target = "/journeys?from=" + fromUUID + "&to=" + toUUID

// targetD needs a ride transfer — no single route serves A→D.
const targetD = "/journeys?from=" + fromUUID + "&to=" + dUUID

func TestPlanRequiresBothParams(t *testing.T) {
	for _, tgt := range []string{"/journeys", "/journeys?from=" + fromUUID, "/journeys?from=x&to=y"} {
		rec := serve(t, baseStore(t), testEngine(t), &fakePlanner{}, tgt)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: status = %d, want 400", tgt, rec.Code)
		}
	}
}

func TestPlanRejectsAnchorConflictAndBadFilters(t *testing.T) {
	eng := testEngine(t)
	for _, tgt := range []string{
		target + "&at=2026-09-28T08:00:00%2B07:00&arriveBy=2026-09-28T09:00:00%2B07:00",
		target + "&at=bogus",
		target + "&arriveBy=bogus",
		target + "&modes=rocket",
		target + "&maxWalkM=-5",
		target + "&maxTransfers=nope",
		target + "&stepFree=maybe",
	} {
		if rec := serve(t, baseStore(t), eng, &fakePlanner{}, tgt); rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: status = %d, want 400", tgt, rec.Code)
		}
	}
}

func TestPlanUnknownStop(t *testing.T) {
	rec := serve(t, baseStore(t), testEngine(t), &fakePlanner{}, "/journeys?from=11111111-2222-3333-4444-555555555555&to="+toUUID)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestPlanReturnsItineraries(t *testing.T) {
	rec := serve(t, baseStore(t), testEngine(t), &fakePlanner{}, target+"&at="+testDay)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := decode(t, rec)
	its := body["itineraries"].([]any)
	if len(its) < 1 {
		t.Fatal("expected at least one itinerary")
	}
	it := its[0].(map[string]any)
	if it["label"] != "fastest" || it["status"] != "scheduled" {
		t.Fatalf("itinerary label/status = %v/%v", it["label"], it["status"])
	}
	if it["departAt"] != "2026-09-28T01:00:00Z" || it["arriveAt"] != "2026-09-28T01:30:00Z" {
		t.Fatalf("times = %v → %v", it["departAt"], it["arriveAt"])
	}
	leg := it["legs"].([]any)[0].(map[string]any)
	if leg["type"] != "ride" || leg["routeId"] != routeR1 || leg["mode"] != "rail" {
		t.Fatalf("leg = %v", leg)
	}
	if leg["depAt"] == nil || leg["arrAt"] == nil {
		t.Fatal("ride leg must carry scheduled times")
	}
	deps := leg["nextDepartures"].([]any)
	if len(deps) != 2 || deps[0].(map[string]any)["time"] != "08:00" || deps[1].(map[string]any)["time"] != "08:20" {
		t.Fatalf("nextDepartures = %v, want [08:00 08:20]", deps)
	}
	src := body["source"].(map[string]any)
	if src["provider"] != "schedule" || src["snapshotAt"] == nil {
		t.Fatalf("source = %v", src)
	}
	q := body["query"].(map[string]any)
	if q["departAt"] != "2026-09-28T08:00:00+07:00" {
		t.Fatalf("query echo = %v", q)
	}
}

func TestPlanWalkAlternativeSurfaces(t *testing.T) {
	rec := serve(t, baseStore(t), testEngine(t), &fakePlanner{}, target+"&at="+testDay)
	its := decode(t, rec)["itineraries"].([]any)
	if len(its) < 2 {
		t.Fatalf("expected the walk+subway alternative, got %v", its)
	}
	alt := its[1].(map[string]any)
	legs := alt["legs"].([]any)
	if legs[0].(map[string]any)["type"] != "walk" || legs[1].(map[string]any)["mode"] != "subway" {
		t.Fatalf("alternative legs = %v", legs)
	}
}

func TestPlanModesFilter(t *testing.T) {
	rec := serve(t, baseStore(t), testEngine(t), &fakePlanner{}, target+"&at="+testDay+"&modes=subway")
	its := decode(t, rec)["itineraries"].([]any)
	if len(its) != 1 {
		t.Fatalf("modes=subway itineraries = %v", its)
	}
	legs := its[0].(map[string]any)["legs"].([]any)
	if legs[1].(map[string]any)["mode"] != "subway" {
		t.Fatalf("expected subway-only ride, got %v", legs)
	}
}

func TestPlanMaxWalkFilter(t *testing.T) {
	// 50 m cap removes the 100 m transfer edge — only the direct rail ride.
	rec := serve(t, baseStore(t), testEngine(t), &fakePlanner{}, target+"&at="+testDay+"&maxWalkM=50")
	its := decode(t, rec)["itineraries"].([]any)
	if len(its) != 1 {
		t.Fatalf("maxWalkM=50 itineraries = %v", its)
	}
	for _, l := range its[0].(map[string]any)["legs"].([]any) {
		if l.(map[string]any)["type"] == "walk" {
			t.Fatalf("walk leg survived the 50 m cap: %v", l)
		}
	}
}

func TestPlanArriveBy(t *testing.T) {
	rec := serve(t, baseStore(t), testEngine(t), &fakePlanner{}, target+"&arriveBy=2026-09-28T09:00:00%2B07:00")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	it := decode(t, rec)["itineraries"].([]any)[0].(map[string]any)
	arr, _ := time.Parse(time.RFC3339, it["arriveAt"].(string))
	if arr.After(time.Date(2026, 9, 28, 2, 0, 0, 0, time.UTC)) {
		t.Fatalf("arriveAt %v exceeds the 09:00 WIB bound", it["arriveAt"])
	}
	q := decode(t, rec)["query"].(map[string]any)
	if q["arriveBy"] == nil || q["departAt"] != nil {
		t.Fatalf("query echo = %v", q)
	}
}

func TestPlanStepFreeFiltersWalkStop(t *testing.T) {
	// C has no elevator — step-free forbids the walk transfer, leaving the
	// direct ride. B must keep its elevator or nothing is reachable.
	rec := serve(t, baseStore(t), testEngine(t), &fakePlanner{}, target+"&at="+testDay+"&stepFree=1")
	its := decode(t, rec)["itineraries"].([]any)
	if len(its) != 1 {
		t.Fatalf("stepFree itineraries = %v", its)
	}
	for _, l := range its[0].(map[string]any)["legs"].([]any) {
		if l.(map[string]any)["type"] == "walk" {
			t.Fatalf("walk through a non-step-free stop: %v", l)
		}
	}
}

func TestPlanNoServiceGivesEmptyList(t *testing.T) {
	// Midnight depart-at: nothing runs inside the horizon — honest empty.
	rec := serve(t, baseStore(t), testEngine(t), &fakePlanner{}, target+"&at=2026-09-29T02:00:00%2B07:00")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if its := decode(t, rec)["itineraries"].([]any); len(its) != 0 {
		t.Fatalf("itineraries = %v, want empty", its)
	}
}

func TestPlanFareReference(t *testing.T) {
	fare := int64(14000)
	pl := &fakePlanner{plan: &commute.FarePlan{
		TotalFare: &fare,
		Segments: []commute.FareSegment{
			{Operator: "MRTJ", From: commute.FareStationRef{ID: "MRTJ-DKA", Name: "Dukuh Atas"}, To: commute.FareStationRef{ID: "MRTJ-LBB", Name: "Lebak Bulus"}, Fare: 14000},
		},
	}}
	store := baseStore(t)
	store.uuidRows = []generated.ListStopIDsByProviderEntityIDsRow{
		{ID: mustUUID(t, cUUID), ProviderEntityID: "MRTJ-DKA"},
		{ID: mustUUID(t, toUUID), ProviderEntityID: "MRTJ-LBB"},
	}
	rec := serve(t, store, testEngine(t), pl, target+"&at="+testDay)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	fareObj := decode(t, rec)["fareReference"].(map[string]any)
	if fareObj["total"].(float64) != 14000 || fareObj["currency"] != "IDR" {
		t.Fatalf("fareReference = %v", fareObj)
	}
}

func TestPlanFareOutageKeepsPlan(t *testing.T) {
	// The fare provider is only a reference now — its outage must never
	// fail or empty the plan.
	rec := serve(t, baseStore(t), testEngine(t), &fakePlanner{err: errors.New("upstream down")}, target+"&at="+testDay)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d — fare outage must not fail the plan", rec.Code)
	}
	body := decode(t, rec)
	if _, ok := body["fareReference"]; ok {
		t.Fatal("fareReference must be omitted on outage")
	}
	if its := body["itineraries"].([]any); len(its) == 0 {
		t.Fatal("itineraries must still be present")
	}
}

// Shape picking is scored against the leg's whole resolved stop sequence —
// endpoints alone can't tell same-termini variants apart.
func TestPlanSliceScoringSendsWholeStopSequence(t *testing.T) {
	store := baseStore(t)
	store.coordRows = []generated.ListStopCoordsRow{
		{ID: mustUUID(t, fromUUID), Lon: 106.870, Lat: -6.169},
		{ID: mustUUID(t, toUUID), Lon: 106.823, Lat: -6.176},
	}
	rec := serve(t, store, testEngine(t), &fakePlanner{}, target+"&at="+testDay)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	got := store.gotSlice
	if got.MaxSnapM <= 0 {
		t.Fatalf("max_snap_m = %v — the honesty bound must be sent", got.MaxSnapM)
	}
	if len(got.Lons) != 2 || got.Lons[0] != 106.870 || got.Lons[1] != 106.823 {
		t.Fatalf("lons = %v, want both endpoints in ride order", got.Lons)
	}
}

// Corridor alternatives are catalog-derived: the candidate route's own stop
// slice and its own shape cut — never the chosen leg's data. They only
// exist for journeys that need a transfer (the fixture queries A→D) and
// only when a scheduled trip actually rides the leg's endpoints (the
// alternative route carries its own T6 run).
func TestPlanLegAlternatives(t *testing.T) {
	store := baseStore(t)
	store.coordRows = []generated.ListStopCoordsRow{
		{ID: mustUUID(t, fromUUID), Lon: 106.870, Lat: -6.169},
		{ID: mustUUID(t, toUUID), Lon: 106.823, Lat: -6.176},
	}
	altRouteID := mustUUID(t, "c2c2c2c2-2c2c-4c2c-8c2c-2c2c2c2c2c2c")
	store.altRows = []generated.ListRouteAlternativesRow{
		{ID: altRouteID, ProviderEntityID: "TJ:2", ShortName: pgtype.Text{String: "2", Valid: true}, Color: pgtype.Text{String: "312F92", Valid: true}, SeqFrom: 10, SeqTo: 1},
	}
	store.colorRows = []generated.ListRouteColorsRow{
		{ID: mustUUID(t, routeR1), Color: pgtype.Text{String: "25B8EB", Valid: true}},
	}
	store.altSlices = map[string][]generated.ListRouteStopSliceRow{
		altRouteID.String(): {
			{ID: mustUUID(t, toUUID), Name: "Monumen Nasional", Seq: 1, Lon: 106.823, Lat: -6.176},
			{ID: mustUUID(t, cUUID), Name: "Kwitang", Seq: 2, Lon: 106.830, Lat: -6.174},
			{ID: mustUUID(t, fromUUID), Name: "Sumur Batu", Seq: 10, Lon: 106.870, Lat: -6.169},
		},
	}
	A, B := mustUUID2(fromUUID), mustUUID2(toUUID)
	eng := testEngine(t,
		schedRow(tripT6, altRouteID.String(), "TJ:2", "bus", "Monas", "T6", A, 1, 8*3600+600, 8*3600+600, false),
		schedRow(tripT6, altRouteID.String(), "TJ:2", "bus", "Monas", "T6", B, 2, 8*3600+2400, 8*3600+2400, false),
	)
	rec := serve(t, store, eng, &fakePlanner{}, targetD+"&at="+testDay)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	legs := decode(t, rec)["itineraries"].([]any)[0].(map[string]any)["legs"].([]any)
	if legs[0].(map[string]any)["color"] != "25B8EB" {
		t.Fatalf("leg color = %v — the ridden corridor's catalog color", legs[0])
	}
	alts := legs[0].(map[string]any)["alternatives"].([]any)
	if len(alts) != 1 {
		t.Fatalf("alternatives = %v", alts)
	}
	alt := alts[0].(map[string]any)
	if alt["line"] != "TJ:2" || alt["shortName"] != "2" || alt["operator"] != "TJ" || alt["stationCount"] != float64(2) || alt["color"] != "312F92" {
		t.Fatalf("alternative = %v", alt)
	}
	stops := alt["stops"].([]any)
	if len(stops) != 1 || stops[0].(map[string]any)["name"] != "Kwitang" {
		t.Fatalf("alternative stops = %v — endpoints excluded, ride order kept", stops)
	}
	if store.gotSlice.RouteID != altRouteID || len(store.gotSlice.Lons) != 3 || store.gotSlice.Lons[0] != 106.870 {
		t.Fatalf("slice params = %+v", store.gotSlice)
	}
}

// A direct ride already answers "bisa satu kali naik" — corridor chips
// would only offer swapping a hop of a journey that needs no swap, so no
// leg may carry alternatives when the plan contains a no-transfer option.
func TestPlanNoAlternativesWhenDirectServes(t *testing.T) {
	store := baseStore(t)
	altRouteID := mustUUID(t, "c2c2c2c2-2c2c-4c2c-8c2c-2c2c2c2c2c2c")
	store.altRows = []generated.ListRouteAlternativesRow{
		{ID: altRouteID, ProviderEntityID: "TJ:2", SeqFrom: 10, SeqTo: 1},
	}
	store.altSlices = map[string][]generated.ListRouteStopSliceRow{
		altRouteID.String(): {
			{ID: mustUUID(t, fromUUID), Name: "Sumur Batu", Seq: 10, Lon: 106.870, Lat: -6.169},
			{ID: mustUUID(t, toUUID), Name: "Monumen Nasional", Seq: 1, Lon: 106.823, Lat: -6.176},
		},
	}
	rec := serve(t, store, testEngine(t), &fakePlanner{}, target+"&at="+testDay)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	for _, it := range decode(t, rec)["itineraries"].([]any) {
		for _, l := range it.(map[string]any)["legs"].([]any) {
			if _, ok := l.(map[string]any)["alternatives"]; ok {
				t.Fatalf("leg carries corridor alternatives on a direct O-D: %v", l)
			}
		}
	}
}

// A folded provider sequence can order the leg's endpoints around a whole
// loop — such an alternative is a different journey, not a same-hop swap,
// and the detour bound drops it.
func TestPlanLegAlternativesDropLoopSlices(t *testing.T) {
	store := baseStore(t)
	saneRouteID := mustUUID(t, "c2c2c2c2-2c2c-4c2c-8c2c-2c2c2c2c2c2c")
	loopRouteID := mustUUID(t, "b1b1b1b1-b1b1-4b1b-8b1b-b1b1b1b1b1b1")
	store.altRows = []generated.ListRouteAlternativesRow{
		{ID: saneRouteID, ProviderEntityID: "TJ:2", SeqFrom: 10, SeqTo: 1},
		{ID: loopRouteID, ProviderEntityID: "TJ:7F", SeqFrom: 3, SeqTo: 25},
	}
	store.altSlices = map[string][]generated.ListRouteStopSliceRow{
		saneRouteID.String(): {
			{ID: mustUUID(t, toUUID), Name: "Monumen Nasional", Seq: 1, Lon: 106.823, Lat: -6.176},
			{ID: mustUUID(t, cUUID), Name: "Kwitang", Seq: 2, Lon: 106.830, Lat: -6.174},
			{ID: mustUUID(t, fromUUID), Name: "Sumur Batu", Seq: 10, Lon: 106.870, Lat: -6.169},
		},
		// The slice swings ~14 km south and back before reaching the
		// endpoint — the 7F-folded-sequence signature.
		loopRouteID.String(): {
			{ID: mustUUID(t, cUUID), Name: "Kwitang", Seq: 3, Lon: 106.838, Lat: -6.181},
			{ID: mustUUID2("e1e1e1e1-e1e1-4e1e-8e1e-e1e1e1e1e1e1"), Name: "Kampung Rambutan", Seq: 18, Lon: 106.872, Lat: -6.309},
			{ID: mustUUID(t, toUUID), Name: "Monumen Nasional", Seq: 25, Lon: 106.823, Lat: -6.176},
		},
	}
	// Both candidate routes carry a real A→B trip — only the loop slice's
	// detour, not a missing service, is what drops TJ:7F here.
	A, B := mustUUID2(fromUUID), mustUUID2(toUUID)
	eng := testEngine(t,
		schedRow(tripT6, saneRouteID.String(), "TJ:2", "bus", "Monas", "T6", A, 1, 8*3600+600, 8*3600+600, false),
		schedRow(tripT6, saneRouteID.String(), "TJ:2", "bus", "Monas", "T6", B, 2, 8*3600+2400, 8*3600+2400, false),
		schedRow(tripT7, loopRouteID.String(), "TJ:7F", "bus", "Monas", "T7", A, 1, 8*3600+600, 8*3600+600, false),
		schedRow(tripT7, loopRouteID.String(), "TJ:7F", "bus", "Monas", "T7", B, 2, 8*3600+2400, 8*3600+2400, false),
	)
	rec := serve(t, store, eng, &fakePlanner{}, targetD+"&at="+testDay)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	legs := decode(t, rec)["itineraries"].([]any)[0].(map[string]any)["legs"].([]any)
	alts := legs[0].(map[string]any)["alternatives"].([]any)
	if len(alts) != 1 || alts[0].(map[string]any)["line"] != "TJ:2" {
		t.Fatalf("alternatives = %v — the loop slice must be dropped, the sane one kept", alts)
	}
}

// When the earliest-arrival plan transfers but a single corridor also
// serves the O-D, the no-transfer itinerary leads the list — every client
// renders itineraries[0] as the answer.
func TestPlanPrefersNoTransferItinerary(t *testing.T) {
	A, B, D := mustUUID2(fromUUID), mustUUID2(toUUID), mustUUID2(dUUID)
	// R2+R3 via B arrives 08:40; the direct R1 ride arrives 09:00.
	eng, err := planner.Load(context.Background(), &fakeLoader{
		rows: []generated.ListScheduleRowsRow{
			schedRow(tripT2, routeR2, "MRTJ:M", "subway", "Lebak Bulus", "T2", A, 1, 8*3600, 8*3600, false),
			schedRow(tripT2, routeR2, "MRTJ:M", "subway", "Lebak Bulus", "T2", B, 2, 8*3600+900, 8*3600+900, false),
			schedRow(tripT5, routeR3, "TJB:9", "bus", "Karet", "T5", B, 1, 8*3600+1500, 8*3600+1500, false),
			schedRow(tripT5, routeR3, "TJB:9", "bus", "Karet", "T5", D, 2, 8*3600+2400, 8*3600+2400, false),
			schedRow(tripT1, routeR1, "KCI:C", "rail", "Karet", "T1", A, 1, 8*3600+1800, 8*3600+1800, false),
			schedRow(tripT1, routeR1, "KCI:C", "rail", "Karet", "T1", D, 2, 8*3600+3600, 8*3600+3600, false),
		},
		stops: []generated.ListPlannerStopsRow{
			stopRow(fromUUID, "Sudirman", false),
			stopRow(toUUID, "Lebak Bulus", true),
			stopRow(dUUID, "Karet", true),
		},
	})
	if err != nil {
		t.Fatalf("engine load: %v", err)
	}
	rec := serve(t, baseStore(t), eng, &fakePlanner{}, targetD+"&at="+testDay)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	its := decode(t, rec)["itineraries"].([]any)
	if len(its) < 2 {
		t.Fatalf("expected the transfer plan plus the direct alternative, got %v", its)
	}
	first := its[0].(map[string]any)
	legs := first["legs"].([]any)
	if first["rideLegs"] != float64(1) || len(legs) != 1 || legs[0].(map[string]any)["routeId"] != routeR1 {
		t.Fatalf("itineraries[0] = %v — the single-ride itinerary must lead", first)
	}
	if its[1].(map[string]any)["label"] != "fastest" {
		t.Fatalf("itineraries[1].label = %v — the transfer plan keeps its earned label", its[1])
	}
}
