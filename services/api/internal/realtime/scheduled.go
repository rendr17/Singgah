package realtime

import (
	"context"
	"time"

	"singgah/services/api/internal/planner"
)

// ScheduledSource is the provider name carried on every estimated vehicle.
// It names what the data actually is — a guess from the timetable — so
// clients and readers never mistake it for a sensor feed.
const ScheduledSource = "schedule"

// engineSource is satisfied by *planner.EngineSource. The estimator asks for
// a fresh-enough snapshot per request; the TTL keeps the cost flat.
type engineSource interface {
	Get(ctx context.Context) (*planner.Engine, error)
}

// ScheduledEstimator produces ESTIMATED vehicles for modes without a
// licensed position feed — today that's rail. Each entry carries
// Estimated=true, so StateAt reports "estimated" regardless of timestamps.
type ScheduledEstimator struct {
	Engine engineSource
	Modes  map[string]bool // e.g. rail/subway/tram — modes with no live feed
}

func (s *ScheduledEstimator) Estimated(ctx context.Context, now time.Time) ([]Vehicle, error) {
	eng, err := s.Engine.Get(ctx)
	if err != nil {
		return nil, err
	}
	positions := eng.EstimatedPositions(now, s.Modes)
	out := make([]Vehicle, 0, len(positions))
	for _, p := range positions {
		v := Vehicle{
			ID:         p.ID,
			Source:     ScheduledSource,
			TripID:     p.TripKey,
			Lat:        p.Lat,
			Lon:        p.Lon,
			Estimated:  true,
			ObservedAt: now,
			ReceivedAt: now,
		}
		if p.RouteID.Valid {
			v.RouteID = p.RouteID.String()
		}
		out = append(out, v)
	}
	return out, nil
}
