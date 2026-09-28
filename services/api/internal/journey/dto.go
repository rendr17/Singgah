package journey

import (
	"encoding/json"
	"time"
)

// Public shapes — canonical UUIDs out, provider ids stay out of the contract.

type StopRef struct {
	ID   string `json:"id,omitempty"` // canonical UUID; empty when upstream ref is unresolvable
	Name string `json:"name"`
}

type Leg struct {
	Type      string     `json:"type"` // walk | ride
	From      StopRef    `json:"from"`
	To        StopRef    `json:"to"`
	DistanceM *float64   `json:"distanceM,omitempty"`
	DepAt     *time.Time `json:"depAt,omitempty"`
	ArrAt     *time.Time `json:"arrAt,omitempty"`
	// ride legs only:
	RouteID  string `json:"routeId,omitempty"`
	Line     string `json:"line,omitempty"`
	Operator string `json:"operator,omitempty"`
	Mode     string `json:"mode,omitempty"`
	// Color is the corridor's published color (hex without '#'), same
	// convention as RouteRef.color — empty when the catalog stores none.
	Color        string    `json:"color,omitempty"`
	StationCount int       `json:"stationCount,omitempty"`
	Stops        []StopRef `json:"stops,omitempty"`
	Headsign     string    `json:"headsign,omitempty"`
	// Estimated marks legs whose times come from a frequency template or a
	// reconstructed (derived) stop_time rather than a published trip time.
	Estimated bool `json:"estimated,omitempty"`
	// Geometry is a GeoJSON LineString sliced from an ingested route shape
	// (GTFS) between the leg's endpoints — the candidate cut that best hugs
	// the leg's stops. Absent when no shape serves the listed stops —
	// clients draw the stop-to-stop polyline instead.
	Geometry json.RawMessage `json:"geometry,omitempty"`
	// NextDepartures lists upcoming boardings at leg.from on this route —
	// computed from the same schedule snapshot the plan rode on.
	NextDepartures []Departure `json:"nextDepartures,omitempty"`
	// Alternatives are other routes that also carry this leg's endpoints —
	// corridors the rider could board instead of the planner's pick.
	Alternatives []LegAlternative `json:"alternatives,omitempty"`
}

// LegAlternative is a catalog-derived option: its stops and geometry come
// from that route's own route_stops/shape rows, so selecting one never
// borrows the chosen leg's stop list to describe a different corridor.
type LegAlternative struct {
	RouteID      string          `json:"routeId"`
	Line         string          `json:"line"` // provider entity id, e.g. "TJ:2"
	ShortName    string          `json:"shortName,omitempty"`
	Name         string          `json:"name,omitempty"` // corridor long name
	Operator     string          `json:"operator,omitempty"`
	Color        string          `json:"color,omitempty"` // corridor color, hex without '#'
	StationCount int             `json:"stationCount,omitempty"`
	Stops        []StopRef       `json:"stops"`
	Geometry     json.RawMessage `json:"geometry,omitempty"`
}

// Departure is one scheduled boarding — Asia/Jakarta wall clock.
type Departure struct {
	Time       string  `json:"time"` // HH:MM
	TripNumber *string `json:"tripNumber"`
	BoundFor   string  `json:"boundFor"`
}

type FareSegment struct {
	Operator string  `json:"operator"`
	From     StopRef `json:"from"`
	To       StopRef `json:"to"`
	Amount   int64   `json:"amount"` // IDR integer — never a float
}

// Fare describes the provider's corridor fare estimate. It is reference
// context, not a per-itinerary truth: the upstream fare service prices its
// own plan which may differ from the itinerary shown.
type Fare struct {
	Currency string        `json:"currency"` // "IDR"
	Total    *int64        `json:"total,omitempty"`
	Segments []FareSegment `json:"segments"`
}

// Itinerary is one end-to-end plan computed by the in-house schedule
// planner. Status is "scheduled" when every ride time is published, or
// "estimated" when any leg rides a frequency template or reconstructed
// stop_time — never realtime.
type Itinerary struct {
	Label         string    `json:"label"` // fastest | fewest_transfers | least_walking | alternative
	Reason        string    `json:"reason,omitempty"`
	DepartAt      time.Time `json:"departAt"`
	ArriveAt      time.Time `json:"arriveAt"`
	DurationSec   int64     `json:"durationSec"`
	Legs          []Leg     `json:"legs"`
	WalkTransfers int       `json:"walkTransfers"` // count of walk legs — computed, not provider's
	RideLegs      int       `json:"rideLegs"`
	Transfers     int       `json:"transfers"` // rideLegs - 1
	WalkM         int       `json:"walkM"`
	Status        string    `json:"status"` // scheduled | estimated
}

// QueryEcho returns the filters the plan was computed under — clients
// never have to guess which defaults applied.
type QueryEcho struct {
	DepartAt     *time.Time `json:"departAt,omitempty"`
	ArriveBy     *time.Time `json:"arriveBy,omitempty"`
	Modes        []string   `json:"modes,omitempty"`
	MaxWalkM     *int       `json:"maxWalkM,omitempty"`
	MaxTransfers *int       `json:"maxTransfers,omitempty"`
	StepFree     bool       `json:"stepFree"`
}

type PlanResponse struct {
	From        StopRef     `json:"from"`
	To          StopRef     `json:"to"`
	Query       QueryEcho   `json:"query"`
	Itineraries []Itinerary `json:"itineraries"` // empty = the planner found no plan
	// FareReference is the provider's fare estimate for the O-D pair —
	// optional context that never gates the plan itself.
	FareReference *Fare      `json:"fareReference,omitempty"`
	Source        SourceMeta `json:"source"`
}

type SourceMeta struct {
	Provider    string    `json:"provider"` // "schedule" — in-house planner over ingested data
	RequestedAt time.Time `json:"requestedAt"`
	SnapshotAt  time.Time `json:"snapshotAt"` // when the schedule snapshot was loaded
}
