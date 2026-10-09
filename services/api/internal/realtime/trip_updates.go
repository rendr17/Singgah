package realtime

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// TripUpdateSource is the seam a GTFS-RT TripUpdates feed implements.
// Same contract as Source/AlertSource: an error opens the breaker, an
// empty slice means the feed answered with nothing to report.
type TripUpdateSource interface {
	FetchTripUpdates(ctx context.Context) ([]TripUpdate, error)
}

// TripUpdate is the normalized view of one feed trip_update. TripID is the
// canonical trip UUID — the resolver already ran, so store keys never hold
// provider strings. Stop delays only carry provider stop ids the caller
// can compare itself; seq numbers stay out because the feed's numbering
// does not have to match our stop_times ordering.
type TripUpdate struct {
	TripID     string // canonical trip UUID — empty when unresolved
	StopKeys   []StopDelay
	DelaySec   *int32 // trip-level propagated delay
	Canceled   bool   // trip canceled outright — the strongest fact a feed gives
	ObservedAt time.Time
}

// StopDelay is one stop_time_update: provider stop id plus the delay the
// feed reports there. DelaySec applies to whatever event the feed gave —
// departure preferred, arrival as fallback (adapter's call).
type StopDelay struct {
	StopKey  string // provider stop entity id, as the feed publishes it
	DelaySec int32
	Skipped  bool // this stop is skipped — boarding here is canceled
}

// TripUpdateStore holds the latest trip updates per canonical trip. Unlike
// alerts, an absent update does not imply on-time — it means "the feed
// said nothing", and the board stays scheduled.
type TripUpdateStore struct {
	mu     sync.RWMutex
	byTrip map[string]TripUpdate
}

func NewTripUpdateStore() *TripUpdateStore {
	return &TripUpdateStore{byTrip: map[string]TripUpdate{}}
}

func (s *TripUpdateStore) Replace(updates []TripUpdate) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := make(map[string]TripUpdate, len(updates))
	for _, u := range updates {
		if u.TripID == "" {
			continue // unresolved trip ids can never join — drop at the door
		}
		m[u.TripID] = u
	}
	s.byTrip = m
}

// DelayAt returns (delaySeconds, canceled, found) for tripID boarding at
// providerStopKey. Order of truth: canceled trip > skipped stop > per-stop
// delay > propagated trip delay > nothing reported.
func (s *TripUpdateStore) DelayAt(tripID, providerStopKey string) (int32, bool, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.byTrip[tripID]
	if !ok {
		return 0, false, false
	}
	if u.Canceled {
		return 0, true, true
	}
	for _, sd := range u.StopKeys {
		if sd.StopKey == providerStopKey {
			if sd.Skipped {
				return 0, true, true
			}
			return sd.DelaySec, false, true
		}
	}
	if u.DelaySec != nil {
		return *u.DelaySec, false, true
	}
	// The feed reported the trip but says nothing about this stop — that is
	// silence, not "on time": nothing to annotate.
	return 0, false, false
}

func (s *TripUpdateStore) Get(tripID string) (TripUpdate, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.byTrip[tripID]
	return u, ok
}

// NewTripUpdatePoller runs a TripUpdateSource through the shared breaker.
func NewTripUpdatePoller(src TripUpdateSource, sourceID string, store *TripUpdateStore, interval time.Duration, log *slog.Logger) *Poller {
	return newPoller(func(ctx context.Context) error {
		us, err := src.FetchTripUpdates(ctx)
		if err != nil {
			return err
		}
		store.Replace(us)
		return nil
	}, sourceID, interval, log)
}
