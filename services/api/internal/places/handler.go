// Package places serves the City Explorer place catalog (ADR-011):
// OSM-sourced, ingest-time imported, ranked by transit usefulness.
package places

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	generated "singgah/services/api/db/generated"
	"singgah/services/api/internal/http/response"
)

// Store is the persistence seam the handler needs — generated.Queries
// satisfies it.
type Store interface {
	GetStop(ctx context.Context, id pgtype.UUID) (generated.GetStopRow, error)
	ListPlacesNearStop(ctx context.Context, arg generated.ListPlacesNearStopParams) ([]generated.ListPlacesNearStopRow, error)
}

type Handler struct {
	store Store
}

func NewHandler(store Store) *Handler {
	return &Handler{store: store}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/places", h.nearStop)
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

func uuidStr(u pgtype.UUID) string {
	b := u.Bytes
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
