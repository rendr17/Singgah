package journey

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	generated "singgah/services/api/db/generated"
	"singgah/services/api/internal/provider/commute"
)

type fakeStore struct {
	stops        map[string]generated.GetStopRow
	getStopErr   error
	uuidRows     []generated.ListStopIDsByProviderEntityIDsRow
	uuidErr      error
	coordRows    []generated.ListStopCoordsRow
	coordErr     error
	routeID      pgtype.UUID
	routeErr     error
	shape        string
	shapeErr     error
	altRows      []generated.ListRouteAlternativesRow
	altSlices    map[string][]generated.ListRouteStopSliceRow // route id -> stops
	gotEntityIDs []string
	gotSlice     generated.SliceRouteShapeParams
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
func (f *fakeStore) GetRouteByProviderEntityID(_ context.Context, _ generated.GetRouteByProviderEntityIDParams) (pgtype.UUID, error) {
	return f.routeID, f.routeErr
}
func (f *fakeStore) SliceRouteShape(_ context.Context, arg generated.SliceRouteShapeParams) (string, error) {
	f.gotSlice = arg
	return f.shape, f.shapeErr
}
func (f *fakeStore) ListRouteAlternatives(_ context.Context, _ generated.ListRouteAlternativesParams) ([]generated.ListRouteAlternativesRow, error) {
	return f.altRows, nil
}
func (f *fakeStore) ListRouteStopSlice(_ context.Context, arg generated.ListRouteStopSliceParams) ([]generated.ListRouteStopSliceRow, error) {
	return f.altSlices[arg.RouteID.String()], nil
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

type fakeTimetabler struct {
	entries  []commute.TimetableEntry
	err      error
	gotOp    string
	gotCode  string
	gotFrom  string
	gotTo    string
	gotCalls int
}

func (f *fakeTimetabler) Timetable(_ context.Context, op, code, from, to string) ([]commute.TimetableEntry, error) {
	f.gotCalls++
	f.gotOp, f.gotCode, f.gotFrom, f.gotTo = op, code, from, to
	return f.entries, f.err
}

func mustUUID(t *testing.T, s string) pgtype.UUID {
	t.Helper()
	var id pgtype.UUID
	if err := id.Scan(s); err != nil {
		t.Fatal(err)
	}
	return id
}

func serve(t *testing.T, store Store, planner FarePlanner, tt Timetabler, target string) *httptest.ResponseRecorder {
	t.Helper()
	r := chi.NewRouter()
	NewHandler(store, planner, tt).RegisterRoutes(r)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	r.ServeHTTP(rec, req)
	return rec
}

// serveAt serves the handler pinned to a fixed clock so the departure
// window math is deterministic.
func serveAt(t *testing.T, store Store, planner FarePlanner, tt Timetabler, target string, now time.Time) *httptest.ResponseRecorder {
	t.Helper()
	h := NewHandler(store, planner, tt)
	h.now = func() time.Time { return now }
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	r.ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response not JSON: %v\n%s", err, rec.Body.String())
	}
	return body
}

const (
	fromUUID  = "b7f4b2a0-1f3d-4e5a-9c6b-2a1d3e4f5a6b"
	toUUID    = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	dkaUUID   = "cccccccc-1111-2222-3333-444444444444"
	routeUUID = "11111111-2222-3333-4444-555555555555"
)

func baseStore(t *testing.T) *fakeStore {
	t.Helper()
	return &fakeStore{
		stops: map[string]generated.GetStopRow{
			fromUUID: {ID: mustUUID(t, fromUUID), Name: "Sudirman", ProviderCode: "commute", ProviderEntityID: "KCI-SUD"},
			toUUID:   {ID: mustUUID(t, toUUID), Name: "Lebak Bulus", ProviderCode: "commute", ProviderEntityID: "MRTJ-LBB"},
		},
	}
}

func TestPlanRequiresBothParams(t *testing.T) {
	for _, target := range []string{"/journeys", "/journeys?from=" + fromUUID, "/journeys?from=x&to=y"} {
		rec := serve(t, baseStore(t), &fakePlanner{}, &fakeTimetabler{}, target)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: status = %d, want 400", target, rec.Code)
		}
	}
}

