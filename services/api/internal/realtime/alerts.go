package realtime

import (
	"context"
	"log/slog"
	"sort"
	"sync"
	"time"
)

// AlertSource is the seam a real alert feed implements — GTFS-RT Alerts is
// the concrete adapter. Same error contract as Source: an error opens the
// breaker, an empty slice means the feed answered "all quiet".
type AlertSource interface {
	FetchAlerts(ctx context.Context) ([]Alert, error)
}

// Alert is the normalized service alert. Cause/Effect are lowercase GTFS-RT
// enum names passed through — they are vocabulary, not decoration, so the
// adapter does not reinterpret them.
type Alert struct {
	ID          string // provider-scoped; unique within Source
	Source      string
	Cause       string
	Effect      string
	Header      string
	Description string
	URL         string
	RouteIDs    []string // canonical route UUIDs resolved from informed entities
	Scoped      bool     // feed gave selectors at all — distinguishes "scoped but unresolvable" from "network-wide"
	Periods     []Period // empty = active until the feed withdraws it
}

// Period is one active window; open ends stay nil.
type Period struct {
	Start *time.Time
	End   *time.Time
}

// ActiveAt reports whether the alert applies at t. No periods = active
// until the feed drops it; otherwise any containing window suffices.
func (a Alert) ActiveAt(t time.Time) bool {
	if len(a.Periods) == 0 {
		return true
	}
	for _, p := range a.Periods {
		if p.Start != nil && t.Before(*p.Start) {
			continue
		}
		if p.End != nil && !t.Before(*p.End) {
			continue
		}
		return true
	}
	return false
}

// AlertStore holds the last successfully fetched alert set — feeds publish
// the current world, so Replace drops withdrawn alerts outright. There is
// no staleness model like vehicles: an alert's own active period decides
// visibility, and a dead feed degrades through feedStatus, not ghost data.
type AlertStore struct {
	mu     sync.RWMutex
	alerts []Alert
}

func NewAlertStore() *AlertStore { return &AlertStore{} }

func (s *AlertStore) Replace(alerts []Alert) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.alerts = alerts
}

// List returns alerts active at t; routeID narrows to alerts informing that
// route plus network-wide ones (no informed entities at all).
func (s *AlertStore) List(t time.Time, routeID string) []Alert {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Alert, 0, len(s.alerts))
	for _, a := range s.alerts {
		if !a.ActiveAt(t) {
			continue
		}
		if routeID != "" && a.Scoped {
			match := false
			for _, rid := range a.RouteIDs {
				if rid == routeID {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		return a.ID < b.ID
	})
	return out
}

// NewAlertPoller runs an AlertSource through the shared breaker. The store
// is replaced wholesale on success — partial alert state is worse than the
// previous complete state.
func NewAlertPoller(src AlertSource, sourceID string, store *AlertStore, interval time.Duration, log *slog.Logger) *Poller {
	return newPoller(func(ctx context.Context) error {
		as, err := src.FetchAlerts(ctx)
		if err != nil {
			return err
		}
		for i := range as {
			if as[i].Source == "" {
				as[i].Source = sourceID
			}
		}
		store.Replace(as)
		return nil
	}, sourceID, interval, log)
}
