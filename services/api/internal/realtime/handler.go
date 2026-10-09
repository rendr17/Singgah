package realtime

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"singgah/services/api/internal/http/response"
)

// Estimator supplies schedule-derived vehicle positions — the honest
// "estimated" answer for modes without a licensed GPS feed. nil disables
// the merge.
type Estimator interface {
	Estimated(ctx context.Context, now time.Time) ([]Vehicle, error)
}

// Handler serves the viewport-scoped realtime snapshot. There is no
// unscoped listing: docs/22 requires viewport scoping — the client never
// receives all of Jakarta's vehicles.
type Handler struct {
	cache       *Cache
	poller      *Poller // nil until a licensed source is configured
	estimator   Estimator
	alertStore  *AlertStore
	alertPoller *Poller         // nil until a licensed alert source is configured
	done        <-chan struct{} // server shutdown — closes SSE streams
}

func NewHandler(cache *Cache, poller *Poller, estimator Estimator, alertStore *AlertStore, alertPoller *Poller) *Handler {
	return &Handler{cache: cache, poller: poller, estimator: estimator, alertStore: alertStore, alertPoller: alertPoller}
}

// WithDone attaches the process-level shutdown channel so open streams
// close promptly on SIGTERM instead of pinning graceful shutdown.
func (h *Handler) WithDone(done <-chan struct{}) *Handler {
	h.done = done
	return h
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/vehicles", h.vehicles)
	r.Get("/realtime/health", h.health)
	r.Get("/alerts", h.alerts)
	r.Get("/realtime/stream", h.stream)
}

type vehicleDTO struct {
	ID         string    `json:"id"`
	Source     string    `json:"source"`
	RouteID    string    `json:"routeId,omitempty"`
	TripID     string    `json:"tripId,omitempty"`
	Lat        float64   `json:"lat"`
	Lon        float64   `json:"lon"`
	Bearing    *float64  `json:"bearing,omitempty"`
	State      State     `json:"state"`
	ObservedAt time.Time `json:"observedAt"`
	ReceivedAt time.Time `json:"receivedAt"`
}

// feedStatus reports live/degraded while a poller exists, and honestly
// unavailable when no source is configured — the default state today.
func (h *Handler) feedStatus() FeedStatus {
	if h.poller == nil {
		return FeedStatus{Status: "unavailable"}
	}
	return h.poller.Status()
}

// GET /api/v1/vehicles?bbox=minLon,minLat,maxLon,maxLat&route_id=
func (h *Handler) vehicles(w http.ResponseWriter, r *http.Request) {
	bbox, ok := parseBBox(w, r, r.URL.Query().Get("bbox"))
	if !ok {
		return
	}
	response.JSON(w, http.StatusOK, h.vehiclePayload(r.Context(), bbox, r.URL.Query().Get("route_id")))
}

// vehiclePayload is the one snapshot shape shared by GET /vehicles and the
// SSE stream — the wire truth must not diverge between push and poll.
func (h *Handler) vehiclePayload(ctx context.Context, bbox [4]float64, routeID string) map[string]any {
	now := time.Now()
	snap := h.cache.Snapshot(now, &bbox, routeID)
	out := make([]vehicleDTO, 0, len(snap))
	liveTrips := map[string]bool{}
	for _, v := range snap {
		if v.TripID != "" {
			liveTrips[v.TripID] = true
		}
		out = append(out, vehicleDTO{
			ID: v.ID, Source: v.Source, RouteID: v.RouteID, TripID: v.TripID,
			Lat: v.Lat, Lon: v.Lon, Bearing: v.Bearing,
			State: v.StateAt(now), ObservedAt: v.ObservedAt, ReceivedAt: v.ReceivedAt,
		})
	}
	// Estimated positions merge into the same response; a live vehicle
	// reporting the same trip always wins over its scheduled guess.
	if h.estimator != nil {
		if est, err := h.estimator.Estimated(ctx, now); err == nil {
			for _, v := range est {
				if v.TripID != "" && liveTrips[v.TripID] {
					continue
				}
				if routeID != "" && v.RouteID != routeID {
					continue
				}
				if !v.inBBox(bbox) {
					continue
				}
				out = append(out, vehicleDTO{
					ID: v.ID, Source: v.Source, RouteID: v.RouteID, TripID: v.TripID,
					Lat: v.Lat, Lon: v.Lon, Bearing: v.Bearing,
					State: v.StateAt(now), ObservedAt: v.ObservedAt, ReceivedAt: v.ReceivedAt,
				})
			}
		}
	}
	return map[string]any{
		"feedStatus": h.feedStatus(),
		"vehicles":   out,
	}
}

