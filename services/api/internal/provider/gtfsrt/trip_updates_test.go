package gtfsrt

import (
	"context"
	"testing"
	"time"

	"github.com/MobilityData/gtfs-realtime-bindings/golang/gtfs"
)

func TestFetchTripUpdatesNormalizes(t *testing.T) {
	delay300 := int32(300)
	feed := &gtfs.FeedMessage{
		Header: &gtfs.FeedHeader{GtfsRealtimeVersion: str("2.0")},
		Entity: []*gtfs.FeedEntity{
			{
				Id: str("u1"),
				TripUpdate: &gtfs.TripUpdate{
					Trip:      &gtfs.TripDescriptor{TripId: str("trip-a")},
					Timestamp: u64(1760000000),
					StopTimeUpdate: []*gtfs.TripUpdate_StopTimeUpdate{
						{
							StopId:    str("stop-s"),
							Departure: &gtfs.TripUpdate_StopTimeEvent{Delay: int32p(240)},
						},
						{
							StopId:               str("stop-t"),
							ScheduleRelationship: gtfs.TripUpdate_StopTimeUpdate_SKIPPED.Enum(),
						},
						{StopId: str("stop-nodata")}, // nothing reported — dropped
					},
				},
			},
			{
				Id: str("u2"),
				TripUpdate: &gtfs.TripUpdate{
					Trip: &gtfs.TripDescriptor{
						TripId:               str("trip-b"),
						ScheduleRelationship: gtfs.TripDescriptor_CANCELED.Enum(),
					},
					Delay: &delay300,
				},
			},
			{Id: str("u3"), TripUpdate: &gtfs.TripUpdate{Trip: &gtfs.TripDescriptor{TripId: str("trip-c")}}},
			{Id: str("v1"), Vehicle: &gtfs.VehiclePosition{}}, // other feeds' job
		},
	}
	srv := serveFeed(t, feed)

	resolveTrip := func(_ context.Context, tid string) (string, bool) {
		return "canon-" + tid, true
	}
	us, err := NewClient(srv.URL, "tj-rt", nil).WithTripResolver(resolveTrip).FetchTripUpdates(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(us) != 3 {
		t.Fatalf("want 3 updates, got %+v", us)
	}

	u := us[0]
	if u.TripID != "canon-trip-a" || u.Canceled {
		t.Fatalf("identity: %+v", u)
	}
	if u.ObservedAt != time.Unix(1760000000, 0).UTC() {
		t.Fatalf("entity timestamp must win over header: %v", u.ObservedAt)
	}
	if len(u.StopKeys) != 2 {
		t.Fatalf("NO_DATA-only update must be dropped: %+v", u.StopKeys)
	}
	if u.StopKeys[0].StopKey != "stop-s" || u.StopKeys[0].DelaySec != 240 {
		t.Fatalf("departure delay: %+v", u.StopKeys[0])
	}
	if !u.StopKeys[1].Skipped {
		t.Fatalf("skipped stop: %+v", u.StopKeys[1])
	}

	if !us[1].Canceled || us[1].DelaySec == nil || *us[1].DelaySec != 300 {
		t.Fatalf("canceled + propagated: %+v", us[1])
	}
	if us[2].TripID != "canon-trip-c" {
		t.Fatalf("bare trip still resolves: %+v", us[2])
	}
}

func TestFetchTripUpdatesUnresolvedDroppedByStore(t *testing.T) {
	feed := &gtfs.FeedMessage{
		Header: &gtfs.FeedHeader{GtfsRealtimeVersion: str("2.0"), Timestamp: u64(1760000000)},
		Entity: []*gtfs.FeedEntity{{
			Id: str("u1"),
			TripUpdate: &gtfs.TripUpdate{
				Trip:           &gtfs.TripDescriptor{TripId: str("mystery")},
				StopTimeUpdate: []*gtfs.TripUpdate_StopTimeUpdate{{StopId: str("s"), Arrival: &gtfs.TripUpdate_StopTimeEvent{Delay: int32p(60)}}},
			},
		}},
	}
	srv := serveFeed(t, feed)
	us, err := NewClient(srv.URL, "tj-rt", nil).FetchTripUpdates(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(us) != 1 || us[0].TripID != "" {
		t.Fatalf("no resolver must leave TripID empty: %+v", us)
	}
}
