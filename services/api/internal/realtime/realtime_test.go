package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

var now = time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)

func veh(id string, age time.Duration) Vehicle {
	return Vehicle{
		ID: id, Source: "test",
		Lat: -6.2, Lon: 106.8,
		ObservedAt: now.Add(-age), ReceivedAt: now.Add(-age),
	}
}

// --- freshness: boundaries are the product claim, test them exactly.

func TestStateAtBoundaries(t *testing.T) {
	cases := []struct {
		name string
		age  time.Duration
		want State
	}{
		{"fresh observation", 5 * time.Second, StateLive},
		{"inside live window", VehicleLiveWindow - time.Second, StateLive},
		{"past live window", VehicleLiveWindow + time.Second, StateStale},
		{"well past", VehicleRetainWindow, StateStale},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := veh("v", tc.age).StateAt(now); got != tc.want {
				t.Fatalf("age %s: got %s, want %s", tc.age, got, tc.want)
			}
		})
	}
}

func TestStateAtEstimatedStaysEstimatedWhileFresh(t *testing.T) {
	v := veh("v", 10*time.Second)
	v.Estimated = true
	if got := v.StateAt(now); got != StateEstimated {
		t.Fatalf("got %s, want estimated", got)
	}
	v.ObservedAt = now.Add(-VehicleLiveWindow - time.Second)
	if got := v.StateAt(now); got != StateStale {
		t.Fatalf("old estimate got %s, want stale", got)
	}
}

func TestStateAtFutureObservationIsStale(t *testing.T) {
	// Provider clock ahead of ours is untrustworthy metadata, not "extra live".
	v := veh("v", -10*time.Minute)
	if got := v.StateAt(now); got != StateStale {
		t.Fatalf("future observation got %s, want stale", got)
	}
}

// --- cache

func TestCacheSnapshotFiltersBBoxAndRoute(t *testing.T) {
	c := NewCache()
	in := veh("in", 10*time.Second)
	in.RouteID = "r1"
	out := veh("out", 10*time.Second)
	out.Lon = 107.5 // outside bbox
	c.Upsert([]Vehicle{in, out}, now)

	bbox := [4]float64{106.7, -6.3, 106.9, -6.1}
	snap := c.Snapshot(now, &bbox, "")
	if len(snap) != 1 || snap[0].ID != "in" {
		t.Fatalf("bbox filter got %+v", snap)
	}
	snap = c.Snapshot(now, &bbox, "other-route")
	if len(snap) != 0 {
		t.Fatalf("route filter should empty the snapshot, got %+v", snap)
	}
	snap = c.Snapshot(now, &bbox, "r1")
	if len(snap) != 1 {
		t.Fatalf("route match lost, got %+v", snap)
	}
}

func TestCacheEvictsPastRetainWindow(t *testing.T) {
	c := NewCache()
	old := veh("old", VehicleRetainWindow+time.Minute)
	c.Upsert([]Vehicle{old}, now)
	if snap := c.Snapshot(now, nil, ""); len(snap) != 0 {
		t.Fatalf("stale entity should be evicted, got %+v", snap)
	}
}

func TestCacheSameIDAcrossSources(t *testing.T) {
	c := NewCache()
	a, b := veh("1", time.Second), veh("1", time.Second)
	a.Source, b.Source = "feed-a", "feed-b"
	c.Upsert([]Vehicle{a, b}, now)
	if snap := c.Snapshot(now, nil, ""); len(snap) != 2 {
		t.Fatalf("provider-scoped ids collided, got %+v", snap)
	}
}

// --- poller / breaker

type fakeSource struct {
	vehicles []Vehicle
	err      error
	calls    *int
}

func (f fakeSource) FetchVehicles(context.Context) ([]Vehicle, error) {
	if f.calls != nil {
		*f.calls++
	}
	return f.vehicles, f.err
}

func testLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestPollerSuccessFeedsCache(t *testing.T) {
	c := NewCache()
	p := NewPoller(fakeSource{vehicles: []Vehicle{veh("v", time.Second)}}, "test", c, time.Second, testLog())
	p.tick(context.Background())
	if snap := c.Snapshot(now, nil, ""); len(snap) != 1 {
		t.Fatalf("cache empty after poll: %+v", snap)
	}
	if s := p.Status(); s.Status != "live" || s.ConsecutiveFailures != 0 {
		t.Fatalf("status = %+v", s)
	}
}

func TestPollerFailuresDegradeThenOpenBreaker(t *testing.T) {
	calls := 0
	c := NewCache()
	p := NewPoller(fakeSource{err: errors.New("upstream down"), calls: &calls}, "test", c, time.Second, testLog())
	for i := 0; i < breakerFailures; i++ {
		p.tick(context.Background())
	}
	if s := p.Status(); s.Status != "degraded" || s.ConsecutiveFailures != breakerFailures {
		t.Fatalf("status = %+v", s)
	}
	// Breaker open: ticks are skipped until openUntil — no new upstream calls.
	p.tick(context.Background())
	if calls != breakerFailures {
		t.Fatalf("breaker did not open, calls=%d", calls)
	}
}

