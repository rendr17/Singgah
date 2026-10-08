// Package places serves the City Explorer place catalog (ADR-011):
// OSM-sourced, ingest-time imported, ranked by transit usefulness.
package places

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	generated "singgah/services/api/db/generated"
	"singgah/services/api/internal/auth"
	"singgah/services/api/internal/http/middleware"
	"singgah/services/api/internal/http/response"
)

// Store is the persistence seam the handler needs — generated.Queries
// satisfies it.
type Store interface {
	GetStop(ctx context.Context, id pgtype.UUID) (generated.GetStopRow, error)
	ListPlacesNearStop(ctx context.Context, arg generated.ListPlacesNearStopParams) ([]generated.ListPlacesNearStopRow, error)
	GetPlaceDetail(ctx context.Context, id pgtype.UUID) (generated.GetPlaceDetailRow, error)
	ListPlaceTransitAccesses(ctx context.Context, id pgtype.UUID) ([]generated.ListPlaceTransitAccessesRow, error)
	SavePlace(ctx context.Context, arg generated.SavePlaceParams) (int64, error)
	UnsavePlace(ctx context.Context, arg generated.UnsavePlaceParams) (int64, error)
	GetPlacePersonalState(ctx context.Context, arg generated.GetPlacePersonalStateParams) (generated.GetPlacePersonalStateRow, error)
	ListSavedPlaces(ctx context.Context, arg generated.ListSavedPlacesParams) ([]generated.ListSavedPlacesRow, error)
	RecordPlaceVisit(ctx context.Context, arg generated.RecordPlaceVisitParams) (generated.RecordPlaceVisitRow, error)
}

type Handler struct {
	store       Store
	requireUser func(http.Handler) http.Handler
}

func NewHandler(store Store, requireUser ...func(http.Handler) http.Handler) *Handler {
	h := &Handler{store: store}
	if len(requireUser) > 0 {
		h.requireUser = requireUser[0]
	}
	return h
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/places", h.nearStop)
	protect := h.requireUser
	if protect == nil {
		protect = func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				response.Error(w, r, http.StatusServiceUnavailable, "UNAVAILABLE", "Autentikasi belum dikonfigurasi")
			})
		}
	}
	r.With(protect).Get("/places/saved", h.savedPlaces)
	r.Get("/places/{id}", h.detail)
	r.With(protect).Get("/places/{id}/personal", h.personalState)
	r.With(middleware.RateLimit(20, time.Minute), protect).Put("/places/{id}/saved", h.savePlace)
	r.With(middleware.RateLimit(20, time.Minute), protect).Delete("/places/{id}/saved", h.unsavePlace)
	r.With(middleware.RateLimit(20, time.Minute), protect).Post("/places/{id}/visits", h.recordVisit)
}

// Page bounds — one station's neighbourhood rarely needs more than a few
// dozen candidates; 50 keeps responses mobile-friendly.
const (
	placesDefaultLimit = 20
	placesMaxLimit     = 50
)

type placeJSON struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Category      string  `json:"category"`
	Lat           float64 `json:"lat"`
	Lon           float64 `json:"lon"`
	WalkDistanceM int32   `json:"walkDistanceM"`
	// WalkSeconds is a straight-line × detour estimate (ADR-011), never a
	// routed path — geometry is NULL for estimates so the API says so.
	WalkSeconds *int32 `json:"walkSeconds"`
	PriceBand   *int16 `json:"priceBand,omitempty"`
	Curated     bool   `json:"curated"`
}

type placeSourceJSON struct {
	Code            string  `json:"code"`
	Name            string  `json:"name"`
	URL             *string `json:"url,omitempty"`
	TermsURL        *string `json:"termsUrl,omitempty"`
	LicenseName     *string `json:"licenseName,omitempty"`
	AttributionText *string `json:"attributionText,omitempty"`
}

type transitAccessJSON struct {
	StopID        string    `json:"stopId"`
	StopName      string    `json:"stopName"`
	StopKind      string    `json:"stopKind"`
	WalkDistanceM int32     `json:"walkDistanceM"`
	WalkSeconds   *int32    `json:"walkSeconds"`
	ComputedAt    time.Time `json:"computedAt"`
}

type placeDetailJSON struct {
	ID              string              `json:"id"`
	Name            string              `json:"name"`
	Category        string              `json:"category"`
	Lat             float64             `json:"lat"`
	Lon             float64             `json:"lon"`
	PriceBand       *int16              `json:"priceBand,omitempty"`
	Curated         bool                `json:"curated"`
	Accessibility   json.RawMessage     `json:"accessibility"`
	SourceUpdatedAt *time.Time          `json:"sourceUpdatedAt,omitempty"`
	Source          placeSourceJSON     `json:"source"`
	TransitAccess   []transitAccessJSON `json:"transitAccess"`
}

