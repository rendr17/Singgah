package commute

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	generated "singgah/services/api/db/generated"
)

// ProviderCode is the canonical registry code for this source.
const ProviderCode = "commute"

// Rejection records a provider entity that failed validation. Rejected rows
// never reach the database and are reported, not silently dropped.
type Rejection struct {
	EntityID string
	Reason   string
}

// ProviderRegistration is the providers-row payload for this source. Registry
// fields follow docs/35_DATA_SOURCES.md; license is ODbL-1.0 per the provider.
var ProviderRegistration = generated.UpsertProviderParams{
	Code:            ProviderCode,
	Name:            "Commute Data Platform",
	SourceUrl:       pgtype.Text{String: "https://data.commute.shiorilabs.id", Valid: true},
	TermsUrl:        pgtype.Text{String: "https://data.commute.shiorilabs.id/docs", Valid: true},
	LicenseName:     pgtype.Text{String: "ODbL-1.0", Valid: true},
	AttributionText: pgtype.Text{String: "Data transit oleh Commute Data Platform (Shiori Labs), lisensi ODbL-1.0", Valid: true},
	AllowedUse:      pgtype.Text{String: "free to use and process with attribution; share-alike on derived databases", Valid: true},
	RefreshCadence:  pgtype.Text{String: "daily", Valid: true},
	Owner:           pgtype.Text{String: "Shiori Labs", Valid: true},
	KnownLimitations: pgtype.Text{
		String: "static catalog and schedules only — no realtime positions; station coordinates may be null for unsurveyed stops",
		Valid:  true,
	},
}

// modeMap translates the provider's TransitMode (GTFS route_type names) onto
// the canonical set constrained in the routes table.
var modeMap = map[string]string{
	"RAIL":   "rail",
	"SUBWAY": "subway",
	"TRAM":   "tram",
	"BUS":    "bus",
	"FERRY":  "ferry",
	// GTFS 12; Jakarta has no monorail — kept as 'other' rather than widening
	// the CHECK for an unused value.
	"MONORAIL": "other",
}

// NormalizeMode maps an upstream TransitMode to the canonical enum; unknown
// and empty values degrade to "other" instead of failing the batch.
func NormalizeMode(upstream string) string {
	if m, ok := modeMap[strings.ToUpper(strings.TrimSpace(upstream))]; ok {
		return m
	}
	return "other"
}

// NormalizeOperator maps an operator to an agency row. provider_entity_id is
// the operator code — stable within this dataset.
func NormalizeOperator(op Operator, fetchedAt pgtype.Timestamptz) generated.UpsertAgencyParams {
	return generated.UpsertAgencyParams{
		ProviderEntityID: op.Code,
		Code:             pgtype.Text{String: op.Code, Valid: true},
		Name:             op.Name,
		Timezone:         op.Timezone,
		FetchedAt:        fetchedAt,
	}
}

// NormalizeLine maps a provider line to a route row. Entity id is the line
// key "{operator}:{lineCode}" — stable and unique across operators. The line's
// own mode wins; the operator-level mode is the documented fallback.
func NormalizeLine(op Operator, line Line, agencyUUID pgtype.UUID, fetchedAt pgtype.Timestamptz) generated.UpsertRouteParams {
	mode := line.Mode
	if mode == "" {
		mode = op.Mode
	}
	return generated.UpsertRouteParams{
		AgencyID:         agencyUUID,
		ProviderEntityID: op.Code + ":" + line.LineCode,
		ShortName:        pgtype.Text{String: line.LineCode, Valid: true},
		LongName:         pgtype.Text{String: line.Name, Valid: true},
		Mode:             NormalizeMode(mode),
		Color:            pgtype.Text{String: line.ColorCode, Valid: line.ColorCode != ""},
		FetchedAt:        fetchedAt,
	}
}

// NormalizeStation validates and maps a station to a stop row. A station
// without surveyed coordinates is rejected — the catalog never fabricates a
// location. Provider-specific fields (officialName alias, region, score,
// searchable flag, lines, amenities) live in metadata, per the JSONB rule.
func NormalizeStation(st Station, fetchedAt pgtype.Timestamptz) (generated.UpsertStopParams, *Rejection) {
	if st.ID == "" || st.Name == "" {
		return generated.UpsertStopParams{}, &Rejection{EntityID: st.ID, Reason: "missing id or name"}
	}
	if st.Latitude == nil || st.Longitude == nil {
		return generated.UpsertStopParams{}, &Rejection{EntityID: st.ID, Reason: "missing coordinates"}
	}
	lat, lon := *st.Latitude, *st.Longitude
	if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return generated.UpsertStopParams{}, &Rejection{
			EntityID: st.ID,
			Reason:   fmt.Sprintf("coordinates out of range: %v,%v", lon, lat),
		}
	}

	metadata, err := json.Marshal(map[string]any{
		"official_name": st.OfficialName,
		"region_code":   st.RegionCode,
		"operator":      st.Operator,
		"lines":         st.Lines,
		"score":         st.Score,
		"searchable":    st.Searchable,
		"amenities":     st.Amenities,
	})
	if err != nil {
		return generated.UpsertStopParams{}, &Rejection{EntityID: st.ID, Reason: "metadata marshal: " + err.Error()}
	}

	return generated.UpsertStopParams{
		ProviderEntityID: st.ID,
		Kind:             "station",
		Code:             pgtype.Text{String: st.Code, Valid: st.Code != ""},
		Name:             st.Name,
		Wgs84Point:       lon,
		Wgs84Point_2:     lat,
		Metadata:         metadata,
		FetchedAt:        fetchedAt,
	}, nil
}

// NormalizeTransfer resolves an INTERNAL transfer's target StationRef to a
// canonical stop UUID via the provider-entity map built during stop ingest.
// EXTERNAL transfers have no resolvable id and are skipped with a note; a
// target absent from the map means the provider references a station it does
// not list — also skipped, loudly.
func NormalizeTransfer(tr Transfer, stopIDs map[string]pgtype.UUID) (generated.UpsertTransferParams, *Rejection) {
	if tr.DataType != "INTERNAL" {
		return generated.UpsertTransferParams{}, &Rejection{
			EntityID: tr.ID,
			Reason:   "EXTERNAL transfer — target outside dataset, not resolvable",
		}
	}
	var ref StationRef
	if err := json.Unmarshal(tr.ToStation, &ref); err != nil || ref.ID == "" {
		return generated.UpsertTransferParams{}, &Rejection{EntityID: tr.ID, Reason: "malformed toStation"}
	}
	toID, ok := stopIDs[ref.ID]
	if !ok {
		return generated.UpsertTransferParams{}, &Rejection{
			EntityID: tr.ID,
			Reason:   "target station " + ref.ID + " not in catalog",
		}
	}
	params := generated.UpsertTransferParams{
		ToStopID:      toID,
		Accessibility: []byte("{}"),
		FareContext:   []byte("{}"),
	}
	if tr.DistanceM != nil {
		params.WalkDistanceM = pgtype.Int4{Int32: int32(math.Round(*tr.DistanceM)), Valid: true}
	}
	if tr.Notes != "" {
		accessibility, _ := json.Marshal(map[string]string{"notes": tr.Notes})
		params.Accessibility = accessibility
	}
	return params, nil
}
