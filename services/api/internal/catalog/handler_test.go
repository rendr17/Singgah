package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	generated "singgah/services/api/db/generated"
	"singgah/services/api/internal/provider/commute"
)

type fakeStore struct {
	searchStops         []generated.SearchStopsRow
	searchStopsErr      error
	listStops           []generated.ListStopsRow
	listStopsErr        error
	stopsInBBox         []generated.ListStopsInBBoxRow
	stopsInBBoxErr      error
	getStop             generated.GetStopRow
	getStopErr          error
	linesServingStop    []generated.ListRoutesServingStopRow
	transfersFromStop   []generated.ListTransfersFromStopRow
	listRoutes          []generated.ListRoutesRow
	listRoutesErr       error
	getRoute            generated.GetRouteRow
	getRouteErr         error
	routeByProviderID   map[string]pgtype.UUID
	stopsOnRoute        []generated.ListStopsOnRouteRow
	providers           []generated.ListProvidersRow
	providersErr        error
	routeLines          []generated.ListRouteLinesInBBoxRow
	routeLinesErr       error
	gotSearchParams     generated.SearchStopsParams
	gotListRoutesParams generated.ListRoutesParams
	gotBBoxParams       generated.ListStopsInBBoxParams
	gotRouteLinesParams generated.ListRouteLinesInBBoxParams
	gotListStopsLimit   int32
}

