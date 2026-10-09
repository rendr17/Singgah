// Package realtime holds the normalized live-transit core: a freshness
// model, an in-memory live cache, and a polling loop over a provider Source.
// No concrete feed adapter exists yet — every piece works from the
// normalized Vehicle shape so a licensed source can plug in without
// touching truth semantics. SCHEDULED is never produced here: timetable
// data stays on the schedule path and never masquerades as live.
package realtime

import "time"

// State is the user-visible freshness claim on a realtime entity.
//   - live:      observed inside the live window
//   - estimated: the source itself declared this an inference — still fresh,
//     but never presented as an observation
//   - stale:     was realtime, now outside the live window; the last-known
//     value stays available so the UI can degrade honestly
type State string

const (
	StateLive      State = "live"
	StateEstimated State = "estimated"
	StateStale     State = "stale"
)

// VehicleLiveWindow bounds how long an observation may claim live. Providers
// worth using publish every ~5–30s; beyond this the position is history.
const VehicleLiveWindow = 90 * time.Second

// VehicleRetainWindow is how long a last-known position survives for STALE
// display before eviction — long enough to bridge a provider hiccup, short
// enough that a vehicle can't claim presence hours after it stopped
// reporting.
const VehicleRetainWindow = 10 * time.Minute

// Vehicle is the canonical normalized vehicle/trip position. Provider fields
// must already be resolved to canonical IDs where possible — this shape is
// the boundary between adapter and product.
type Vehicle struct {
	// ID is provider-scoped and unique only within Source.
	ID      string
	Source  string
	RouteID string // canonical route UUID; empty when the adapter can't resolve it
	TripID  string // canonical/provider trip id; empty when unknown
	Lat     float64
	Lon     float64
	Bearing *float64
	// Estimated marks positions the source inferred rather than observed.
	Estimated bool
	// ObservedAt is the provider's own timestamp — the freshness truth.
	ObservedAt time.Time
	// ReceivedAt is our ingest timestamp; kept for clock-skew diagnosis.
	ReceivedAt time.Time
}

// StateAt classifies v at `now`. Age is measured from the provider's
// observed timestamp — receivedAt would hide upstream delay.
func (v Vehicle) StateAt(now time.Time) State {
	if now.Sub(v.ObservedAt) > VehicleLiveWindow || v.ObservedAt.After(now.Add(time.Minute)) {
		// The second clause catches provider clock skew: a position "from the
		// future" is not more trustworthy, it is untrustworthy metadata.
		return StateStale
	}
	if v.Estimated {
		return StateEstimated
	}
	return StateLive
}

// inBBox reports whether the vehicle sits inside minLon,minLat,maxLon,maxLat.
func (v Vehicle) inBBox(b [4]float64) bool {
	return v.Lon >= b[0] && v.Lon <= b[2] && v.Lat >= b[1] && v.Lat <= b[3]
}
