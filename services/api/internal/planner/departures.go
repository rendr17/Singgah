package planner

import (
	"sort"

	"github.com/jackc/pgx/v5/pgtype"
)

// Departure is one scheduled boarding at a stop — the engine's answer to
// "kapan berangkat", replacing the upstream timetable call the provider
// never offered for most operators.
type Departure struct {
	Unix      int64
	TripKey   string
	TripID    pgtype.UUID // canonical trip — join key for realtime updates
	Headsign  string
	Estimated bool // frequency template or derived times
	RouteID   pgtype.UUID
	RouteKey  string // "KCI:C" — provider entity id of the route
	Mode      string
}

// DeparturesBetween lists every boarding at stop inside [from, to) across
// all routes, soonest first — the station board view. Concrete trips emit
// each departure; frequency templates emit every headway slot in range,
// all Estimated. A small dedicated space is built per call; at this fleet
// size one extra scan is cheap.
func (e *Engine) DeparturesBetween(stop pgtype.UUID, from, to int64) []Departure {
	sp := e.build(Query{MaxWalkM: -1, MaxTransfers: -1}, from-3600, to)
	var out []Departure
	for _, ci := range sp.depIdx[stop] {
		c := &sp.conns[ci]
		if c.dep < from || c.dep >= to {
			continue
		}
		out = append(out, e.departure(c, c.dep))
	}
	for _, ci := range sp.tpl[stop] {
		c := &sp.conns[ci]
		tr := &e.trips[c.trip]
		f := tr.freq
		h := int64(f.headway)
		dep0First := c.dep - int64(tr.dep[c.i]-tr.dep[0])
		dep0Last := dep0First - int64(tr.dep[0]) + int64(f.end)
		k := (from - c.dep + h - 1) / h // first slot ≥ from
		if k < 0 {
			k = 0
		}
		for ; dep0First+k*h <= dep0Last; k++ {
			dep := c.dep + k*h
			if dep >= to {
				break
			}
			out = append(out, e.departure(c, dep))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Unix < out[j].Unix })
	return out
}

// DeparturesAfter lists upcoming boardings at stop on routeID, soonest
// first — the per-route nextDepartures attached to planned legs.
func (e *Engine) DeparturesAfter(routeID, stop pgtype.UUID, after int64, limit int) []Departure {
	seen := map[int64]bool{}
	var out []Departure
	for _, d := range e.DeparturesBetween(stop, after, after+departuresWindowSec) {
		if d.RouteID != routeID || seen[d.Unix] {
			continue
		}
		seen[d.Unix] = true
		out = append(out, d)
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (e *Engine) departure(c *conn, unix int64) Departure {
	tr := &e.trips[c.trip]
	return Departure{
		Unix:      unix,
		TripKey:   tr.key,
		TripID:    tr.id,
		Headsign:  tr.headsign,
		Estimated: c.derived || c.tpl,
		RouteID:   tr.routeID,
		RouteKey:  tr.routeKey,
		Mode:      tr.mode,
	}
}

const departuresWindowSec = 3 * 3600
