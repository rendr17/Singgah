package journey

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	generated "singgah/services/api/db/generated"
	httpapi "singgah/services/api/internal/http/response"
	"singgah/services/api/internal/planner"
	"singgah/services/api/internal/provider/commute"
)

// jakarta is WIB (UTC+7) permanently — no DST since 1964, so a fixed zone is
// exact and needs no tzdata on the host.
var jakarta = time.FixedZone("Asia/Jakarta", 7*60*60)

const providerCode = "commute"

// maxDepartures caps how many boardings the leg carries — enough to answer
// "kapan berangkat" without turning the plan into a timetable page.
const maxDepartures = 3

// maxShapeSnapM bounds how far a listed stop may sit from the drawn slice
// before the shape is rejected: surveyed halte sit within ~120 m of their
// true shape, while same-termini variants miss mid-route stops by 600 m+.
// Beyond it, drawing the shape would lie about the path — the leg carries
// no geometry and the client falls back to the stop-to-stop polyline.
const maxShapeSnapM = 250

// altMaxDetourFactor/altMaxDetourKm bound a corridor alternative's stop
// path against the leg's straight-line distance. Folded provider
// sequences can order the leg's endpoints around an entire loop (TJ:7F
// offered "Kwitang→Monas, 22 perhentian" via Kampung Rambutan) — a slice
// that detours this much is a different journey wearing the leg's
// endpoints, not an interchangeable boarding.
const (
	altMaxDetourFactor = 3.0
	altMaxDetourKm     = 4.0
)

// knownModes are the catalog's mode vocabulary — the modes filter is
// validated against it so typos fail loudly instead of silently planning
// with an empty set.
var knownModes = map[string]bool{
	"rail": true, "subway": true, "tram": true,
	"bus": true, "ferry": true, "other": true,
}

type Handler struct {
	store  Store
	engine *planner.EngineSource
	fares  FarePlanner
	now    func() time.Time
}

func NewHandler(store Store, engine *planner.EngineSource, fares FarePlanner) *Handler {
	return &Handler{store: store, engine: engine, fares: fares, now: time.Now}
}

// RegisterRoutes mounts the domain's paths on an existing mux — the router
// composes several domains under one /api/v1.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/journeys", h.plan)
}

// GET /journeys?from=<uuid>&to=<uuid>[&at=…|&arriveBy=…][&modes=…]
// [&maxWalkM=…][&maxTransfers=…][&stepFree=1]
//
// The in-house planner answers from the ingested schedule snapshot;
// /fares is only consulted afterwards as a fare reference — an upstream
// outage degrades the reference, never the plan.
func (h *Handler) plan(w http.ResponseWriter, r *http.Request) {
	q, echo, ok := h.parseQuery(w, r)
	if !ok {
		return
	}
	from, err := h.store.GetStop(r.Context(), q.From)
	if err != nil {
		writeErr(w, r, err, "Origin station not found")
		return
	}
	to, err := h.store.GetStop(r.Context(), q.To)
	if err != nil {
		writeErr(w, r, err, "Destination station not found")
		return
	}

	eng, err := h.engine.Get(r.Context())
	if err != nil {
		httpapi.Error(w, r, http.StatusServiceUnavailable, "PLANNER_UNAVAILABLE",
			"Data jadwal sedang tidak tersedia")
		return
	}
	its, err := eng.Plan(q)
	if err != nil {
		httpapi.Error(w, r, http.StatusInternalServerError, "INTERNAL", "Internal server error")
		return
	}
	itineraries := h.mapItineraries(r.Context(), eng, its)
	h.attachDepartures(eng, itineraries)
	writePlan(w, r, from, to, echo, itineraries, h.fareReference(r.Context(), from, to, q.DepartAt), eng)
}