func (f *fakeStore) SearchStops(_ context.Context, arg generated.SearchStopsParams) ([]generated.SearchStopsRow, error) {
	f.gotSearchParams = arg
	return f.searchStops, f.searchStopsErr
}
func (f *fakeStore) ListStops(_ context.Context, limit int32) ([]generated.ListStopsRow, error) {
	f.gotListStopsLimit = limit
	return f.listStops, f.listStopsErr
}
func (f *fakeStore) ListStopsInBBox(_ context.Context, arg generated.ListStopsInBBoxParams) ([]generated.ListStopsInBBoxRow, error) {
	f.gotBBoxParams = arg
	return f.stopsInBBox, f.stopsInBBoxErr
}
func (f *fakeStore) GetStop(_ context.Context, _ pgtype.UUID) (generated.GetStopRow, error) {
	return f.getStop, f.getStopErr
}
func (f *fakeStore) ListRoutesServingStop(_ context.Context, _ pgtype.UUID) ([]generated.ListRoutesServingStopRow, error) {
	return f.linesServingStop, nil
}
func (f *fakeStore) ListTransfersFromStop(_ context.Context, _ pgtype.UUID) ([]generated.ListTransfersFromStopRow, error) {
	return f.transfersFromStop, nil
}
func (f *fakeStore) ListRoutes(_ context.Context, arg generated.ListRoutesParams) ([]generated.ListRoutesRow, error) {
	f.gotListRoutesParams = arg
	return f.listRoutes, f.listRoutesErr
}
func (f *fakeStore) GetRoute(_ context.Context, _ pgtype.UUID) (generated.GetRouteRow, error) {
	return f.getRoute, f.getRouteErr
}
func (f *fakeStore) ListStopsOnRoute(_ context.Context, _ pgtype.UUID) ([]generated.ListStopsOnRouteRow, error) {
	return f.stopsOnRoute, nil
}
func (f *fakeStore) ListProviders(_ context.Context) ([]generated.ListProvidersRow, error) {
	return f.providers, f.providersErr
}
func (f *fakeStore) GetRouteByProviderEntityID(_ context.Context, arg generated.GetRouteByProviderEntityIDParams) (pgtype.UUID, error) {
	id, ok := f.routeByProviderID[arg.ProviderEntityID]
	if !ok {
		return pgtype.UUID{}, pgx.ErrNoRows
	}
	return id, nil
}
func (f *fakeStore) ListRouteLinesInBBox(_ context.Context, arg generated.ListRouteLinesInBBoxParams) ([]generated.ListRouteLinesInBBoxRow, error) {
	f.gotRouteLinesParams = arg
	return f.routeLines, f.routeLinesErr
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

func serve(t *testing.T, store Store, target string) *httptest.ResponseRecorder {
	t.Helper()
	return serveTT(t, store, &fakeTimetabler{}, target)
}

func serveTT(t *testing.T, store Store, tt Timetabler, target string) *httptest.ResponseRecorder {
	t.Helper()
	return serveAt(t, store, tt, target, time.Now())
}

func serveAt(t *testing.T, store Store, tt Timetabler, target string, now time.Time) *httptest.ResponseRecorder {
	t.Helper()
	h := NewHandler(store, tt)
	h.now = func() time.Time { return now }
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	h.Routes().ServeHTTP(rec, req)
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

func TestListStationsEmptyQuery(t *testing.T) {
	var id pgtype.UUID
	if err := id.Scan("b7f4b2a0-1f3d-4e5a-9c6b-2a1d3e4f5a6b"); err != nil {
		t.Fatal(err)
	}
	store := &fakeStore{listStops: []generated.ListStopsRow{{
		ID: id, Kind: "station", Name: "Sudirman", Lon: 106.823, Lat: -6.202, ProviderCode: "commute",
	}}}
	rec := serve(t, store, "/stations")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	stations := decode(t, rec)["stations"].([]any)
	if len(stations) != 1 || stations[0].(map[string]any)["name"] != "Sudirman" {
		t.Fatalf("stations = %v", stations)
	}
	if store.gotListStopsLimit != defaultLimit || store.gotSearchParams.Name != "" {
		t.Fatal("expected unfiltered ListStops with default limit")
	}
}

func TestListStationsBBox(t *testing.T) {
	var id pgtype.UUID
	if err := id.Scan("b7f4b2a0-1f3d-4e5a-9c6b-2a1d3e4f5a6b"); err != nil {
		t.Fatal(err)
	}
	store := &fakeStore{stopsInBBox: []generated.ListStopsInBBoxRow{{
		ID: id, Kind: "station", Name: "Sudirman", Lon: 106.823, Lat: -6.202, ProviderCode: "commute",
	}}}
	rec := serve(t, store, "/stations?bbox=106.7,-6.3,106.9,-6.1")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	p := store.gotBBoxParams
	if p.Column1 != 106.7 || p.Column2 != -6.3 || p.Column3 != 106.9 || p.Column4 != -6.1 {
		t.Fatalf("bbox params = %+v", p)
	}
	if p.Name != "%%" {
		t.Fatalf("bbox without query must pass %% wildcard, got %q", p.Name)
	}
	if n := len(decode(t, rec)["stations"].([]any)); n != 1 {
		t.Fatalf("stations len = %d", n)
	}
}

// bbox+query is filtered inside SQL (same alias-aware predicate as
// SearchStops) — the handler only wraps the term in % wildcards. Filtering
// after LIMIT silently dropped matches, and Go-side filtering could not see
// the official_name alias.
func TestListStationsBBoxQueryPushedToStore(t *testing.T) {
	var id pgtype.UUID
	if err := id.Scan("b7f4b2a0-1f3d-4e5a-9c6b-2a1d3e4f5a6b"); err != nil {
		t.Fatal(err)
	}
	store := &fakeStore{stopsInBBox: []generated.ListStopsInBBoxRow{
		{ID: id, Kind: "station", Name: "Sudirman", ProviderCode: "commute"},
	}}
	rec := serve(t, store, "/stations?bbox=106.7,-6.3,106.9,-6.1&query=sudirman")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if store.gotBBoxParams.Name != "%sudirman%" {
		t.Fatalf("bbox name param = %q, want %%sudirman%%", store.gotBBoxParams.Name)
	}
	if n := len(decode(t, rec)["stations"].([]any)); n != 1 {
		t.Fatalf("stations len = %d", n)
	}
}

func TestListStationsBadBBox(t *testing.T) {
	for _, bbox := range []string{
		"1,2,3",                 // wrong arity
		"a,b,c,d",               // non-numeric
		"200,-6,106,-5",         // lon out of range
		"106.9,-6.3,106.7,-6.1", // minLon > maxLon
		"106.7,-95,106.9,-6.1",  // lat out of range
	} {
		rec := serve(t, &fakeStore{}, "/stations?bbox="+bbox)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("bbox %q: status = %d, want 400", bbox, rec.Code)
		}
	}
}

func TestListStationsSearch(t *testing.T) {
	var id pgtype.UUID
	if err := id.Scan("b7f4b2a0-1f3d-4e5a-9c6b-2a1d3e4f5a6b"); err != nil {
		t.Fatal(err)
	}
	store := &fakeStore{searchStops: []generated.SearchStopsRow{{
		ID: id, Kind: "station", Code: pgtype.Text{String: "SUD", Valid: true},
		Name: "Sudirman", Lon: 106.823, Lat: -6.202, ProviderCode: "commute", Operator: "KCI",
	}}}
	rec := serve(t, store, "/stations?query=sudirman&limit=5")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if store.gotSearchParams.Name != "%sudirman%" || store.gotSearchParams.Limit != 5 {
		t.Fatalf("params = %+v", store.gotSearchParams)
	}
	stations := decode(t, rec)["stations"].([]any)
	if len(stations) != 1 {
		t.Fatalf("stations len = %d", len(stations))
	}
	s := stations[0].(map[string]any)
	if s["id"] != "b7f4b2a0-1f3d-4e5a-9c6b-2a1d3e4f5a6b" || s["code"] != "SUD" || s["providerCode"] != "commute" || s["operator"] != "KCI" {
		t.Fatalf("station = %v", s)
	}
}

func TestListStationsBadLimit(t *testing.T) {
	for _, target := range []string{"/stations?query=a&limit=0", "/stations?query=a&limit=501", "/stations?query=a&limit=abc"} {
		rec := serve(t, &fakeStore{}, target)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: status = %d, want 400", target, rec.Code)
		}
	}
}

