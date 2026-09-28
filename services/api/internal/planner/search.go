package planner

import (
	"container/heap"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// Leg is one hop of an itinerary: a continuous ride on one trip
// (Kind "ride") or a walk between stops (Kind "walk"). Fields stay flat —
// the handler maps them straight onto the API DTO.
type Leg struct {
	Kind     string      // "ride" | "walk"
	From, To pgtype.UUID // ride: board/alight; walk: endpoints
	Dep, Arr int64       // absolute unix
	// ride legs:
	RouteID   pgtype.UUID
	RouteKey  string // "KCI:C" — provider entity id
	Line      string // short name for display
	Mode      string
	Headsign  string
	Stops     []pgtype.UUID // boarded sequence, inclusive
	Estimated bool          // frequency template or derived times
	// walk legs:
	DistM int
	Secs  int64
}

// Itinerary is one end-to-end plan.
type Itinerary struct {
	Legs      []Leg
	Depart    int64 // unix
	Arrive    int64
	RideLegs  int
	Transfers int // rideLegs - 1
	WalkM     int
	Estimated bool   // any leg rode a template or derived times
	Signature string // dedupe key: route sequence + endpoints
	Label     string // fastest | fewest_transfers | least_walking | alternative
	Reason    string // e.g. "avoids KCI:C" — handler renders copy
}

// conn is one ride edge trip i→i+1 materialized onto a service day. tpl
// marks frequency-template connections — dep/arr hold instance 0's times;
// later instances shift by k*headway.
type conn struct {
	trip     int // trips[] index
	i        int // stop index within the trip
	dep, arr int64
	from, to pgtype.UUID
	tpl      bool
	derived  bool
}

type space struct {
	conns  []conn
	depIdx map[pgtype.UUID][]int // conns departing stop, dep-sorted
	arrIdx map[pgtype.UUID][]int // conns arriving stop, arr-sorted
	tpl    map[pgtype.UUID][]int // template conns departing stop
	tplArr map[pgtype.UUID][]int // template conns arriving stop
}

// build materializes connections for every service day overlapping the
// window. DayMask picks the weekday bits; service-day seconds beyond 24h
// keep post-midnight trips on their owning day — never split.
func (e *Engine) build(q Query, t0, t1 int64) *space {
	loc := e.loc
	first := time.Unix(t0, 0).In(loc)
	firstBase := time.Date(first.Year(), first.Month(), first.Day(), 0, 0, 0, 0, loc).Add(-24 * time.Hour)
	last := time.Unix(t1, 0).In(loc)
	lastBase := time.Date(last.Year(), last.Month(), last.Day(), 0, 0, 0, 0, loc)

	sp := &space{
		depIdx: map[pgtype.UUID][]int{},
		arrIdx: map[pgtype.UUID][]int{},
		tpl:    map[pgtype.UUID][]int{},
		tplArr: map[pgtype.UUID][]int{},
	}
	for base := firstBase; !base.After(lastBase); base = base.AddDate(0, 0, 1) {
		baseUnix := base.Unix()
		if baseUnix+32*3600 < t0 || baseUnix > t1 {
			continue
		}
		weekday := int(base.Weekday())          // 0=sun..6=sat
		mask := int16(1 << ((weekday + 6) % 7)) // GTFS bit0=monday
		for i := range e.trips {
			tr := &e.trips[i]
			if len(q.Modes) > 0 && !q.Modes[tr.mode] {
				continue
			}
			s := tr.service
			if s.dayMask&mask == 0 || baseUnix < s.start.Unix() || baseUnix > s.end.Unix() {
				continue
			}
			tpl := tr.freq != nil
			for j := 0; j+1 < len(tr.stops); j++ {
				c := conn{
					trip: i, i: j,
					from: tr.stops[j], to: tr.stops[j+1],
					dep: baseUnix + int64(tr.dep[j]),
					arr: baseUnix + int64(tr.arr[j+1]),
					tpl: tpl,
				}
				if tr.derived[j] || tr.derived[j+1] {
					c.derived = true
				}
				idx := len(sp.conns)
				sp.conns = append(sp.conns, c)
				if tpl {
					sp.tpl[c.from] = append(sp.tpl[c.from], idx)
					sp.tplArr[c.to] = append(sp.tplArr[c.to], idx)
				} else {
					sp.depIdx[c.from] = append(sp.depIdx[c.from], idx)
					sp.arrIdx[c.to] = append(sp.arrIdx[c.to], idx)
				}
			}
		}
	}
	for _, l := range sp.depIdx {
		sort.Slice(l, func(a, b int) bool { return sp.conns[l[a]].dep < sp.conns[l[b]].dep })
	}
	for _, l := range sp.arrIdx {
		sort.Slice(l, func(a, b int) bool { return sp.conns[l[a]].arr < sp.conns[l[b]].arr })
	}
	return sp
}

// nextTplDep computes the smallest instance departure of a template
// connection ≥ req. The frequency window bounds the FIRST stop's
// departure to ≤ end (GTFS semantics).
func (e *Engine) nextTplDep(c conn, req int64) (int64, bool) {
	tr := &e.trips[c.trip]
	f := tr.freq
	h := int64(f.headway)
	dep0First := c.dep - int64(tr.dep[c.i]-tr.dep[0]) // stop0 dep at k=0
	dep0Last := dep0First - int64(tr.dep[0]) + int64(f.end)
	k := (req - c.dep + h - 1) / h // ceil for positive diff
	if k < 0 {
		k = 0
	}
	if dep0First+k*h > dep0Last {
		return 0, false
	}
	return c.dep + k*h, true
}

// prevTplArr is the arrive-by mirror: the largest instance arrival ≤
// bound, or none once the window's first-stop departure is exceeded.
func (e *Engine) prevTplArr(c conn, bound int64) (int64, bool) {
	tr := &e.trips[c.trip]
	f := tr.freq
	h := int64(f.headway)
	dep0First := c.dep - int64(tr.dep[c.i]-tr.dep[0])
	dep0Last := dep0First - int64(tr.dep[0]) + int64(f.end)
	k := (bound - c.arr) / h // floor for positive diff
	if dep0First+k*h > dep0Last {
		k = (dep0Last - dep0First) / h
	}
	if k < 0 || dep0First+k*h > dep0Last {
		return 0, false
	}
	return c.arr + k*h, true
}

// ---------------------------------------------------------------------------
// Label-setting Dijkstra. A label (t, boardings) survives only when
// non-dominated — an earlier arrival with no extra boarding is strictly
// better. Boarding a new trip costs minChangeSec plus one boarding;
// continuing aboard costs neither.
// ---------------------------------------------------------------------------

type label struct {
	t  int64 // arrival (forward) or departure (backward)
	tr int   // ride boardings
}

// dominates: existing label l is at least as good as candidate (t,tr).
func dominates(l label, t int64, tr int, fwd bool) bool {
	if fwd {
		return l.t <= t && l.tr <= tr
	}
	return l.t >= t && l.tr <= tr
}

func strictlyDominates(l label, t int64, tr int, fwd bool) bool {
	return dominates(l, t, tr, fwd) && (l.t != t || l.tr < tr)
}

func dominatedBy(ls []label, t int64, tr int, fwd bool) bool {
	for _, l := range ls {
		if strictlyDominates(l, t, tr, fwd) {
			return true
		}
	}
	return false
}

// addLabel inserts (t,tr); false when an existing label already dominates
// it. Labels the new one dominates are dropped.
func addLabel(ls []label, t int64, tr int, fwd bool) ([]label, bool) {
	for _, l := range ls {
		if dominates(l, t, tr, fwd) {
			return ls, false
		}
	}
	out := ls[:0]
	for _, l := range ls {
		newDominates := fwd && t <= l.t && tr <= l.tr || !fwd && t >= l.t && tr <= l.tr
		if !newDominates {
			out = append(out, l)
		}
	}
	return append(out, label{t, tr}), true
}

type state struct {
	t  int64
	at pgtype.UUID
	vt int // aboard trips[] index, -1 on foot
	tr int // ride boardings so far
}

// pq is a heap over states — min-t for depart searches, max-t for
// arrive-by.
type pq struct {
	items []state
	rev   bool
}

func (p *pq) Len() int { return len(p.items) }
func (p *pq) Less(i, j int) bool {
	if p.rev {
		return p.items[i].t > p.items[j].t
	}
	return p.items[i].t < p.items[j].t
}
func (p *pq) Swap(i, j int) { p.items[i], p.items[j] = p.items[j], p.items[i] }
func (p *pq) Push(x any)    { p.items = append(p.items, x.(state)) }
func (p *pq) Pop() any {
	old := p.items
	s := old[len(old)-1]
	p.items = old[:len(old)-1]
	return s
}

// hop is the reconstruction record for one relaxed state — the connection
// or walk taken plus the times actually used (template instances differ
// from their base times).
type hop struct {
	prevKey  string
	conn     int // conns index, -1 when this hop is a walk
	walk     walkEdge
	dep, arr int64 // absolute times used for this hop
}

func stateKey(s state) string {
	return fmt.Sprintf("%s|%d|%d|%d", uuidStr(s.at), s.vt, s.tr, s.t)
}

func uuidStr(u pgtype.UUID) string {
	b := u.Bytes
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// boardReq is the earliest a connection can be boarded from state s: a
// change allowance applies to every new trip except the very first
// boarding at the origin.
func (e *Engine) boardReq(q Query, s state, c *conn) int64 {
	if c.trip == s.vt {
		return s.t
	}
	if s.tr == 0 && s.vt == -1 && s.at == q.From {
		return s.t // first boarding: depart-at means "ready at From now"
	}
	return s.t + minChangeSec
}

// searchDepart runs the forward earliest-arrival scan.
func (e *Engine) searchDepart(q Query, depart int64, sp *space, exclude map[pgtype.UUID]bool) *Itinerary {
	root := state{t: depart, at: q.From, vt: -1}
	dist := map[pgtype.UUID][]label{q.From: {{t: depart, tr: 0}}}
	parent := map[string]hop{stateKey(root): {conn: -2}} // root marker
	queue := &pq{}
	heap.Push(queue, root)
	end := depart + horizonSec

	for queue.Len() > 0 {
		s := heap.Pop(queue).(state)
		if s.t > end {
			break
		}
		if dominatedBy(dist[s.at], s.t, s.tr, true) {
			continue
		}
		if s.at == q.To {
			return e.assemble(q, s, sp, parent, true)
		}
		if q.StepFree && s.at != q.From && !e.stops[s.at].StepFree {
			continue
		}
		e.relaxDepart(q, s, sp, exclude, dist, parent, queue, end)
	}
	return nil
}

func (e *Engine) relaxDepart(q Query, s state, sp *space, exclude map[pgtype.UUID]bool, dist map[pgtype.UUID][]label, parent map[string]hop, queue *pq, end int64) {
	try := func(c *conn, ci int, dep, arr int64) {
		tr := &e.trips[c.trip]
		if exclude[tr.routeID] {
			return
		}
		if dep < e.boardReq(q, s, c) || dep > end {
			return
		}
		ntr := s.tr
		if c.trip != s.vt {
			ntr++
			if q.MaxTransfers >= 0 && ntr-1 > q.MaxTransfers {
				return
			}
		}
		ns := state{t: arr, at: c.to, vt: c.trip, tr: ntr}
		nl, ok := addLabel(dist[c.to], arr, ntr, true)
		if !ok || len(nl) > maxLabelsPerStop {
			return
		}
		dist[c.to] = nl
		parent[stateKey(ns)] = hop{prevKey: stateKey(s), conn: ci, dep: dep, arr: arr}
		heap.Push(queue, ns)
	}
	for _, ci := range sp.depIdx[s.at] {
		c := &sp.conns[ci]
		if c.dep > end {
			break
		}
		try(c, ci, c.dep, c.arr)
	}
	for _, ci := range sp.tpl[s.at] {
		c := &sp.conns[ci]
		dep, ok := e.nextTplDep(*c, e.boardReq(q, s, c))
		if ok {
			try(c, ci, dep, dep+(c.arr-c.dep))
		}
	}
	for _, w := range e.edges[s.at] {
		if q.MaxWalkM >= 0 && w.distM > q.MaxWalkM {
			continue
		}
		if q.StepFree && !e.stops[w.to].StepFree {
			continue
		}
		ns := state{t: s.t + w.secs, at: w.to, vt: -1, tr: s.tr}
		nl, ok := addLabel(dist[w.to], ns.t, ns.tr, true)
		if !ok {
			continue
		}
		dist[w.to] = nl
		parent[stateKey(ns)] = hop{prevKey: stateKey(s), conn: -1, walk: w, dep: s.t, arr: ns.t}
		heap.Push(queue, ns)
	}
}

// searchArriveBy mirrors searchDepart — scanning connections backward by
// arrival, maximizing the latest feasible departure.
func (e *Engine) searchArriveBy(q Query, arrive int64, sp *space, exclude map[pgtype.UUID]bool) *Itinerary {
	root := state{t: arrive, at: q.To, vt: -1}
	dist := map[pgtype.UUID][]label{q.To: {{t: arrive, tr: 0}}}
	parent := map[string]hop{stateKey(root): {conn: -2}}
	queue := &pq{rev: true}
	heap.Push(queue, root)
	start := arrive - horizonSec

	for queue.Len() > 0 {
		s := heap.Pop(queue).(state)
		if s.t < start {
			break
		}
		if dominatedBy(dist[s.at], s.t, s.tr, false) {
			continue
		}
		if s.at == q.From {
			return e.assemble(q, s, sp, parent, false)
		}
		if q.StepFree && s.at != q.To && !e.stops[s.at].StepFree {
			continue
		}
		l := sp.arrIdx[s.at]
		for k := len(l) - 1; k >= 0; k-- {
			ci := l[k]
			c := &sp.conns[ci]
			if c.arr < start {
				break
			}
			if c.arr <= e.alightReq(q, s, c) {
				e.relaxArriveConn(q, s, sp, exclude, dist, parent, queue, ci, c.dep, c.arr, start)
			}
		}
		for _, ci := range sp.tplArr[s.at] {
			c := &sp.conns[ci]
			arr, ok := e.prevTplArr(*c, e.alightReq(q, s, c))
			if ok {
				e.relaxArriveConn(q, s, sp, exclude, dist, parent, queue, ci, arr-(c.arr-c.dep), arr, start)
			}
		}
		for _, w := range e.edgesIn[s.at] {
			if q.MaxWalkM >= 0 && w.distM > q.MaxWalkM {
				continue
			}
			if q.StepFree && !e.stops[w.from].StepFree {
				continue
			}
			ns := state{t: s.t - w.secs, at: w.from, vt: -1, tr: s.tr}
			nl, ok := addLabel(dist[w.from], ns.t, ns.tr, false)
			if !ok {
				continue
			}
			dist[w.from] = nl
			parent[stateKey(ns)] = hop{prevKey: stateKey(s), conn: -1, walk: w, dep: ns.t, arr: s.t}
			heap.Push(queue, ns)
		}
	}
	return nil
}

// alightReq mirrors boardReq: a connection arriving at s.at is usable when
// it reaches before the state's time minus the change allowance.
func (e *Engine) alightReq(q Query, s state, c *conn) int64 {
	if c.trip == s.vt {
		return s.t
	}
	if s.tr == 0 && s.vt == -1 && s.at == q.To {
		return s.t
	}
	return s.t - minChangeSec
}

func (e *Engine) relaxArriveConn(q Query, s state, sp *space, exclude map[pgtype.UUID]bool, dist map[pgtype.UUID][]label, parent map[string]hop, queue *pq, ci int, dep, arr, start int64) {
	c := &sp.conns[ci]
	tr := &e.trips[c.trip]
	if exclude[tr.routeID] || dep < start {
		return
	}
	ntr := s.tr
	if c.trip != s.vt {
		ntr++
		if q.MaxTransfers >= 0 && ntr-1 > q.MaxTransfers {
			return
		}
	}
	ns := state{t: dep, at: c.from, vt: c.trip, tr: ntr}
	nl, ok := addLabel(dist[c.from], dep, ntr, false)
	if !ok || len(nl) > maxLabelsPerStop {
		return
	}
	dist[c.from] = nl
	parent[stateKey(ns)] = hop{prevKey: stateKey(s), conn: ci, dep: dep, arr: arr}
	heap.Push(queue, ns)
}

// assemble walks the parent chain and groups consecutive connections of
// the same trip into ride legs. Forward searches collect hops
// destination→origin; backward searches already collect origin→dest.
func (e *Engine) assemble(q Query, end state, sp *space, parent map[string]hop, fwd bool) *Itinerary {
	type seg struct {
		conn     int
		walk     walkEdge
		hasWalk  bool
		dep, arr int64
	}
	var segs []seg
	key := stateKey(end)
	for {
		h, ok := parent[key]
		if !ok || h.conn == -2 {
			break
		}
		segs = append(segs, seg{conn: h.conn, dep: h.dep, arr: h.arr, walk: h.walk, hasWalk: h.conn == -1})
		key = h.prevKey
	}
	if fwd {
		for i, j := 0, len(segs)-1; i < j; i, j = i+1, j-1 {
			segs[i], segs[j] = segs[j], segs[i]
		}
	}

	it := &Itinerary{}
	var open *Leg
	openTrip, openI := -1, -1
	closeRide := func() {
		if open == nil {
			return
		}
		it.Legs = append(it.Legs, *open)
		open = nil
	}
	for _, sg := range segs {
		if sg.hasWalk {
			closeRide()
			w := sg.walk
			it.Legs = append(it.Legs, Leg{
				Kind: "walk", From: w.from, To: w.to,
				DistM: w.distM, Secs: w.secs,
				Dep: sg.dep, Arr: sg.arr,
			})
			it.WalkM += w.distM
			continue
		}
		c := &sp.conns[sg.conn]
		tr := &e.trips[c.trip]
		if open != nil && c.trip == openTrip && c.i == openI+1 {
			open.Stops = append(open.Stops, c.to)
			open.To = c.to
			open.Arr = sg.arr
			open.Estimated = open.Estimated || c.derived || c.tpl
			openI = c.i
			continue
		}
		closeRide()
		openTrip, openI = c.trip, c.i
		open = &Leg{
			Kind:    "ride",
			RouteID: tr.routeID, RouteKey: tr.routeKey, Line: tr.lineName,
			Mode: tr.mode, Headsign: tr.headsign,
			From: c.from, To: c.to,
			Stops:     []pgtype.UUID{c.from, c.to},
			Dep:       sg.dep,
			Arr:       sg.arr,
			Estimated: c.derived || c.tpl,
		}
	}
	closeRide()

	if len(it.Legs) == 0 {
		return nil
	}
	it.Depart = it.Legs[0].Dep
	it.Arrive = it.Legs[len(it.Legs)-1].Arr
	for _, l := range it.Legs {
		if l.Kind == "ride" {
			it.RideLegs++
			if l.Estimated {
				it.Estimated = true
			}
		}
	}
	it.Transfers = it.RideLegs - 1

	var sig []string
	for _, l := range it.Legs {
		if l.Kind == "ride" {
			sig = append(sig, l.RouteKey+":"+uuidStr(l.From)+">"+uuidStr(l.To))
		} else {
			sig = append(sig, "walk:"+uuidStr(l.From)+">"+uuidStr(l.To))
		}
	}
	it.Signature = strings.Join(sig, "|")
	return it
}

// walkEdge needs From for assemble and the backward scan.
// (field lives on the shared struct in engine.go)