// GET /api/v1/realtime/health — feed-level status for ops and clients that
// want to know whether empty means "nothing moving" or "feed is dead".
func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	response.JSON(w, http.StatusOK, h.feedStatus())
}

type alertDTO struct {
	ID            string      `json:"id"`
	Source        string      `json:"source"`
	Cause         string      `json:"cause,omitempty"`
	Effect        string      `json:"effect,omitempty"`
	Header        string      `json:"headerText,omitempty"`
	Description   string      `json:"descriptionText,omitempty"`
	URL           string      `json:"url,omitempty"`
	RouteIDs      []string    `json:"routeIds,omitempty"`
	ActivePeriods []periodDTO `json:"activePeriods,omitempty"`
}

type periodDTO struct {
	Start *time.Time `json:"start,omitempty"`
	End   *time.Time `json:"end,omitempty"`
}

// GET /api/v1/alerts?route_id= — service alerts are not viewport-scoped;
// their own active periods decide visibility, and a missing feed reports
// feedStatus=unavailable with an empty list rather than pretending all-quiet.
func (h *Handler) alerts(w http.ResponseWriter, r *http.Request) {
	status := FeedStatus{Status: "unavailable"}
	if h.alertPoller != nil {
		status = h.alertPoller.Status()
	}
	var list []Alert
	if h.alertStore != nil {
		list = h.alertStore.List(time.Now(), r.URL.Query().Get("route_id"))
	}
	out := make([]alertDTO, 0, len(list))
	for _, a := range list {
		d := alertDTO{
			ID: a.ID, Source: a.Source, Cause: a.Cause, Effect: a.Effect,
			Header: a.Header, Description: a.Description, URL: a.URL,
			RouteIDs: a.RouteIDs,
		}
		for _, p := range a.Periods {
			d.ActivePeriods = append(d.ActivePeriods, periodDTO{Start: p.Start, End: p.End})
		}
		out = append(out, d)
	}
	response.JSON(w, http.StatusOK, map[string]any{
		"feedStatus": status,
		"alerts":     out,
	})
}

// parseBBox mirrors catalog's envelope parsing: same param shape, same 400s.
// Kept local — the two validators diverge the day realtime needs different
// bounds (e.g. max viewport area), and premature sharing hides that.
func parseBBox(w http.ResponseWriter, r *http.Request, raw string) ([4]float64, bool) {
	var env [4]float64
	if raw == "" {
		response.Error(w, r, http.StatusBadRequest, "BAD_REQUEST", "bbox is required — vehicles are viewport-scoped")
		return env, false
	}
	parts := strings.Split(raw, ",")
	if len(parts) != 4 {
		response.Error(w, r, http.StatusBadRequest, "BAD_REQUEST", "bbox must be minLon,minLat,maxLon,maxLat")
		return env, false
	}
	for i, p := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			response.Error(w, r, http.StatusBadRequest, "BAD_REQUEST", "bbox must be numeric")
			return env, false
		}
		env[i] = v
	}
	if env[0] < -180 || env[0] > 180 || env[2] < -180 || env[2] > 180 ||
		env[1] < -90 || env[1] > 90 || env[3] < -90 || env[3] > 90 ||
		env[0] >= env[2] || env[1] >= env[3] {
		response.Error(w, r, http.StatusBadRequest, "BAD_REQUEST", "bbox out of range or min >= max")
		return env, false
	}
	return env, true
}