func TestGetStationNotFound(t *testing.T) {
	store := &fakeStore{getStopErr: pgx.ErrNoRows}
	rec := serve(t, store, "/stations/b7f4b2a0-1f3d-4e5a-9c6b-2a1d3e4f5a6b")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
	errObj := decode(t, rec)["error"].(map[string]any)
	if errObj["code"] != "NOT_FOUND" {
		t.Fatalf("code = %v", errObj["code"])
	}
}

func TestGetStationMalformedID(t *testing.T) {
	rec := serve(t, &fakeStore{}, "/stations/not-a-uuid")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
	if decode(t, rec)["error"].(map[string]any)["code"] != "BAD_REQUEST" {
		t.Fatal("expected BAD_REQUEST")
	}
}

func TestGetStationDetail(t *testing.T) {
	var stopID, routeID, toStopID pgtype.UUID
	for _, s := range []struct {
		id *pgtype.UUID
		v  string
	}{
		{&stopID, "b7f4b2a0-1f3d-4e5a-9c6b-2a1d3e4f5a6b"},
		{&routeID, "11111111-2222-3333-4444-555555555555"},
		{&toStopID, "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"},
	} {
		if err := s.id.Scan(s.v); err != nil {
			t.Fatal(err)
		}
	}
	fetched := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	store := &fakeStore{
		getStop: generated.GetStopRow{
			ID: stopID, Kind: "station", Code: pgtype.Text{String: "SUD", Valid: true},
			Name: "Sudirman", Lon: 106.823, Lat: -6.202,
			Metadata: []byte(`{"official_name":"Stasiun Sudirman","amenities":[` +
				`{"type":"TOILET","text":"Concourse"},` +
				`{"type":"ELEVATOR_PAID","text":""},` +
				`{"type":"TOILET_ACCESSIBLE","text":""}]}`),
			FetchedAt:    pgtype.Timestamptz{Time: fetched, Valid: true},
			ProviderCode: "commute",
		},
		linesServingStop: []generated.ListRoutesServingStopRow{{
			ID: routeID, ShortName: pgtype.Text{String: "C", Valid: true}, Mode: "rail",
			AgencyCode: pgtype.Text{String: "KCI", Valid: true}, AgencyName: "KAI Commuter",
		}},
		transfersFromStop: []generated.ListTransfersFromStopRow{{
			ID: stopID, ToStopID: toStopID, ToStopName: "BNI City",
			ToStopCode: pgtype.Text{String: "BNI", Valid: true},
			ToLon:      106.82, ToLat: -6.2,
			WalkDistanceM: pgtype.Int4{Int32: 350, Valid: true},
			Notes:         "via skybridge",
		}},
	}
	rec := serve(t, store, "/stations/b7f4b2a0-1f3d-4e5a-9c6b-2a1d3e4f5a6b")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	station := decode(t, rec)["station"].(map[string]any)
	if station["officialName"] != "Stasiun Sudirman" {
		t.Fatalf("officialName = %v", station["officialName"])
	}
	src := station["source"].(map[string]any)
	if src["provider"] != "commute" || src["fetchedAt"] != "2026-09-24T10:00:00Z" {
		t.Fatalf("source = %v", src)
	}
	lines := station["lines"].([]any)
	if len(lines) != 1 || lines[0].(map[string]any)["id"] != "11111111-2222-3333-4444-555555555555" {
		t.Fatalf("lines = %v", lines)
	}
	transfers := station["transfers"].([]any)
	tr := transfers[0].(map[string]any)
	toStop := tr["toStop"].(map[string]any)
	if toStop["id"] != "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee" || toStop["lat"] != -6.2 {
		t.Fatalf("transfer = %v", tr)
	}
	if tr["walkDistanceM"].(float64) != 350 || tr["notes"] != "via skybridge" {
		t.Fatalf("transfer fields = %v", tr)
	}
	facs := station["facilities"].([]any)
	if len(facs) != 3 {
		t.Fatalf("facilities = %v", facs)
	}
	if facs[0].(map[string]any)["type"] != "toilet" || facs[0].(map[string]any)["text"] != "Concourse" {
		t.Fatalf("facility[0] = %v", facs[0])
	}
	if facs[0].(map[string]any)["accessibilityRelevant"] != nil {
		t.Fatal("plain toilet must not be accessibility-relevant")
	}
	for _, i := range []int{1, 2} {
		if facs[i].(map[string]any)["accessibilityRelevant"] != true {
			t.Fatalf("facility[%d] should be accessibility-relevant: %v", i, facs[i])
		}
	}
}

