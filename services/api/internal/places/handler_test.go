package places

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	generated "singgah/services/api/db/generated"
	"singgah/services/api/internal/auth"
)

type fakeStore struct {
	rows         []generated.ListPlacesNearStopRow
	stopErr      error
	listErr      error
	detailRow    generated.GetPlaceDetailRow
	detailErr    error
	accessRows   []generated.ListPlaceTransitAccessesRow
	savedRows    []generated.ListSavedPlacesRow
	personal     generated.GetPlacePersonalStateRow
	visitRow     generated.RecordPlaceVisitRow
	visitErr     error
	saveErr      error
	unsaveErr    error
	saveArgs     generated.SavePlaceParams
	unsaveArgs   generated.UnsavePlaceParams
	personalArgs generated.GetPlacePersonalStateParams
	visitArgs    generated.RecordPlaceVisitParams
	gotStopID    pgtype.UUID
	gotLimit     int32
	callParams   generated.ListPlacesNearStopParams
}

func (f *fakeStore) GetStop(ctx context.Context, id pgtype.UUID) (generated.GetStopRow, error) {
	f.gotStopID = id
	if f.stopErr != nil {
		return generated.GetStopRow{}, f.stopErr
	}
	return generated.GetStopRow{ID: id, Name: "Test Stop"}, nil
}

func (f *fakeStore) ListPlacesNearStop(ctx context.Context, arg generated.ListPlacesNearStopParams) ([]generated.ListPlacesNearStopRow, error) {
	f.callParams = arg
	f.gotLimit = arg.Limit
	return f.rows, f.listErr
}

func (f *fakeStore) GetPlaceDetail(ctx context.Context, id pgtype.UUID) (generated.GetPlaceDetailRow, error) {
	if f.detailErr != nil {
		return generated.GetPlaceDetailRow{}, f.detailErr
	}
	row := f.detailRow
	row.ID = id
	return row, nil
}

func (f *fakeStore) ListPlaceTransitAccesses(ctx context.Context, id pgtype.UUID) ([]generated.ListPlaceTransitAccessesRow, error) {
	return f.accessRows, nil
}

func (f *fakeStore) SavePlace(ctx context.Context, arg generated.SavePlaceParams) (int64, error) {
	f.saveArgs = arg
	return 1, f.saveErr
}

func (f *fakeStore) UnsavePlace(ctx context.Context, arg generated.UnsavePlaceParams) (int64, error) {
	f.unsaveArgs = arg
	return 1, f.unsaveErr
}

func (f *fakeStore) GetPlacePersonalState(ctx context.Context, arg generated.GetPlacePersonalStateParams) (generated.GetPlacePersonalStateRow, error) {
	f.personalArgs = arg
	return f.personal, nil
}

func (f *fakeStore) ListSavedPlaces(ctx context.Context, arg generated.ListSavedPlacesParams) ([]generated.ListSavedPlacesRow, error) {
	return f.savedRows, nil
}

func (f *fakeStore) RecordPlaceVisit(ctx context.Context, arg generated.RecordPlaceVisitParams) (generated.RecordPlaceVisitRow, error) {
	f.visitArgs = arg
	if f.visitErr != nil {
		return generated.RecordPlaceVisitRow{}, f.visitErr
	}
	return f.visitRow, nil
}

func scanUUID(t *testing.T, s string) pgtype.UUID {
	t.Helper()
	var u pgtype.UUID
	if err := u.Scan(s); err != nil {
		t.Fatalf("scan uuid: %v", err)
	}
	return u
}

const stopIDStr = "11111111-2222-3333-4444-555555555555"

