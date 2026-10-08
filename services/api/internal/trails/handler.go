package trails

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	generated "singgah/services/api/db/generated"
	"singgah/services/api/internal/http/response"
)

const (
	minTrailStops       = 2
	maxTrailStops       = 6
	walkMetersPerMinute = 80.0
	walkDetourFactor    = 1.3
)

var trailSlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type Store interface {
	ListTrails(context.Context) ([]generated.ListTrailsRow, error)
	GetTrail(context.Context, string) (generated.GetTrailRow, error)
	ListTrailStops(context.Context, string) ([]generated.ListTrailStopsRow, error)
}

type Handler struct {
	store Store
}

func NewHandler(store Store) *Handler {
	return &Handler{store: store}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/trails", h.list)
	r.Get("/trails/{slug}", h.detail)
}

type transitJSON struct {
	ID   string  `json:"id"`
	Name string  `json:"name"`
	Lat  float64 `json:"lat"`
	Lon  float64 `json:"lon"`
}

type sourceJSON struct {
	Provider        string  `json:"provider"`
	Name            string  `json:"name"`
	SourceURL       *string `json:"sourceUrl,omitempty"`
	TermsURL        *string `json:"termsUrl,omitempty"`
	LicenseName     *string `json:"licenseName,omitempty"`
	Attribution     *string `json:"attribution,omitempty"`
	SourceUpdatedAt *string `json:"sourceUpdatedAt,omitempty"`
}

type placeJSON struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	Category       string          `json:"category"`
	PriceBand      *int32          `json:"priceBand,omitempty"`
	Curated        bool            `json:"curated"`
	Lat            float64         `json:"lat"`
	Lon            float64         `json:"lon"`
	Accessibility  json.RawMessage `json:"accessibility,omitempty"`
	OperatingHours string          `json:"operatingHoursStatus"`
	Source         sourceJSON      `json:"source"`
}

type trailStopJSON struct {
	Sequence                  int16     `json:"sequence"`
	StayMinutes               int16     `json:"stayMinutes"`
	Notes                     string    `json:"notes,omitempty"`
	Place                     placeJSON `json:"place"`
	WalkDistanceFromPreviousM int32     `json:"walkDistanceFromPreviousM"`
}

