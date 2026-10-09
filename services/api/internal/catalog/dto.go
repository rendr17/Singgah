package catalog

import (
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"singgah/services/api/internal/realtime"
)

// Public DTOs — the API contract shape. Canonical UUIDs only; provider ids
// stay diagnostic (metadata), per docs/34_API_SPEC.md.

type SourceMeta struct {
	Provider        string     `json:"provider"`
	FetchedAt       *time.Time `json:"fetchedAt,omitempty"`
	SourceUpdatedAt *time.Time `json:"sourceUpdatedAt,omitempty"`
}

type StopRef struct {
	ID   string  `json:"id"`
	Name string  `json:"name"`
	Code string  `json:"code,omitempty"`
	Lat  float64 `json:"lat"`
	Lon  float64 `json:"lon"`
}

type StationSummary struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Code         string  `json:"code,omitempty"`
	Kind         string  `json:"kind"`
	Lat          float64 `json:"lat"`
	Lon          float64 `json:"lon"`
	ProviderCode string  `json:"providerCode"`
	// Operator is the provider-declared operator code (TJ, MRTJ, KCI…) —
	// the client keys official brand marks off it; empty when unsurveyed.
	Operator string `json:"operator,omitempty"`
	// Lines are the catalog routes serving this stop — populated on text
	// search (combobox badges); absent on bbox and reference listings.
	Lines []RouteRef `json:"lines,omitempty"`
}

type RouteRef struct {
	ID         string `json:"id"`
	ShortName  string `json:"shortName,omitempty"`
	LongName   string `json:"longName,omitempty"`
	Mode       string `json:"mode"`
	Color      string `json:"color,omitempty"`
	AgencyCode string `json:"agencyCode,omitempty"`
	AgencyName string `json:"agencyName,omitempty"`
}

type RouteSummary struct {
	RouteRef
	ProviderCode string `json:"providerCode"`
}

type TransferDTO struct {
	ToStop        StopRef `json:"toStop"`
	WalkDistanceM *int32  `json:"walkDistanceM,omitempty"`
	Notes         string  `json:"notes,omitempty"`
}

// Facility is a provider-declared amenity. accessibilityRelevant classifies
// the type only — presence says nothing about whether it works today.
type Facility struct {
	Type                  string `json:"type"`
	Text                  string `json:"text,omitempty"`
	AccessibilityRelevant bool   `json:"accessibilityRelevant,omitempty"`
}

type StationDetail struct {
	StationSummary
	OfficialName string        `json:"officialName,omitempty"`
	Lines        []RouteRef    `json:"lines"`
	Transfers    []TransferDTO `json:"transfers"`
	Facilities   []Facility    `json:"facilities"`
	Source       SourceMeta    `json:"source"`
}

// Departure is one scheduled boarding — Asia/Jakarta wall clock. Estimated
// marks a headway template slot or a time the source never published
// (docs/09: such times are ESTIMATED, not SCHEDULED).
type Departure struct {
	Time       string  `json:"time"` // HH:MM
	TripNumber *string `json:"tripNumber"`
	BoundFor   string  `json:"boundFor"`
	Estimated  bool    `json:"estimated,omitempty"`
	// DelaySec/Canceled come from a trip-updates feed — both absent when no
	// feed reports on this trip, so "no news" and "on time" stay distinct.
	DelaySec *int32 `json:"delaySec,omitempty"`
	Canceled bool   `json:"canceled,omitempty"`
}

// DepartureDirection is one boundFor group on a line. PreviousDeparture is
// the latest boarding already gone inside the lookback window.
type DepartureDirection struct {
	BoundFor          string      `json:"boundFor"`
	Departures        []Departure `json:"departures"`
	PreviousDeparture *Departure  `json:"previousDeparture,omitempty"`
}

// DepartureLine groups directions on one provider line. Route is nil when
// the provider line does not resolve to a canonical route — LineCode then
// is the only identity shown.
type DepartureLine struct {
	LineCode   string               `json:"lineCode"`
	Route      *RouteRef            `json:"route,omitempty"`
	Directions []DepartureDirection `json:"directions"`
}

