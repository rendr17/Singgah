package realtime

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"singgah/services/api/internal/http/response"
)

// The stream pushes on the same cadence the poller fetches — the payload
// can only change when the feed does, so a faster tick invents churn. The
// 15s frames double as keep-alives, so no separate heartbeat is needed.
const streamTick = 15 * time.Second

// GET /api/v1/realtime/stream?bbox=…&route_id=
//
// SSE — the data only ever flows server→client, so WebSocket buys nothing
// but complexity here. Rescoping the viewport is a reconnect with a new
// bbox, which the client already debounces for the REST poll.
func (h *Handler) stream(w http.ResponseWriter, r *http.Request) {
	bbox, ok := parseBBox(w, r, r.URL.Query().Get("bbox"))
	if !ok {
		return
	}
	routeID := r.URL.Query().Get("route_id")

	flusher, ok := w.(http.Flusher)
	if !ok {
		response.Error(w, r, http.StatusInternalServerError, "INTERNAL", "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no") // nginx-style proxies must not buffer SSE

	emit := func() bool {
		payload, err := json.Marshal(h.vehiclePayload(r.Context(), bbox, routeID))
		if err != nil {
			return false
		}
		if _, err := fmt.Fprintf(w, "event: vehicles\ndata: %s\n\n", payload); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}
	if !emit() {
		return
	}

	tick := time.NewTicker(streamTick)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			if !emit() {
				return
			}
		case <-r.Context().Done():
			return
		case <-h.done:
			return
		}
	}
}
