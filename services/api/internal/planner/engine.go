// Package planner runs schedule-aware journey planning over the ingested
// services/trips/stop_times/frequencies/transfers tables. It is pure Go —
// the snapshot loads wholesale (the schedule is static) and queries run an
// earliest-arrival Dijkstra over ride connections plus transfer edges.
//
// Semantics preserved from the sources:
//   - frequency trips (TJ GTFS exact_times=0) stay headway templates —
//     boarding computes the next slot lazily, and every leg they produce
//     is marked Estimated;
//   - reconstructed timetable trips keep their Derived flag — a leg that
//     touches a derived stop_time marks the itinerary Estimated;
//   - arrivals the provider never published (commute termini) are already
//     derived rows — nothing invents times downstream.
package planner

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	generated "singgah/services/api/db/generated"
)

// Stop is the planner-facing view of a catalog stop.
type Stop struct {
	ID       pgtype.UUID
	EntityID string
	Name     string
	StepFree bool // an ELEVATOR_* amenity exists in provider metadata
}

type walkEdge struct {
	from, to pgtype.UUID
	distM    int
	secs     int64
}

type serviceRec struct {
	dayMask int16 // bit0=monday … bit6=sunday, GTFS convention
	start   time.Time
	end     time.Time
}

type freqRec struct {
	start, end, headway int32 // service-day seconds
}

type tripRec struct {
	id       pgtype.UUID
	key      string
	routeID  pgtype.UUID
	routeKey string
	lineName string
	mode     string
	headsign string
	service  serviceRec
	stops    []pgtype.UUID
	arr, dep []int32 // service-day seconds; GTFS may exceed 24h
	derived  []bool
	freq     *freqRec
}

// Query is the planning request; exactly one of DepartAt/ArriveBy is set.
type Query struct {
	From, To     pgtype.UUID
	DepartAt     *time.Time
	ArriveBy     *time.Time
	Modes        map[string]bool // empty = all modes
	MaxWalkM     int             // <0 = unlimited
	MaxTransfers int             // <0 = unlimited
	StepFree     bool
}

// Engine is an immutable schedule snapshot; Load rebuilds it and callers
// refresh on a TTL — no mutable package state.
type Engine struct {
	loc     *time.Location
	trips   []tripRec
	stops   map[pgtype.UUID]Stop
	edges   map[pgtype.UUID][]walkEdge // outgoing: from-stop → walks
	edgesIn map[pgtype.UUID][]walkEdge // incoming: to-stop → walks (arrive-by)
	builtAt time.Time
}

const (
	// sameTripBoarding needs no change time; a new trip at the same stop
	// gets a flat interchange allowance (upstream publishes none).
	minChangeSec     int64 = 180
	walkMPerSec            = 1.25 // ~4.5 km/h
	horizonSec             = 6 * 3600
	maxLabelsPerStop       = 8
	maxAlternatives        = 3
)

// Loader supplies the snapshot — satisfied by *generated.Queries.
type Loader interface {
	ListScheduleRows(ctx context.Context) ([]generated.ListScheduleRowsRow, error)
	ListAllFrequencies(ctx context.Context) ([]generated.Frequency, error)
	ListTransferEdges(ctx context.Context) ([]generated.ListTransferEdgesRow, error)
	ListPlannerStops(ctx context.Context) ([]generated.ListPlannerStopsRow, error)
}