// parseQuery turns the raw query string into a planner.Query plus its echo.
// exactly one anchor: `at` (depart) or `arriveBy` — neither means "leave now".
func (h *Handler) parseQuery(w http.ResponseWriter, r *http.Request) (planner.Query, QueryEcho, bool) {
	var q planner.Query
	var echo QueryEcho
	q.MaxWalkM, q.MaxTransfers = -1, -1

	fromID, ok := parseUUID(w, r, "from")
	if !ok {
		return q, echo, false
	}
	q.From = fromID
	toID, ok := parseUUID(w, r, "to")
	if !ok {
		return q, echo, false
	}
	q.To = toID

	at, arriveBy := r.URL.Query().Get("at"), r.URL.Query().Get("arriveBy")
	if at != "" && arriveBy != "" {
		httpapi.Error(w, r, http.StatusBadRequest, "BAD_REQUEST", "Use at or arriveBy, not both")
		return q, echo, false
	}
	anchor, err := parseTimeParam(at)
	if err != "" {
		httpapi.Error(w, r, http.StatusBadRequest, "BAD_REQUEST", "Invalid at timestamp")
		return q, echo, false
	}
	if anchor != nil {
		q.DepartAt = anchor
	}
	anchor, err = parseTimeParam(arriveBy)
	if err != "" {
		httpapi.Error(w, r, http.StatusBadRequest, "BAD_REQUEST", "Invalid arriveBy timestamp")
		return q, echo, false
	}
	if anchor != nil {
		q.ArriveBy = anchor
	}
	if q.DepartAt == nil && q.ArriveBy == nil {
		now := h.now()
		q.DepartAt = &now // leave now
	}
	echo.DepartAt, echo.ArriveBy = q.DepartAt, q.ArriveBy

	if raw := r.URL.Query().Get("modes"); raw != "" {
		q.Modes = map[string]bool{}
		for m := range strings.SplitSeq(raw, ",") {
			m = strings.TrimSpace(m)
			if !knownModes[m] {
				httpapi.Error(w, r, http.StatusBadRequest, "BAD_REQUEST", "Unknown mode: "+m)
				return q, echo, false
			}
			q.Modes[m] = true
			echo.Modes = append(echo.Modes, m)
		}
		sort.Strings(echo.Modes)
	}
	if v, ok := parseIntParam(w, r, "maxWalkM", 0); !ok {
		return q, echo, false
	} else if v != nil {
		q.MaxWalkM, echo.MaxWalkM = *v, v
	}
	if v, ok := parseIntParam(w, r, "maxTransfers", 0); !ok {
		return q, echo, false
	} else if v != nil {
		q.MaxTransfers, echo.MaxTransfers = *v, v
	}
	if raw := r.URL.Query().Get("stepFree"); raw == "1" || raw == "true" {
		q.StepFree, echo.StepFree = true, true
	} else if raw != "" && raw != "0" && raw != "false" {
		httpapi.Error(w, r, http.StatusBadRequest, "BAD_REQUEST", "Invalid stepFree flag")
		return q, echo, false
	}
	return q, echo, true
}

// mapItineraries converts planner results to public DTOs and attaches the
// enrichments that need catalog lookups: geometry cuts, corridor
// alternatives, and next departures on first boardings.
func (h *Handler) mapItineraries(ctx context.Context, eng *planner.Engine, its []*planner.Itinerary) []Itinerary {
	out := make([]Itinerary, 0, len(its))
	coords := h.stopCoords(ctx, eng, its)
	colors := h.routeColors(ctx, its)
	// Per-leg corridor swaps are a transfer tool: when an itinerary covers
	// the O-D in a single ride, "bisa juga naik koridor X" chips are noise
	// around an answer that needs no choosing — the itinerary list itself
	// already presents the direct corridors.
	needsTransit := !slices.ContainsFunc(its, func(it *planner.Itinerary) bool {
		return it.Transfers <= 0
	})
	for _, pit := range its {
		it := Itinerary{
			Label: pit.Label, Reason: pit.Reason,
			DepartAt:    time.Unix(pit.Depart, 0).UTC(),
			ArriveAt:    time.Unix(pit.Arrive, 0).UTC(),
			DurationSec: pit.Arrive - pit.Depart,
			RideLegs:    pit.RideLegs,
			Transfers:   pit.Transfers,
			WalkM:       pit.WalkM,
			Status:      "scheduled",
		}
		if pit.Estimated {
			it.Status = "estimated"
		}
		for _, pl := range pit.Legs {
			l := h.mapLeg(ctx, eng, pl, coords, colors)
			if l.Type == "walk" {
				it.WalkTransfers++
			} else if needsTransit {
				l.Alternatives = h.legAlternatives(ctx, eng, pl.RouteID, pl.From, pl.To)
			}
			it.Legs = append(it.Legs, l)
		}
		out = append(out, it)
	}
	return out
}

func (h *Handler) mapLeg(ctx context.Context, eng *planner.Engine, pl planner.Leg, coords map[string]generated.ListStopCoordsRow, colors map[string]string) Leg {
	l := Leg{
		Type: pl.Kind,
		From: h.stopRef(eng, pl.From),
		To:   h.stopRef(eng, pl.To),
	}
	dep, arr := time.Unix(pl.Dep, 0).UTC(), time.Unix(pl.Arr, 0).UTC()
	l.DepAt, l.ArrAt = &dep, &arr
	if pl.Kind == "walk" {
		d := float64(pl.DistM)
		l.DistanceM = &d
		return l
	}
	l.RouteID = pl.RouteID.String()
	l.Line = pl.RouteKey
	l.Operator, _ = splitLineKey(pl.RouteKey)
	l.Mode = pl.Mode
	l.Color = colors[pl.RouteID.String()]
	l.Headsign = pl.Headsign
	l.Estimated = pl.Estimated
	l.StationCount = len(pl.Stops) - 1
	for _, s := range pl.Stops {
		l.Stops = append(l.Stops, h.stopRef(eng, s))
	}
	l.Geometry = h.legGeometry(ctx, pl, coords)
	return l
}

