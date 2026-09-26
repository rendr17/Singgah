package journey

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	generated "singgah/services/api/db/generated"
	httpapi "singgah/services/api/internal/http/response"
	"singgah/services/api/internal/provider/commute"
)

const providerCode = "commute"

// jakarta is WIB (UTC+7) permanently — no DST since 1964, so a fixed zone is
// exact and needs no tzdata on the host.
var jakarta = time.FixedZone("Asia/Jakarta", 7*60*60)

// departuresWindow bounds the upstream timetable fetch in minutes.
const departuresWindow = 3 * 60

// maxDepartures caps how many boardings the leg carries — enough to answer
// "kapan berangkat" without turning the plan into a timetable page.
const maxDepartures = 3

// maxShapeSnapM bounds how far a listed stop may sit from the drawn slice
// before the shape is rejected: surveyed halte sit within ~120 m of their
// true shape, while same-termini variants miss mid-route stops by 600 m+.
// Beyond it, drawing the shape would lie about the path — the leg carries
// no geometry and the client falls back to the stop-to-stop polyline.
const maxShapeSnapM = 250

type Handler struct {
	store   Store
	planner FarePlanner
	tt      Timetabler
	now     func() time.Time
}

func NewHandler(store Store, planner FarePlanner, tt Timetabler) *Handler {
	return &Handler{store: store, planner: planner, tt: tt, now: time.Now}
}

// RegisterRoutes mounts the domain's paths on an existing mux — the router
// composes several domains under one /api/v1.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/journeys", h.plan)
}

// GET /journeys?from=<uuid>&to=<uuid>
func (h *Handler) plan(w http.ResponseWriter, r *http.Request) {
	fromID, ok := parseUUID(w, r, "from")
	if !ok {
		return
	}
	toID, ok := parseUUID(w, r, "to")
	if !ok {
		return
	}
	at, ok := parseAt(w, r)
	if !ok {
		return
	}
	from, err := h.store.GetStop(r.Context(), fromID)
	if err != nil {
		writeErr(w, r, err, "Origin station not found")
		return
	}
	to, err := h.store.GetStop(r.Context(), toID)
	if err != nil {
		writeErr(w, r, err, "Destination station not found")
		return
	}
	if from.ProviderCode != providerCode || to.ProviderCode != providerCode {
		httpapi.Error(w, r, http.StatusUnprocessableEntity, "PROVIDER_UNSUPPORTED",
			"Journey planning is not available for this stop's provider")
		return
	}

	plan, err := h.planner.Fares(r.Context(), from.ProviderEntityID, to.ProviderEntityID, at)
	if errors.Is(err, commute.ErrStationUnknown) {
		writePlan(w, r, from, to, nil, at)
		return
	}
	if err != nil {
		httpapi.Error(w, r, http.StatusBadGateway, "PROVIDER_UNAVAILABLE",
			"Provider rute sedang tidak tersedia")
		return
	}
	itinerary, err := h.normalize(r.Context(), plan)
	if err != nil {
		httpapi.Error(w, r, http.StatusInternalServerError, "INTERNAL", "Internal server error")
		return
	}
	// at anchors the departures window when given; otherwise "leave now".
	anchor := h.now()
	if at != nil {
		anchor = *at
	}
	h.attachDepartures(r.Context(), itinerary, plan, anchor)
	writePlan(w, r, from, to, itinerary, at)
}

