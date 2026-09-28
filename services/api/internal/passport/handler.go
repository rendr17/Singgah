// Package passport owns user-owned transit progress: check-ins (visit
// events), progress views, and journal entries (docs/16, 42, ADR-010).
package passport

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"singgah/services/api/db/generated"
	"singgah/services/api/internal/auth"
	"singgah/services/api/internal/http/middleware"
	"singgah/services/api/internal/http/response"
)

// geofenceRadiusM is the honest check-in threshold: a device fix inside it
// marks the visit confirmed; outside — or absent — records low_confidence
// instead of being rejected (docs/42: mark, don't block).
const geofenceRadiusM = 200

// observedAtFutureSlack tolerates small client clock skew on observed_at.
const observedAtFutureSlack = 15 * time.Minute

// observedAtMin rejects garbage timestamps: a Singgah visit cannot predate
// the product, and anything earlier is client junk, not a real trip.
var observedAtMin = time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)

type Store interface {
	GetStop(ctx context.Context, id pgtype.UUID) (generated.GetStopRow, error)
	StopDistanceM(ctx context.Context, arg generated.StopDistanceMParams) (float64, error)
	RecordVisitEvent(ctx context.Context, arg generated.RecordVisitEventParams) (generated.RecordVisitEventRow, error)
	ListVisitEvents(ctx context.Context, arg generated.ListVisitEventsParams) ([]generated.VisitEvent, error)
	PassportProgressTotal(ctx context.Context, userID pgtype.UUID) (generated.PassportProgressTotalRow, error)
	PassportProgressByMode(ctx context.Context, userID pgtype.UUID) ([]generated.PassportProgressByModeRow, error)
	PassportProgressByRoute(ctx context.Context, userID pgtype.UUID) ([]generated.PassportProgressByRouteRow, error)
	PassportProgressByCollection(ctx context.Context, userID pgtype.UUID) ([]generated.PassportProgressByCollectionRow, error)
	journalStore
}

type Handler struct {
	store Store
	auth  *auth.Service
}

func NewHandler(store Store, authSvc *auth.Service) *Handler {
	return &Handler{store: store, auth: authSvc}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.With(middleware.RateLimit(20, time.Minute), h.auth.RequireUser).
		Post("/visits", h.createVisit)
	r.With(h.auth.RequireUser).Get("/visits", h.listVisits)
	r.With(h.auth.RequireUser).Get("/passport/progress", h.progress)
	r.With(middleware.RateLimit(20, time.Minute), h.auth.RequireUser).
		Post("/journal", h.createEntry)
	r.With(h.auth.RequireUser).Get("/journal", h.listEntries)
	r.With(h.auth.RequireUser).Patch("/journal/{entryID}", h.updateEntry)
	r.With(h.auth.RequireUser).Delete("/journal/{entryID}", h.deleteEntry)
}

type visitRequest struct {
	StopID           string     `json:"stopId"`
	ClientMutationID string     `json:"clientMutationId"`
	ObservedAt       *time.Time `json:"observedAt"`
	Lat              *float64   `json:"lat"`
	Lon              *float64   `json:"lon"`
}

type visitDTO struct {
	ID               string    `json:"id"`
	StopID           string    `json:"stopId"`
	ObservedAt       time.Time `json:"observedAt"`
	ValidationMethod string    `json:"validationMethod"`
	Status           string    `json:"status"`
	DistanceM        *float64  `json:"distanceM,omitempty"`
	CreatedAt        time.Time `json:"createdAt"`
}

func (h *Handler) createVisit(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFrom(r.Context())
	var req visitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, http.StatusBadRequest, "INVALID_BODY", "Body bukan JSON valid")
		return
	}
	params, status := h.evaluate(req, w, r)
	if params == nil {
		return
	}
	row, err := h.store.RecordVisitEvent(r.Context(), generated.RecordVisitEventParams{
		UserID:           userID,
		StopID:           params.stopID,
		ClientMutationID: params.mutationID,
		ObservedAt:       pgtype.Timestamptz{Time: *req.ObservedAt, Valid: true},
		ValidationMethod: params.method,
		DistanceM:        params.distanceM,
		Status:           status,
	})
	if err != nil {
		response.Error(w, r, http.StatusInternalServerError, "INTERNAL", "Gagal merekam kunjungan")
		return
	}
	if !row.WasInserted &&
		(row.StopID != params.stopID || !row.ObservedAt.Time.Equal(*req.ObservedAt)) {
		// The idempotency key was reused for a DIFFERENT visit — report the
		// conflict honestly instead of masquerading as a successful replay.
		response.Error(w, r, http.StatusConflict, "IDEMPOTENCY_CONFLICT",
			"clientMutationId sudah dipakai untuk kunjungan lain")
		return
	}
	out := visitDTO{
		ID:               uuidStr(row.ID),
		StopID:           uuidStr(row.StopID),
		ObservedAt:       row.ObservedAt.Time,
		ValidationMethod: row.ValidationMethod,
		Status:           row.Status,
		CreatedAt:        row.CreatedAt.Time,
	}
	if row.DistanceM.Valid {
		if f, err := row.DistanceM.Float64Value(); err == nil && f.Valid {
			out.DistanceM = &f.Float64
		}
	}
	code := http.StatusCreated
	if !row.WasInserted {
		// Replay: the row returned is the original — answer honestly.
		code = http.StatusOK
	}
	response.JSON(w, code, struct {
		Visit    visitDTO `json:"visit"`
		Replayed bool     `json:"replayed"`
	}{Visit: out, Replayed: !row.WasInserted})
}

type evaluated struct {
	stopID     pgtype.UUID
	mutationID pgtype.UUID
	method     string
	distanceM  pgtype.Numeric
}

