package trails

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	generated "singgah/services/api/db/generated"
)

const testTrailSlug = "cikini-90-menit"

var (
	startStopID = uuid("11111111-2222-3333-4444-555555555555")
	endStopID   = uuid("66666666-7777-8888-9999-aaaaaaaaaaaa")
	placeOneID  = uuid("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
	placeTwoID  = uuid("bbbbbbbb-cccc-dddd-eeee-ffffffffffff")
)

type fakeStore struct {
	listRows []generated.ListTrailsRow
	trail    generated.GetTrailRow
	stops    []generated.ListTrailStopsRow
	trailErr error
}

func (f *fakeStore) ListTrails(context.Context) ([]generated.ListTrailsRow, error) {
	return f.listRows, nil
}

func (f *fakeStore) GetTrail(context.Context, string) (generated.GetTrailRow, error) {
	return f.trail, f.trailErr
}

func (f *fakeStore) ListTrailStops(context.Context, string) ([]generated.ListTrailStopsRow, error) {
	return f.stops, nil
}

func uuid(s string) pgtype.UUID {
	var id pgtype.UUID
	if err := id.Scan(s); err != nil {
		panic(err)
	}
	return id
}

func int4(value int32) pgtype.Int4 {
	return pgtype.Int4{Int32: value, Valid: true}
}

func trailRow() generated.GetTrailRow {
	return generated.GetTrailRow{
		ID:   uuid("cccccccc-dddd-eeee-ffff-aaaaaaaaaaaa"),
		Slug: testTrailSlug, Title: "Cikini 90 menit", Description: "Jalan santai dari stasiun.", Theme: "culture",
		StartStopID: startStopID, StartStopName: "Cikini", StartLat: -6.197, StartLon: 106.841,
		EndStopID: endStopID, EndStopName: "Gondangdia", EndLat: -6.185, EndLon: 106.832,
		ExpectedStopCount: 2, AvailableStopCount: 2, EndAccessCount: 1,
	}
}

func TestListTrailsIncludesTransitContextAndAvailability(t *testing.T) {
	store := &fakeStore{listRows: []generated.ListTrailsRow{{
		ID: trailRow().ID, Slug: testTrailSlug, Title: trailRow().Title,
		Description: trailRow().Description, Theme: trailRow().Theme,
		StartStopID: startStopID, StartStopName: "Cikini", StartLat: -6.197, StartLon: 106.841,
		EndStopID: endStopID, EndStopName: "Gondangdia", EndLat: -6.185, EndLon: 106.832,
		ExpectedStopCount: 2, AvailableStopCount: 2, EndAccessCount: 1,
	}}}
	r := chi.NewRouter()
	NewHandler(store).RegisterRoutes(r)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/trails", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Trails []struct {
			CanStart  bool  `json:"canStart"`
			Expected  int64 `json:"expectedStopCount"`
			Available int64 `json:"availableStopCount"`
			Start     struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"startTransit"`
			End struct {
				Name string `json:"name"`
			} `json:"endTransit"`
		} `json:"trails"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Trails) != 1 {
		t.Fatalf("got %d trails", len(body.Trails))
	}
	trail := body.Trails[0]
	if !trail.CanStart || trail.Expected != 2 || trail.Available != 2 {
		t.Fatalf("incorrect trail availability: %+v", trail)
	}
	if trail.Start.ID == "" || trail.Start.Name != "Cikini" || trail.End.Name != "Gondangdia" {
		t.Fatalf("transit anchors missing: %+v", trail)
	}
}