func TestNearStop(t *testing.T) {
	store := &fakeStore{rows: []generated.ListPlacesNearStopRow{
		{
			Name:            "Kafe A",
			PrimaryCategory: "ngopi",
			WalkDistanceM:   120,
			WalkSeconds:     pgtype.Int4{Int32: 117, Valid: true},
			Lat:             -6.2,
			Lon:             106.8,
			EditorialStatus: "curated",
		},
		{
			Name:            "Warung B",
			PrimaryCategory: "makan",
			WalkDistanceM:   340,
			Lat:             -6.21,
			Lon:             106.81,
			EditorialStatus: "unreviewed",
		},
	}}
	h := NewHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/places?near_stop_id="+stopIDStr, nil)
	rec := httptest.NewRecorder()
	h.nearStop(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d: %s", rec.Code, rec.Body.String())
	}
	if store.gotLimit != placesDefaultLimit {
		t.Fatalf("limit %d want %d", store.gotLimit, placesDefaultLimit)
	}
	var resp struct {
		Places []struct {
			Name          string `json:"name"`
			Category      string `json:"category"`
			WalkDistanceM int32  `json:"walkDistanceM"`
			WalkSeconds   *int32 `json:"walkSeconds"`
			Curated       bool   `json:"curated"`
		} `json:"places"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Places) != 2 {
		t.Fatalf("got %d places want 2", len(resp.Places))
	}
	if !resp.Places[0].Curated || resp.Places[1].Curated {
		t.Fatal("curated flag wrong")
	}
	if resp.Places[0].WalkSeconds == nil || *resp.Places[0].WalkSeconds != 117 {
		t.Fatal("walkSeconds not surfaced")
	}
	if resp.Places[1].WalkSeconds != nil {
		t.Fatal("null walkSeconds must stay null")
	}
}

func TestNearStopValidation(t *testing.T) {
	h := NewHandler(&fakeStore{})
	for _, path := range []string{
		"/api/v1/places",
		"/api/v1/places?near_stop_id=bukan-uuid",
	} {
		rec := httptest.NewRecorder()
		h.nearStop(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: got %d want 400", path, rec.Code)
		}
	}
}

func TestNearStopUnknownStop404(t *testing.T) {
	h := NewHandler(&fakeStore{stopErr: errors.New("no rows")})
	rec := httptest.NewRecorder()
	h.nearStop(rec, httptest.NewRequest(http.MethodGet, "/api/v1/places?near_stop_id="+stopIDStr, nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("got %d want 404", rec.Code)
	}
}

func TestNearStopLimitClamp(t *testing.T) {
	store := &fakeStore{}
	h := NewHandler(store)
	rec := httptest.NewRecorder()
	h.nearStop(rec, httptest.NewRequest(http.MethodGet, "/api/v1/places?near_stop_id="+stopIDStr+"&limit=999", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d", rec.Code)
	}
	if store.gotLimit != placesMaxLimit {
		t.Fatalf("limit %d want clamped %d", store.gotLimit, placesMaxLimit)
	}
}

func TestPlaceDetailRouteExists(t *testing.T) {
	r := chi.NewRouter()
	store := &fakeStore{
		detailRow: generated.GetPlaceDetailRow{
			Name: "Taman Contoh", PrimaryCategory: "taman", Lat: -6.2, Lon: 106.8,
			EditorialStatus: "curated", Accessibility: []byte(`{"stepFree":true}`),
			SourceCode: "osm", SourceName: "OpenStreetMap",
			LicenseName:     pgtype.Text{String: "ODbL-1.0", Valid: true},
			AttributionText: pgtype.Text{String: "© kontributor OpenStreetMap", Valid: true},
		},
		accessRows: []generated.ListPlaceTransitAccessesRow{{
			StopID: scanUUID(t, stopIDStr), StopName: "Cikini", StopKind: "station",
			WalkDistanceM: 180, WalkSeconds: pgtype.Int4{Int32: 168, Valid: true},
		}},
	}
	NewHandler(store).RegisterRoutes(r)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/places/"+stopIDStr, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /places/{id}: got %d, want 200", rec.Code)
	}
	var body struct {
		Place struct {
			Name   string `json:"name"`
			Source struct {
				LicenseName     string `json:"licenseName"`
				AttributionText string `json:"attributionText"`
			} `json:"source"`
			TransitAccess []struct {
				StopName      string `json:"stopName"`
				WalkDistanceM int32  `json:"walkDistanceM"`
				WalkSeconds   *int32 `json:"walkSeconds"`
			} `json:"transitAccess"`
		} `json:"place"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	if body.Place.Name != "Taman Contoh" || body.Place.Source.LicenseName != "ODbL-1.0" || body.Place.Source.AttributionText == "" {
		t.Fatalf("detail/provenance missing: %+v", body.Place)
	}
	if len(body.Place.TransitAccess) != 1 || body.Place.TransitAccess[0].StopName != "Cikini" || body.Place.TransitAccess[0].WalkSeconds == nil {
		t.Fatalf("transit context missing: %+v", body.Place.TransitAccess)
	}
}

func testRequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(auth.ContextWithUserID(r.Context(), scanUUIDFromString("99999999-8888-7777-6666-555555555555"))))
	})
}

