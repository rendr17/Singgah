package catalog

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	generated "singgah/services/api/db/generated"
	"singgah/services/api/internal/http/response"
)

const defaultLimit = 20

// Handler serves the catalog endpoints under /api/v1.
type Handler struct {
	store Store
}

func NewHandler(store Store) *Handler {
	return &Handler{store: store}
}

// RegisterRoutes mounts the domain's paths on an existing mux — the router
// composes several domains under one /api/v1 prefix.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/stations", h.listStations)
	r.Get("/stations/{id}", h.getStation)
	r.Get("/routes", h.listRoutes)
	r.Get("/routes/{id}", h.getRoute)
}

func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r
}

// GET /stations?query=&bbox=&limit= — three modes: bbox viewport listing,
// text search, or the unfiltered reference list. bbox + query combine as
// "search within the viewport".
func (h *Handler) listStations(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("query")
	limit, ok := parseLimit(w, r, defaultLimit)
	if !ok {
		return
	}

	var stations []StationSummary
	if bbox := r.URL.Query().Get("bbox"); bbox != "" {
		env, ok := parseBBox(w, r, bbox)
		if !ok {
			return
		}
		rows, err := h.store.ListStopsInBBox(r.Context(), generated.ListStopsInBBoxParams{
			Column1: env[0], Column2: env[1], Column3: env[2], Column4: env[3], Limit: limit,
		})
		if err != nil {
			writeErr(w, r, err)
			return
		}
		for _, row := range rows {
			if q != "" && !matchesQuery(row.Name, textOrEmpty(row.Code), q) {
				continue
			}
			stations = append(stations, stationSummary(row.ID, row.Name, row.Code, row.Kind, row.Lat, row.Lon, row.ProviderCode))
		}
	} else if q != "" {
		rows, err := h.store.SearchStops(r.Context(), generated.SearchStopsParams{
			Name:  "%" + q + "%",
			Limit: limit,
		})
		if err != nil {
			writeErr(w, r, err)
			return
		}
		for _, row := range rows {
			stations = append(stations, stationSummary(row.ID, row.Name, row.Code, row.Kind, row.Lat, row.Lon, row.ProviderCode))
		}
	} else {
		rows, err := h.store.ListStops(r.Context(), limit)
		if err != nil {
			writeErr(w, r, err)
			return
		}
		for _, row := range rows {
			stations = append(stations, stationSummary(row.ID, row.Name, row.Code, row.Kind, row.Lat, row.Lon, row.ProviderCode))
		}
	}
	if stations == nil {
		stations = []StationSummary{}
	}
	response.JSON(w, http.StatusOK, map[string]any{"stations": stations})
}

func stationSummary(id pgtype.UUID, name string, code pgtype.Text, kind string, lat, lon float64, provider string) StationSummary {
	return StationSummary{
		ID:           id.String(),
		Name:         name,
		Code:         textOrEmpty(code),
		Kind:         kind,
		Lat:          lat,
		Lon:          lon,
		ProviderCode: provider,
	}
}

// parseBBox reads "minLon,minLat,maxLon,maxLat" and enforces WGS84 ranges
// before the envelope ever reaches PostGIS.
func parseBBox(w http.ResponseWriter, r *http.Request, raw string) ([4]float64, bool) {
	var env [4]float64
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

func matchesQuery(name, code, q string) bool {
	return strings.Contains(strings.ToLower(name), strings.ToLower(q)) ||
		strings.Contains(strings.ToLower(code), strings.ToLower(q))
}

// GET /stations/{id} — detail with serving lines and transfers.
func (h *Handler) getStation(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r)
	if !ok {
		return
	}
	stop, err := h.store.GetStop(r.Context(), id)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	lines, err := h.store.ListRoutesServingStop(r.Context(), id)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	transfers, err := h.store.ListTransfersFromStop(r.Context(), id)
	if err != nil {
		writeErr(w, r, err)
		return
	}

	detail := StationDetail{
		StationSummary: StationSummary{
			ID:           stop.ID.String(),
			Name:         stop.Name,
			Code:         textOrEmpty(stop.Code),
			Kind:         stop.Kind,
			Lat:          stop.Lat,
			Lon:          stop.Lon,
			ProviderCode: stop.ProviderCode,
		},
		OfficialName: officialName(stop.Metadata),
		Source:       sourceMeta(stop.ProviderCode, stop.FetchedAt, stop.SourceUpdatedAt),
	}
	for _, l := range lines {
		detail.Lines = append(detail.Lines, routeRef(l.ID, l.ShortName, l.LongName, l.Mode, l.Color, l.AgencyCode, pgtype.Text{String: l.AgencyName, Valid: l.AgencyName != ""}))
	}
	for _, t := range transfers {
		dto := TransferDTO{ToStop: stopRef(t.ToStopID, t.ToStopName, t.ToStopCode, t.ToLon, t.ToLat)}
		if t.WalkDistanceM.Valid {
			d := t.WalkDistanceM.Int32
			dto.WalkDistanceM = &d
		}
		dto.Notes = t.Notes
		detail.Transfers = append(detail.Transfers, dto)
	}
	response.JSON(w, http.StatusOK, map[string]any{"station": detail})
}

