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

type StationDetail struct {
	StationSummary
	OfficialName string        `json:"officialName,omitempty"`
	Lines        []RouteRef    `json:"lines"`
	Transfers    []TransferDTO `json:"transfers"`
	Source       SourceMeta    `json:"source"`
}

type RouteDetail struct {
	RouteSummary
	Stops  []StopRef  `json:"stops"`
	Source SourceMeta `json:"source"`
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