func TestPlanUnknownStop(t *testing.T) {
	rec := serve(t, baseStore(t), &fakePlanner{}, &fakeTimetabler{}, "/journeys?from=11111111-2222-3333-4444-555555555555&to="+toUUID)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
	if decode(t, rec)["error"].(map[string]any)["code"] != "NOT_FOUND" {
		t.Fatal("expected NOT_FOUND")
	}
}

func TestPlanNormalizesItinerary(t *testing.T) {
	store := baseStore(t)
	store.routeID = mustUUID(t, routeUUID)
	store.uuidRows = []generated.ListStopIDsByProviderEntityIDsRow{
		{ID: mustUUID(t, fromUUID), ProviderEntityID: "KCI-SUD"},
		{ID: mustUUID(t, toUUID), ProviderEntityID: "MRTJ-LBB"},
		{ID: mustUUID(t, dkaUUID), ProviderEntityID: "MRTJ-DKA"},
	}
	fare := int64(14000)
	dist := 90.0
	planner := &fakePlanner{plan: &commute.FarePlan{
		From: commute.FareStationRef{ID: "KCI-SUD", Name: "Sudirman"},
		To:   commute.FareStationRef{ID: "MRTJ-LBB", Name: "Lebak Bulus"},
		Legs: []commute.FareLeg{
			{Type: "TRANSFER", From: commute.FareStationRef{ID: "KCI-SUD"}, To: commute.FareStationRef{ID: "MRTJ-DKA", Name: "Dukuh Atas BNI"}, DistanceM: &dist},
			{Type: "RIDE", Line: "MRTJ:M", Operator: "MRTJ", From: commute.FareStationRef{ID: "MRTJ-DKA"}, To: commute.FareStationRef{ID: "MRTJ-LBB"}, StationCount: 12, Headsign: "Lebak Bulus", Stops: []commute.FareStationRef{{ID: "MRTJ-DKA"}, {ID: "MRTJ-LBB"}}, DistanceM: &dist},
		},
		Segments:      []commute.FareSegment{{Operator: "MRTJ", From: commute.FareStationRef{ID: "MRTJ-DKA"}, To: commute.FareStationRef{ID: "MRTJ-LBB"}, Fare: 14000}},
		TotalFare:     &fare,
		TotalDistance: 13388,
		TransferCount: 1,
	}}
	rec := serve(t, store, planner, &fakeTimetabler{}, "/journeys?from="+fromUUID+"&to="+toUUID)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := decode(t, rec)
	itin := body["itinerary"].(map[string]any)
	if itin["status"] != "scheduled" {
		t.Fatalf("status = %v — must be scheduled, never live", itin["status"])
	}
	if itin["walkTransfers"].(float64) != 1 || itin["rideLegs"].(float64) != 1 {
		t.Fatalf("leg counts = %v", itin)
	}
	legs := itin["legs"].([]any)
	walk := legs[0].(map[string]any)
	if walk["type"] != "walk" || walk["to"].(map[string]any)["id"] != dkaUUID {
		t.Fatalf("walk leg = %v", walk)
	}
	ride := legs[1].(map[string]any)
	if ride["type"] != "ride" || ride["routeId"] != routeUUID || ride["stationCount"].(float64) != 12 {
		t.Fatalf("ride leg = %v", ride)
	}
	fareObj := itin["fare"].(map[string]any)
	if fareObj["currency"] != "IDR" || fareObj["total"].(float64) != 14000 {
		t.Fatalf("fare = %v", fareObj)
	}
	src := body["source"].(map[string]any)
	if src["provider"] != "commute" || src["requestedAt"] == nil {
		t.Fatalf("source = %v", src)
	}
}