func (h *Handler) normalize(ctx context.Context, plan *commute.FarePlan) (*Itinerary, error) {
	// One reverse lookup for every provider stop id in the plan.
	ids := make(map[string]struct{})
	collect := func(s commute.FareStationRef) {
		if s.ID != "" {
			ids[s.ID] = struct{}{}
		}
	}
	collect(plan.From)
	collect(plan.To)
	for _, leg := range plan.Legs {
		collect(leg.From)
		collect(leg.To)
		for _, s := range leg.Stops {
			collect(s)
		}
	}
	for _, seg := range plan.Segments {
		collect(seg.From)
		collect(seg.To)
	}
	idList := make([]string, 0, len(ids))
	for id := range ids {
		idList = append(idList, id)
	}
	uuids := map[string]string{}
	if len(idList) > 0 {
		rows, err := h.store.ListStopIDsByProviderEntityIDs(ctx, generated.ListStopIDsByProviderEntityIDsParams{
			Code:      providerCode,
			EntityIds: idList,
		})
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			uuids[row.ProviderEntityID] = row.ID.String()
		}
	}
	ref := func(s commute.FareStationRef) StopRef {
		return StopRef{ID: uuids[s.ID], Name: s.Name}
	}

	// Coordinates of the resolved stops — one fetch, used to slice route
	// shapes per leg. A coords lookup failure degrades to no geometry; it
	// must not fail a plan the provider already computed.
	coords := map[string]generated.ListStopCoordsRow{}
	if len(uuids) > 0 {
		ids := make([]pgtype.UUID, 0, len(uuids))
		for _, u := range uuids {
			var id pgtype.UUID
			if id.Scan(u) == nil {
				ids = append(ids, id)
			}
		}
		if rows, err := h.store.ListStopCoords(ctx, ids); err == nil {
			for _, r := range rows {
				coords[r.ID.String()] = r
			}
		}
	}

	itin := &Itinerary{
		Status:         "scheduled",
		TotalDistanceM: plan.TotalDistance,
		Legs:           make([]Leg, 0, len(plan.Legs)),
	}
	for _, leg := range plan.Legs {
		l := Leg{From: ref(leg.From), To: ref(leg.To), DistanceM: leg.DistanceM}
		switch leg.Type {
		case "TRANSFER":
			l.Type = "walk"
			itin.WalkTransfers++
		case "RIDE":
			l.Type = "ride"
			itin.RideLegs++
			l.Line = leg.Line
			l.Operator = leg.Operator
			l.StationCount = leg.StationCount
			l.Headsign = leg.Headsign
			for _, s := range leg.Stops {
				l.Stops = append(l.Stops, ref(s))
			}
			if routeID, err := h.store.GetRouteByProviderEntityID(ctx, generated.GetRouteByProviderEntityIDParams{
				Code:             providerCode,
				ProviderEntityID: leg.Line,
			}); err == nil {
				l.RouteID = routeID.String()
				l.Geometry = h.sliceGeometry(ctx, routeID, leg, uuids, coords)
				l.Alternatives = h.legAlternatives(ctx, routeID, uuids[leg.From.ID], uuids[leg.To.ID])
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return nil, err
			}
		default:
			l.Type = leg.Type
		}
		itin.Legs = append(itin.Legs, l)
	}
	if len(plan.Segments) > 0 || plan.TotalFare != nil {
		fare := &Fare{Currency: "IDR", Total: plan.TotalFare, Segments: make([]FareSegment, 0, len(plan.Segments))}
		for _, seg := range plan.Segments {
			fare.Segments = append(fare.Segments, FareSegment{
				Operator: seg.Operator,
				From:     ref(seg.From),
				To:       ref(seg.To),
				Amount:   seg.Fare,
			})
		}
		itin.Fare = fare
	}
	return itin, nil
}

// sliceGeometry cuts the ingested route shape between a leg's endpoints.
// The pick is scored against the whole resolved stop sequence — same-termini
// variants that detour past unlisted streets lose to the shape the leg
// actually rides. Missing shapes/coords or a slice error all degrade to no
// geometry — the client then draws the stop-to-stop polyline. Geometry is
// an enhancement, never a reason to fail a computed plan.
func (h *Handler) sliceGeometry(ctx context.Context, routeID pgtype.UUID, leg commute.FareLeg, uuids map[string]string, coords map[string]generated.ListStopCoordsRow) json.RawMessage {
	coord := func(r commute.FareStationRef) (generated.ListStopCoordsRow, bool) {
		c, ok := coords[uuids[r.ID]]
		return c, ok
	}
	f, okF := coord(leg.From)
	t, okT := coord(leg.To)
	if !okF || !okT {
		return nil
	}
	lons := make([]float64, 0, len(leg.Stops)+2)
	lats := make([]float64, 0, len(leg.Stops)+2)
	lons = append(lons, f.Lon)
	lats = append(lats, f.Lat)
	for _, s := range leg.Stops {
		if c, ok := coord(s); ok {
			lons = append(lons, c.Lon)
			lats = append(lats, c.Lat)
		}
	}
	lons = append(lons, t.Lon)
	lats = append(lats, t.Lat)
	return h.sliceShape(ctx, routeID, lons, lats)
}

