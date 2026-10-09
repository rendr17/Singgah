package realtime

import (
	"testing"
)

// DelayAt's order of truth: canceled trip beats everything, a skipped stop
// beats a delayed one, per-stop delay beats propagated trip delay, and a
// trip the feed never mentioned is distinct from an on-time report.
func TestTripUpdateDelayAt(t *testing.T) {
	d := int32(300)
	s := NewTripUpdateStore()
	s.Replace([]TripUpdate{
		{TripID: "canceled", Canceled: true, StopKeys: []StopDelay{{StopKey: "S", DelaySec: 60}}},
		{TripID: "skipped", StopKeys: []StopDelay{{StopKey: "S", Skipped: true}}},
		{TripID: "stopd", StopKeys: []StopDelay{{StopKey: "S", DelaySec: 240}, {StopKey: "T", DelaySec: 300}}, DelaySec: &d},
		{TripID: "prop", DelaySec: &d},
		{TripID: "quiet", StopKeys: []StopDelay{{StopKey: "X", DelaySec: 10}}},
	})

	if delay, cancel, found := s.DelayAt("canceled", "S"); !found || !cancel {
		t.Fatalf("canceled trip: (%v,%v,%v)", delay, cancel, found)
	}
	if _, cancel, found := s.DelayAt("skipped", "S"); !found || !cancel {
		t.Fatal("skipped stop is canceled boarding at that stop")
	}
	if delay, _, _ := s.DelayAt("stopd", "T"); delay != 300 {
		t.Fatalf("per-stop delay = %v want 300", delay)
	}
	if delay, _, found := s.DelayAt("stopd", "Q"); !found || delay != 300 {
		t.Fatalf("unlisted stop falls back to propagated: %v", delay)
	}
	if delay, _, found := s.DelayAt("prop", "anywhere"); !found || delay != 300 {
		t.Fatalf("propagated delay = %v", delay)
	}
	if _, _, found := s.DelayAt("quiet", "S"); found {
		t.Fatal("a trip silent on this stop has nothing to say — not a 0s delay")
	}
	if _, _, found := s.DelayAt("absent", "S"); found {
		t.Fatal("trip absent from feed must not look reported")
	}
}

// Unresolved trip ids can never join the schedule — Replace drops them so a
// feed publishing stranger trips does not grow the store.
func TestTripUpdateStoreDropsUnresolved(t *testing.T) {
	s := NewTripUpdateStore()
	s.Replace([]TripUpdate{{TripID: ""}, {TripID: "kept"}})
	if _, ok := s.Get(""); ok {
		t.Fatal("empty TripID must not be stored")
	}
	if _, ok := s.Get("kept"); !ok {
		t.Fatal("resolved update lost")
	}
}