func TestPollerEmptyAnswerIsNotFailure(t *testing.T) {
	c := NewCache()
	p := NewPoller(fakeSource{vehicles: nil}, "test", c, time.Second, testLog())
	p.tick(context.Background())
	if s := p.Status(); s.Status != "live" {
		t.Fatalf("empty-but-successful fetch marked degraded: %+v", s)
	}
}

// --- HTTP surface

func serve(h *Handler, target string) *httptest.ResponseRecorder {
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	r.ServeHTTP(rec, req)
	return rec
}

func TestVehiclesRequiresBBox(t *testing.T) {
	rec := serve(NewHandler(NewCache(), nil, nil, NewAlertStore(), nil), "/vehicles")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing bbox: got %d", rec.Code)
	}
}

// --- SSE stream: the same snapshot, pushed.

func TestStreamRequiresBBox(t *testing.T) {
	rec := serve(NewHandler(NewCache(), nil, nil, NewAlertStore(), nil), "/realtime/stream")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing bbox: got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); strings.Contains(ct, "event-stream") {
		t.Fatal("a 400 must not masquerade as an SSE stream")
	}
}

func TestStreamEmitsSnapshotThenClosesOnCancel(t *testing.T) {
	h := NewHandler(NewCache(), nil, nil, NewAlertStore(), nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/realtime/stream?bbox=106.7,-6.3,106.9,-6.1", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		r.ServeHTTP(rec, req)
		close(done)
	}()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && rec.Body.Len() == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("stream must close when the client disconnects")
	}
	body := rec.Body.String()
	if !strings.HasPrefix(body, "event: vehicles\ndata: ") {
		t.Fatalf("first frame must be a vehicles event, got %q", body)
	}
	var payload struct {
		FeedStatus FeedStatus   `json:"feedStatus"`
		Vehicles   []vehicleDTO `json:"vehicles"`
	}
	data := strings.TrimSpace(strings.SplitN(body, "data: ", 2)[1])
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		t.Fatalf("event payload must be the VehicleList shape: %v", err)
	}
	if payload.FeedStatus.Status != "unavailable" {
		t.Fatalf("no configured feed must stream unavailable: %+v", payload.FeedStatus)
	}
}

func TestVehiclesUnconfiguredFeedIsHonest(t *testing.T) {
	c := NewCache()
	c.Upsert([]Vehicle{veh("v", 10*time.Second)}, time.Now())
	rec := serve(NewHandler(c, nil, nil, NewAlertStore(), nil), "/vehicles?bbox=106.7,-6.3,106.9,-6.1")
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d: %s", rec.Code, rec.Body)
	}
	var body struct {
		FeedStatus FeedStatus   `json:"feedStatus"`
		Vehicles   []vehicleDTO `json:"vehicles"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	// No poller = no licensed source configured; the feed must not claim live.
	if body.FeedStatus.Status != "unavailable" {
		t.Fatalf("feedStatus = %+v", body.FeedStatus)
	}
	if len(body.Vehicles) != 1 || body.Vehicles[0].State == "" {
		t.Fatalf("vehicles = %+v", body.Vehicles)
	}
}

// --- estimated merge: schedule guesses join the snapshot, live wins.

type fakeEstimator struct {
	vs  []Vehicle
	err error
}

func (f fakeEstimator) Estimated(context.Context, time.Time) ([]Vehicle, error) {
	return f.vs, f.err
}

func TestVehiclesMergesEstimatedAndLiveWins(t *testing.T) {
	c := NewCache()
	live := veh("bus-1", 10*time.Second)
	live.TripID = "trip-1"
	live.ObservedAt = time.Now().Add(-10 * time.Second) // fixture `now` has passed
	live.ReceivedAt = live.ObservedAt
	c.Upsert([]Vehicle{live}, time.Now())
	estAt := time.Now()
	est := fakeEstimator{vs: []Vehicle{
		{ID: "trip-1~0", TripID: "trip-1", Source: ScheduledSource, Lat: -6.2, Lon: 106.8, Estimated: true, ObservedAt: estAt, ReceivedAt: estAt},
		{ID: "trip-9", TripID: "trip-9", Source: ScheduledSource, Lat: -6.2, Lon: 106.8, Estimated: true, ObservedAt: estAt, ReceivedAt: estAt},
	}}
	rec := serve(NewHandler(c, nil, est, NewAlertStore(), nil), "/vehicles?bbox=106.7,-6.3,106.9,-6.1")
	var body struct {
		Vehicles []vehicleDTO `json:"vehicles"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Vehicles) != 2 {
		t.Fatalf("dedupe should leave 2 vehicles, got %+v", body.Vehicles)
	}
	var liveSeen, estSeen bool
	for _, v := range body.Vehicles {
		if v.ID == "bus-1" && v.State == StateLive {
			liveSeen = true
		}
		if v.ID == "trip-9" && v.State == StateEstimated {
			estSeen = true
		}
	}
	if !liveSeen || !estSeen {
		t.Fatalf("live+estimated merge: %+v", body.Vehicles)
	}
}