// evaluate validates the request and derives method/status from evidence —
// the client never claims a validation method, it only supplies a fix.
func (h *Handler) evaluate(req visitRequest, w http.ResponseWriter, r *http.Request) (*evaluated, string) {
	bad := func(msg string) (*evaluated, string) {
		response.Error(w, r, http.StatusBadRequest, "INVALID_INPUT", msg)
		return nil, ""
	}
	var stopID, mutationID pgtype.UUID
	if err := stopID.Scan(req.StopID); err != nil || !stopID.Valid {
		return bad("stopId wajib UUID valid")
	}
	if err := mutationID.Scan(req.ClientMutationID); err != nil || !mutationID.Valid {
		return bad("clientMutationId wajib UUID valid")
	}
	if req.ObservedAt == nil {
		return bad("observedAt wajib diisi")
	}
	if req.ObservedAt.Before(observedAtMin) {
		return bad("observedAt terlalu lampau")
	}
	if time.Until(*req.ObservedAt) > observedAtFutureSlack {
		return bad("observedAt tidak boleh jauh di masa depan")
	}
	if _, err := h.store.GetStop(r.Context(), stopID); err != nil {
		response.Error(w, r, http.StatusNotFound, "STOP_NOT_FOUND", "Stasiun/halte tidak ditemukan")
		return nil, ""
	}
	e := &evaluated{stopID: stopID, mutationID: mutationID, method: "manual"}
	hasLat, hasLon := req.Lat != nil, req.Lon != nil
	if hasLat != hasLon {
		return bad("lat dan lon harus diisi bersamaan")
	}
	if !hasLat {
		return e, "low_confidence"
	}
	if *req.Lat < -90 || *req.Lat > 90 || *req.Lon < -180 || *req.Lon > 180 {
		return bad("lat/lon di luar rentang WGS84")
	}
	dist, err := h.store.StopDistanceM(r.Context(), generated.StopDistanceMParams{
		StopID: stopID, Lat: *req.Lat, Lon: *req.Lon,
	})
	if err != nil {
		response.Error(w, r, http.StatusInternalServerError, "INTERNAL", "Gagal menghitung jarak")
		return nil, ""
	}
	e.method = "geofence"
	_ = e.distanceM.Scan(fmt.Sprintf("%.1f", dist))
	status := "low_confidence"
	if dist <= geofenceRadiusM {
		status = "confirmed"
	}
	return e, status
}

// Visit list page bounds — 50 covers a normal scrollback; 200 keeps one
// response under mobile-friendly size.
const (
	visitsDefaultLimit = 50
	visitsMaxLimit     = 200
)

// visitCursor encodes the keyset position opaquely so clients never build
// cursors themselves: "RFC3339Nano|uuid".
func visitCursor(v generated.VisitEvent) string {
	return base64.RawURLEncoding.EncodeToString(
		[]byte(v.ObservedAt.Time.Format(time.RFC3339Nano) + "|" + uuidStr(v.ID)))
}

func parseVisitCursor(s string) (beforeAt time.Time, beforeID pgtype.UUID, err error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return beforeAt, beforeID, err
	}
	ts, id, ok := strings.Cut(string(raw), "|")
	if !ok {
		return beforeAt, beforeID, errors.New("malformed cursor")
	}
	if beforeAt, err = time.Parse(time.RFC3339Nano, ts); err != nil {
		return beforeAt, beforeID, err
	}
	if err = beforeID.Scan(id); err != nil {
		return beforeAt, beforeID, err
	}
	return beforeAt, beforeID, nil
}

func (h *Handler) listVisits(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFrom(r.Context())
	q := r.URL.Query()
	limit := int64(visitsDefaultLimit)
	if v := q.Get("limit"); v != "" {
		n, err := strconv.ParseInt(v, 10, 32)
		if err != nil || n < 1 || n > visitsMaxLimit {
			response.Error(w, r, http.StatusBadRequest, "VALIDATION", "limit harus 1-200")
			return
		}
		limit = n
	}
	arg := generated.ListVisitEventsParams{
		UserID: userID,
		Limit:  int32(limit + 1), // +1 row detects whether a next page exists
	}
	if c := q.Get("cursor"); c != "" {
		at, id, err := parseVisitCursor(c)
		if err != nil {
			response.Error(w, r, http.StatusBadRequest, "VALIDATION", "cursor tidak valid")
			return
		}
		arg.BeforeAt = pgtype.Timestamptz{Time: at, Valid: true}
		arg.BeforeID = id
	}
	rows, err := h.store.ListVisitEvents(r.Context(), arg)
	if err != nil {
		response.Error(w, r, http.StatusInternalServerError, "INTERNAL", "Gagal memuat kunjungan")
		return
	}
	var nextCursor *string
	if int64(len(rows)) > limit {
		c := visitCursor(rows[limit-1])
		nextCursor = &c
		rows = rows[:limit]
	}
	out := make([]visitDTO, 0, len(rows))
	for _, v := range rows {
		d := visitDTO{
			ID:               uuidStr(v.ID),
			StopID:           uuidStr(v.StopID),
			ObservedAt:       v.ObservedAt.Time,
			ValidationMethod: v.ValidationMethod,
			Status:           v.Status,
			CreatedAt:        v.CreatedAt.Time,
		}
		if v.DistanceM.Valid {
			if f, err := v.DistanceM.Float64Value(); err == nil && f.Valid {
				d.DistanceM = &f.Float64
			}
		}
		out = append(out, d)
	}
	response.JSON(w, http.StatusOK, struct {
		Visits     []visitDTO `json:"visits"`
		NextCursor *string    `json:"nextCursor,omitempty"`
	}{Visits: out, NextCursor: nextCursor})
}

func uuidStr(u pgtype.UUID) string {
	b := u.Bytes
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