// Shape picking is scored against the leg's whole resolved stop sequence —
// endpoints alone can't tell same-termini variants apart (TJ:7F's detour
// pattern won the old endpoint-only score by ~2 m).
func TestPlanSliceScoringSendsWholeStopSequence(t *testing.T) {
	store := baseStore(t)
	store.routeID = mustUUID(t, routeUUID)
	store.uuidRows = []generated.ListStopIDsByProviderEntityIDsRow{
		{ID: mustUUID(t, fromUUID), ProviderEntityID: "TJ-A"},
		{ID: mustUUID(t, toUUID), ProviderEntityID: "TJ-C"},
		{ID: mustUUID(t, dkaUUID), ProviderEntityID: "TJ-B"},
	}
	store.coordRows = []generated.ListStopCoordsRow{
		{ID: mustUUID(t, fromUUID), Lon: 106.870, Lat: -6.169},
		{ID: mustUUID(t, dkaUUID), Lon: 106.855, Lat: -6.174},
		{ID: mustUUID(t, toUUID), Lon: 106.823, Lat: -6.176},
	}
	planner := &fakePlanner{plan: &commute.FarePlan{
		From: commute.FareStationRef{ID: "TJ-A", Name: "Sumur Batu"},
		To:   commute.FareStationRef{ID: "TJ-C", Name: "Monumen Nasional"},
		Legs: []commute.FareLeg{
			{Type: "RIDE", Line: "TJ:7F", From: commute.FareStationRef{ID: "TJ-A"}, To: commute.FareStationRef{ID: "TJ-C"},
				Stops: []commute.FareStationRef{{ID: "TJ-B", Name: "Galur"}}},
		},
	}}
	rec := serve(t, store, planner, &fakeTimetabler{}, "/journeys?from="+fromUUID+"&to="+toUUID)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	got := store.gotSlice
	if got.MaxSnapM <= 0 {
		t.Fatalf("max_snap_m = %v — the honesty bound must be sent", got.MaxSnapM)
	}
	if len(got.Lons) != 3 || len(got.Lats) != 3 {
		t.Fatalf("lons/lats = %v/%v, want the full 3-stop sequence", got.Lons, got.Lats)
	}
	found := false
	for i, lon := range got.Lons {
		if lon == 106.855 && got.Lats[i] == -6.174 {
			found = true
		}
	}
	if !found {
		t.Fatalf("intermediate stop coords missing from scoring params: %v %v", got.Lons, got.Lats)
	}
}

