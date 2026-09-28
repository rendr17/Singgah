package planner

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

// Plan answers one journey query with up to 1+maxAlternatives itineraries:
// the earliest-arrival (or latest-departure) optimum plus alternatives
// found by re-running with each of the primary routes excluded — real
// corridor choices, labelled by the metric they actually win.
//
// Alternatives that lose on every axis still surface with
// Label="alternative" — an honest different path, not a decoration.
func (e *Engine) Plan(q Query) ([]*Itinerary, error) {
	fwd := q.DepartAt != nil
	if fwd == (q.ArriveBy != nil) {
		return nil, errors.New("planner: exactly one of DepartAt/ArriveBy is required")
	}
	if q.From == q.To {
		return nil, errors.New("planner: from == to")
	}
	var anchor int64
	if fwd {
		anchor = q.DepartAt.Unix()
	} else {
		anchor = q.ArriveBy.Unix()
	}
	// Window covers the horizon plus overflow margin on the anchored side.
	t0, t1 := anchor-horizonSec, anchor+horizonSec
	sp := e.build(q, t0, t1)

	search := func(exclude map[pgtype.UUID]bool) *Itinerary {
		if fwd {
			return e.searchDepart(q, anchor, sp, exclude)
		}
		return e.searchArriveBy(q, anchor, sp, exclude)
	}

	primary := search(nil)
	if primary == nil {
		return nil, nil
	}
	primary.Label = "fastest"
	primary.Reason = "earliest possible arrival"

	out := []*Itinerary{primary}
	seen := map[string]bool{primary.Signature: true}

	// Exclusion reruns: one per distinct route in the primary itinerary —
	// each surfaces a plan that doesn't touch that corridor.
	var primRoutes []pgtype.UUID
	routeSeen := map[pgtype.UUID]bool{}
	for _, l := range primary.Legs {
		if l.Kind == "ride" && !routeSeen[l.RouteID] {
			routeSeen[l.RouteID] = true
			primRoutes = append(primRoutes, l.RouteID)
		}
	}
	for _, rid := range primRoutes {
		if len(out) > maxAlternatives {
			break
		}
		alt := search(map[pgtype.UUID]bool{rid: true})
		if alt == nil || seen[alt.Signature] {
			continue
		}
		seen[alt.Signature] = true
		out = append(out, alt)
	}

	// Labels come from measured trade-offs, not a hidden score: an
	// alternative earns "fewest_transfers"/"least_walking" only where it
	// strictly beats the primary; otherwise it's a plain alternative.
	rk := map[pgtype.UUID]string{}
	for _, t := range e.trips {
		rk[t.routeID] = t.routeKey
	}
	for i, alt := range out[1:] {
		_ = i
		alt.Label = "alternative"
		alt.Reason = fmt.Sprintf("avoids %s", rk[excludedRoute(alt, primary)])
		if alt.Transfers < primary.Transfers {
			alt.Label = "fewest_transfers"
		} else if alt.WalkM < primary.WalkM {
			alt.Label = "least_walking"
		}
	}
	return out, nil
}

// excludedRoute finds which of the primary's routes a result avoided.
func excludedRoute(alt, primary *Itinerary) pgtype.UUID {
	inAlt := map[pgtype.UUID]bool{}
	for _, l := range alt.Legs {
		if l.Kind == "ride" {
			inAlt[l.RouteID] = true
		}
	}
	for _, l := range primary.Legs {
		if l.Kind == "ride" && !inAlt[l.RouteID] {
			return l.RouteID
		}
	}
	return pgtype.UUID{}
}
