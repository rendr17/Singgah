package auth

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"singgah/services/api/internal/http/response"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/auth/session", h.createSession)
	r.With(h.svc.RequireUser).Delete("/auth/session", h.revokeSession)
	r.With(h.svc.RequireUser).Delete("/auth/account", h.deleteAccount)
}

type sessionResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
	UserID    string    `json:"userId"`
}

func (h *Handler) createSession(w http.ResponseWriter, r *http.Request) {
	token, userID, expiresAt, err := h.svc.Issue(r.Context())
	if err != nil {
		response.Error(w, r, http.StatusInternalServerError, "INTERNAL", "Gagal membuat sesi")
		return
	}
	response.JSON(w, http.StatusCreated, sessionResponse{
		Token:     token,
		ExpiresAt: expiresAt,
		UserID:    uuidString(userID),
	})
}

func (h *Handler) revokeSession(w http.ResponseWriter, r *http.Request) {
	token, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if err := h.svc.Revoke(r.Context(), token); err != nil {
		response.Error(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Sesi tidak valid atau kedaluwarsa")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) deleteAccount(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserIDFrom(r.Context())
	if err := h.svc.DeleteAccount(r.Context(), userID); err != nil {
		response.Error(w, r, http.StatusInternalServerError, "INTERNAL", "Gagal menghapus akun")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func uuidString(u pgtype.UUID) string {
	b := u.Bytes
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