// Corridor alternatives are catalog-derived: the candidate route's own stop
// slice (reversed into ride order when its seq runs the other way) and its
// own shape cut — never the chosen leg's data.
func TestPlanLegAlternatives(t *testing.T) {
	store := baseStore(t)
	store.routeID = mustUUID(t, routeUUID)
	store.uuidRows = []generated.ListStopIDsByProviderEntityIDsRow{
		{ID: mustUUID(t, fromUUID), ProviderEntityID: "TJ-A"},
		{ID: mustUUID(t, toUUID), ProviderEntityID: "TJ-C"},
	}
	store.coordRows = []generated.ListStopCoordsRow{
		{ID: mustUUID(t, fromUUID), Lon: 106.870, Lat: -6.169},
		{ID: mustUUID(t, toUUID), Lon: 106.823, Lat: -6.176},
	}
	altRouteID := mustUUID(t, "c2c2c2c2-2c2c-4c2c-8c2c-2c2c2c2c2c2c")
	store.altRows = []generated.ListRouteAlternativesRow{
		{ID: altRouteID, ProviderEntityID: "TJ:2", ShortName: pgtype.Text{String: "2", Valid: true}, SeqFrom: 10, SeqTo: 1},
	}
	store.altSlices = map[string][]generated.ListRouteStopSliceRow{
		altRouteID.String(): {
			{ID: mustUUID(t, toUUID), Name: "Monumen Nasional", Seq: 1, Lon: 106.823, Lat: -6.176},
			{ID: mustUUID(t, dkaUUID), Name: "Kwitang", Seq: 2, Lon: 106.830, Lat: -6.174},
			{ID: mustUUID(t, fromUUID), Name: "Sumur Batu", Seq: 10, Lon: 106.870, Lat: -6.169},
		},
	}
	planner := &fakePlanner{plan: &commute.FarePlan{
		From: commute.FareStationRef{ID: "TJ-A", Name: "Sumur Batu"},
		To:   commute.FareStationRef{ID: "TJ-C", Name: "Monumen Nasional"},
		Legs: []commute.FareLeg{
			{Type: "RIDE", Line: "TJ:7F", From: commute.FareStationRef{ID: "TJ-A"}, To: commute.FareStationRef{ID: "TJ-C"}},
		},
	}}
	rec := serve(t, store, planner, &fakeTimetabler{}, "/journeys?from="+fromUUID+"&to="+toUUID)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	legs := decode(t, rec)["itinerary"].(map[string]any)["legs"].([]any)
	alts := legs[0].(map[string]any)["alternatives"].([]any)
	if len(alts) != 1 {
		t.Fatalf("alternatives = %v", alts)
	}
	alt := alts[0].(map[string]any)
	if alt["line"] != "TJ:2" || alt["shortName"] != "2" || alt["operator"] != "TJ" || alt["stationCount"] != float64(2) {
		t.Fatalf("alternative = %v", alt)
	}
	stops := alt["stops"].([]any)
	if len(stops) != 1 || stops[0].(map[string]any)["name"] != "Kwitang" {
		t.Fatalf("alternative stops = %v — endpoints excluded, ride order kept", stops)
	}
	// the alt geometry scoring ran the alt route's slice in ride order
	if store.gotSlice.RouteID != altRouteID || len(store.gotSlice.Lons) != 3 || store.gotSlice.Lons[0] != 106.870 {
		t.Fatalf("slice params = %+v", store.gotSlice)
	}
}

func TestPlanNoUpstreamStation(t *testing.T) {
	rec := serve(t, baseStore(t), &fakePlanner{err: commute.ErrStationUnknown}, &fakeTimetabler{}, "/journeys?from="+fromUUID+"&to="+toUUID)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if decode(t, rec)["itinerary"] != nil {
		t.Fatal("expected null itinerary")
	}
}

func TestPlanProviderOutage(t *testing.T) {
	rec := serve(t, baseStore(t), &fakePlanner{err: errors.New("connection refused")}, &fakeTimetabler{}, "/journeys?from="+fromUUID+"&to="+toUUID)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
	if decode(t, rec)["error"].(map[string]any)["code"] != "PROVIDER_UNAVAILABLE" {
		t.Fatal("expected PROVIDER_UNAVAILABLE")
	}
}

func TestPlanSameModeSingleRide(t *testing.T) {
	store := baseStore(t)
	store.stops[toUUID] = generated.GetStopRow{ID: mustUUID(t, toUUID), Name: "Manggarai", ProviderCode: "commute", ProviderEntityID: "KCI-MRI"}
	store.routeID = mustUUID(t, routeUUID)
	store.uuidRows = []generated.ListStopIDsByProviderEntityIDsRow{
		{ID: mustUUID(t, fromUUID), ProviderEntityID: "KCI-SUD"},
		{ID: mustUUID(t, toUUID), ProviderEntityID: "KCI-MRI"},
	}
	fare := int64(3000)
	dist := 8100.0
	planner := &fakePlanner{plan: &commute.FarePlan{
		From: commute.FareStationRef{ID: "KCI-SUD", Name: "Sudirman"},
		To:   commute.FareStationRef{ID: "KCI-MRI", Name: "Manggarai"},
		Legs: []commute.FareLeg{
			{Type: "RIDE", Line: "KCI:BOO", Operator: "KCI", From: commute.FareStationRef{ID: "KCI-SUD"}, To: commute.FareStationRef{ID: "KCI-MRI"}, StationCount: 5, Headsign: "Bogor", Stops: []commute.FareStationRef{{ID: "KCI-SUD"}, {ID: "KCI-MRI"}}, DistanceM: &dist},
		},
		Segments:      []commute.FareSegment{{Operator: "KCI", From: commute.FareStationRef{ID: "KCI-SUD"}, To: commute.FareStationRef{ID: "KCI-MRI"}, Fare: 3000}},
		TotalFare:     &fare,
		TotalDistance: 8100,
	}}
	rec := serve(t, store, planner, &fakeTimetabler{}, "/journeys?from="+fromUUID+"&to="+toUUID)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	itin := decode(t, rec)["itinerary"].(map[string]any)
	if itin["rideLegs"].(float64) != 1 || itin["walkTransfers"].(float64) != 0 {
		t.Fatalf("leg counts = %v, want 1 ride 0 walks", itin)
	}
	legs := itin["legs"].([]any)
	if len(legs) != 1 || legs[0].(map[string]any)["type"] != "ride" {
		t.Fatalf("legs = %v, want single ride", legs)
	}
}

