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

type progressCollection struct {
	CollectionID string `json:"collectionId"`
	Slug         string `json:"slug"`
	Title        string `json:"title"`
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
	colls, err := h.store.PassportProgressByCollection(r.Context(), userID)
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
	byCollection := make([]progressCollection, 0, len(colls))
	for _, c := range colls {
		byCollection = append(byCollection, progressCollection{
			CollectionID:  uuidStr(c.ID),
			Slug:          c.Slug,
			Title:         c.Title,
			progressCount: progressCount{Visited: c.VisitedStops, Total: c.TotalStops},
		})
	}
	response.JSON(w, http.StatusOK, struct {
		progressCount
		ByMode       []progressMode       `json:"byMode"`
		ByRoute      []progressRoute      `json:"byRoute"`
		ByCollection []progressCollection `json:"byCollection"`
	}{
		progressCount: progressCount{Visited: total.VisitedStops, Total: total.TotalStops},
		ByMode:        byMode,
		ByRoute:       byRoute,
		ByCollection:  byCollection,
	})
}
