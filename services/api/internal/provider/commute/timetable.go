package commute

import (
	"fmt"
	"sort"
	"strings"
)

// TopoStop is one station on a line, in declared route_stops order.
type TopoStop struct {
	EntityID string // "KCI-SUD"
	Name     string
}

// LineTopology is the canonical stop sequence of one line — what timetable
// entries are reconstructed against.
type LineTopology struct {
	RouteKey string // "KCI:C" — canonical route provider_entity_id
	Stops    []TopoStop
}

// ReconstructedStop is one inferred stop_time. Derived marks a time the
// provider never published — a terminus arrival estimated from the reverse
// segment, or a stop grouped by heuristic chaining.
type ReconstructedStop struct {
	StopEntityID string
	ArrivalSec   int
	DepartureSec int
	Derived      bool
}

// ReconstructedTrip is one inferred run from station timetables.
type ReconstructedTrip struct {
	RouteKey  string
	EntityKey string // unique within the sweep — not necessarily upstream's
	Headsign  string // boundFor, as published
	Direction int    // 0 = topo seq ascending, 1 = descending, -1 = unknown
	Stops     []ReconstructedStop
	Derived   bool
}

// BuildResult carries reconstructed trips plus every reject — reconstruction
// never fails silently on odd upstream data.
type BuildResult struct {
	Trips        []ReconstructedTrip
	Rejections   []string
	SkippedStops int // entries for stations off their line's topology
}

// BuildTrips reconstructs runs from per-station timetable entries.
//
// Two upstream shapes exist (verified 26 Sep 2026):
//   - run identity: tripNumber is a real per-run id (KCI train numbers,
//     MRTJ/LRTJBDB/APCGK generated run ids) — entries chain by
//     (route, tripNumber, boundFor);
//   - per-entry ids: tripNumber equals the entry id (LRTJ) — no run
//     identity, so runs are chained greedily by direction + plausible
//     spacing and marked Derived.
//
// estimatedArrival semantics differ per operator: KCI publishes the
// terminus arrival (usable when > departure); other operators echo the
// departure or 00:00 — unusable, so their terminus arrivals are estimated
// from the reverse-direction segment and marked Derived.
func BuildTrips(entries []TimetableEntry, topo map[string]LineTopology) BuildResult {
	var res BuildResult

	type runGroup struct {
		routeKey string
		boundFor string
		entries  []TimetableEntry
	}
	runs := map[string]*runGroup{}   // chainable: (route, tripNumber, boundFor)
	chains := map[string]*runGroup{} // synthetic: (route, boundFor)
	order := []string{}
	chainOrder := []string{}

	for _, e := range entries {
		op, _, ok := SplitStationID(e.StationID)
		if !ok {
			res.Rejections = append(res.Rejections, e.ID+": malformed station id")
			continue
		}
		routeKey := op + ":" + e.LineCode
		if _, ok := topo[routeKey]; !ok {
			res.SkippedStops++
			continue
		}
		// A tripNumber prefixed with the station's own id is a station-local
		// bookkeeping key, not a run id — LRTJ emits
		// "{station}-{HH:MM}-{DIR}-BOUND" (with a sometimes-wrong direction
		// token at termini), which never repeats across stations.
		chainable := e.TripNumber != nil && *e.TripNumber != "" &&
			!strings.HasPrefix(*e.TripNumber, e.StationID+"-")
		m := chains
		tripKey := e.BoundFor
		if chainable {
			m = runs
			tripKey = *e.TripNumber + "|" + e.BoundFor
		}
		k := routeKey + "|" + tripKey
		g, ok := m[k]
		if !ok {
			g = &runGroup{routeKey: routeKey, boundFor: e.BoundFor}
			m[k] = g
			if chainable {
				order = append(order, k)
			} else {
				chainOrder = append(chainOrder, k)
			}
		}
		g.entries = append(g.entries, e)
	}

	var trips []ReconstructedTrip
	for _, k := range order {
		g := runs[k]
		trip, rej, skipped := buildRun(g.entries, topo[g.routeKey], g.routeKey, k, g.boundFor)
		res.SkippedStops += skipped
		if rej != "" {
			res.Rejections = append(res.Rejections, rej)
			continue
		}
		trips = append(trips, trip)
	}

	// Median observed first-segment runtimes per (route, first-stop) — the
	// reverse-direction view of inbound terminus arrivals.
	segEst := segmentEstimates(trips)
	for i := range trips {
		appendTerminus(&trips[i], topo[trips[i].RouteKey], segEst)
	}

	for _, k := range chainOrder {
		g := chains[k]
		trips = append(trips, chainRuns(g.entries, topo[g.routeKey], g.routeKey, g.boundFor, segEst, &res)...)
	}

	sort.Slice(trips, func(i, j int) bool {
		a, b := trips[i], trips[j]
		if a.RouteKey != b.RouteKey {
			return a.RouteKey < b.RouteKey
		}
		return firstDep(a) < firstDep(b)
	})
	res.Trips = trips
	return res
}