func TestPlanFareUnknown(t *testing.T) {
	// Upstream returned a route but no fare data — the fare block must be
	// omitted entirely, never emitted as a zero-rupiah fare.
	store := baseStore(t)
	store.uuidRows = []generated.ListStopIDsByProviderEntityIDsRow{
		{ID: mustUUID(t, fromUUID), ProviderEntityID: "KCI-SUD"},
		{ID: mustUUID(t, toUUID), ProviderEntityID: "MRTJ-LBB"},
	}
	planner := &fakePlanner{plan: &commute.FarePlan{
		From: commute.FareStationRef{ID: "KCI-SUD", Name: "Sudirman"},
		To:   commute.FareStationRef{ID: "MRTJ-LBB", Name: "Lebak Bulus"},
		Legs: []commute.FareLeg{
			{Type: "RIDE", Line: "MRTJ:M", Operator: "MRTJ", From: commute.FareStationRef{ID: "KCI-SUD"}, To: commute.FareStationRef{ID: "MRTJ-LBB"}, StationCount: 13},
		},
	}}
	rec := serve(t, store, planner, &fakeTimetabler{}, "/journeys?from="+fromUUID+"&to="+toUUID)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	itin := decode(t, rec)["itinerary"].(map[string]any)
	if f, ok := itin["fare"]; ok {
		t.Fatalf("fare must be omitted when upstream has none, got %v", f)
	}
}

func TestPlanProviderUnsupported(t *testing.T) {
	store := baseStore(t)
	store.stops[fromUUID] = generated.GetStopRow{ID: mustUUID(t, fromUUID), Name: "Sudirman", ProviderCode: "legacy-gtfs", ProviderEntityID: "X-SUD"}
	rec := serve(t, store, &fakePlanner{}, &fakeTimetabler{}, "/journeys?from="+fromUUID+"&to="+toUUID)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	if decode(t, rec)["error"].(map[string]any)["code"] != "PROVIDER_UNSUPPORTED" {
		t.Fatal("expected PROVIDER_UNSUPPORTED")
	}
}

func TestPlanUnknownLegTypePassesThrough(t *testing.T) {
	// Provider adds a leg type we don't model yet — pass the raw type through
	// instead of guessing, and don't count it as walk or ride.
	store := baseStore(t)
	store.uuidRows = []generated.ListStopIDsByProviderEntityIDsRow{
		{ID: mustUUID(t, fromUUID), ProviderEntityID: "KCI-SUD"},
		{ID: mustUUID(t, toUUID), ProviderEntityID: "MRTJ-LBB"},
	}
	planner := &fakePlanner{plan: &commute.FarePlan{
		From: commute.FareStationRef{ID: "KCI-SUD", Name: "Sudirman"},
		To:   commute.FareStationRef{ID: "MRTJ-LBB", Name: "Lebak Bulus"},
		Legs: []commute.FareLeg{
			{Type: "FUTURE_MODE", From: commute.FareStationRef{ID: "KCI-SUD"}, To: commute.FareStationRef{ID: "MRTJ-LBB"}},
		},
	}}
	rec := serve(t, store, planner, &fakeTimetabler{}, "/journeys?from="+fromUUID+"&to="+toUUID)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	itin := decode(t, rec)["itinerary"].(map[string]any)
	leg := itin["legs"].([]any)[0].(map[string]any)
	if leg["type"] != "FUTURE_MODE" {
		t.Fatalf("leg type = %v, want raw passthrough", leg["type"])
	}
	if itin["rideLegs"].(float64) != 0 || itin["walkTransfers"].(float64) != 0 {
		t.Fatalf("leg counts = %v, want 0/0", itin)
	}
}

