package passport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"singgah/services/api/db/generated"
	"singgah/services/api/internal/auth"
	"singgah/services/api/internal/http/response"
)

// journalBodyMaxLen keeps notes short per the UX spec (docs/16).
const journalBodyMaxLen = 2000

// journalListDefault/Max bound the unpaginated list until cursors land.
const (
	journalListDefault = 200
	journalListMax     = 500
)

type journalStore interface {
	CreateJournalEntry(ctx context.Context, arg generated.CreateJournalEntryParams) (generated.JournalEntry, error)
	GetJournalEntry(ctx context.Context, arg generated.GetJournalEntryParams) (generated.JournalEntry, error)
	ListJournalEntries(ctx context.Context, arg generated.ListJournalEntriesParams) ([]generated.JournalEntry, error)
	UpdateJournalEntry(ctx context.Context, arg generated.UpdateJournalEntryParams) (generated.JournalEntry, error)
	DeleteJournalEntry(ctx context.Context, arg generated.DeleteJournalEntryParams) (int64, error)
	GetVisitEventOwner(ctx context.Context, id pgtype.UUID) (pgtype.UUID, error)
}

type journalRequest struct {
	StopID        *string    `json:"stopId"`
	VisitEventID  *string    `json:"visitEventId"`
	Body          string     `json:"body"`
	BaseUpdatedAt *time.Time `json:"baseUpdatedAt"`
}

