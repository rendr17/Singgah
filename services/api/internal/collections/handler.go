package collections

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	generated "singgah/services/api/db/generated"
	"singgah/services/api/internal/http/response"
)

// Store is the persistence seam the handler needs — generated.Queries
// satisfies it.
type Store interface {
	ListCollections(ctx context.Context) ([]generated.ListCollectionsRow, error)
}

type Handler struct {
	store Store
}

func NewHandler(store Store) *Handler {
	return &Handler{store: store}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/collections", h.list)
}

type collectionJSON struct {
	ID          string  `json:"id"`
	Slug        string  `json:"slug"`
	Title       string  `json:"title"`
	Description *string `json:"description,omitempty"`
	Kind        string  `json:"kind"`
	ItemCount   int64   `json:"itemCount"`
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	rows, err := h.store.ListCollections(r.Context())
	if err != nil {
		response.Error(w, r, http.StatusInternalServerError, "INTERNAL", "Gagal memuat koleksi")
		return
	}
	out := make([]collectionJSON, 0, len(rows))
	for _, c := range rows {
		var desc *string
		if c.Description.Valid {
			d := c.Description.String
			desc = &d
		}
		out = append(out, collectionJSON{
			ID:          uuidStr(c.ID),
			Slug:        c.Slug,
			Title:       c.Title,
			Description: desc,
			Kind:        c.Kind,
			ItemCount:   c.ItemCount,
		})
	}
	response.JSON(w, http.StatusOK, struct {
		Collections []collectionJSON `json:"collections"`
	}{Collections: out})
}

func uuidStr(u pgtype.UUID) string {
	b := u.Bytes
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