func TestGetRouteNotFound(t *testing.T) {
	store := &fakeStore{getRouteErr: pgx.ErrNoRows}
	rec := serve(t, store, "/routes/11111111-2222-3333-4444-555555555555")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestListRoutes(t *testing.T) {
	var id pgtype.UUID
	if err := id.Scan("11111111-2222-3333-4444-555555555555"); err != nil {
		t.Fatal(err)
	}
	store := &fakeStore{listRoutes: []generated.ListRoutesRow{{
		ID: id, ShortName: pgtype.Text{String: "C", Valid: true}, Mode: "rail",
		AgencyCode: pgtype.Text{String: "KCI", Valid: true}, ProviderCode: "commute",
	}}}
	rec := serve(t, store, "/routes?query=bogor")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if store.gotListRoutesParams.Column1 != "bogor" || store.gotListRoutesParams.Limit != 200 {
		t.Fatalf("params = %+v", store.gotListRoutesParams)
	}
	routes := decode(t, rec)["routes"].([]any)
	if len(routes) != 1 || routes[0].(map[string]any)["shortName"] != "C" {
		t.Fatalf("routes = %v", routes)
	}
}

func TestGetStationFacilitiesEmpty(t *testing.T) {
	var stopID pgtype.UUID
	if err := stopID.Scan("b7f4b2a0-1f3d-4e5a-9c6b-2a1d3e4f5a6b"); err != nil {
		t.Fatal(err)
	}
	store := &fakeStore{getStop: generated.GetStopRow{
		ID: stopID, Kind: "station", Name: "NoMeta", Metadata: []byte(`{}`), ProviderCode: "commute",
	}}
	rec := serve(t, store, "/stations/b7f4b2a0-1f3d-4e5a-9c6b-2a1d3e4f5a6b")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	station := decode(t, rec)["station"].(map[string]any)
	if facs := station["facilities"].([]any); len(facs) != 0 {
		t.Fatalf("facilities = %v, want []", facs)
	}
}

func TestListProviders(t *testing.T) {
	success := time.Date(2026, 9, 24, 8, 0, 0, 0, time.UTC)
	store := &fakeStore{providers: []generated.ListProvidersRow{{
		Code: "commute", Name: "Commute Data Platform", IsActive: true,
		LicenseName:      pgtype.Text{String: "ODbL-1.0", Valid: true},
		AttributionText:  pgtype.Text{String: "Data transit oleh Commute Data Platform", Valid: true},
		RefreshCadence:   pgtype.Text{String: "daily", Valid: true},
		KnownLimitations: pgtype.Text{String: "no realtime", Valid: true},
		LastSuccessAt:    pgtype.Timestamptz{Time: success, Valid: true},
		LastAttemptAt:    pgtype.Timestamptz{Time: success, Valid: true},
	}}}
	rec := serve(t, store, "/providers")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	providers := decode(t, rec)["providers"].([]any)
	if len(providers) != 1 {
		t.Fatalf("providers = %v", providers)
	}
	p := providers[0].(map[string]any)
	if p["code"] != "commute" || p["licenseName"] != "ODbL-1.0" || p["lastSuccessAt"] != "2026-09-24T08:00:00Z" {
		t.Fatalf("provider = %v", p)
	}
	if p["lastAttemptAt"] != "2026-09-24T08:00:00Z" {
		t.Fatalf("lastAttemptAt = %v", p)
	}
}

func TestListProvidersNeverIngested(t *testing.T) {
	store := &fakeStore{providers: []generated.ListProvidersRow{{
		Code: "gtfs-x", Name: "Future Feed", IsActive: false,
	}}}
	rec := serve(t, store, "/providers")
	p := decode(t, rec)["providers"].([]any)[0].(map[string]any)
	if _, ok := p["lastSuccessAt"]; ok {
		t.Fatalf("never-ingested provider must omit lastSuccessAt: %v", p)
	}
	if _, ok := p["lastAttemptAt"]; ok {
		t.Fatalf("never-attempted provider must omit lastAttemptAt: %v", p)
	}
	if p["isActive"] != false {
		t.Fatal("isActive should be false")
	}
}

// A failed run is visible: attempt stamped, no success — the health surface
// must distinguish "ingest failed" from "never ran".
func TestListProvidersFailedAttempt(t *testing.T) {
	attempt := time.Date(2026, 9, 24, 9, 0, 0, 0, time.UTC)
	store := &fakeStore{providers: []generated.ListProvidersRow{{
		Code: "commute", Name: "Commute Data Platform", IsActive: true,
		LastAttemptAt: pgtype.Timestamptz{Time: attempt, Valid: true},
	}}}
	rec := serve(t, store, "/providers")
	p := decode(t, rec)["providers"].([]any)[0].(map[string]any)
	if p["lastAttemptAt"] != "2026-09-24T09:00:00Z" {
		t.Fatalf("lastAttemptAt = %v", p)
	}
	if _, ok := p["lastSuccessAt"]; ok {
		t.Fatalf("failed provider must not invent lastSuccessAt: %v", p)
	}
}

func TestInternalErrorMaps500(t *testing.T) {
	store := &fakeStore{searchStopsErr: errors.New("db down")}
	rec := serve(t, store, "/stations?query=x")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
	if decode(t, rec)["error"].(map[string]any)["code"] != "INTERNAL" {
		t.Fatal("expected INTERNAL")
	}
}

// --- Departures board ---

func departureStop(t *testing.T) generated.GetStopRow {
	return generated.GetStopRow{
		ID:               mustUUID(t, "b7f4b2a0-1f3d-4e5a-9c6b-2a1d3e4f5a6b"),
		Kind:             "station",
		Code:             pgtype.Text{String: "SW", Valid: true},
		Name:             "Sawah Besar",
		Lon:              106.83,
		Lat:              -6.16,
		ProviderEntityID: "KCI-SW",
		ProviderCode:     "commute",
	}
}

func mustUUID(t *testing.T, s string) pgtype.UUID {
	t.Helper()
	var id pgtype.UUID
	if err := id.Scan(s); err != nil {
		t.Fatal(err)
	}
	return id
}

func ttEntry(line, boundFor, dep string) commute.TimetableEntry {
	return commute.TimetableEntry{LineCode: line, BoundFor: boundFor, EstimatedDeparture: dep + ":00"}
}

// Anchor 10:00 WIB — window default 240m covers 10:00-14:00, lookback 120m
// reaches 08:00.
var departuresNow = time.Date(2026, 9, 24, 3, 0, 0, 0, time.UTC)

func TestDeparturesGroupedAndWindowed(t *testing.T) {
	store := &fakeStore{getStop: departureStop(t)}
	tt := &fakeTimetabler{entries: []commute.TimetableEntry{
		ttEntry("B", "Bogor", "10:05"),
		ttEntry("B", "Jakarta Kota", "09:55"), // 5m ago -> previousDeparture
		ttEntry("B", "Jakarta Kota", "10:50"),
		ttEntry("B", "Jakarta Kota", "10:20"),
		ttEntry("B", "Bogor", "18:00"), // outside window -> dropped
		ttEntry("M", "Bundaran HI", "10:15"),
	}}
	rec := serveAt(t, store, tt, "/stations/b7f4b2a0-1f3d-4e5a-9c6b-2a1d3e4f5a6b/departures", departuresNow)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	// Upstream window: lookback 120m -> 08:00, +240m -> 14:00.
	if tt.gotOp != "KCI" || tt.gotCode != "SW" || tt.gotFrom != "08:00" || tt.gotTo != "14:00" {
		t.Fatalf("timetable args = %s %s %s-%s", tt.gotOp, tt.gotCode, tt.gotFrom, tt.gotTo)
	}
	board := decode(t, rec)["departures"].(map[string]any)
	if board["status"] != "scheduled" || board["windowMinutes"] != 240.0 {
		t.Fatalf("board meta = %v", board)
	}
	if board["station"].(map[string]any)["name"] != "Sawah Besar" {
		t.Fatalf("station = %v", board["station"])
	}
	lines := board["lines"].([]any)
	// Line B's soonest (10:05) precedes M's (10:15).
	if len(lines) != 2 || lines[0].(map[string]any)["lineCode"] != "B" {
		t.Fatalf("lines = %v", lines)
	}
	dirs := lines[0].(map[string]any)["directions"].([]any)
	if len(dirs) != 2 || dirs[0].(map[string]any)["boundFor"] != "Bogor" {
		t.Fatalf("directions = %v", dirs)
	}
	bogor := dirs[0].(map[string]any)["departures"].([]any)
	if len(bogor) != 1 || bogor[0].(map[string]any)["time"] != "10:05" {
		t.Fatalf("bogor departures = %v — 18:00 must be windowed out", bogor)
	}
	kota := dirs[1].(map[string]any)
	got := kota["departures"].([]any)
	if len(got) != 2 || got[0].(map[string]any)["time"] != "10:20" || got[1].(map[string]any)["time"] != "10:50" {
		t.Fatalf("kota departures = %v", got)
	}
	prev := kota["previousDeparture"].(map[string]any)
	if prev["time"] != "09:55" {
		t.Fatalf("previousDeparture = %v", prev)
	}
}

func TestDeparturesResolvesCanonicalRoute(t *testing.T) {
	routeID := mustUUID(t, "11111111-2222-3333-4444-555555555555")
	store := &fakeStore{
		getStop:           departureStop(t),
		routeByProviderID: map[string]pgtype.UUID{"KCI:B": routeID},
		getRoute: generated.GetRouteRow{
			ID:        routeID,
			ShortName: pgtype.Text{String: "Bogor", Valid: true},
			Mode:      "rail",
			Color:     pgtype.Text{String: "c62f38", Valid: true},
		},
	}
	tt := &fakeTimetabler{entries: []commute.TimetableEntry{ttEntry("B", "Bogor", "10:05")}}
	rec := serveAt(t, store, tt, "/stations/b7f4b2a0-1f3d-4e5a-9c6b-2a1d3e4f5a6b/departures", departuresNow)
	line := decode(t, rec)["departures"].(map[string]any)["lines"].([]any)[0].(map[string]any)
	route := line["route"].(map[string]any)
	if route["id"] != routeID.String() || route["shortName"] != "Bogor" || route["color"] != "c62f38" {
		t.Fatalf("route = %v", route)
	}
}

func TestDeparturesMidnightWrap(t *testing.T) {
	store := &fakeStore{getStop: departureStop(t)}
	tt := &fakeTimetabler{entries: []commute.TimetableEntry{ttEntry("B", "Bogor", "00:40")}}
	// 23:30 WIB -> 00:40 is +70m, inside the window.
	rec := serveAt(t, store, tt, "/stations/b7f4b2a0-1f3d-4e5a-9c6b-2a1d3e4f5a6b/departures", time.Date(2026, 9, 24, 16, 30, 0, 0, time.UTC))
	dirs := decode(t, rec)["departures"].(map[string]any)["lines"].([]any)[0].(map[string]any)["directions"].([]any)
	deps := dirs[0].(map[string]any)["departures"].([]any)
	if len(deps) != 1 || deps[0].(map[string]any)["time"] != "00:40" {
		t.Fatalf("departures past midnight = %v", deps)
	}
}

func TestDeparturesUpstreamDown(t *testing.T) {
	store := &fakeStore{getStop: departureStop(t)}
	tt := &fakeTimetabler{err: errors.New("upstream down")}
	rec := serveAt(t, store, tt, "/stations/b7f4b2a0-1f3d-4e5a-9c6b-2a1d3e4f5a6b/departures", departuresNow)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d", rec.Code)
	}
	if decode(t, rec)["error"].(map[string]any)["code"] != "PROVIDER_UNAVAILABLE" {
		t.Fatal("expected PROVIDER_UNAVAILABLE")
	}
}