// Load builds a fresh snapshot from the schedule tables.
func Load(ctx context.Context, q Loader) (*Engine, error) {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return nil, fmt.Errorf("planner: timezone: %w", err)
	}
	e := &Engine{
		loc:     loc,
		stops:   map[pgtype.UUID]Stop{},
		edges:   map[pgtype.UUID][]walkEdge{},
		edgesIn: map[pgtype.UUID][]walkEdge{},
	}

	rows, err := q.ListScheduleRows(ctx)
	if err != nil {
		return nil, fmt.Errorf("planner: schedule rows: %w", err)
	}
	freqs, err := q.ListAllFrequencies(ctx)
	if err != nil {
		return nil, fmt.Errorf("planner: frequencies: %w", err)
	}
	freqByTrip := map[pgtype.UUID][]generated.Frequency{}
	for _, f := range freqs {
		freqByTrip[f.TripID] = append(freqByTrip[f.TripID], f)
	}

	// Rows come ordered by (trip_id, seq) — fold into trip records.
	var cur *tripRec
	var curID pgtype.UUID
	flush := func() {
		if cur != nil {
			if fs := freqByTrip[cur.id]; len(fs) > 0 {
				f := fs[0]
				cur.freq = &freqRec{start: f.StartSeconds, end: f.EndSeconds, headway: f.HeadwaySeconds}
			}
			e.trips = append(e.trips, *cur)
		}
	}
	for _, r := range rows {
		if cur == nil || r.TripID != curID {
			flush()
			curID = r.TripID
			cur = &tripRec{
				id:       r.TripID,
				key:      r.TripKey,
				routeID:  r.RouteID,
				routeKey: r.RouteKey,
				lineName: lineName(r.ShortName, r.LongName),
				mode:     r.Mode,
				headsign: r.Headsign.String,
				service: serviceRec{
					dayMask: r.DayMask,
					start:   r.StartDate.Time,
					end:     r.EndDate.Time,
				},
			}
		}
		cur.stops = append(cur.stops, r.StopID)
		cur.arr = append(cur.arr, r.ArrivalSeconds)
		cur.dep = append(cur.dep, r.DepartureSeconds)
		cur.derived = append(cur.derived, r.Derived)
	}
	flush()

	stops, err := q.ListPlannerStops(ctx)
	if err != nil {
		return nil, fmt.Errorf("planner: stops: %w", err)
	}
	for _, s := range stops {
		e.stops[s.ID] = Stop{
			ID: s.ID, EntityID: s.ProviderEntityID, Name: s.Name,
			StepFree: hasElevator(s.Metadata),
		}
	}
	edges, err := q.ListTransferEdges(ctx)
	if err != nil {
		return nil, fmt.Errorf("planner: transfers: %w", err)
	}
	for _, t := range edges {
		d := 0
		if t.WalkDistanceM.Valid {
			d = int(t.WalkDistanceM.Int32)
		}
		we := walkEdge{
			from: t.FromStopID, to: t.ToStopID, distM: d,
			secs: int64(math.Ceil(float64(d) / walkMPerSec)),
		}
		e.edges[we.from] = append(e.edges[we.from], we)
		e.edgesIn[we.to] = append(e.edgesIn[we.to], we)
	}
	e.builtAt = time.Now()
	return e, nil
}

// Snapshot stats for diagnostics and tests.
func (e *Engine) TripCount() int { return len(e.trips) }
func (e *Engine) StopCount() int { return len(e.stops) }

// StopName resolves a stop's display name ("" when unknown).
func (e *Engine) StopName(id pgtype.UUID) string { return e.stops[id].Name }

// Stop returns the planner-facing stop record (false when unknown).
func (e *Engine) Stop(id pgtype.UUID) (Stop, bool) {
	s, ok := e.stops[id]
	return s, ok
}

// RouteServes reports whether any scheduled trip on the route serves
// `from` before `to` in ride order — the schedule-truth check behind
// corridor alternatives: published topology alone can't tell whether a
// real trip rides the slice or only its reverse.
func (e *Engine) RouteServes(routeID, from, to pgtype.UUID) bool {
	for _, t := range e.trips {
		if t.routeID != routeID {
			continue
		}
		boarded := false
		for _, s := range t.stops {
			if s == from {
				boarded = true
			} else if boarded && s == to {
				return true
			}
		}
	}
	return false
}

// BuiltAt is when the snapshot was loaded — source freshness for the API.
func (e *Engine) BuiltAt() time.Time { return e.builtAt }

func lineName(short, long pgtype.Text) string {
	if short.Valid && short.String != "" {
		return short.String
	}
	return long.String
}

// hasElevator treats an ELEVATOR_* amenity (paid or unpaid area) as the
// only provider signal of step-free access — conservative and partial.
func hasElevator(meta []byte) bool {
	// metadata.amenities = [{"type": "ELEVATOR_PAID", ...}] — substring
	// match avoids a jsonb struct dependency for one flag.
	return bytes.Contains(meta, []byte("ELEVATOR_"))
}
