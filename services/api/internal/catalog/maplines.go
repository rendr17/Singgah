package catalog

import (
	"encoding/json"
	"net/http"

	generated "singgah/services/api/db/generated"
	"singgah/services/api/internal/http/response"
)

// GET /map/lines?bbox= — drawable route geometry for the integrated network
// map, as a GeoJSON FeatureCollection. Real ingested shapes win; routes
// without one fall back to a polyline through their ordered stops —
// properties.source on each feature says which it is.
func (h *Handler) listRouteLines(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("bbox")
	if raw == "" {
		response.Error(w, r, http.StatusBadRequest, "BAD_REQUEST", "bbox is required")
		return
	}
	env, ok := parseBBox(w, r, raw)
	if !ok {
		return
	}
	rows, err := h.store.ListRouteLinesInBBox(r.Context(), generated.ListRouteLinesInBBoxParams{
		Column1: env[0], Column2: env[1], Column3: env[2], Column4: env[3],
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	fc := RouteLineCollection{Type: "FeatureCollection", Features: make([]RouteLineFeature, 0, len(rows))}
	for _, row := range rows {
		fc.Features = append(fc.Features, RouteLineFeature{
			Type:     "Feature",
			Geometry: json.RawMessage(row.Geometry),
			Properties: RouteLineProps{
				RouteID:    row.ID.String(),
				ShortName:  textOrEmpty(row.ShortName),
				LongName:   textOrEmpty(row.LongName),
				Mode:       row.Mode,
				Color:      textOrEmpty(row.Color),
				AgencyName: textOrEmpty(row.AgencyName),
				Source:     row.GeomSource,
			},
		})
	}
	response.JSON(w, http.StatusOK, map[string]any{"lines": fc})
}