func TestPlanNormalizeStoreError(t *testing.T) {
	store := baseStore(t)
	store.uuidErr = errors.New("db down")
	planner := &fakePlanner{plan: &commute.FarePlan{
		From: commute.FareStationRef{ID: "KCI-SUD", Name: "Sudirman"},
		To:   commute.FareStationRef{ID: "MRTJ-LBB", Name: "Lebak Bulus"},
		Legs: []commute.FareLeg{
			{Type: "RIDE", Line: "MRTJ:M", Operator: "MRTJ", From: commute.FareStationRef{ID: "KCI-SUD"}, To: commute.FareStationRef{ID: "MRTJ-LBB"}},
		},
	}}
	rec := serve(t, store, planner, &fakeTimetabler{}, "/journeys?from="+fromUUID+"&to="+toUUID)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

// departureStore resolves the walk+ride plan's boarding stops.
func departureStore(t *testing.T) *fakeStore {
	t.Helper()
	store := baseStore(t)
	store.uuidRows = []generated.ListStopIDsByProviderEntityIDsRow{
		{ID: mustUUID(t, fromUUID), ProviderEntityID: "KCI-SUD"},
		{ID: mustUUID(t, toUUID), ProviderEntityID: "MRTJ-LBB"},
		{ID: mustUUID(t, dkaUUID), ProviderEntityID: "MRTJ-DKA"},
	}
	return store
}

// departurePlan is TRANSFER walk KCI-SUD -> MRTJ-DKA, then RIDE MRTJ:M
// southbound to MRTJ-LBB — the boarding station is MRTJ-DKA.
func departurePlan() *commute.FarePlan {
	return &commute.FarePlan{
		From: commute.FareStationRef{ID: "KCI-SUD", Name: "Sudirman"},
		To:   commute.FareStationRef{ID: "MRTJ-LBB", Name: "Lebak Bulus"},
		Legs: []commute.FareLeg{
			{Type: "TRANSFER", From: commute.FareStationRef{ID: "KCI-SUD"}, To: commute.FareStationRef{ID: "MRTJ-DKA", Name: "Dukuh Atas BNI"}},
			{Type: "RIDE", Line: "MRTJ:M", Operator: "MRTJ", From: commute.FareStationRef{ID: "MRTJ-DKA"}, To: commute.FareStationRef{ID: "MRTJ-LBB"}, StationCount: 12, Headsign: "Lebak Bulus Bank Syariah Indonesia"},
		},
	}
}

const departureTarget = "/journeys?from=b7f4b2a0-1f3d-4e5a-9c6b-2a1d3e4f5a6b&to=aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"

func rideLegDeps(t *testing.T, rec *httptest.ResponseRecorder) []any {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	legs := decode(t, rec)["itinerary"].(map[string]any)["legs"].([]any)
	deps, _ := legs[len(legs)-1].(map[string]any)["nextDepartures"].([]any)
	return deps
}

func TestPlanAttachesNextDepartures(t *testing.T) {
	trip := "MRTJ-1023"
	tt := &fakeTimetabler{entries: []commute.TimetableEntry{
		{EstimatedDeparture: "07:30:00", BoundFor: "Lebak Bulus Bank Syariah Indonesia", LineCode: "M"},
		{EstimatedDeparture: "07:04:00", BoundFor: "Lebak Bulus Bank Syariah Indonesia", LineCode: "M", TripNumber: &trip},
		{EstimatedDeparture: "07:11:00", BoundFor: "Lebak Bulus Bank Syariah Indonesia", LineCode: "M"},
		{EstimatedDeparture: "07:17:00", BoundFor: "Lebak Bulus Bank Syariah Indonesia", LineCode: "M"},
		{EstimatedDeparture: "07:05:00", BoundFor: "Bundaran HI Bank Jakarta", LineCode: "M"},           // wrong direction
		{EstimatedDeparture: "07:06:00", BoundFor: "Lebak Bulus Bank Syariah Indonesia", LineCode: "C"}, // wrong line
		{EstimatedDeparture: "06:50:00", BoundFor: "Lebak Bulus Bank Syariah Indonesia", LineCode: "M"}, // already left
	}}
	// 2026-09-24 07:00 WIB = 00:00 UTC.
	rec := serveAt(t, departureStore(t), &fakePlanner{plan: departurePlan()}, tt,
		departureTarget, time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC))

	if tt.gotCalls != 1 || tt.gotOp != "MRTJ" || tt.gotCode != "DKA" {
		t.Fatalf("timetable fetch = op %q code %q calls %d", tt.gotOp, tt.gotCode, tt.gotCalls)
	}
	if tt.gotFrom != "07:00" || tt.gotTo != "10:00" {
		t.Fatalf("window = %s-%s, want 07:00-10:00", tt.gotFrom, tt.gotTo)
	}
	legs := decode(t, rec)["itinerary"].(map[string]any)["legs"].([]any)
	if _, ok := legs[0].(map[string]any)["nextDepartures"]; ok {
		t.Fatal("walk leg must not carry departures")
	}
	deps := legs[1].(map[string]any)["nextDepartures"].([]any)
	if len(deps) != 3 {
		t.Fatalf("departures = %v, want 3 sorted and capped", deps)
	}
	for i, want := range []string{"07:04", "07:11", "07:17"} {
		if deps[i].(map[string]any)["time"] != want {
			t.Fatalf("departures[%d] = %v", i, deps[i])
		}
	}
	if deps[0].(map[string]any)["tripNumber"] != "MRTJ-1023" {
		t.Fatalf("tripNumber = %v", deps[0])
	}
	if deps[1].(map[string]any)["tripNumber"] != nil {
		t.Fatalf("null tripNumber must stay null: %v", deps[1])
	}
}

