package catalog

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
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