type savedPlaceJSON struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Category        string          `json:"category"`
	Lat             float64         `json:"lat"`
	Lon             float64         `json:"lon"`
	PriceBand       *int16          `json:"priceBand,omitempty"`
	WalkDistanceM   int32           `json:"walkDistanceM"`
	WalkSeconds     *int32          `json:"walkSeconds"`
	SavedAt         time.Time       `json:"savedAt"`
	VisitedAt       *time.Time      `json:"visitedAt"`
	Curated         bool            `json:"curated"`
	TransitStopID   string          `json:"transitStopId"`
	TransitStopName string          `json:"transitStopName"`
	Source          placeSourceJSON `json:"source"`
}

type personalStateJSON struct {
	Saved     bool       `json:"saved"`
	VisitedAt *time.Time `json:"visitedAt"`
}

type placeVisitRequest struct {
	ClientMutationID string     `json:"clientMutationId"`
	ObservedAt       *time.Time `json:"observedAt"`
}

type placeVisitJSON struct {
	ID               string    `json:"id"`
	PlaceID          string    `json:"placeId"`
	ObservedAt       time.Time `json:"observedAt"`
	ValidationMethod string    `json:"validationMethod"`
	CreatedAt        time.Time `json:"createdAt"`
}

const observedAtFutureSlack = 15 * time.Minute

func (h *Handler) nearStop(w http.ResponseWriter, r *http.Request) {
	var stopID pgtype.UUID
	if raw := r.URL.Query().Get("near_stop_id"); raw == "" {
		response.Error(w, r, http.StatusBadRequest, "INVALID_INPUT", "near_stop_id wajib diisi")
		return
	} else if err := stopID.Scan(raw); err != nil || !stopID.Valid {
		response.Error(w, r, http.StatusBadRequest, "INVALID_INPUT", "near_stop_id wajib UUID valid")
		return
	}

	limit := placesDefaultLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			limit = min(n, placesMaxLimit)
		}
	}

	if _, err := h.store.GetStop(r.Context(), stopID); err != nil {
		response.Error(w, r, http.StatusNotFound, "STOP_NOT_FOUND", "Stasiun/halte tidak ditemukan")
		return
	}

	rows, err := h.store.ListPlacesNearStop(r.Context(), generated.ListPlacesNearStopParams{
		StopID: stopID,
		Limit:  int32(limit),
	})
	if err != nil {
		response.Error(w, r, http.StatusInternalServerError, "INTERNAL", "Gagal memuat tempat sekitar")
		return
	}
	out := make([]placeJSON, 0, len(rows))
	for _, p := range rows {
		var walkSec *int32
		if p.WalkSeconds.Valid {
			v := p.WalkSeconds.Int32
			walkSec = &v
		}
		var price *int16
		if p.PriceBand.Valid {
			v := p.PriceBand.Int16
			price = &v
		}
		out = append(out, placeJSON{
			ID:            uuidStr(p.ID),
			Name:          p.Name,
			Category:      p.PrimaryCategory,
			Lat:           p.Lat,
			Lon:           p.Lon,
			WalkDistanceM: p.WalkDistanceM,
			WalkSeconds:   walkSec,
			PriceBand:     price,
			Curated:       p.EditorialStatus == "curated",
		})
	}
	response.JSON(w, http.StatusOK, struct {
		Places []placeJSON `json:"places"`
	}{Places: out})
}