func TestPlanDeparturesWrapMidnight(t *testing.T) {
	tt := &fakeTimetabler{entries: []commute.TimetableEntry{
		{EstimatedDeparture: "23:45:00", BoundFor: "Lebak Bulus Bank Syariah Indonesia", LineCode: "M"},
		{EstimatedDeparture: "00:15:00", BoundFor: "Lebak Bulus Bank Syariah Indonesia", LineCode: "M"}, // next day, still in window
		{EstimatedDeparture: "03:30:00", BoundFor: "Lebak Bulus Bank Syariah Indonesia", LineCode: "M"}, // outside 3h window
		{EstimatedDeparture: "23:00:00", BoundFor: "Lebak Bulus Bank Syariah Indonesia", LineCode: "M"}, // already left
	}}
	// 23:30 WIB = 16:30 UTC.
	rec := serveAt(t, departureStore(t), &fakePlanner{plan: departurePlan()}, tt,
		departureTarget, time.Date(2026, 9, 24, 16, 30, 0, 0, time.UTC))

	if tt.gotFrom != "23:30" || tt.gotTo != "02:30" {
		t.Fatalf("window = %s-%s, want 23:30-02:30", tt.gotFrom, tt.gotTo)
	}
	deps := rideLegDeps(t, rec)
	if len(deps) != 2 || deps[0].(map[string]any)["time"] != "23:45" || deps[1].(map[string]any)["time"] != "00:15" {
		t.Fatalf("departures = %v, want [23:45 00:15]", deps)
	}
}

func TestPlanTimetableOutageKeepsPlan(t *testing.T) {
	tt := &fakeTimetabler{err: errors.New("upstream down")}
	rec := serve(t, departureStore(t), &fakePlanner{plan: departurePlan()}, tt, departureTarget)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d — timetable outage must not fail the plan", rec.Code)
	}
	if deps := rideLegDeps(t, rec); deps != nil {
		t.Fatalf("departures = %v, want omitted", deps)
	}
}