type trailJSON struct {
	Slug                     string          `json:"slug"`
	Title                    string          `json:"title"`
	Description              string          `json:"description"`
	Theme                    string          `json:"theme"`
	StartTransit             transitJSON     `json:"startTransit"`
	EndTransit               transitJSON     `json:"endTransit"`
	ExpectedStopCount        int64           `json:"expectedStopCount"`
	AvailableStopCount       int64           `json:"availableStopCount"`
	CanStart                 bool            `json:"canStart"`
	BudgetMinIDR             *int32          `json:"budgetMinIDR,omitempty"`
	BudgetMaxIDR             *int32          `json:"budgetMaxIDR,omitempty"`
	WalkDistanceM            *int32          `json:"walkDistanceM,omitempty"`
	EstimatedDurationMinutes *int32          `json:"estimatedDurationMinutes,omitempty"`
	WalkingEstimateBasis     string          `json:"walkingEstimateBasis,omitempty"`
	DurationIncludes         string          `json:"durationIncludes,omitempty"`
	Stops                    []trailStopJSON `json:"stops,omitempty"`
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	rows, err := h.store.ListTrails(r.Context())
	if err != nil {
		response.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not load trails")
		return
	}

	trails := make([]trailJSON, 0, len(rows))
	for _, row := range rows {
		trails = append(trails, trailJSON{
			Slug: row.Slug, Title: row.Title, Description: row.Description, Theme: row.Theme,
			StartTransit:      transitJSON{ID: uuidString(row.StartStopID), Name: row.StartStopName, Lat: row.StartLat, Lon: row.StartLon},
			EndTransit:        transitJSON{ID: uuidString(row.EndStopID), Name: row.EndStopName, Lat: row.EndLat, Lon: row.EndLon},
			ExpectedStopCount: row.ExpectedStopCount, AvailableStopCount: row.AvailableStopCount,
			CanStart:     trailAvailable(row.ExpectedStopCount, row.AvailableStopCount, row.EndAccessCount > 0),
			BudgetMinIDR: optionalInt4(row.BudgetMinIdr), BudgetMaxIDR: optionalInt4(row.BudgetMaxIdr),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"trails": trails})
}

func (h *Handler) detail(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if len(slug) > 80 || !trailSlugPattern.MatchString(slug) {
		response.Error(w, r, http.StatusBadRequest, "INVALID_TRAIL_SLUG", "Trail slug is invalid")
		return
	}

	row, err := h.store.GetTrail(r.Context(), slug)
	if errors.Is(err, pgx.ErrNoRows) {
		response.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Trail not found")
		return
	}
	if err != nil {
		response.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not load trail")
		return
	}
	stops, err := h.store.ListTrailStops(r.Context(), slug)
	if err != nil {
		response.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not load trail stops")
		return
	}
	sort.Slice(stops, func(i, j int) bool { return stops[i].Sequence < stops[j].Sequence })

	canStart := trailAvailable(row.ExpectedStopCount, row.AvailableStopCount, row.EndAccessCount > 0) && int64(len(stops)) == row.ExpectedStopCount && completeSequence(stops) && completeWalkingDistances(stops)
	trail := trailJSON{
		Slug: row.Slug, Title: row.Title, Description: row.Description, Theme: row.Theme,
		StartTransit:      transitJSON{ID: uuidString(row.StartStopID), Name: row.StartStopName, Lat: row.StartLat, Lon: row.StartLon},
		EndTransit:        transitJSON{ID: uuidString(row.EndStopID), Name: row.EndStopName, Lat: row.EndLat, Lon: row.EndLon},
		ExpectedStopCount: row.ExpectedStopCount, AvailableStopCount: row.AvailableStopCount,
		CanStart: canStart, BudgetMinIDR: optionalInt4(row.BudgetMinIdr), BudgetMaxIDR: optionalInt4(row.BudgetMaxIdr),
	}
	if canStart {
		trail.Stops = make([]trailStopJSON, 0, len(stops))
		var distance int64
		var stayMinutes int64
		for _, stop := range stops {
			distance += int64(stop.WalkDistanceFromPreviousM.Int32)
			stayMinutes += int64(stop.StayMinutes)
			trail.Stops = append(trail.Stops, trailStopJSON{
				Sequence: stop.Sequence, StayMinutes: stop.StayMinutes, Notes: stop.Notes,
				Place: placeJSON{
					ID: uuidString(stop.PlaceID), Name: stop.Name, Category: stop.PrimaryCategory,
					PriceBand: optionalInt2(stop.PriceBand), Curated: stop.EditorialStatus == "curated",
					Lat: stop.Lat, Lon: stop.Lon, Accessibility: json.RawMessage(stop.Accessibility),
					OperatingHours: "not-provided",
					Source: sourceJSON{
						Provider: stop.SourceCode, Name: stop.SourceName,
						SourceURL: optionalText(stop.SourceUrl), TermsURL: optionalText(stop.TermsUrl),
						LicenseName: optionalText(stop.LicenseName), Attribution: optionalText(stop.AttributionText),
						SourceUpdatedAt: optionalTimestamp(stop.SourceUpdatedAt),
					},
				},
				WalkDistanceFromPreviousM: stop.WalkDistanceFromPreviousM.Int32,
			})
		}

		last := stops[len(stops)-1]
		distance += int64(last.EndWalkDistanceM.Int32)
		walkingMinutes := int64(math.Ceil(float64(distance) * walkDetourFactor / walkMetersPerMinute))
		duration := int32(walkingMinutes + stayMinutes)
		distanceValue := int32(distance)
		trail.WalkDistanceM = &distanceValue
		trail.EstimatedDurationMinutes = &duration
		trail.WalkingEstimateBasis = "straight-line distance × 1.3 detour factor at 80 m/min; not a pedestrian route"
		trail.DurationIncludes = "estimated walking plus planned stop stays; excludes transit time and opening hours"
	}
	writeJSON(w, http.StatusOK, trail)
}

func trailAvailable(expected, available int64, hasEndAccess bool) bool {
	return hasEndAccess && expected >= minTrailStops && expected <= maxTrailStops && available == expected
}

func completeSequence(stops []generated.ListTrailStopsRow) bool {
	for i, stop := range stops {
		if int64(stop.Sequence) != int64(i+1) {
			return false
		}
	}
	return true
}

func completeWalkingDistances(stops []generated.ListTrailStopsRow) bool {
	if len(stops) == 0 {
		return false
	}
	for _, stop := range stops {
		if !stop.WalkDistanceFromPreviousM.Valid || stop.WalkDistanceFromPreviousM.Int32 < 0 {
			return false
		}
	}
	last := stops[len(stops)-1]
	return last.EndWalkDistanceM.Valid && last.EndWalkDistanceM.Int32 >= 0
}

func uuidString(id pgtype.UUID) string {
	if !id.Valid {
		return ""
	}
	return id.String()
}

func optionalInt4(value pgtype.Int4) *int32 {
	if !value.Valid {
		return nil
	}
	return &value.Int32
}

func optionalInt2(value pgtype.Int2) *int32 {
	if !value.Valid {
		return nil
	}
	v := int32(value.Int16)
	return &v
}

func optionalText(value pgtype.Text) *string {
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return nil
	}
	return &value.String
}

func optionalTimestamp(value pgtype.Timestamptz) *string {
	if !value.Valid {
		return nil
	}
	formatted := value.Time.UTC().Format("2006-01-02T15:04:05.999999999Z07:00")
	return &formatted
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