func scanUUIDFromString(s string) pgtype.UUID {
	var id pgtype.UUID
	if err := id.Scan(s); err != nil {
		panic(fmt.Sprintf("invalid test UUID: %v", err))
	}
	return id
}

func TestPersonalPlaceRoutesRequireAuthentication(t *testing.T) {
	r := chi.NewRouter()
	NewHandler(&fakeStore{}, testRequireUser).RegisterRoutes(r)
	paths := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/places/saved"},
		{http.MethodGet, "/places/" + stopIDStr + "/personal"},
		{http.MethodPut, "/places/" + stopIDStr + "/saved"},
		{http.MethodDelete, "/places/" + stopIDStr + "/saved"},
		{http.MethodPost, "/places/" + stopIDStr + "/visits"},
	}
	for _, tc := range paths {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("got %d want 401", rec.Code)
			}
		})
	}
}

func TestSavePlaceUsesAuthenticatedUser(t *testing.T) {
	store := &fakeStore{}
	r := chi.NewRouter()
	NewHandler(store, testRequireUser).RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodPut, "/places/"+stopIDStr+"/saved", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("got %d: %s", rec.Code, rec.Body.String())
	}
	if store.saveArgs.UserID != scanUUIDFromString("99999999-8888-7777-6666-555555555555") || store.saveArgs.PlaceID != scanUUID(t, stopIDStr) {
		t.Fatalf("wrong owner/place in save query: %+v", store.saveArgs)
	}
}

func TestRecordPlaceVisitRejectsFutureTime(t *testing.T) {
	r := chi.NewRouter()
	NewHandler(&fakeStore{}, testRequireUser).RegisterRoutes(r)
	body := fmt.Sprintf(`{"clientMutationId":"%s","observedAt":%q}`,
		"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", time.Now().Add(2*time.Hour).Format(time.RFC3339Nano))
	req := httptest.NewRequest(http.MethodPost, "/places/"+stopIDStr+"/visits", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got %d want 400: %s", rec.Code, rec.Body.String())
	}
}

func TestRecordPlaceVisitRejectsIdempotencyConflict(t *testing.T) {
	observedAt := time.Now().Add(-time.Minute).UTC()
	store := &fakeStore{visitRow: generated.RecordPlaceVisitRow{
		ID:         scanUUID(t, "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"),
		PlaceID:    scanUUID(t, "22222222-3333-4444-5555-666666666666"),
		ObservedAt: pgtype.Timestamptz{Time: observedAt, Valid: true}, WasInserted: false,
	}}
	r := chi.NewRouter()
	NewHandler(store, testRequireUser).RegisterRoutes(r)
	body := fmt.Sprintf(`{"clientMutationId":"%s","observedAt":%q}`,
		"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", observedAt.Format(time.RFC3339Nano))
	req := httptest.NewRequest(http.MethodPost, "/places/"+stopIDStr+"/visits", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("got %d want 409: %s", rec.Code, rec.Body.String())
	}
}