// buildRun folds entries sharing one tripNumber into a time-ordered trip.
// Stations are ordered by published departure — the trip serves exactly the
// stations that listed it (skips stay skips; express trains exist).
func buildRun(entries []TimetableEntry, topo LineTopology, routeKey, groupKey, boundFor string) (ReconstructedTrip, string, int) {
	sorted := append([]TimetableEntry(nil), entries...)
	sort.Slice(sorted, func(i, j int) bool {
		di, dj := departureSec(sorted[i]), departureSec(sorted[j])
		if di != dj {
			return di < dj
		}
		return sorted[i].StationID < sorted[j].StationID
	})

	seqOf := seqIndex(topo)
	skipped := 0
	trip := ReconstructedTrip{
		RouteKey:  routeKey,
		EntityKey: routeKey + ":" + groupKey[len(routeKey)+1:],
		Headsign:  boundFor,
		Direction: -1,
	}
	seen := map[string]bool{}
	usableArrival := -1
	for _, e := range sorted {
		if seen[e.StationID] {
			continue
		}
		seen[e.StationID] = true
		if _, ok := seqOf[e.StationID]; !ok {
			skipped++
			continue
		}
		dep := departureSec(e)
		trip.Stops = append(trip.Stops, ReconstructedStop{
			StopEntityID: e.StationID,
			ArrivalSec:   dep,
			DepartureSec: dep,
		})
		if arr := arrivalSec(e); arr > dep {
			usableArrival = arr // KCI-style published terminus arrival
		}
	}
	// A published terminus counts as a served stop — append before the
	// minimum-size check (a board at the trip's origin alone is one
	// published stop + the terminus the provider named).
	if usableArrival > 0 {
		appendPublishedTerminus(&trip, topo, usableArrival)
	}
	if len(trip.Stops) < 2 {
		return trip, fmt.Sprintf("%s: fewer than 2 served stations", groupKey), skipped
	}
	trip.Direction, _ = travelDirection(trip, topo, boundFor)
	return trip, "", skipped
}

// appendPublishedTerminus adds the boundFor station with the arrival the
// provider published — not derived.
func appendPublishedTerminus(trip *ReconstructedTrip, topo LineTopology, arrivalSec int) {
	idx, ok := resolveBoundFor(trip.Headsign, topo.Stops)
	if !ok || topo.Stops[idx].EntityID == trip.Stops[len(trip.Stops)-1].StopEntityID {
		return
	}
	trip.Stops = append(trip.Stops, ReconstructedStop{
		StopEntityID: topo.Stops[idx].EntityID,
		ArrivalSec:   arrivalSec,
		DepartureSec: arrivalSec,
	})
}

// appendTerminus fills the terminus for trips lacking a published
// terminus arrival. The estimate is the reverse-direction segment median
// for that terminus, falling back to the trip's own observed pacing — and
// it is only applied when the terminus is adjacent to the last published
// stop, because one segment's runtime cannot cover stations in between.
// No adjacent estimate means an honest truncation, not an invented time.
func appendTerminus(trip *ReconstructedTrip, topo LineTopology, segEst map[string]int) {
	idx, ok := resolveBoundFor(trip.Headsign, topo.Stops)
	if !ok {
		return
	}
	term := topo.Stops[idx]
	last := trip.Stops[len(trip.Stops)-1]
	if term.EntityID == last.StopEntityID {
		return
	}
	seqOf := seqIndex(topo)
	if a, b := seqOf[term.EntityID], seqOf[last.StopEntityID]; a-b < -1 || a-b > 1 {
		return // terminus not adjacent — the gap is more than one segment
	}
	est, ok := segEst[trip.RouteKey+"|"+term.EntityID]
	if !ok {
		est, ok = ownPace(trip.Stops)
	}
	if !ok {
		return
	}
	arr := last.DepartureSec + est
	trip.Stops = append(trip.Stops, ReconstructedStop{
		StopEntityID: term.EntityID,
		ArrivalSec:   arr,
		DepartureSec: arr,
		Derived:      true,
	})
	trip.Derived = true
}

