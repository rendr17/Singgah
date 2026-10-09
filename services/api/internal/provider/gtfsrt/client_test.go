package gtfsrt

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MobilityData/gtfs-realtime-bindings/golang/gtfs"
	"google.golang.org/protobuf/proto"
)

// serveFeed serializes a FeedMessage fixture behind httptest — the adapter is
// exercised against real wire bytes, not a mocked decode.
func serveFeed(t *testing.T, feed *gtfs.FeedMessage) *httptest.Server {
	t.Helper()
	body, err := proto.Marshal(feed)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-protobuf")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func u32(v uint64) *uint64   { return &v }
func u64(v uint64) *uint64   { return &v }
func str(v string) *string   { return &v }
func f32(v float32) *float32 { return &v }
func int32p(v int32) *int32  { return &v }

func TestFetchVehiclesNormalizes(t *testing.T) {
	obs := uint64(1760000000)
	bearing := float32(271.5)
	feed := &gtfs.FeedMessage{
		Header: &gtfs.FeedHeader{
			GtfsRealtimeVersion: str("2.0"),
			Timestamp:           u32(obs),
		},
		Entity: []*gtfs.FeedEntity{
			{
				Id: str("e1"),
				Vehicle: &gtfs.VehiclePosition{
					Vehicle:   &gtfs.VehicleDescriptor{Id: str("bus-4411")},
					Trip:      &gtfs.TripDescriptor{TripId: str("t-77"), RouteId: str("TJ:1")},
					Position:  &gtfs.Position{Latitude: f32(-6.175), Longitude: f32(106.828), Bearing: &bearing},
					Timestamp: u32(obs + 5), // entity beats header
				},
			},
			{
				Id: str("e2"),
				Vehicle: &gtfs.VehiclePosition{
					Vehicle:  &gtfs.VehicleDescriptor{Id: str("bus-9902")},
					Position: &gtfs.Position{Latitude: f32(-6.2), Longitude: f32(106.81)},
					// no entity timestamp → falls back to header
				},
			},
			{Id: str("e3"), TripUpdate: &gtfs.TripUpdate{Trip: &gtfs.TripDescriptor{TripId: str("t-x")}}},
			{Id: str("e4"), Vehicle: &gtfs.VehiclePosition{Vehicle: &gtfs.VehicleDescriptor{Id: str("nopos")}}},
		},
	}
	srv := serveFeed(t, feed)

	resolve := func(_ context.Context, rid string) (string, bool) {
		if rid == "TJ:1" {
			return "canonical-route-uuid", true
		}
		return "", false
	}
	c := NewClient(srv.URL, "tj-gtfs-rt", resolve)
	vs, err := c.FetchVehicles(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 2 {
		t.Fatalf("want 2 positioned vehicles, got %+v", vs)
	}
	v := vs[0]
	if v.ID != "bus-4411" || v.Source != "tj-gtfs-rt" || v.TripID != "t-77" {
		t.Fatalf("normalized identity wrong: %+v", v)
	}
	if v.RouteID != "canonical-route-uuid" {
		t.Fatalf("route resolution lost: %+v", v)
	}
	if v.ObservedAt != time.Unix(int64(obs+5), 0).UTC() {
		t.Fatalf("entity timestamp not honored: %s", v.ObservedAt)
	}
	if v.Bearing == nil || *v.Bearing != 271.5 {
		t.Fatalf("bearing lost: %+v", v.Bearing)
	}
	if vs[1].ObservedAt != time.Unix(int64(obs), 0).UTC() {
		t.Fatalf("header fallback failed: %s", vs[1].ObservedAt)
	}
}

func TestFetchVehiclesNoTimestampFails(t *testing.T) {
	feed := &gtfs.FeedMessage{
		Header: &gtfs.FeedHeader{GtfsRealtimeVersion: str("2.0")},
		Entity: []*gtfs.FeedEntity{{
			Id: str("e1"),
			Vehicle: &gtfs.VehiclePosition{
				Vehicle:  &gtfs.VehicleDescriptor{Id: str("v")},
				Position: &gtfs.Position{Latitude: f32(-6.17), Longitude: f32(106.8)},
			},
		}},
	}
	c := NewClient(serveFeed(t, feed).URL, "test", nil)
	if _, err := c.FetchVehicles(context.Background()); err == nil {
		t.Fatal("a timestamp-free feed must fail — freshness cannot be verified")
	}
}

func TestFetchVehiclesHTTPErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "test", nil)
	if _, err := c.FetchVehicles(context.Background()); err == nil {
		t.Fatal("non-200 must be a fetch error so the breaker counts it")
	}
}

func TestFetchVehiclesGarbageFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("this is not protobuf"))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "test", nil)
	if _, err := c.FetchVehicles(context.Background()); err == nil {
		t.Fatal("unparseable payload must be a fetch error")
	}
}
