// Package commute adapts the Commute Data Platform API
// (https://api.commute.shiorilabs.id, ODbL-1.0) to the canonical catalog.
// Raw types mirror the provider's JSON exactly — normalization, never these
// structs, is what the rest of the system consumes.
package commute

import "encoding/json"

// envelope wraps every response: {"status": 200, "data": ...} or
// {"status": 404, "error": {"code","message"}} on failure — the error field is
// an object, decoded lazily so success paths never pay for it.
type envelope[T any] struct {
	Status int             `json:"status"`
	Error  json.RawMessage `json:"error"`
	Data   T               `json:"data"`
}

type Operator struct {
	Code     string `json:"code"` // KCI | MRTJ | LRTJ | LRTJBDB | TJ | APCGK
	Name     string `json:"name"`
	URL      string `json:"url"`      // official site — same role as GTFS agency_url
	Timezone string `json:"timezone"` // always "Asia/Jakarta" per provider docs
	Lang     string `json:"lang"`
	Mode     string `json:"mode"` // TransitMode, GTFS route_type names
	Lines    []Line `json:"lines"`
}

type Line struct {
	Name       string `json:"name"`
	LineCode   string `json:"lineCode"`
	ColorCode  string `json:"colorCode"`
	Mode       string `json:"mode"`       // may be empty — optional upstream
	Searchable *bool  `json:"searchable"` // nil = searchable
}

// Station identity format: "{operator}-{code}", e.g. "KCI-SUD".
type Station struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	OfficialName string    `json:"officialName"` // operator's own name; search alias
	Code         string    `json:"code"`
	RegionCode   string    `json:"regionCode"` // CGK | BDO | YIA
	Operator     string    `json:"operator"`
	Lines        []string  `json:"lines"` // line keys "{operator}:{lineCode}"
	Amenities    []Amenity `json:"amenities"`
	Latitude     *float64  `json:"latitude"` // null = unsurveyed — reject, never invent
	Longitude    *float64  `json:"longitude"`
	Score        float64   `json:"score"`
	Searchable   bool      `json:"searchable"`
}

type Amenity struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Transfer is discriminated by DataType: INTERNAL keeps the passenger inside
// the paid area and resolves toStation as a StationRef; EXTERNAL requires a
// tap-out and its toStation is only {name, operatorName} — no resolvable id.
// toStation is decoded lazily per dataType.
type Transfer struct {
	ID        string          `json:"id"`
	DataType  string          `json:"dataType"` // INTERNAL | EXTERNAL
	DistanceM *float64        `json:"distanceM"`
	Notes     string          `json:"notes"`
	ToStation json.RawMessage `json:"toStation"`
}

type StationRef struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Code     string `json:"code"`
	Operator string `json:"operator"`
}

// ExternalStationRef is the EXTERNAL transfer target — a service outside this
// dataset. It has no stable id and cannot join to our stops table.
type ExternalStationRef struct {
	Name         string `json:"name"`
	OperatorName string `json:"operatorName"`
}

// LineDetail is the provider's /lines/{operator}/{lineCode} response — the
// declared stop sequence per line. Segments model trunk/branch topology:
// TRUNK is the main run; BRANCH rejoins at joinsAtCode.
type LineDetail struct {
	Operator OperatorRef `json:"operator"`
	Line     Line        `json:"line"`
	Segments []Segment   `json:"segments"`
}

type OperatorRef struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type Segment struct {
	Kind        string           `json:"kind"` // TRUNK | BRANCH
	JoinsAtCode string           `json:"joinsAtCode"`
	Stations    []SegmentStation `json:"stations"`
}

type SegmentStation struct {
	ID            string   `json:"id"` // "{operator}-{code}"
	Code          string   `json:"code"`
	Name          string   `json:"name"`
	StationNumber string   `json:"stationNumber"` // "M01" — empty for some operators
	IsInterchange bool     `json:"isInterchange"`
	OtherLines    []string `json:"otherLines"` // "{operator}:{lineCode}"
}

// TimetableEntry is one scheduled departure from /stations/{op}/{code}/
// timetable — times are Asia/Jakarta wall clock. id/dayMask/updatedAt are
// upstream bookkeeping outside the documented Schedule schema: carried raw,
// but no logic may depend on them until the provider documents semantics.
type TimetableEntry struct {
	ID                 string  `json:"id"`
	StationID          string  `json:"stationId"`
	TripNumber         *string `json:"tripNumber"`         // null when the operator publishes none
	EstimatedDeparture string  `json:"estimatedDeparture"` // HH:MM:SS
	EstimatedArrival   string  `json:"estimatedArrival"`
	BoundFor           string  `json:"boundFor"`
	LineCode           string  `json:"lineCode"`
	CreatedAt          string  `json:"createdAt"` // "YYYY-MM-DD HH:MM:SS" upstream local
	UpdatedAt          string  `json:"updatedAt"`
	DayMask            int     `json:"dayMask"` // undocumented — observed constant per operator
}

// FarePlan is the provider's /fares/{from}/{to} response — a computed
// station-to-station itinerary with legs, fare segments, and totals. Leg types
// observed: TRANSFER (walk between stops) and RIDE (line ride with an ordered
// stop list). Anything else passes through with its raw type.
type FarePlan struct {
	From          FareStationRef `json:"from"`
	To            FareStationRef `json:"to"`
	Legs          []FareLeg      `json:"legs"`
	Segments      []FareSegment  `json:"segments"`
	TotalFare     *int64         `json:"totalFare"` // IDR, integer minor-unit-safe
	TotalDistance float64        `json:"totalDistanceM"`
	TransferCount int            `json:"transferCount"`
}

// FareStationRef is the lightweight stop ref used inside fare legs — unlike
// StationRef it carries no operator field.
type FareStationRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type FareLeg struct {
	Type         string           `json:"type"` // TRANSFER | RIDE | (unknown passthrough)
	From         FareStationRef   `json:"from"`
	To           FareStationRef   `json:"to"`
	DistanceM    *float64         `json:"distanceM"`
	Line         string           `json:"line"`         // RIDE only — "{operator}:{lineCode}"
	Operator     string           `json:"operator"`     // RIDE only
	StationCount int              `json:"stationCount"` // RIDE only
	Stops        []FareStationRef `json:"stops"`        // RIDE only, ordered
	Headsign     string           `json:"headsign"`     // RIDE only
}

type FareSegment struct {
	Operator string         `json:"operator"`
	From     FareStationRef `json:"from"`
	To       FareStationRef `json:"to"`
	Fare     int64          `json:"fare"` // IDR integer
}