// ownPace medians a trip's observed consecutive departures — the fallback
// estimate when no reverse-direction segment exists (e.g. lines where every
// trip was chained heuristically).
func ownPace(stops []ReconstructedStop) (int, bool) {
	var diffs []int
	for i := 1; i < len(stops); i++ {
		d := stops[i].DepartureSec - stops[i-1].DepartureSec
		if d > 0 && d < 1800 {
			diffs = append(diffs, d)
		}
	}
	if len(diffs) == 0 {
		return 0, false
	}
	sort.Ints(diffs)
	return diffs[len(diffs)/2], true
}

// segmentEstimates medians the observed first-segment runtimes of trips
// departing each stop — the reverse-direction view of inbound arrivals
// there.
func segmentEstimates(trips []ReconstructedTrip) map[string]int {
	obs := map[string][]int{}
	for _, t := range trips {
		if len(t.Stops) < 2 {
			continue
		}
		d := t.Stops[1].DepartureSec - t.Stops[0].DepartureSec
		if d > 0 && d < 1800 {
			k := t.RouteKey + "|" + t.Stops[0].StopEntityID
			obs[k] = append(obs[k], d)
		}
	}
	out := map[string]int{}
	for k, ds := range obs {
		sort.Ints(ds)
		out[k] = ds[len(ds)/2]
	}
	return out
}

// chainRuns reconstructs trips whose upstream ids carry no run identity —
// the LRTJ shape where tripNumber equals the per-station entry id.
//
// Each side of the resolved terminus forms an independent direction: below
// the terminus, trains ascend; above it, they descend. A chain starts at
// the earliest unused departure and consumes at most one departure per
// station inside a plausible spacing window; it ends honestly where the
// window fails. Every stop is Derived — times are published, the grouping
// is inference.
func chainRuns(entries []TimetableEntry, topo LineTopology, routeKey, boundFor string, segEst map[string]int, res *BuildResult) []ReconstructedTrip {
	termIdx, ok := resolveBoundFor(boundFor, topo.Stops)
	if !ok {
		res.Rejections = append(res.Rejections, fmt.Sprintf("%s: boundFor %q unresolvable", routeKey, boundFor))
		return nil
	}
	pos := map[string]int{}
	for i, s := range topo.Stops {
		pos[s.EntityID] = i
	}

	type depList struct {
		times []int
		used  []bool
	}
	// sides: -1 = station below the terminus (ascending travel),
	//        +1 = above (descending travel).
	sideDeps := map[int]map[string]*depList{-1: {}, 1: {}}
	for _, e := range entries {
		p, ok := pos[e.StationID]
		if !ok || p == termIdx {
			res.SkippedStops++
			continue
		}
		side := -1
		if p > termIdx {
			side = 1
		}
		l := sideDeps[side][e.StationID]
		if l == nil {
			l = &depList{}
			sideDeps[side][e.StationID] = l
		}
		l.times = append(l.times, departureSec(e))
	}

	var out []ReconstructedTrip
	for _, side := range []int{-1, 1} {
		deps := sideDeps[side]
		if len(deps) == 0 {
			continue
		}
		// Travel order: station positions moving toward the terminus.
		travel := make([]TopoStop, 0, len(topo.Stops))
		for i, s := range topo.Stops {
			if side < 0 && i < termIdx || side > 0 && i > termIdx {
				travel = append(travel, s)
			}
		}
		if side > 0 {
			for i, j := 0, len(travel)-1; i < j; i, j = i+1, j-1 {
				travel[i], travel[j] = travel[j], travel[i]
			}
		}
		for _, l := range deps {
			sort.Ints(l.times)
			l.used = make([]bool, len(l.times))
		}

		const (
			minGap = 30  // the next stop can't be under ~30 s away
			maxGap = 480 // beyond ~8 min the departure belongs to another run
		)
		for {
			start := ""
			startT := -1
			for _, s := range travel {
				l := deps[s.EntityID]
				if l == nil {
					continue
				}
				for i, t := range l.times {
					if !l.used[i] {
						start, startT = s.EntityID, t
						break
					}
				}
				if startT >= 0 {
					break
				}
			}
			if startT < 0 {
				break
			}

			dir := 0
			if side > 0 {
				dir = 1
			}
			trip := ReconstructedTrip{
				RouteKey:  routeKey,
				Headsign:  boundFor,
				Direction: dir,
				Derived:   true,
				Stops: []ReconstructedStop{{
					StopEntityID: start, ArrivalSec: startT,
					DepartureSec: startT, Derived: true,
				}},
			}
			lastT := startT
			pastStart := false
			for _, s := range travel {
				if s.EntityID == start {
					pastStart = true
					continue
				}
				if !pastStart {
					continue
				}
				l := deps[s.EntityID]
				if l == nil {
					break // station without departures ends the run
				}
				picked := -1
				for i, t := range l.times {
					if l.used[i] || t-lastT < minGap {
						continue
					}
					if t-lastT > maxGap {
						break // sorted — every later departure is another run's
					}
					picked = i
					break
				}
				if picked < 0 {
					break
				}
				l.used[picked] = true
				lastT = l.times[picked]
				trip.Stops = append(trip.Stops, ReconstructedStop{
					StopEntityID: s.EntityID, ArrivalSec: lastT,
					DepartureSec: lastT, Derived: true,
				})
			}
			for i, t := range deps[start].times {
				if t == startT && !deps[start].used[i] {
					deps[start].used[i] = true
					break
				}
			}

			if len(trip.Stops) < 2 {
				res.Rejections = append(res.Rejections,
					fmt.Sprintf("%s %q@%d: chain died after 1 station", routeKey, boundFor, startT))
				continue
			}
			trip.EntityKey = fmt.Sprintf("%s:%s@%d", routeKey, boundFor, trip.Stops[0].DepartureSec)
			appendTerminus(&trip, topo, segEst)
			out = append(out, trip)
		}
	}
	return out
}

