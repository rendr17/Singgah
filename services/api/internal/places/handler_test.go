package places

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	generated "singgah/services/api/db/generated"
)

type fakeStore struct {
	rows       []generated.ListPlacesNearStopRow
	stopErr    error
	listErr    error
	gotStopID  pgtype.UUID
	gotLimit   int32
	callParams generated.ListPlacesNearStopParams
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
