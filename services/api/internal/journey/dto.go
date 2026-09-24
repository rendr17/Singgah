package journey

import "time"

// Public shapes — canonical UUIDs out, provider ids stay out of the contract.

type StopRef struct {
	ID   string `json:"id,omitempty"` // canonical UUID; empty when upstream ref is unresolvable
	Name string `json:"name"`
}

type Leg struct {
	Type      string   `json:"type"` // walk | ride | <raw upstream type>
	From      StopRef  `json:"from"`
	To        StopRef  `json:"to"`
	DistanceM *float64 `json:"distanceM,omitempty"`
	// ride legs only:
	RouteID      string    `json:"routeId,omitempty"`
	Line         string    `json:"line,omitempty"`
	Operator     string    `json:"operator,omitempty"`
	StationCount int       `json:"stationCount,omitempty"`
	Stops        []StopRef `json:"stops,omitempty"`
	Headsign     string    `json:"headsign,omitempty"`
	// NextDepartures fills the first ride leg only — later boardings would
	// need arrival-time propagation the provider doesn't compute.
	NextDepartures []Departure `json:"nextDepartures,omitempty"`
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

type Fare struct {
	Currency string        `json:"currency"` // "IDR"
	Total    *int64        `json:"total,omitempty"`
	Segments []FareSegment `json:"segments"`
}

// Itinerary is one provider-computed station-to-station plan. Status is fixed
// "scheduled" — the source is static fare/route data with no realtime feed.
type Itinerary struct {
	Legs           []Leg   `json:"legs"`
	WalkTransfers  int     `json:"walkTransfers"` // count of walk legs — computed, not provider's
	RideLegs       int     `json:"rideLegs"`
	Fare           *Fare   `json:"fare,omitempty"`
	TotalDistanceM float64 `json:"totalDistanceM"`
	Status         string  `json:"status"` // always "scheduled" today
}

type PlanResponse struct {
	From      StopRef    `json:"from"`
	To        StopRef    `json:"to"`
	Itinerary *Itinerary `json:"itinerary"` // null = provider found no plan
	Source    SourceMeta `json:"source"`
}

type SourceMeta struct {
	Provider    string    `json:"provider"`
	RequestedAt time.Time `json:"requestedAt"`
}