func TestVehiclesEstimatedRespectsViewport(t *testing.T) {
	est := fakeEstimator{vs: []Vehicle{
		{ID: "far", TripID: "t", Source: ScheduledSource, Lat: -7.0, Lon: 107.0, Estimated: true},
	}}
	rec := serve(NewHandler(NewCache(), nil, est, NewAlertStore(), nil), "/vehicles?bbox=106.7,-6.3,106.9,-6.1")
	var body struct {
		Vehicles []vehicleDTO `json:"vehicles"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Vehicles) != 0 {
		t.Fatalf("out-of-viewport estimate leaked: %+v", body.Vehicles)
	}
}

func TestVehiclesEstimatorFailureDegrades(t *testing.T) {
	est := fakeEstimator{err: errors.New("schedule snapshot down")}
	rec := serve(NewHandler(NewCache(), nil, est, NewAlertStore(), nil), "/vehicles?bbox=106.7,-6.3,106.9,-6.1")
	if rec.Code != http.StatusOK {
		t.Fatalf("estimator failure must not break the snapshot: %d", rec.Code)
	}
}

// --- alerts: active windows, route scoping, and honest unavailable.

func TestAlertStoreList(t *testing.T) {
	end := now.Add(2 * time.Hour)
	past := now.Add(-time.Hour)
	s := NewAlertStore()
	s.Replace([]Alert{
		{ID: "a1", Source: "x", Scoped: true, RouteIDs: []string{"r-1"}},
		{ID: "a2", Source: "x"}, // network-wide
		{ID: "a3", Source: "x", Periods: []Period{{Start: &past, End: &end}}},
		{ID: "a4", Source: "x", Periods: []Period{{Start: &end}}}, // future
	})
	all := s.List(now, "")
	ids := map[string]bool{}
	for _, a := range all {
		ids[a.ID] = true
	}
	if !ids["a1"] || !ids["a2"] || !ids["a3"] || ids["a4"] {
		t.Fatalf("unfiltered list = %+v", all)
	}
	filtered := s.List(now, "r-1")
	if len(filtered) != 3 {
		// a1 matches, a2 is network-wide, a3 is scoped? no — a3 has no
		// selectors so it is also network-wide.
		t.Fatalf("route filter = %+v", filtered)
	}
	filtered = s.List(now, "r-9")
	for _, a := range filtered {
		if a.ID == "a1" {
			t.Fatalf("scoped alert leaked into r-9: %+v", filtered)
		}
	}
}

func TestAlertsEndpointUnconfiguredIsHonest(t *testing.T) {
	rec := serve(NewHandler(NewCache(), nil, nil, NewAlertStore(), nil), "/alerts")
	var body struct {
		FeedStatus FeedStatus `json:"feedStatus"`
		Alerts     []alertDTO `json:"alerts"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.FeedStatus.Status != "unavailable" || len(body.Alerts) != 0 {
		t.Fatalf("unconfigured alerts: %+v", body)
	}
}

func TestAlertsEndpointServesStore(t *testing.T) {
	s := NewAlertStore()
	s.Replace([]Alert{{ID: "a1", Source: "gtfs-rt", Header: "Detour", Scoped: true, RouteIDs: []string{"r-1"}}})
	rec := serve(NewHandler(NewCache(), nil, nil, s, nil), "/alerts?route_id=r-1")
	var body struct {
		Alerts []alertDTO `json:"alerts"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Alerts) != 1 || body.Alerts[0].ID != "a1" {
		t.Fatalf("alerts = %+v", body.Alerts)
	}
	rec = serve(NewHandler(NewCache(), nil, nil, s, nil), "/alerts?route_id=r-9")
	body.Alerts = nil
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Alerts) != 0 {
		t.Fatalf("scoped alert leaked: %+v", body.Alerts)
	}
}

func TestRealtimeHealth(t *testing.T) {
	rec := serve(NewHandler(NewCache(), nil, nil, NewAlertStore(), nil), "/realtime/health")
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d", rec.Code)
	}
	var s FeedStatus
	if err := json.NewDecoder(rec.Body).Decode(&s); err != nil {
		t.Fatal(err)
	}
	if s.Status != "unavailable" {
		t.Fatalf("status = %+v", s)
	}
}