// legAlternatives lists other catalog routes that also carry the leg's
// endpoints — "bisa juga naik koridor 2". Each entry carries that route's
// own stop slice and shape cut, so a picked alternative never borrows the
// chosen leg's stops to describe a different corridor. Catalogue gaps
// degrade to no alternatives, never a failed plan.
func (h *Handler) legAlternatives(ctx context.Context, routeID pgtype.UUID, fromUUID, toUUID string) []LegAlternative {
	var fromID, toID pgtype.UUID
	if fromID.Scan(fromUUID) != nil || toID.Scan(toUUID) != nil {
		return nil
	}
	rows, err := h.store.ListRouteAlternatives(ctx, generated.ListRouteAlternativesParams{
		StopID:   fromID,
		StopID_2: toID,
		ID:       routeID,
	})
	if err != nil {
		return nil
	}
	seen := map[pgtype.UUID]bool{}
	var out []LegAlternative
	for _, row := range rows {
		if seen[row.ID] {
			continue // min-hops seq pair already taken — rows are ABS-diff ordered
		}
		seen[row.ID] = true
		slice, err := h.store.ListRouteStopSlice(ctx, generated.ListRouteStopSliceParams{
			RouteID: row.ID,
			Seq:     min(row.SeqFrom, row.SeqTo),
			Seq_2:   max(row.SeqFrom, row.SeqTo),
		})
		if err != nil || len(slice) < 2 {
			continue
		}
		if row.SeqFrom > row.SeqTo {
			slices.Reverse(slice)
		}
		lons := make([]float64, 0, len(slice))
		lats := make([]float64, 0, len(slice))
		stops := make([]StopRef, 0, len(slice)-2)
		for i, s := range slice {
			lons = append(lons, s.Lon)
			lats = append(lats, s.Lat)
			if i > 0 && i < len(slice)-1 {
				stops = append(stops, StopRef{ID: s.ID.String(), Name: s.Name})
			}
		}
		op, _ := splitLineKey(row.ProviderEntityID)
		alt := LegAlternative{
			RouteID:      row.ID.String(),
			Line:         row.ProviderEntityID,
			Operator:     op,
			StationCount: len(slice) - 1,
			Stops:        stops,
			Geometry:     h.sliceShape(ctx, row.ID, lons, lats),
		}
		if row.ShortName.Valid {
			alt.ShortName = row.ShortName.String
		}
		if row.LongName.Valid {
			alt.Name = row.LongName.String
		}
		out = append(out, alt)
	}
	return out
}

// sliceShape runs the scored shape cut for a sequence of stop coordinates —
// shared by provider legs and corridor alternatives alike.
func (h *Handler) sliceShape(ctx context.Context, routeID pgtype.UUID, lons, lats []float64) json.RawMessage {
	if len(lons) < 2 {
		return nil
	}
	g, err := h.store.SliceRouteShape(ctx, generated.SliceRouteShapeParams{
		RouteID:  routeID,
		FromLon:  lons[0],
		FromLat:  lats[0],
		ToLon:    lons[len(lons)-1],
		ToLat:    lats[len(lats)-1],
		Lons:     lons,
		Lats:     lats,
		MaxSnapM: maxShapeSnapM,
	})
	if err != nil || g == "" {
		return nil
	}
	return json.RawMessage(g)
}