func (h *Handler) stopRef(eng *planner.Engine, id pgtype.UUID) StopRef {
	s, ok := eng.Stop(id)
	if !ok {
		return StopRef{ID: id.String()}
	}
	return StopRef{ID: id.String(), Name: s.Name}
}

// attachDepartures fills nextDepartures on the first ride leg of each
// itinerary — upcoming boardings on the same route at the same stop,
// computed from the same snapshot the plan rode.
func (h *Handler) attachDepartures(eng *planner.Engine, itineraries []Itinerary) {
	for i := range itineraries {
		for j := range itineraries[i].Legs {
			l := &itineraries[i].Legs[j]
			if l.Type != "ride" || l.DepAt == nil {
				continue
			}
			var rid, sid pgtype.UUID
			if rid.Scan(l.RouteID) != nil || sid.Scan(l.From.ID) != nil {
				break
			}
			for _, d := range eng.DeparturesAfter(rid, sid, l.DepAt.Unix()-60, maxDepartures) {
				trip := d.TripKey
				l.NextDepartures = append(l.NextDepartures, Departure{
					Time:       time.Unix(d.Unix, 0).In(jakarta).Format("15:04"),
					TripNumber: &trip,
					BoundFor:   d.Headsign,
				})
			}
			break // first ride leg only
		}
	}
}

// legGeometry cuts the ingested route shape along the leg's ridden stops.
// Missing shapes/coords degrade to no geometry — the client draws the
// stop-to-stop polyline.
func (h *Handler) legGeometry(ctx context.Context, pl planner.Leg, coords map[string]generated.ListStopCoordsRow) json.RawMessage {
	lons := make([]float64, 0, len(pl.Stops))
	lats := make([]float64, 0, len(pl.Stops))
	for _, s := range pl.Stops {
		c, ok := coords[s.String()]
		if !ok {
			continue
		}
		lons = append(lons, c.Lon)
		lats = append(lats, c.Lat)
	}
	return h.sliceShape(ctx, pl.RouteID, lons, lats)
}

// routeColors resolves the catalog color of every route the itineraries
// ride — one batched lookup so legs can wear the corridor's published hue.
// A lookup failure degrades to no colors, never a failed plan.
func (h *Handler) routeColors(ctx context.Context, its []*planner.Itinerary) map[string]string {
	seen := map[pgtype.UUID]bool{}
	var ids []pgtype.UUID
	for _, it := range its {
		for _, l := range it.Legs {
			if l.Kind == "ride" && !seen[l.RouteID] {
				seen[l.RouteID] = true
				ids = append(ids, l.RouteID)
			}
		}
	}
	out := map[string]string{}
	if len(ids) == 0 {
		return out
	}
	rows, err := h.store.ListRouteColors(ctx, ids)
	if err != nil {
		return out
	}
	for _, r := range rows {
		if r.Color.Valid {
			out[r.ID.String()] = r.Color.String
		}
	}
	return out
}

// stopCoords fetches coordinates once for every stop the itineraries touch.
func (h *Handler) stopCoords(ctx context.Context, eng *planner.Engine, its []*planner.Itinerary) map[string]generated.ListStopCoordsRow {
	seen := map[pgtype.UUID]bool{}
	var ids []pgtype.UUID
	for _, it := range its {
		for _, l := range it.Legs {
			for _, s := range l.Stops {
				if !seen[s] {
					seen[s] = true
					ids = append(ids, s)
				}
			}
			for _, id := range []pgtype.UUID{l.From, l.To} {
				if !seen[id] {
					seen[id] = true
					ids = append(ids, id)
				}
			}
		}
	}
	out := map[string]generated.ListStopCoordsRow{}
	if len(ids) == 0 {
		return out
	}
	rows, err := h.store.ListStopCoords(ctx, ids)
	if err != nil {
		return out // no geometry is a degrade, never a failed plan
	}
	for _, r := range rows {
		out[r.ID.String()] = r
	}
	return out
}