func TestDeparturesUnknownUpstreamIsEmptyBoard(t *testing.T) {
	store := &fakeStore{getStop: departureStop(t)}
	tt := &fakeTimetabler{err: commute.ErrStationUnknown}
	rec := serveAt(t, store, tt, "/stations/b7f4b2a0-1f3d-4e5a-9c6b-2a1d3e4f5a6b/departures", departuresNow)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if lines := decode(t, rec)["departures"].(map[string]any)["lines"].([]any); len(lines) != 0 {
		t.Fatalf("lines = %v", lines)
	}
}

func TestDeparturesUnsupportedProvider(t *testing.T) {
	stop := departureStop(t)
	stop.ProviderCode = "other"
	store := &fakeStore{getStop: stop}
	tt := &fakeTimetabler{}
	rec := serveAt(t, store, tt, "/stations/b7f4b2a0-1f3d-4e5a-9c6b-2a1d3e4f5a6b/departures", departuresNow)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d", rec.Code)
	}
	if tt.gotCalls != 0 {
		t.Fatal("timetable must not be fetched for an unsupported provider")
	}
}

func TestDeparturesWindowValidation(t *testing.T) {
	store := &fakeStore{getStop: departureStop(t)}
	for _, target := range []string{"/stations/b7f4b2a0-1f3d-4e5a-9c6b-2a1d3e4f5a6b/departures?window=5", "/stations/b7f4b2a0-1f3d-4e5a-9c6b-2a1d3e4f5a6b/departures?window=2000", "/stations/b7f4b2a0-1f3d-4e5a-9c6b-2a1d3e4f5a6b/departures?window=abc"} {
		rec := serveAt(t, store, &fakeTimetabler{}, target, departuresNow)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d", target, rec.Code)
		}
	}
	// Full-day window asks upstream for the whole service day.
	tt := &fakeTimetabler{}
	rec := serveAt(t, store, tt, "/stations/b7f4b2a0-1f3d-4e5a-9c6b-2a1d3e4f5a6b/departures?window=1440", departuresNow)
	if rec.Code != http.StatusOK || tt.gotFrom != "00:00" || tt.gotTo != "23:59" {
		t.Fatalf("window=1440 -> %s-%s (status %d)", tt.gotFrom, tt.gotTo, rec.Code)
	}
}