func (h *Handler) detail(w http.ResponseWriter, r *http.Request) {
	var id pgtype.UUID
	if err := id.Scan(chi.URLParam(r, "id")); err != nil || !id.Valid {
		response.Error(w, r, http.StatusBadRequest, "INVALID_INPUT", "place id wajib UUID valid")
		return
	}
	row, err := h.store.GetPlaceDetail(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		response.Error(w, r, http.StatusNotFound, "PLACE_NOT_FOUND", "Tempat tidak ditemukan")
		return
	}
	if err != nil {
		response.Error(w, r, http.StatusInternalServerError, "INTERNAL", "Gagal memuat detail tempat")
		return
	}
	if !json.Valid(row.Accessibility) {
		response.Error(w, r, http.StatusInternalServerError, "INTERNAL", "Metadata aksesibilitas tempat tidak valid")
		return
	}
	accesses, err := h.store.ListPlaceTransitAccesses(r.Context(), id)
	if err != nil {
		response.Error(w, r, http.StatusInternalServerError, "INTERNAL", "Gagal memuat akses transit tempat")
		return
	}
	var price *int16
	if row.PriceBand.Valid {
		v := row.PriceBand.Int16
		price = &v
	}
	var sourceUpdatedAt *time.Time
	if row.SourceUpdatedAt.Valid {
		v := row.SourceUpdatedAt.Time
		sourceUpdatedAt = &v
	}
	out := placeDetailJSON{
		ID: uuidStr(row.ID), Name: row.Name, Category: row.PrimaryCategory,
		Lat: row.Lat, Lon: row.Lon, PriceBand: price,
		Curated: row.EditorialStatus == "curated", Accessibility: json.RawMessage(row.Accessibility),
		SourceUpdatedAt: sourceUpdatedAt,
		Source: placeSourceJSON{
			Code: row.SourceCode, Name: row.SourceName,
			URL: textPtr(row.SourceUrl), TermsURL: textPtr(row.TermsUrl),
			LicenseName: textPtr(row.LicenseName), AttributionText: textPtr(row.AttributionText),
		},
		TransitAccess: make([]transitAccessJSON, 0, len(accesses)),
	}
	for _, access := range accesses {
		var walkSeconds *int32
		if access.WalkSeconds.Valid {
			v := access.WalkSeconds.Int32
			walkSeconds = &v
		}
		out.TransitAccess = append(out.TransitAccess, transitAccessJSON{
			StopID: uuidStr(access.StopID), StopName: access.StopName, StopKind: access.StopKind,
			WalkDistanceM: access.WalkDistanceM, WalkSeconds: walkSeconds,
			ComputedAt: access.ComputedAt.Time,
		})
	}
	response.JSON(w, http.StatusOK, struct {
		Place placeDetailJSON `json:"place"`
	}{Place: out})
}

func (h *Handler) savedPlaces(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFrom(r.Context())
	if !ok {
		response.Error(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Token sesi diperlukan")
		return
	}
	limit := placesDefaultLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			response.Error(w, r, http.StatusBadRequest, "INVALID_INPUT", "limit harus bilangan positif")
			return
		}
		limit = min(n, placesMaxLimit)
	}
	rows, err := h.store.ListSavedPlaces(r.Context(), generated.ListSavedPlacesParams{UserID: userID, Limit: int32(limit)})
	if err != nil {
		response.Error(w, r, http.StatusInternalServerError, "INTERNAL", "Gagal memuat tempat tersimpan")
		return
	}
	places := make([]savedPlaceJSON, 0, len(rows))
	for _, row := range rows {
		var walkSeconds *int32
		if row.WalkSeconds.Valid {
			v := row.WalkSeconds.Int32
			walkSeconds = &v
		}
		var visitedAt *time.Time
		if row.VisitedAt.Valid {
			v := row.VisitedAt.Time
			visitedAt = &v
		}
		var price *int16
		if row.PriceBand.Valid {
			v := row.PriceBand.Int16
			price = &v
		}
		places = append(places, savedPlaceJSON{
			ID: uuidStr(row.ID), Name: row.Name, Category: row.PrimaryCategory, PriceBand: price,
			Lat: row.Lat, Lon: row.Lon, WalkDistanceM: row.WalkDistanceM,
			WalkSeconds: walkSeconds, SavedAt: row.SavedAt.Time, VisitedAt: visitedAt,
			Curated: row.EditorialStatus == "curated", TransitStopID: uuidStr(row.StopID),
			TransitStopName: row.StopName,
			Source: placeSourceJSON{Code: row.SourceCode, Name: row.SourceName,
				URL: textPtr(row.SourceUrl), TermsURL: textPtr(row.TermsUrl),
				LicenseName: textPtr(row.LicenseName), AttributionText: textPtr(row.AttributionText)},
		})
	}
	response.JSON(w, http.StatusOK, struct {
		Places []savedPlaceJSON `json:"places"`
	}{Places: places})
}

