package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"
)

// --- failure drill: kill the provider mid-run, the whole surface must
// degrade honestly instead of erroring or freezing.

// flakySource reports the vehicle until killed — the way a real feed dies
// between polls.
type flakySource struct {
	mu      sync.Mutex
	dead    bool
	vehicle Vehicle
}

func (f *flakySource) FetchVehicles(context.Context) ([]Vehicle, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.dead {
		return nil, errors.New("upstream gone")
	}
	return []Vehicle{f.vehicle}, nil
}

func (f *flakySource) kill() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.dead = true
}

func TestProviderDeathDrill(t *testing.T) {
	src := &flakySource{vehicle: Vehicle{
		ID: "bus-1", Lat: -6.2, Lon: 106.8,
		// Already at the edge of the live window so the death story reads
		// as last-known STALE, not live.
		ObservedAt: time.Now().Add(-(VehicleLiveWindow + 5*time.Second)),
		ReceivedAt: time.Now(),
	}}
	c := NewCache()
	p := NewPoller(src, "drill", c, 20*time.Millisecond, testLog())
	ctx, cancel := context.WithCancel(context.Background())
	done := p.Run(ctx)
	defer func() {
		cancel()
		<-done
	}()

	deadline := time.Now().Add(2 * time.Second)
	waitFor := func(what string, cond func() bool) {
		t.Helper()
		for time.Now().Before(deadline) {
			if cond() {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatalf("timed out waiting for %s", what)
	}

	waitFor("first successful poll", func() bool {
		return len(c.Snapshot(time.Now(), nil, "")) > 0
	})
	src.kill()
	waitFor("degraded status", func() bool {
		return p.Status().Status == "degraded"
	})

	h := NewHandler(c, p, nil, NewAlertStore(), nil)

	// The snapshot must not 500 on a dead provider — last-known serves as
	// STALE and the feed admits the outage.
	rec := serve(h, "/vehicles?bbox=106.7,-6.3,106.9,-6.1")
	if rec.Code != http.StatusOK {
		t.Fatalf("dead provider must not break the endpoint: %d", rec.Code)
	}
	var body struct {
		FeedStatus FeedStatus   `json:"feedStatus"`
		Vehicles   []vehicleDTO `json:"vehicles"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.FeedStatus.Status != "degraded" {
		t.Fatalf("feedStatus = %+v", body.FeedStatus)
	}
	if len(body.Vehicles) != 1 || body.Vehicles[0].State != StateStale {
		t.Fatalf("last-known must surface as stale: %+v", body.Vehicles)
	}

	// Health tells the same truth — a client checking first learns the feed
	// is down before trusting any marker.
	rec = serve(h, "/realtime/health")
	var s FeedStatus
	if err := json.NewDecoder(rec.Body).Decode(&s); err != nil {
		t.Fatal(err)
	}
	if s.Status != "degraded" || s.ConsecutiveFailures == 0 {
		t.Fatalf("health = %+v", s)
	}
}