// --- Network map lines ---

func TestListRouteLinesFeatureCollection(t *testing.T) {
	routeID := mustUUID(t, "11111111-2222-3333-4444-555555555555")
	geom := `{"type":"MultiLineString","coordinates":[[[106.8,-6.2],[106.9,-6.3]]]}`
	store := &fakeStore{routeLines: []generated.ListRouteLinesInBBoxRow{{
		ID: routeID, ShortName: pgtype.Text{String: "B", Valid: true},
		LongName: pgtype.Text{String: "Lin Bogor", Valid: true}, Mode: "rail",
		Color:      pgtype.Text{String: "d62126", Valid: true},
		AgencyName: pgtype.Text{String: "KAI Commuter", Valid: true},
		GeomSource: "shape", Geometry: geom,
	}}}
	rec := serve(t, store, "/map/lines?bbox=106.6,-6.5,107.1,-6.0")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	p := store.gotRouteLinesParams
	if p.Column1 != 106.6 || p.Column2 != -6.5 || p.Column3 != 107.1 || p.Column4 != -6.0 {
		t.Fatalf("bbox params = %+v", p)
	}
	lines := decode(t, rec)["lines"].(map[string]any)
	if lines["type"] != "FeatureCollection" {
		t.Fatalf("type = %v", lines["type"])
	}
	features := lines["features"].([]any)
	if len(features) != 1 {
		t.Fatalf("features = %v", features)
	}
	f := features[0].(map[string]any)
	if f["type"] != "Feature" {
		t.Fatalf("feature type = %v", f["type"])
	}
	// st_asgeojson output must pass through verbatim — same geometry content,
	// no re-shaping into a different GeoJSON type.
	g := f["geometry"].(map[string]any)
	if g["type"] != "MultiLineString" {
		t.Fatalf("geometry type = %v", g["type"])
	}
	first := g["coordinates"].([]any)[0].([]any)[0].([]any)
	if first[0] != 106.8 || first[1] != -6.2 {
		t.Fatalf("geometry coordinates = %v", g["coordinates"])
	}
	props := f["properties"].(map[string]any)
	if props["routeId"] != "11111111-2222-3333-4444-555555555555" ||
		props["shortName"] != "B" || props["mode"] != "rail" ||
		props["color"] != "d62126" || props["source"] != "shape" {
		t.Fatalf("properties = %v", props)
	}
}