// fareReference asks the provider for its corridor fare estimate for the
// O-D pair. The fare prices the provider's own plan which may differ from
// the itinerary shown — it is reference context, not per-leg truth, and
// an upstream outage degrades it to nil.
func (h *Handler) fareReference(ctx context.Context, from, to generated.GetStopRow, at *time.Time) *Fare {
	plan, err := h.fares.Fares(ctx, from.ProviderEntityID, to.ProviderEntityID, at)
	if err != nil || plan == nil || (len(plan.Segments) == 0 && plan.TotalFare == nil) {
		return nil
	}
	ids := map[string]struct{}{}
	for _, seg := range plan.Segments {
		if seg.From.ID != "" {
			ids[seg.From.ID] = struct{}{}
		}
		if seg.To.ID != "" {
			ids[seg.To.ID] = struct{}{}
		}
	}
	idList := make([]string, 0, len(ids))
	for id := range ids {
		idList = append(idList, id)
	}
	uuids := map[string]string{}
	if rows, err := h.store.ListStopIDsByProviderEntityIDs(ctx, generated.ListStopIDsByProviderEntityIDsParams{
		Code: providerCode, EntityIds: idList,
	}); err == nil {
		for _, row := range rows {
			uuids[row.ProviderEntityID] = row.ID.String()
		}
	}
	ref := func(s commute.FareStationRef) StopRef {
		return StopRef{ID: uuids[s.ID], Name: s.Name}
	}
	fare := &Fare{Currency: "IDR", Total: plan.TotalFare, Segments: make([]FareSegment, 0, len(plan.Segments))}
	for _, seg := range plan.Segments {
		fare.Segments = append(fare.Segments, FareSegment{
			Operator: seg.Operator,
			From:     ref(seg.From),
			To:       ref(seg.To),
			Amount:   seg.Fare,
		})
	}
	return fare
}

// legAlternatives lists other catalog routes that also carry the leg's
// endpoints — "bisa juga naik koridor 2". A candidate only counts when a
// scheduled trip actually rides from→to in order: the topology table can
// order stops around folds and directions no trip serves. Each entry
// carries that route's own stop slice and shape cut, so a picked
// alternative never borrows the chosen leg's stops to describe a
// different corridor. Catalogue gaps degrade to no alternatives, never a
// failed plan.
func (h *Handler) legAlternatives(ctx context.Context, eng *planner.Engine, routeID, fromID, toID pgtype.UUID) []LegAlternative {
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
		if !eng.RouteServes(row.ID, fromID, toID) {
			continue
		}
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
		if pathKm(lons, lats) > haversineKm(lons[0], lats[0], lons[len(lons)-1], lats[len(lats)-1])*altMaxDetourFactor+altMaxDetourKm {
			continue
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
		if row.Color.Valid {
			alt.Color = row.Color.String
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

// haversineKm is exact enough for a detour bound at Jakarta's latitude —
// equirectangular, one cosine per segment.
func haversineKm(lon1, lat1, lon2, lat2 float64) float64 {
	dx := (lon2 - lon1) * 111.2 * math.Cos(lat1*math.Pi/180)
	dy := (lat2 - lat1) * 111.2
	return math.Hypot(dx, dy)
}

// pathKm sums straight segments over a stop slice — an underestimate of
// the real driven path, which suits a conservative detour bound.
func pathKm(lons, lats []float64) float64 {
	var d float64
	for i := 1; i < len(lons); i++ {
		d += haversineKm(lons[i-1], lats[i-1], lons[i], lats[i])
	}
	return d
}

// splitLineKey turns "MRTJ:M" into operator "MRTJ" and line code "M".
func splitLineKey(key string) (op, code string) {
	i := strings.IndexByte(key, ':')
	if i <= 0 || i == len(key)-1 {
		return "", ""
	}
	return key[:i], key[i+1:]
}

func writePlan(w http.ResponseWriter, r *http.Request, from, to generated.GetStopRow, echo QueryEcho, itineraries []Itinerary, fare *Fare, eng *planner.Engine) {
	httpapi.JSON(w, http.StatusOK, PlanResponse{
		From:          StopRef{ID: from.ID.String(), Name: from.Name},
		To:            StopRef{ID: to.ID.String(), Name: to.Name},
		Query:         echo,
		Itineraries:   itineraries,
		FareReference: fare,
		Source: SourceMeta{
			Provider:    "schedule",
			RequestedAt: time.Now().UTC(),
			SnapshotAt:  eng.BuiltAt().UTC(),
		},
	})
}

func parseTimeParam(v string) (*time.Time, string) {
	if v == "" {
		return nil, ""
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return nil, "bad"
	}
	return &t, ""
}

func parseIntParam(w http.ResponseWriter, r *http.Request, name string, min int) (*int, bool) {
	v := r.URL.Query().Get(name)
	if v == "" {
		return nil, true
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < min {
		httpapi.Error(w, r, http.StatusBadRequest, "BAD_REQUEST", "Invalid "+name)
		return nil, false
	}
	return &n, true
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