func (h *Handler) personalState(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFrom(r.Context())
	if !ok {
		response.Error(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Token sesi diperlukan")
		return
	}
	placeID, ok := h.placeID(w, r)
	if !ok {
		return
	}
	if _, err := h.store.GetPlaceDetail(r.Context(), placeID); err != nil {
		h.placeLookupError(w, r, err)
		return
	}
	state, err := h.store.GetPlacePersonalState(r.Context(), generated.GetPlacePersonalStateParams{
		ID: userID, PlaceID: placeID,
	})
	if err != nil {
		response.Error(w, r, http.StatusInternalServerError, "INTERNAL", "Gagal memuat status pribadi tempat")
		return
	}
	out := personalStateJSON{Saved: state.Saved}
	if state.VisitedAt.Valid {
		v := state.VisitedAt.Time
		out.VisitedAt = &v
	}
	response.JSON(w, http.StatusOK, struct {
		Personal personalStateJSON `json:"personal"`
	}{Personal: out})
}

func (h *Handler) savePlace(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFrom(r.Context())
	if !ok {
		response.Error(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Token sesi diperlukan")
		return
	}
	placeID, ok := h.placeID(w, r)
	if !ok {
		return
	}
	if _, err := h.store.GetPlaceDetail(r.Context(), placeID); err != nil {
		h.placeLookupError(w, r, err)
		return
	}
	if _, err := h.store.SavePlace(r.Context(), generated.SavePlaceParams{UserID: userID, PlaceID: placeID}); err != nil {
		response.Error(w, r, http.StatusInternalServerError, "INTERNAL", "Gagal menyimpan tempat")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) unsavePlace(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFrom(r.Context())
	if !ok {
		response.Error(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Token sesi diperlukan")
		return
	}
	placeID, ok := h.placeID(w, r)
	if !ok {
		return
	}
	if _, err := h.store.UnsavePlace(r.Context(), generated.UnsavePlaceParams{UserID: userID, PlaceID: placeID}); err != nil {
		response.Error(w, r, http.StatusInternalServerError, "INTERNAL", "Gagal menghapus tempat tersimpan")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) recordVisit(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFrom(r.Context())
	if !ok {
		response.Error(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Token sesi diperlukan")
		return
	}
	placeID, ok := h.placeID(w, r)
	if !ok {
		return
	}
	var req placeVisitRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		response.Error(w, r, http.StatusBadRequest, "INVALID_BODY", "Body bukan JSON valid")
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		response.Error(w, r, http.StatusBadRequest, "INVALID_BODY", "Body harus berisi satu objek JSON")
		return
	}
	var mutationID pgtype.UUID
	if err := mutationID.Scan(req.ClientMutationID); err != nil || !mutationID.Valid {
		response.Error(w, r, http.StatusBadRequest, "INVALID_INPUT", "clientMutationId wajib UUID valid")
		return
	}
	if req.ObservedAt == nil || req.ObservedAt.IsZero() {
		response.Error(w, r, http.StatusBadRequest, "INVALID_INPUT", "observedAt wajib diisi")
		return
	}
	if time.Until(*req.ObservedAt) > observedAtFutureSlack {
		response.Error(w, r, http.StatusBadRequest, "INVALID_INPUT", "observedAt tidak boleh jauh di masa depan")
		return
	}
	if _, err := h.store.GetPlaceDetail(r.Context(), placeID); err != nil {
		h.placeLookupError(w, r, err)
		return
	}
	row, err := h.store.RecordPlaceVisit(r.Context(), generated.RecordPlaceVisitParams{
		UserID: userID, PlaceID: placeID, ClientMutationID: mutationID,
		ObservedAt: pgtype.Timestamptz{Time: *req.ObservedAt, Valid: true},
	})
	if err != nil {
		response.Error(w, r, http.StatusInternalServerError, "INTERNAL", "Gagal merekam kunjungan tempat")
		return
	}
	if !row.WasInserted && (row.PlaceID != placeID || !row.ObservedAt.Time.Equal(*req.ObservedAt)) {
		response.Error(w, r, http.StatusConflict, "IDEMPOTENCY_CONFLICT", "clientMutationId sudah dipakai untuk kunjungan lain")
		return
	}
	status := http.StatusCreated
	if !row.WasInserted {
		status = http.StatusOK
	}
	response.JSON(w, status, struct {
		Visit    placeVisitJSON `json:"visit"`
		Replayed bool           `json:"replayed"`
	}{Visit: placeVisitJSON{
		ID: uuidStr(row.ID), PlaceID: uuidStr(row.PlaceID), ObservedAt: row.ObservedAt.Time,
		ValidationMethod: row.ValidationMethod, CreatedAt: row.CreatedAt.Time,
	}, Replayed: !row.WasInserted})
}

func (h *Handler) placeID(w http.ResponseWriter, r *http.Request) (pgtype.UUID, bool) {
	var id pgtype.UUID
	if err := id.Scan(chi.URLParam(r, "id")); err != nil || !id.Valid {
		response.Error(w, r, http.StatusBadRequest, "INVALID_INPUT", "place id wajib UUID valid")
		return pgtype.UUID{}, false
	}
	return id, true
}

func (h *Handler) placeLookupError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, pgx.ErrNoRows) {
		response.Error(w, r, http.StatusNotFound, "PLACE_NOT_FOUND", "Tempat tidak ditemukan")
		return
	}
	response.Error(w, r, http.StatusInternalServerError, "INTERNAL", "Gagal memuat tempat")
}

func textPtr(v pgtype.Text) *string {
	if !v.Valid {
		return nil
	}
	value := v.String
	return &value
}

func uuidStr(u pgtype.UUID) string {
	b := u.Bytes
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