// attachDepartures fills nextDepartures on the first ride leg — "kapan
// berangkat" is answered at the first boarding; later legs would need
// arrival-time propagation the provider doesn't compute. A timetable outage
// degrades to no departures, never a failed plan.
func (h *Handler) attachDepartures(ctx context.Context, itin *Itinerary, plan *commute.FarePlan, anchor time.Time) {
	for i, fl := range plan.Legs {
		if fl.Type != "RIDE" || i >= len(itin.Legs) {
			continue
		}
		op, stn := splitStationID(fl.From.ID)
		_, line := splitLineKey(fl.Line)
		if op == "" || line == "" {
			return
		}
		anchor = anchor.In(jakarta)
		entries, err := h.tt.Timetable(ctx, op, stn,
			anchor.Format("15:04"), anchor.Add(departuresWindow*time.Minute).Format("15:04"))
		if err != nil {
			return
		}
		anchorMin := anchor.Hour()*60 + anchor.Minute()
		type cand struct {
			d Departure
			m int // minutes from now, wrapped at midnight
		}
		var cands []cand
		for _, e := range entries {
			if e.LineCode != line || (fl.Headsign != "" && e.BoundFor != fl.Headsign) {
				continue
			}
			depMin, ok := minutesOfDay(e.EstimatedDeparture)
			if !ok {
				continue
			}
			m := (depMin - anchorMin + 1440) % 1440
			if m > departuresWindow {
				continue
			}
			cands = append(cands, cand{d: Departure{
				Time:       e.EstimatedDeparture[:5],
				TripNumber: e.TripNumber,
				BoundFor:   e.BoundFor,
			}, m: m})
		}
		sort.Slice(cands, func(a, b int) bool { return cands[a].m < cands[b].m })
		if len(cands) > maxDepartures {
			cands = cands[:maxDepartures]
		}
		if len(cands) > 0 {
			deps := make([]Departure, 0, len(cands))
			for _, c := range cands {
				deps = append(deps, c.d)
			}
			itin.Legs[i].NextDepartures = deps
		}
		return
	}
}

// splitStationID turns "MRTJ-DKA" into operator "MRTJ" and code "DKA".
func splitStationID(id string) (op, code string) {
	i := strings.IndexByte(id, '-')
	if i <= 0 || i == len(id)-1 {
		return "", ""
	}
	return id[:i], id[i+1:]
}

// splitLineKey turns "MRTJ:M" into operator "MRTJ" and line code "M".
func splitLineKey(key string) (op, code string) {
	i := strings.IndexByte(key, ':')
	if i <= 0 || i == len(key)-1 {
		return "", ""
	}
	return key[:i], key[i+1:]
}

// minutesOfDay parses provider "HH:MM:SS" wall-clock strings.
func minutesOfDay(s string) (int, bool) {
	t, err := time.Parse("15:04:05", s)
	if err != nil {
		return 0, false
	}
	return t.Hour()*60 + t.Minute(), true
}

func writePlan(w http.ResponseWriter, r *http.Request, from, to generated.GetStopRow, itin *Itinerary, at *time.Time) {
	httpapi.JSON(w, http.StatusOK, PlanResponse{
		From:      StopRef{ID: from.ID.String(), Name: from.Name},
		To:        StopRef{ID: to.ID.String(), Name: to.Name},
		At:        at,
		Itinerary: itin,
		Source:    SourceMeta{Provider: providerCode, RequestedAt: time.Now().UTC()},
	})
}

// parseAt reads the optional departure context — an RFC 3339 instant the plan
// is computed for ("leave at"). It goes upstream for fare selection and
// anchors the departures window; absent means "now".
func parseAt(w http.ResponseWriter, r *http.Request) (*time.Time, bool) {
	v := r.URL.Query().Get("at")
	if v == "" {
		return nil, true
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		httpapi.Error(w, r, http.StatusBadRequest, "BAD_REQUEST", "Invalid at timestamp")
		return nil, false
	}
	return &t, true
}

func parseUUID(w http.ResponseWriter, r *http.Request, param string) (pgtype.UUID, bool) {
	var id pgtype.UUID
	v := r.URL.Query().Get(param)
	if v == "" || id.Scan(v) != nil {
		httpapi.Error(w, r, http.StatusBadRequest, "BAD_REQUEST", "Invalid "+param+" id")
		return id, false
	}
	return id, true
}

func writeErr(w http.ResponseWriter, r *http.Request, err error, notFoundMsg string) {
	if errors.Is(err, pgx.ErrNoRows) {
		httpapi.Error(w, r, http.StatusNotFound, "NOT_FOUND", notFoundMsg)
		return
	}
	httpapi.Error(w, r, http.StatusInternalServerError, "INTERNAL", "Internal server error")
}
