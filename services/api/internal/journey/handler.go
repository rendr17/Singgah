package journey

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	generated "singgah/services/api/db/generated"
	httpapi "singgah/services/api/internal/http/response"
	"singgah/services/api/internal/provider/commute"
)

const providerCode = "commute"

type Handler struct {
	store   Store
	planner FarePlanner
}

func NewHandler(store Store, planner FarePlanner) *Handler {
	return &Handler{store: store, planner: planner}
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

	plan, err := h.planner.Fares(r.Context(), from.ProviderEntityID, to.ProviderEntityID)
	if errors.Is(err, commute.ErrStationUnknown) {
		writePlan(w, r, from, to, nil)
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
	writePlan(w, r, from, to, itinerary)
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
			Code:    providerCode,
			Column2: idList,
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

func writePlan(w http.ResponseWriter, r *http.Request, from, to generated.GetStopRow, itin *Itinerary) {
	httpapi.JSON(w, http.StatusOK, PlanResponse{
		From:      StopRef{ID: from.ID.String(), Name: from.Name},
		To:        StopRef{ID: to.ID.String(), Name: to.Name},
		Itinerary: itin,
		Source:    SourceMeta{Provider: providerCode, RequestedAt: time.Now().UTC()},
	})
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
