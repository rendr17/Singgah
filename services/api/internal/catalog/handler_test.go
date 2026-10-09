package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	generated "singgah/services/api/db/generated"
	"singgah/services/api/internal/planner"
	"singgah/services/api/internal/realtime"
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
	linesServingStops   []generated.ListRoutesServingStopsRow
	transfersFromStop   []generated.ListTransfersFromStopRow
	listRoutes          []generated.ListRoutesRow
	listRoutesErr       error
	getRoute            generated.GetRouteRow
	getRouteErr         error
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
func (f *fakeStore) ListRoutesServingStops(_ context.Context, _ []pgtype.UUID) ([]generated.ListRoutesServingStopsRow, error) {
	return f.linesServingStops, nil
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
func (f *fakeStore) ListRouteLinesInBBox(_ context.Context, arg generated.ListRouteLinesInBBoxParams) ([]generated.ListRouteLinesInBBoxRow, error) {
	f.gotRouteLinesParams = arg
	return f.routeLines, f.routeLinesErr
}

// fakeLoader feeds planner.Load a synthetic schedule — the board then reads
// the real engine, not a mock of it.
type fakeLoader struct {
	rows  []generated.ListScheduleRowsRow
	freqs []generated.Frequency
	stops []generated.ListPlannerStopsRow
}

func (f *fakeLoader) ListScheduleRows(context.Context) ([]generated.ListScheduleRowsRow, error) {
	return f.rows, nil
}
func (f *fakeLoader) ListAllFrequencies(context.Context) ([]generated.Frequency, error) {
	return f.freqs, nil
}
func (f *fakeLoader) ListTransferEdges(context.Context) ([]generated.ListTransferEdgesRow, error) {
	return nil, nil
}
func (f *fakeLoader) ListPlannerStops(context.Context) ([]generated.ListPlannerStopsRow, error) {
	return f.stops, nil
}

func engineSource(t *testing.T, rows []generated.ListScheduleRowsRow, freqs []generated.Frequency) *planner.EngineSource {
	t.Helper()
	l := &fakeLoader{rows: rows, freqs: freqs}
	return planner.NewEngineSource(func(ctx context.Context) (*planner.Engine, error) {
		return planner.Load(ctx, l)
	}, time.Hour)
}

func failingEngine(err error) *planner.EngineSource {
	return planner.NewEngineSource(func(context.Context) (*planner.Engine, error) {
		return nil, err
	}, time.Hour)
}

func serve(t *testing.T, store Store, target string) *httptest.ResponseRecorder {
	t.Helper()
	return serveEng(t, store, nil, target, time.Now())
}

func serveEng(t *testing.T, store Store, eng *planner.EngineSource, target string, now time.Time) *httptest.ResponseRecorder {
	t.Helper()
	h := NewHandler(store, eng)
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
	store := &fakeStore{
		searchStops: []generated.SearchStopsRow{{
			ID: id, Kind: "station", Code: pgtype.Text{String: "SUD", Valid: true},
			Name: "Sudirman", Lon: 106.823, Lat: -6.202, ProviderCode: "commute", Operator: "KCI",
		}},
		linesServingStops: []generated.ListRoutesServingStopsRow{{
			StopID: id, ID: mustUUID(t, "11111111-2222-3333-4444-555555555555"),
			ShortName: pgtype.Text{String: "C", Valid: true}, Mode: "rail",
			Color: pgtype.Text{String: "25B8EB", Valid: true}, AgencyName: "Commuter Line",
		}},
	}
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
	// Text search carries the serving lines — the picker's corridor badges.
	lines := s["lines"].([]any)
	if len(lines) != 1 || lines[0].(map[string]any)["shortName"] != "C" || lines[0].(map[string]any)["color"] != "25B8EB" {
		t.Fatalf("lines = %v", lines)
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

const (
	depStopUUID = "b7f4b2a0-1f3d-4e5a-9c6b-2a1d3e4f5a6b" // S "Sawah Besar"
	depTermUUID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee" // T terminal
	depRouteC   = "11111111-2222-3333-4444-555555555555" // KCI:C
	depRouteM   = "66666666-7777-8888-9999-000000000000" // MRTJ:M
)

func departureStop(t *testing.T) generated.GetStopRow {
	return generated.GetStopRow{
		ID:           mustUUID(t, depStopUUID),
		Kind:         "station",
		Code:         pgtype.Text{String: "SW", Valid: true},
		Name:         "Sawah Besar",
		Lon:          106.83,
		Lat:          -6.16,
		ProviderCode: "commute",
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

// depRow is one stop_time — a trip is S(dep) -> T(arr+30m).
func depRow(tripID, routeID, routeKey, headsign string, seq int32, depMin int) []generated.ListScheduleRowsRow {
	S, T := mustUUID2(depStopUUID), mustUUID2(depTermUUID)
	return []generated.ListScheduleRowsRow{
		{
			TripID: mustUUID2(tripID), TripKey: tripID[len(tripID)-3:],
			Headsign:  pgtype.Text{String: headsign, Valid: headsign != ""},
			DayMask:   127,
			StartDate: pgtype.Date{Time: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true},
			EndDate:   pgtype.Date{Time: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true},
			RouteID:   mustUUID2(routeID), RouteKey: routeKey, Mode: "rail",
			Seq: seq, StopID: S,
			ArrivalSeconds: int32(depMin * 60), DepartureSeconds: int32(depMin * 60),
		},
		{
			TripID: mustUUID2(tripID), TripKey: tripID[len(tripID)-3:],
			Headsign:  pgtype.Text{String: headsign, Valid: headsign != ""},
			DayMask:   127,
			StartDate: pgtype.Date{Time: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true},
			EndDate:   pgtype.Date{Time: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true},
			RouteID:   mustUUID2(routeID), RouteKey: routeKey, Mode: "rail",
			Seq: seq + 1, StopID: T,
			ArrivalSeconds: int32(depMin*60 + 1800), DepartureSeconds: int32(depMin*60 + 1800),
		},
	}
}

func mustUUID2(s string) pgtype.UUID {
	var id pgtype.UUID
	if err := id.Scan(s); err != nil {
		panic(err)
	}
	return id
}

// depFixture is the standard board: KCI:C serves Bogor and Jakarta Kota
// directions; MRTJ:M serves Bundaran HI.
func depFixture() []generated.ListScheduleRowsRow {
	var rows []generated.ListScheduleRowsRow
	rows = append(rows, depRow("11111111-aaaa-4aaa-8aaa-aaaaaaaaaaaa", depRouteC, "KCI:C", "Bogor", 1, 605)...)
	rows = append(rows, depRow("22222222-bbbb-4bbb-8bbb-bbbbbbbbbbbb", depRouteC, "KCI:C", "Jakarta Kota", 1, 595)...) // 5m ago -> prev
	rows = append(rows, depRow("33333333-cccc-4ccc-8ccc-cccccccccccc", depRouteC, "KCI:C", "Jakarta Kota", 1, 620)...)
	rows = append(rows, depRow("44444444-dddd-4ddd-8ddd-dddddddddddd", depRouteC, "KCI:C", "Jakarta Kota", 1, 650)...)
	rows = append(rows, depRow("55555555-eeee-4eee-8eee-eeeeeeeeeeee", depRouteC, "KCI:C", "Bogor", 1, 1080)...) // 18:00 -> windowed out
	rows = append(rows, depRow("77777777-ffff-4fff-8fff-ffffffffffff", depRouteM, "MRTJ:M", "Bundaran HI", 1, 615)...)
	return rows
}

// Anchor 10:00 WIB — window default 240m covers 10:00-14:00, lookback 120m
// reaches 08:00.
var departuresNow = time.Date(2026, 9, 24, 3, 0, 0, 0, time.UTC)

const depTarget = "/stations/" + depStopUUID + "/departures"

func TestDeparturesGroupedAndWindowed(t *testing.T) {
	store := &fakeStore{getStop: departureStop(t)}
	rec := serveEng(t, store, engineSource(t, depFixture(), nil), depTarget, departuresNow)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	board := decode(t, rec)["departures"].(map[string]any)
	if board["status"] != "scheduled" || board["windowMinutes"] != 240.0 {
		t.Fatalf("board meta = %v", board)
	}
	if board["station"].(map[string]any)["name"] != "Sawah Besar" {
		t.Fatalf("station = %v", board["station"])
	}
	src := board["source"].(map[string]any)
	if src["provider"] != "schedule" || src["snapshotAt"] == nil {
		t.Fatalf("source = %v", src)
	}
	lines := board["lines"].([]any)
	// Line C's soonest (10:05) precedes M's (10:15).
	if len(lines) != 2 || lines[0].(map[string]any)["lineCode"] != "C" {
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
	if bogor[0].(map[string]any)["estimated"] != nil {
		t.Fatal("concrete departure must not be flagged estimated")
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
	routeID := mustUUID(t, depRouteC)
	store := &fakeStore{
		getStop: departureStop(t),
		getRoute: generated.GetRouteRow{
			ID:        routeID,
			ShortName: pgtype.Text{String: "Bogor", Valid: true},
			Mode:      "rail",
			Color:     pgtype.Text{String: "c62f38", Valid: true},
		},
	}
	rows := depRow("11111111-aaaa-4aaa-8aaa-aaaaaaaaaaaa", depRouteC, "KCI:C", "Bogor", 1, 605)
	rec := serveEng(t, store, engineSource(t, rows, nil), depTarget, departuresNow)
	line := decode(t, rec)["departures"].(map[string]any)["lines"].([]any)[0].(map[string]any)
	route := line["route"].(map[string]any)
	if route["id"] != routeID.String() || route["shortName"] != "Bogor" || route["color"] != "c62f38" {
		t.Fatalf("route = %v", route)
	}
}

func TestDeparturesMidnightWrap(t *testing.T) {
	store := &fakeStore{getStop: departureStop(t)}
	rows := depRow("11111111-aaaa-4aaa-8aaa-aaaaaaaaaaaa", depRouteC, "KCI:C", "Bogor", 1, 40) // 00:40
	// 23:30 WIB -> 00:40 next service day is +70m, inside the window.
	rec := serveEng(t, store, engineSource(t, rows, nil), depTarget, time.Date(2026, 9, 24, 16, 30, 0, 0, time.UTC))
	dirs := decode(t, rec)["departures"].(map[string]any)["lines"].([]any)[0].(map[string]any)["directions"].([]any)
	deps := dirs[0].(map[string]any)["departures"].([]any)
	if len(deps) != 1 || deps[0].(map[string]any)["time"] != "00:40" {
		t.Fatalf("departures past midnight = %v", deps)
	}
}

func TestDeparturesEngineDown(t *testing.T) {
	store := &fakeStore{getStop: departureStop(t)}
	rec := serveEng(t, store, failingEngine(errors.New("snapshot load failed")), depTarget, departuresNow)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", rec.Code)
	}
	if decode(t, rec)["error"].(map[string]any)["code"] != "PLANNER_UNAVAILABLE" {
		t.Fatal("expected PLANNER_UNAVAILABLE")
	}
}

// A trip-updates feed annotates the board: per-stop delay where the feed
// names this station, canceled stays visible flagged, trips the feed never
// mentions keep the schedule's silence — and the feed's health travels on
// the board itself.
func TestDeparturesTripUpdates(t *testing.T) {
	stop := departureStop(t)
	stop.ProviderEntityID = "KCI:SW"
	store := &fakeStore{getStop: stop}

	upd := realtime.NewTripUpdateStore()
	d := int32(240)
	upd.Replace([]realtime.TripUpdate{
		// 10:05 Bogor — feed delays this stop's departure by 4m.
		{TripID: "11111111-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
			StopKeys: []realtime.StopDelay{{StopKey: "KCI:SW", DelaySec: 240}}},
		// 09:55 Kota — already gone, but canceled beats delayed.
		{TripID: "22222222-bbbb-4bbb-8bbb-bbbbbbbbbbbb", Canceled: true},
		// 10:20 Kota — only a propagated delay, no stop named.
		{TripID: "33333333-cccc-4ccc-8ccc-cccccccccccc", DelaySec: &d},
		// 10:50 Kota — trip reported, but skips this station.
		{TripID: "44444444-dddd-4ddd-8ddd-dddddddddddd",
			StopKeys: []realtime.StopDelay{{StopKey: "KCI:SW", Skipped: true}}},
		// MRTJ:M 10:15 — feed knows the trip but nothing about this stop:
		// silence, not a fake "on time".
		{TripID: "77777777-ffff-4fff-8fff-ffffffffffff"},
	})
	// A live poller proves the feed health rides the board; it writes to its
	// own store so one successful poll can't clobber the fixture.
	poller := realtime.NewTripUpdatePoller(&quietUpdates{}, "tj-rt", realtime.NewTripUpdateStore(), time.Minute, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancel := context.WithCancel(context.Background())
	done := poller.Run(ctx)
	waitFor(t, "updates poll", func() bool { return poller.Status().Status == "live" })
	cancel()
	<-done

	h := NewHandler(store, engineSource(t, depFixture(), nil)).WithTripUpdates(upd, poller)
	h.now = func() time.Time { return departuresNow }
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, depTarget, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	board := decode(t, rec)["departures"].(map[string]any)
	if board["status"] != "scheduled" {
		t.Fatal("schedule stays the source of truth even with a live updates feed")
	}
	rt := board["realtime"].(map[string]any)
	if rt["status"] != "live" || rt["source"] != "tj-rt" {
		t.Fatalf("realtime = %v", rt)
	}

	line := board["lines"].([]any)[0].(map[string]any)
	bogor := line["directions"].([]any)[0].(map[string]any)["departures"].([]any)
	if bogor[0].(map[string]any)["delaySec"] != 240.0 {
		t.Fatalf("stop delay = %v", bogor[0])
	}
	kota := line["directions"].([]any)[1].(map[string]any)
	kotaDeps := kota["departures"].([]any)
	if kotaDeps[0].(map[string]any)["delaySec"] != 240.0 {
		t.Fatalf("propagated delay = %v", kotaDeps[0])
	}
	if kotaDeps[1].(map[string]any)["canceled"] != true {
		t.Fatalf("skipped stop must read canceled: %v", kotaDeps[1])
	}
	prev := kota["previousDeparture"].(map[string]any)
	if prev["canceled"] != true {
		t.Fatalf("previousDeparture keeps its flag: %v", prev)
	}
	mrt := board["lines"].([]any)[1].(map[string]any)["directions"].([]any)[0].(map[string]any)["departures"].([]any)[0].(map[string]any)
	if _, ok := mrt["delaySec"]; ok {
		t.Fatalf("trip with no stop report must not claim on-time: %v", mrt)
	}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// quietUpdates answers an empty update list — the poller only exists so
// Status() reports a live feed; the store content is the test's own.
type quietUpdates struct{}

func (quietUpdates) FetchTripUpdates(context.Context) ([]realtime.TripUpdate, error) {
	return nil, nil
}

func TestDeparturesNoScheduleIsEmptyBoard(t *testing.T) {
	store := &fakeStore{getStop: departureStop(t)}
	rec := serveEng(t, store, engineSource(t, nil, nil), depTarget, departuresNow)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if lines := decode(t, rec)["departures"].(map[string]any)["lines"].([]any); len(lines) != 0 {
		t.Fatalf("lines = %v", lines)
	}
}

// Frequency-template trips (TJ exact_times=0) emit their headway slots —
// every one honestly flagged estimated.
func TestDeparturesFrequencyMarkedEstimated(t *testing.T) {
	store := &fakeStore{getStop: departureStop(t)}
	tpl := depRow("11111111-aaaa-4aaa-8aaa-aaaaaaaaaaaa", depRouteC, "TJ:4B", "Pulo Gadung", 1, 600) // template dep 10:00
	freqs := []generated.Frequency{{
		TripID:       mustUUID2("11111111-aaaa-4aaa-8aaa-aaaaaaaaaaaa"),
		StartSeconds: 600 * 60, EndSeconds: 12 * 3600, HeadwaySeconds: 1800,
	}}
	rec := serveEng(t, store, engineSource(t, tpl, freqs), depTarget, departuresNow)
	deps := decode(t, rec)["departures"].(map[string]any)["lines"].([]any)[0].(map[string]any)["directions"].([]any)[0].(map[string]any)["departures"].([]any)
	if len(deps) < 4 { // 10:00,10:30,...,13:30 inside the 4h window
		t.Fatalf("headway slots = %v", deps)
	}
	for _, d := range deps {
		if d.(map[string]any)["estimated"] != true {
			t.Fatalf("frequency slot must be estimated: %v", d)
		}
	}
	if deps[0].(map[string]any)["time"] != "10:00" {
		t.Fatalf("first slot = %v", deps[0])
	}
}

func TestDeparturesWindowValidation(t *testing.T) {
	store := &fakeStore{getStop: departureStop(t)}
	for _, target := range []string{depTarget + "?window=5", depTarget + "?window=2000", depTarget + "?window=abc"} {
		rec := serveEng(t, store, engineSource(t, depFixture(), nil), target, departuresNow)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d", target, rec.Code)
		}
	}
	// Full-day window returns the whole service day — including 18:00.
	rec := serveEng(t, store, engineSource(t, depFixture(), nil), depTarget+"?window=1440", departuresNow)
	if rec.Code != http.StatusOK {
		t.Fatalf("window=1440 status = %d", rec.Code)
	}
	lines := decode(t, rec)["departures"].(map[string]any)["lines"].([]any)
	found := false
	for _, l := range lines {
		for _, d := range l.(map[string]any)["directions"].([]any) {
			for _, dep := range d.(map[string]any)["departures"].([]any) {
				if dep.(map[string]any)["time"] == "18:00" {
					found = true
				}
			}
		}
	}
	if !found {
		t.Fatal("window=1440 must include the 18:00 departure")
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
