package journey

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

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
	routeID      pgtype.UUID
	routeErr     error
	gotEntityIDs []string
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
	f.gotEntityIDs = arg.Column2
	return f.uuidRows, f.uuidErr
}
func (f *fakeStore) GetRouteByProviderEntityID(_ context.Context, _ generated.GetRouteByProviderEntityIDParams) (pgtype.UUID, error) {
	return f.routeID, f.routeErr
}

type fakePlanner struct {
	plan *commute.FarePlan
	err  error
}

func (f *fakePlanner) Fares(_ context.Context, _, _ string) (*commute.FarePlan, error) {
	return f.plan, f.err
}

func mustUUID(t *testing.T, s string) pgtype.UUID {
	t.Helper()
	var id pgtype.UUID
	if err := id.Scan(s); err != nil {
		t.Fatal(err)
	}
	return id
}

func serve(t *testing.T, store Store, planner FarePlanner, target string) *httptest.ResponseRecorder {
	t.Helper()
	r := chi.NewRouter()
	NewHandler(store, planner).RegisterRoutes(r)
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
		rec := serve(t, baseStore(t), &fakePlanner{}, target)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: status = %d, want 400", target, rec.Code)
		}
	}
}

func TestPlanUnknownStop(t *testing.T) {
	rec := serve(t, baseStore(t), &fakePlanner{}, "/journeys?from=11111111-2222-3333-4444-555555555555&to="+toUUID)
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
	rec := serve(t, store, planner, "/journeys?from="+fromUUID+"&to="+toUUID)
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

func TestPlanNoUpstreamStation(t *testing.T) {
	rec := serve(t, baseStore(t), &fakePlanner{err: commute.ErrStationUnknown}, "/journeys?from="+fromUUID+"&to="+toUUID)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if decode(t, rec)["itinerary"] != nil {
		t.Fatal("expected null itinerary")
	}
}

func TestPlanProviderOutage(t *testing.T) {
	rec := serve(t, baseStore(t), &fakePlanner{err: errors.New("connection refused")}, "/journeys?from="+fromUUID+"&to="+toUUID)
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
	rec := serve(t, store, planner, "/journeys?from="+fromUUID+"&to="+toUUID)
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
	rec := serve(t, store, planner, "/journeys?from="+fromUUID+"&to="+toUUID)
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
	rec := serve(t, store, &fakePlanner{}, "/journeys?from="+fromUUID+"&to="+toUUID)
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
	rec := serve(t, store, planner, "/journeys?from="+fromUUID+"&to="+toUUID)
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
	rec := serve(t, store, planner, "/journeys?from="+fromUUID+"&to="+toUUID)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}
