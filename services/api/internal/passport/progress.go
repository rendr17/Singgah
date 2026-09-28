package passport

import (
	"net/http"

	"singgah/services/api/internal/auth"
	"singgah/services/api/internal/http/response"
)

type progressCount struct {
	Visited int64 `json:"visitedStops"`
	Total   int64 `json:"totalStops"`
}

type progressMode struct {
	Mode string `json:"mode"`
	progressCount
}

type progressRoute struct {
	RouteID  string  `json:"routeId"`
	RouteKey string  `json:"routeKey"`
	Name     string  `json:"name"`
	Mode     string  `json:"mode"`
	Color    *string `json:"color,omitempty"`
	progressCount
}

func (h *Handler) progress(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFrom(r.Context())
	total, err := h.store.PassportProgressTotal(r.Context(), userID)
	if err != nil {
		response.Error(w, r, http.StatusInternalServerError, "INTERNAL", "Gagal menghitung progres")
		return
	}
	modes, err := h.store.PassportProgressByMode(r.Context(), userID)
	if err != nil {
		response.Error(w, r, http.StatusInternalServerError, "INTERNAL", "Gagal menghitung progres")
		return
	}
	routes, err := h.store.PassportProgressByRoute(r.Context(), userID)
	if err != nil {
		response.Error(w, r, http.StatusInternalServerError, "INTERNAL", "Gagal menghitung progres")
		return
	}

	byMode := make([]progressMode, 0, len(modes))
	for _, m := range modes {
		byMode = append(byMode, progressMode{
			Mode:          m.Mode,
			progressCount: progressCount{Visited: m.VisitedStops, Total: m.TotalStops},
		})
	}
	byRoute := make([]progressRoute, 0, len(routes))
	for _, rt := range routes {
		name := rt.LongName.String
		if name == "" {
			name = rt.ShortName.String
		}
		var color *string
		if rt.Color.Valid {
			c := rt.Color.String
			color = &c
		}
		byRoute = append(byRoute, progressRoute{
			RouteID:  uuidStr(rt.ID),
			RouteKey: rt.RouteKey,
			Name:     name,
			Mode:     rt.Mode,
			Color:    color,
			progressCount: progressCount{
				Visited: rt.VisitedStops, Total: rt.TotalStops,
			},
		})
	}
	response.JSON(w, http.StatusOK, struct {
		progressCount
		ByMode  []progressMode  `json:"byMode"`
		ByRoute []progressRoute `json:"byRoute"`
	}{
		progressCount: progressCount{Visited: total.VisitedStops, Total: total.TotalStops},
		ByMode:        byMode,
		ByRoute:       byRoute,
	})
}