func TestPlanDeparturesNoHeadsignMatchesAnyDirection(t *testing.T) {
	plan := departurePlan()
	plan.Legs[1].Headsign = ""
	tt := &fakeTimetabler{entries: []commute.TimetableEntry{
		{EstimatedDeparture: "07:05:00", BoundFor: "Bundaran HI Bank Jakarta", LineCode: "M"},
		{EstimatedDeparture: "07:04:00", BoundFor: "Lebak Bulus Bank Syariah Indonesia", LineCode: "M"},
	}}
	rec := serveAt(t, departureStore(t), &fakePlanner{plan: plan}, tt,
		departureTarget, time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC))
	deps := rideLegDeps(t, rec)
	if len(deps) != 2 || deps[0].(map[string]any)["time"] != "07:04" || deps[1].(map[string]any)["time"] != "07:05" {
		t.Fatalf("departures = %v, want both directions sorted", deps)
	}
}

func TestPlanRejectsInvalidAt(t *testing.T) {
	rec := serve(t, baseStore(t), &fakePlanner{}, &fakeTimetabler{}, departureTarget+"&at=bogus")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestPlanAtForwardedAndEchoed(t *testing.T) {
	planner := &fakePlanner{plan: departurePlan()}
	rec := serve(t, departureStore(t), planner, &fakeTimetabler{}, departureTarget+"&at=2026-09-25T08%3A00%3A00%2B07%3A00")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if planner.gotAt == nil || planner.gotAt.Format(time.RFC3339) != "2026-09-25T08:00:00+07:00" {
		t.Fatalf("at forwarded = %v", planner.gotAt)
	}
	if at := decode(t, rec)["at"]; at != "2026-09-25T08:00:00+07:00" {
		t.Fatalf("at echoed = %v", at)
	}
}

func TestPlanAtAnchorsDepartures(t *testing.T) {
	tt := &fakeTimetabler{entries: []commute.TimetableEntry{
		{EstimatedDeparture: "08:04:00", BoundFor: "Lebak Bulus Bank Syariah Indonesia", LineCode: "M"},
		{EstimatedDeparture: "07:04:00", BoundFor: "Lebak Bulus Bank Syariah Indonesia", LineCode: "M"}, // before the at anchor
	}}
	// The wall clock is irrelevant — ?at= anchors the window at 08:00 WIB.
	rec := serveAt(t, departureStore(t), &fakePlanner{plan: departurePlan()}, tt,
		departureTarget+"&at=2026-09-25T08%3A00%3A00%2B07%3A00",
		time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC))
	if tt.gotFrom != "08:00" || tt.gotTo != "11:00" {
		t.Fatalf("window = %s-%s, want 08:00-11:00", tt.gotFrom, tt.gotTo)
	}
	deps := rideLegDeps(t, rec)
	if len(deps) != 1 || deps[0].(map[string]any)["time"] != "08:04" {
		t.Fatalf("departures = %v, want [08:04]", deps)
	}
}

func TestPlanWalkOnlySkipsTimetable(t *testing.T) {
	tt := &fakeTimetabler{}
	plan := &commute.FarePlan{
		From: commute.FareStationRef{ID: "KCI-SUD", Name: "Sudirman"},
		To:   commute.FareStationRef{ID: "MRTJ-DKA", Name: "Dukuh Atas BNI"},
		Legs: []commute.FareLeg{
			{Type: "TRANSFER", From: commute.FareStationRef{ID: "KCI-SUD"}, To: commute.FareStationRef{ID: "MRTJ-DKA"}},
		},
	}
	rec := serve(t, departureStore(t), &fakePlanner{plan: plan}, tt, departureTarget)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if tt.gotCalls != 0 {
		t.Fatalf("timetable fetched %d times for a walk-only plan", tt.gotCalls)
	}
}