func TestListRouteLinesStopsFallback(t *testing.T) {
	store := &fakeStore{routeLines: []generated.ListRouteLinesInBBoxRow{{
		ID:        mustUUID(t, "22222222-2222-3333-4444-555555555555"),
		ShortName: pgtype.Text{String: "KLB", Valid: true}, Mode: "other",
		GeomSource: "stops",
		Geometry:   `{"type":"MultiLineString","coordinates":[[[106.7,-6.1],[106.71,-6.11]]]}`,
	}}}
	rec := serve(t, store, "/map/lines?bbox=106.6,-6.5,107.1,-6.0")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	props := decode(t, rec)["lines"].(map[string]any)["features"].([]any)[0].(map[string]any)["properties"].(map[string]any)
	if props["source"] != "stops" || props["color"] != "" || props["agencyName"] != "" {
		t.Fatalf("properties = %v", props)
	}
}

func TestListRouteLinesEmpty(t *testing.T) {
	rec := serve(t, &fakeStore{}, "/map/lines?bbox=106.6,-6.5,107.1,-6.0")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	features := decode(t, rec)["lines"].(map[string]any)["features"].([]any)
	if len(features) != 0 {
		t.Fatalf("features = %v, want []", features)
	}
}

func TestListRouteLinesBBoxRequired(t *testing.T) {
	rec := serve(t, &fakeStore{}, "/map/lines")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestListRouteLinesBadBBox(t *testing.T) {
	rec := serve(t, &fakeStore{}, "/map/lines?bbox=200,-6,106,-5")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestListRouteLinesStoreError(t *testing.T) {
	store := &fakeStore{routeLinesErr: errors.New("db down")}
	rec := serve(t, store, "/map/lines?bbox=106.6,-6.5,107.1,-6.0")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
}
