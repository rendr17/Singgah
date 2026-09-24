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
	stopsOnRoute        []generated.ListStopsOnRouteRow
	providers           []generated.ListProvidersRow
	providersErr        error
	gotSearchParams     generated.SearchStopsParams
	gotListRoutesParams generated.ListRoutesParams
	gotBBoxParams       generated.ListStopsInBBoxParams
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

func serve(t *testing.T, store Store, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	NewHandler(store).Routes().ServeHTTP(rec, req)
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
	if n := len(decode(t, rec)["stations"].([]any)); n != 1 {
		t.Fatalf("stations len = %d", n)
	}
}

func TestListStationsBBoxQueryPostFilter(t *testing.T) {
	var id pgtype.UUID
	if err := id.Scan("b7f4b2a0-1f3d-4e5a-9c6b-2a1d3e4f5a6b"); err != nil {
		t.Fatal(err)
	}
	store := &fakeStore{stopsInBBox: []generated.ListStopsInBBoxRow{
		{ID: id, Kind: "station", Name: "Sudirman", ProviderCode: "commute"},
		{ID: id, Kind: "station", Name: "Pasar Minggu", ProviderCode: "commute"},
	}}
	rec := serve(t, store, "/stations?bbox=106.7,-6.3,106.9,-6.1&query=sudirman")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	stations := decode(t, rec)["stations"].([]any)
	if len(stations) != 1 || stations[0].(map[string]any)["name"] != "Sudirman" {
		t.Fatalf("stations = %v", stations)
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
		Name: "Sudirman", Lon: 106.823, Lat: -6.202, ProviderCode: "commute",
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
	if s["id"] != "b7f4b2a0-1f3d-4e5a-9c6b-2a1d3e4f5a6b" || s["code"] != "SUD" || s["providerCode"] != "commute" {
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
	if p["isActive"] != false {
		t.Fatal("isActive should be false")
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
