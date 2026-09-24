// Package commute adapts the Commute Data Platform API
// (https://api.commute.shiorilabs.id, ODbL-1.0) to the canonical catalog.
// Raw types mirror the provider's JSON exactly — normalization, never these
// structs, is what the rest of the system consumes.
package commute

import "encoding/json"

// envelope wraps every response: {"status": 200, "data": ...} or
// {"status": 404, "error": "..."} on failure.
type envelope[T any] struct {
	Status int    `json:"status"`
	Error  string `json:"error"`
	Data   T      `json:"data"`
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