type DepartureSource struct {
	Provider    string     `json:"provider"`
	RequestedAt time.Time  `json:"requestedAt"`
	SnapshotAt  *time.Time `json:"snapshotAt,omitempty"`
}

// StationDepartures is a station's departure board. Status stays
// "scheduled" — the timetable is the source of truth — while Realtime
// reports the trip-updates feed's health when one is wired (omitted when
// none exists; the board never claims realtime it does not have).
type StationDepartures struct {
	Station       StopRef              `json:"station"`
	Status        string               `json:"status"`
	WindowMinutes int                  `json:"windowMinutes"`
	Lines         []DepartureLine      `json:"lines"`
	Source        DepartureSource      `json:"source"`
	Realtime      *realtime.FeedStatus `json:"realtime,omitempty"`
}

// RouteStop is a StopRef plus its position on the route — seq is the
// flattened provider order across segments; stationNumber is the operator's
// own number (M01…) when published; segmentKind keeps TRUNK/BRANCH topology.
type RouteStop struct {
	StopRef
	Seq           int32  `json:"seq"`
	StationNumber string `json:"stationNumber,omitempty"`
	SegmentKind   string `json:"segmentKind,omitempty"`
}

type RouteDetail struct {
	RouteSummary
	Stops  []RouteStop `json:"stops"`
	Source SourceMeta  `json:"source"`
}

// RouteLineProps are the feature properties for one drawable route line.
// Color follows RouteRef: hex without '#', empty when unpublished. Source
// says whether the geometry is a real ingested path ('shape') or a straight
// polyline through ordered stops ('stops').
type RouteLineProps struct {
	RouteID    string `json:"routeId"`
	ShortName  string `json:"shortName"`
	LongName   string `json:"longName"`
	Mode       string `json:"mode"`
	Color      string `json:"color"`
	AgencyName string `json:"agencyName"`
	Source     string `json:"source"`
}

// RouteLineFeature — geometry passes through verbatim from st_asgeojson.
type RouteLineFeature struct {
	Type       string          `json:"type"` // "Feature"
	Geometry   json.RawMessage `json:"geometry"`
	Properties RouteLineProps  `json:"properties"`
}

// RouteLineCollection is a GeoJSON FeatureCollection for map rendering —
// display geometry only, carrying no schedule or realtime meaning.
type RouteLineCollection struct {
	Type     string             `json:"type"` // "FeatureCollection"
	Features []RouteLineFeature `json:"features"`
}

// ProviderHealth is the registry row plus its ingest freshness signal.
// LastAttemptAt is stamped before each ingest run — when it is newer than
// LastSuccessAt (or success is absent), the last run failed or is running.
type ProviderHealth struct {
	Code             string     `json:"code"`
	Name             string     `json:"name"`
	LicenseName      string     `json:"licenseName,omitempty"`
	AttributionText  string     `json:"attributionText,omitempty"`
	AllowedUse       string     `json:"allowedUse,omitempty"`
	RefreshCadence   string     `json:"refreshCadence,omitempty"`
	Owner            string     `json:"owner,omitempty"`
	KnownLimitations string     `json:"knownLimitations,omitempty"`
	IsActive         bool       `json:"isActive"`
	LastSuccessAt    *time.Time `json:"lastSuccessAt,omitempty"`
	LastAttemptAt    *time.Time `json:"lastAttemptAt,omitempty"`
}

func sourceMeta(provider string, fetchedAt, sourceUpdatedAt pgtype.Timestamptz) SourceMeta {
	m := SourceMeta{Provider: provider}
	if fetchedAt.Valid {
		m.FetchedAt = &fetchedAt.Time
	}
	if sourceUpdatedAt.Valid {
		m.SourceUpdatedAt = &sourceUpdatedAt.Time
	}
	return m
}

func stopRef(id pgtype.UUID, name string, code pgtype.Text, lon, lat float64) StopRef {
	r := StopRef{ID: id.String(), Name: name, Lat: lat, Lon: lon}
	if code.Valid {
		r.Code = code.String
	}
	return r
}