// travelDirection compares the resolved terminus position with the first
// served stop — 0 when the ride advances seq, 1 when it recedes.
func travelDirection(trip ReconstructedTrip, topo LineTopology, boundFor string) (int, bool) {
	idx, ok := resolveBoundFor(boundFor, topo.Stops)
	if !ok {
		return -1, false
	}
	seqOf := seqIndex(topo)
	first, ok := seqOf[trip.Stops[0].StopEntityID]
	if !ok {
		return -1, false
	}
	if idx >= first {
		return 0, true
	}
	return 1, true
}

func seqIndex(topo LineTopology) map[string]int {
	m := make(map[string]int, len(topo.Stops))
	for i, s := range topo.Stops {
		m[s.EntityID] = i
	}
	return m
}

// resolveBoundFor matches a published destination name onto the line's
// stations — exact first, then longest containment ("Lebak Bulus Bank
// Syariah Indonesia" lands on station "Lebak Bulus").
func resolveBoundFor(boundFor string, stops []TopoStop) (int, bool) {
	norm := func(s string) string { return strings.Join(strings.Fields(strings.ToLower(s)), " ") }
	bf := norm(boundFor)
	for i, s := range stops {
		if norm(s.Name) == bf {
			return i, true
		}
	}
	best, bestLen := -1, 0
	for i, s := range stops {
		n := norm(s.Name)
		if n == "" {
			continue
		}
		if strings.Contains(bf, n) || strings.Contains(n, bf) {
			if len(n) > bestLen {
				best, bestLen = i, len(n)
			}
		}
	}
	if best >= 0 {
		return best, true
	}
	return -1, false
}

// SplitStationID parses the provider's "{operator}-{code}" station id —
// the same shape the catalog's provider_entity_id keeps.
func SplitStationID(id string) (op, code string, ok bool) {
	i := strings.IndexByte(id, '-')
	if i <= 0 || i == len(id)-1 {
		return "", "", false
	}
	return id[:i], id[i+1:], true
}

func departureSec(e TimetableEntry) int {
	sec, _ := hhmmss(e.EstimatedDeparture)
	return sec
}

func arrivalSec(e TimetableEntry) int {
	sec, _ := hhmmss(e.EstimatedArrival)
	return sec
}

// hhmmss parses "HH:MM:SS" (provider wall clock, always < 24 h observed).
func hhmmss(s string) (int, bool) {
	var h, m, sec int
	if _, err := fmt.Sscanf(s, "%d:%d:%d", &h, &m, &sec); err != nil {
		return 0, false
	}
	if m > 59 || sec > 59 {
		return 0, false
	}
	return h*3600 + m*60 + sec, true
}

func firstDep(t ReconstructedTrip) int {
	if len(t.Stops) == 0 {
		return 0
	}
	return t.Stops[0].DepartureSec
}