type journalDTO struct {
	ID           string    `json:"id"`
	StopID       *string   `json:"stopId,omitempty"`
	VisitEventID *string   `json:"visitEventId,omitempty"`
	Body         string    `json:"body"`
	Visibility   string    `json:"visibility"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

func journalOut(e generated.JournalEntry) journalDTO {
	d := journalDTO{
		ID:         uuidStr(e.ID),
		Body:       e.Body,
		Visibility: e.Visibility,
		CreatedAt:  e.CreatedAt.Time,
		UpdatedAt:  e.UpdatedAt.Time,
	}
	if e.StopID.Valid {
		s := uuidStr(e.StopID)
		d.StopID = &s
	}
	if e.VisitEventID.Valid {
		s := uuidStr(e.VisitEventID)
		d.VisitEventID = &s
	}
	return d
}

func (h *Handler) createEntry(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFrom(r.Context())
	var req journalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, http.StatusBadRequest, "INVALID_BODY", "Body bukan JSON valid")
		return
	}
	body := strings.TrimSpace(req.Body)
	if body == "" || len(body) > journalBodyMaxLen {
		response.Error(w, r, http.StatusBadRequest, "INVALID_INPUT", "body wajib 1–2000 karakter")
		return
	}
	var stopID, visitID pgtype.UUID
	if req.StopID != nil {
		if err := stopID.Scan(*req.StopID); err != nil || !stopID.Valid {
			response.Error(w, r, http.StatusBadRequest, "INVALID_INPUT", "stopId wajib UUID valid")
			return
		}
		if _, err := h.store.GetStop(r.Context(), stopID); err != nil {
			response.Error(w, r, http.StatusNotFound, "STOP_NOT_FOUND", "Stasiun/halte tidak ditemukan")
			return
		}
	}
	if req.VisitEventID != nil {
		if err := visitID.Scan(*req.VisitEventID); err != nil || !visitID.Valid {
			response.Error(w, r, http.StatusBadRequest, "INVALID_INPUT", "visitEventId wajib UUID valid")
			return
		}
		// Link only the caller's own visit — foreign ids answer 404 rather
		// than revealing someone else's visit exists.
		owner, err := h.store.GetVisitEventOwner(r.Context(), visitID)
		if err != nil || owner != userID {
			response.Error(w, r, http.StatusNotFound, "VISIT_NOT_FOUND", "Kunjungan tidak ditemukan")
			return
		}
	}
	entry, err := h.store.CreateJournalEntry(r.Context(), generated.CreateJournalEntryParams{
		UserID:       userID,
		StopID:       stopID,
		VisitEventID: visitID,
		Body:         body,
	})
	if err != nil {
		response.Error(w, r, http.StatusInternalServerError, "INTERNAL", "Gagal menyimpan jurnal")
		return
	}
	response.JSON(w, http.StatusCreated, struct {
		Entry journalDTO `json:"entry"`
	}{Entry: journalOut(entry)})
}

func (h *Handler) listEntries(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFrom(r.Context())
	limit := int32(journalListDefault)
	if q := r.URL.Query().Get("limit"); q != "" {
		var n int
		if _, err := fmt.Sscanf(q, "%d", &n); err != nil || n < 1 || n > journalListMax {
			response.Error(w, r, http.StatusBadRequest, "INVALID_INPUT", "limit 1–500")
			return
		}
		limit = int32(n)
	}
	rows, err := h.store.ListJournalEntries(r.Context(), generated.ListJournalEntriesParams{
		UserID: userID, Limit: limit,
	})
	if err != nil {
		response.Error(w, r, http.StatusInternalServerError, "INTERNAL", "Gagal memuat jurnal")
		return
	}
	out := make([]journalDTO, 0, len(rows))
	for _, e := range rows {
		out = append(out, journalOut(e))
	}
	response.JSON(w, http.StatusOK, struct {
		Entries []journalDTO `json:"entries"`
	}{Entries: out})
}

func (h *Handler) updateEntry(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFrom(r.Context())
	var id pgtype.UUID
	if err := id.Scan(chi.URLParam(r, "entryID")); err != nil || !id.Valid {
		response.Error(w, r, http.StatusBadRequest, "INVALID_INPUT", "entryID wajib UUID valid")
		return
	}
	var req journalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, http.StatusBadRequest, "INVALID_BODY", "Body bukan JSON valid")
		return
	}
	body := strings.TrimSpace(req.Body)
	if body == "" || len(body) > journalBodyMaxLen {
		response.Error(w, r, http.StatusBadRequest, "INVALID_INPUT", "body wajib 1–2000 karakter")
		return
	}
	existing, err := h.store.GetJournalEntry(r.Context(), generated.GetJournalEntryParams{ID: id, UserID: userID})
	if err != nil {
		code := http.StatusInternalServerError
		if errors.Is(err, pgx.ErrNoRows) {
			code = http.StatusNotFound
		}
		response.Error(w, r, code, "ENTRY_NOT_FOUND", "Jurnal tidak ditemukan")
		return
	}
	// Optimistic version check (docs/24): a baseUpdatedAt that doesn't match
	// means someone edited after the client's copy — say so, don't overwrite.
	if req.BaseUpdatedAt != nil && !existing.UpdatedAt.Time.Equal(*req.BaseUpdatedAt) {
		response.Error(w, r, http.StatusConflict, "VERSION_CONFLICT", "Entri sudah berubah — muat ulang dulu")
		return
	}
	entry, err := h.store.UpdateJournalEntry(r.Context(), generated.UpdateJournalEntryParams{
		ID: id, UserID: userID, Body: body,
	})
	if err != nil {
		response.Error(w, r, http.StatusInternalServerError, "INTERNAL", "Gagal memperbarui jurnal")
		return
	}
	response.JSON(w, http.StatusOK, struct {
		Entry journalDTO `json:"entry"`
	}{Entry: journalOut(entry)})
}

func (h *Handler) deleteEntry(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFrom(r.Context())
	var id pgtype.UUID
	if err := id.Scan(chi.URLParam(r, "entryID")); err != nil || !id.Valid {
		response.Error(w, r, http.StatusBadRequest, "INVALID_INPUT", "entryID wajib UUID valid")
		return
	}
	n, err := h.store.DeleteJournalEntry(r.Context(), generated.DeleteJournalEntryParams{ID: id, UserID: userID})
	if err != nil {
		response.Error(w, r, http.StatusInternalServerError, "INTERNAL", "Gagal menghapus jurnal")
		return
	}
	if n == 0 {
		response.Error(w, r, http.StatusNotFound, "ENTRY_NOT_FOUND", "Jurnal tidak ditemukan")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