// GET /routes?query=&limit=
func (h *Handler) listRoutes(w http.ResponseWriter, r *http.Request) {
	limit, ok := parseLimit(w, r, 200)
	if !ok {
		return
	}
	rows, err := h.store.ListRoutes(r.Context(), generated.ListRoutesParams{
		Column1: r.URL.Query().Get("query"),
		Limit:   limit,
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	routes := make([]RouteSummary, 0, len(rows))
	for _, row := range rows {
		routes = append(routes, RouteSummary{
			RouteRef:     routeRef(row.ID, row.ShortName, row.LongName, row.Mode, row.Color, row.AgencyCode, row.AgencyName),
			ProviderCode: row.ProviderCode,
		})
	}
	response.JSON(w, http.StatusOK, map[string]any{"routes": routes})
}

// GET /routes/{id} — detail with served stops.
func (h *Handler) getRoute(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r)
	if !ok {
		return
	}
	route, err := h.store.GetRoute(r.Context(), id)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	stops, err := h.store.ListStopsOnRoute(r.Context(), id)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	detail := RouteDetail{
		RouteSummary: RouteSummary{
			RouteRef:     routeRef(route.ID, route.ShortName, route.LongName, route.Mode, route.Color, route.AgencyCode, route.AgencyName),
			ProviderCode: route.ProviderCode,
		},
		Source: sourceMeta(route.ProviderCode, route.FetchedAt, route.SourceUpdatedAt),
	}
	for _, s := range stops {
		detail.Stops = append(detail.Stops, stopRef(s.ID, s.Name, s.Code, s.Lon, s.Lat))
	}
	response.JSON(w, http.StatusOK, map[string]any{"route": detail})
}

func routeRef(id pgtype.UUID, short, long pgtype.Text, mode string, color, agencyCode, agencyName pgtype.Text) RouteRef {
	return RouteRef{
		ID:         id.String(),
		ShortName:  textOrEmpty(short),
		LongName:   textOrEmpty(long),
		Mode:       mode,
		Color:      textOrEmpty(color),
		AgencyCode: textOrEmpty(agencyCode),
		AgencyName: textOrEmpty(agencyName),
	}
}

func parseUUID(w http.ResponseWriter, r *http.Request) (pgtype.UUID, bool) {
	var id pgtype.UUID
	if err := id.Scan(chi.URLParam(r, "id")); err != nil {
		response.Error(w, r, http.StatusBadRequest, "BAD_REQUEST", "Invalid id")
		return id, false
	}
	return id, true
}

func parseLimit(w http.ResponseWriter, r *http.Request, def int32) (int32, bool) {
	v := r.URL.Query().Get("limit")
	if v == "" {
		return def, true
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 || n > 500 {
		response.Error(w, r, http.StatusBadRequest, "BAD_REQUEST", "Invalid limit")
		return 0, false
	}
	return int32(n), true
}

func writeErr(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, pgx.ErrNoRows) {
		response.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Resource not found")
		return
	}
	response.Error(w, r, http.StatusInternalServerError, "INTERNAL", "Internal server error")
}

func textOrEmpty(t pgtype.Text) string {
	if t.Valid {
		return t.String
	}
	return ""
}

func officialName(metadata []byte) string {
	var m map[string]any
	if err := json.Unmarshal(metadata, &m); err != nil {
		return ""
	}
	s, _ := m["official_name"].(string)
	return s
}