func TestTrailDetailReturnsOrderedStopsAndHonestWalkingEstimate(t *testing.T) {
	store := &fakeStore{
		trail: trailRow(),
		stops: []generated.ListTrailStopsRow{
			{Sequence: 2, StayMinutes: 20, PlaceID: placeTwoID, Name: "Museum B", PrimaryCategory: "budaya", EditorialStatus: "curated", Lat: -6.19, Lon: 106.84, SourceCode: "osm", SourceName: "OpenStreetMap", AttributionText: pgtype.Text{String: "© OpenStreetMap contributors", Valid: true}, WalkDistanceFromPreviousM: int4(100), EndWalkDistanceM: int4(50)},
			{Sequence: 1, StayMinutes: 30, PlaceID: placeOneID, Name: "Museum A", PrimaryCategory: "budaya", EditorialStatus: "curated", Lat: -6.191, Lon: 106.839, SourceCode: "osm", SourceName: "OpenStreetMap", AttributionText: pgtype.Text{String: "© OpenStreetMap contributors", Valid: true}, WalkDistanceFromPreviousM: int4(150)},
		},
	}
	r := chi.NewRouter()
	NewHandler(store).RegisterRoutes(r)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/trails/"+testTrailSlug, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		CanStart                 bool   `json:"canStart"`
		WalkDistanceM            int32  `json:"walkDistanceM"`
		EstimatedDurationMinutes int32  `json:"estimatedDurationMinutes"`
		WalkingEstimateBasis     string `json:"walkingEstimateBasis"`
		Stops                    []struct {
			Sequence int16 `json:"sequence"`
			Place    struct {
				Name   string `json:"name"`
				Source struct {
					Provider    string `json:"provider"`
					Attribution string `json:"attribution"`
				} `json:"source"`
			} `json:"place"`
		} `json:"stops"`
		EndTransit struct {
			Name string `json:"name"`
		} `json:"endTransit"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.CanStart || len(body.Stops) != 2 || body.Stops[0].Sequence != 1 || body.Stops[0].Place.Name != "Museum A" || body.Stops[1].Place.Name != "Museum B" {
		t.Fatalf("stops not ordered or trail incomplete: %+v", body)
	}
	if body.WalkDistanceM != 300 || body.EstimatedDurationMinutes != 55 || body.WalkingEstimateBasis == "" {
		t.Fatalf("walking estimate is not explicit: %+v", body)
	}
	if body.Stops[0].Place.Source.Provider != "osm" || body.Stops[0].Place.Source.Attribution == "" || body.EndTransit.Name != "Gondangdia" {
		t.Fatalf("provenance or transit end missing: %+v", body)
	}
}

func TestTrailWithMissingCuratedPlaceCannotBeStarted(t *testing.T) {
	trail := trailRow()
	trail.AvailableStopCount = 1
	store := &fakeStore{trail: trail, stops: []generated.ListTrailStopsRow{{
		Sequence: 1, StayMinutes: 30, PlaceID: placeOneID, Name: "Museum A", WalkDistanceFromPreviousM: int4(150),
	}}}
	r := chi.NewRouter()
	NewHandler(store).RegisterRoutes(r)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/trails/"+testTrailSlug, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d: %s", rec.Code, rec.Body.String())
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	var canStart bool
	if err := json.Unmarshal(body["canStart"], &canStart); err != nil {
		t.Fatal(err)
	}
	if canStart {
		t.Fatal("incomplete curated trail must not be startable")
	}
	if _, exists := body["estimatedDurationMinutes"]; exists {
		t.Fatal("incomplete trail must not claim a full-route duration")
	}
}

func TestTrailMissingEndTransitAccessCannotBeStarted(t *testing.T) {
	trail := trailRow()
	trail.EndAccessCount = 0
	store := &fakeStore{trail: trail, stops: []generated.ListTrailStopsRow{
		{Sequence: 1, StayMinutes: 30, PlaceID: placeOneID, Name: "Museum A", WalkDistanceFromPreviousM: int4(150)},
		{Sequence: 2, StayMinutes: 20, PlaceID: placeTwoID, Name: "Museum B", WalkDistanceFromPreviousM: int4(100), EndWalkDistanceM: int4(50)},
	}}
	r := chi.NewRouter()
	NewHandler(store).RegisterRoutes(r)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/trails/"+testTrailSlug, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d: %s", rec.Code, rec.Body.String())
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	var canStart bool
	if err := json.Unmarshal(body["canStart"], &canStart); err != nil {
		t.Fatal(err)
	}
	if canStart {
		t.Fatal("trail without a transit-linked final stop must not be startable")
	}
	if _, exists := body["stops"]; exists {
		t.Fatal("unavailable trail must not expose a startable stop sequence")
	}
}

func TestTrailWithMissingWalkingDistanceCannotBeStarted(t *testing.T) {
	store := &fakeStore{trail: trailRow(), stops: []generated.ListTrailStopsRow{
		{Sequence: 1, StayMinutes: 30, PlaceID: placeOneID, Name: "Museum A", WalkDistanceFromPreviousM: int4(150)},
		{Sequence: 2, StayMinutes: 20, PlaceID: placeTwoID, Name: "Museum B", WalkDistanceFromPreviousM: pgtype.Int4{}, EndWalkDistanceM: int4(50)},
	}}
	r := chi.NewRouter()
	NewHandler(store).RegisterRoutes(r)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/trails/"+testTrailSlug, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d: %s", rec.Code, rec.Body.String())
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	var canStart bool
	if err := json.Unmarshal(body["canStart"], &canStart); err != nil {
		t.Fatal(err)
	}
	if canStart {
		t.Fatal("trail with a missing walking segment must not be startable")
	}
	for _, field := range []string{"stops", "walkDistanceM", "estimatedDurationMinutes"} {
		if _, exists := body[field]; exists {
			t.Fatalf("unavailable trail must not expose %q", field)
		}
	}
}

func TestUnknownTrailReturnsNotFound(t *testing.T) {
	store := &fakeStore{trailErr: pgx.ErrNoRows}
	r := chi.NewRouter()
	NewHandler(store).RegisterRoutes(r)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/trails/not-a-trail", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("got %d want 404", rec.Code)
	}
}
