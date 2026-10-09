// Departures board: GET /stations/{id}/departures. The source is the same
// in-house schedule snapshot the journey planner rides (GTFS + reconstructed
// timetables), so the board is always labelled "scheduled" — never live —
// and per-departure `estimated` marks frequency/derived times (docs/09).
package catalog

import (
	"context"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	generated "singgah/services/api/db/generated"
	"singgah/services/api/internal/http/response"
	"singgah/services/api/internal/planner"
)

// jakarta is WIB (UTC+7) permanently — no DST since 1964, so a fixed zone is
// exact and needs no tzdata on the host.
var jakarta = time.FixedZone("Asia/Jakarta", 7*60*60)

// departuresWindowDefault answers "kapan berangkat" without shipping a whole
// service day; departuresLookback keeps the just-missed boarding so a
// direction can show "lalu 23.57". Minutes.
const (
	departuresWindowDefault = 240
	departuresWindowMax     = 24 * 60
	departuresLookback      = 120
)

// GET /stations/{id}/departures?window=
func (h *Handler) getDepartures(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r)
	if !ok {
		return
	}
	window, ok := parseWindow(w, r)
	if !ok {
		return
	}
	stop, err := h.store.GetStop(r.Context(), id)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if h.engine == nil {
		response.Error(w, r, http.StatusServiceUnavailable, "PLANNER_UNAVAILABLE",
			"Data jadwal sedang tidak tersedia")
		return
	}
	eng, err := h.engine.Get(r.Context())
	if err != nil {
		response.Error(w, r, http.StatusServiceUnavailable, "PLANNER_UNAVAILABLE",
			"Data jadwal sedang tidak tersedia")
		return
	}

	anchor := h.now()
	from := anchor.Add(-departuresLookback * time.Minute).Unix()
	to := anchor.Add(time.Duration(window) * time.Minute).Unix()
	board := h.buildBoard(r.Context(), stop, eng,
		eng.DeparturesBetween(id, from, to), anchor.Unix(), window)
	response.JSON(w, http.StatusOK, map[string]any{"departures": board})
}

// buildBoard groups snapshot departures by route and headsign, keeps the
// latest already-gone boarding per direction as previousDeparture, and
// orders groups by soonest upcoming departure.
func (h *Handler) buildBoard(
	ctx context.Context,
	stop generated.GetStopRow,
	eng *planner.Engine,
	deps []planner.Departure,
	anchorUnix int64,
	window int,
) StationDepartures {
	type cand struct {
		d Departure
		u int64 // unix — HH:MM strings can't order across midnight
	}
	type dirAgg struct {
		next []cand
		prev *cand
	}
	type lineAgg struct {
		routeID pgtype.UUID
		dirs    map[string]*dirAgg
	}
	lineOrder := []string{}
	lines := map[string]*lineAgg{}
	seen := map[string]bool{}
	for _, d := range deps {
		la := lines[d.RouteKey]
		if la == nil {
			la = &lineAgg{routeID: d.RouteID, dirs: map[string]*dirAgg{}}
			lines[d.RouteKey] = la
			lineOrder = append(lineOrder, d.RouteKey)
		}
		dir := la.dirs[d.Headsign]
		if dir == nil {
			dir = &dirAgg{}
			la.dirs[d.Headsign] = dir
		}
		key := d.RouteKey + "|" + d.Headsign + "|" + strconv.FormatInt(d.Unix, 10)
		if seen[key] {
			continue
		}
		seen[key] = true
		trip := d.TripKey
		dep := Departure{
			Time:       time.Unix(d.Unix, 0).In(jakarta).Format("15:04"),
			TripNumber: &trip,
			BoundFor:   d.Headsign,
			Estimated:  d.Estimated,
		}
		// Trip-updates annotation — the feed's stop ids are provider ids, so
		// the join compares against this station's own provider_entity_id.
		// A canceled trip stays on the board flagged, not silently dropped.
		if h.tripUpdates != nil && d.TripID.Valid {
			if delay, canceled, found := h.tripUpdates.DelayAt(d.TripID.String(), stop.ProviderEntityID); found {
				if canceled {
					dep.Canceled = true
				} else {
					dep.DelaySec = &delay
				}
			}
		}
		c := cand{d: dep, u: d.Unix}
		if d.Unix < anchorUnix {
			if dir.prev == nil || dir.prev.u < c.u {
				dir.prev = &c
			}
			continue
		}
		dir.next = append(dir.next, c)
	}

	// nextMin is minutes until the group's soonest upcoming departure; groups
	// with none sort last, alphabetically by headsign.
	nextMin := func(dir *dirAgg) int64 {
		if len(dir.next) == 0 {
			return 1 << 40
		}
		var m int64 = 1 << 40
		for _, c := range dir.next {
			if c.u < m {
				m = c.u
			}
		}
		return m
	}
	snap := eng.BuiltAt().UTC()
	board := StationDepartures{
		Station:       stopRef(stop.ID, stop.Name, stop.Code, stop.Lon, stop.Lat),
		Status:        "scheduled",
		WindowMinutes: window,
		Lines:         make([]DepartureLine, 0, len(lines)),
		Source:        DepartureSource{Provider: "schedule", RequestedAt: h.now().UTC(), SnapshotAt: &snap},
	}
	// The updates feed's own health travels with the board — a dead feed
	// degrades the whole surface, delays are last-known not silent absence.
	if h.updatesPoller != nil {
		s := h.updatesPoller.Status()
		board.Realtime = &s
	}
	// Soonest-departure-first ordering across lines; lines with no upcoming
	// boarding sort last, alphabetically.
	lineMin := map[string]int64{}
	for _, rk := range lineOrder {
		m := int64(1 << 40)
		for _, dir := range lines[rk].dirs {
			if nm := nextMin(dir); nm < m {
				m = nm
			}
		}
		lineMin[rk] = m
	}
	sort.Slice(lineOrder, func(a, b int) bool {
		if ma, mb := lineMin[lineOrder[a]], lineMin[lineOrder[b]]; ma != mb {
			return ma < mb
		}
		return lineCode(lineOrder[a]) < lineCode(lineOrder[b])
	})

	routeCache := map[pgtype.UUID]*RouteRef{}
	for _, rk := range lineOrder {
		la := lines[rk]
		names := make([]string, 0, len(la.dirs))
		for hs := range la.dirs {
			names = append(names, hs)
		}
		sort.Slice(names, func(a, b int) bool {
			if ma, mb := nextMin(la.dirs[names[a]]), nextMin(la.dirs[names[b]]); ma != mb {
				return ma < mb
			}
			return names[a] < names[b]
		})
		line := DepartureLine{
			LineCode:   lineCode(rk),
			Route:      h.routeFor(ctx, la.routeID, routeCache),
			Directions: make([]DepartureDirection, 0, len(names)),
		}
		for _, hs := range names {
			dir := la.dirs[hs]
			sort.Slice(dir.next, func(a, b int) bool { return dir.next[a].u < dir.next[b].u })
			out := DepartureDirection{
				BoundFor:   hs,
				Departures: make([]Departure, 0, len(dir.next)),
			}
			if dir.prev != nil {
				out.PreviousDeparture = &dir.prev.d
			}
			for _, c := range dir.next {
				out.Departures = append(out.Departures, c.d)
			}
			line.Directions = append(line.Directions, out)
		}
		board.Lines = append(board.Lines, line)
	}
	return board
}

// routeFor resolves a departure's canonical route id to its RouteRef; a
// miss stays nil rather than failing the board.
func (h *Handler) routeFor(ctx context.Context, routeID pgtype.UUID, cache map[pgtype.UUID]*RouteRef) *RouteRef {
	if r, seen := cache[routeID]; seen {
		return r
	}
	var out *RouteRef
	if row, err := h.store.GetRoute(ctx, routeID); err == nil {
		r := routeRef(row.ID, row.ShortName, row.LongName, row.Mode, row.Color, row.AgencyCode, row.AgencyName)
		out = &r
	}
	cache[routeID] = out
	return out
}

// lineCode is the display half of "KCI:C" — the provider's line code.
func lineCode(routeKey string) string {
	for i := len(routeKey) - 1; i >= 0; i-- {
		if routeKey[i] == ':' {
			return routeKey[i+1:]
		}
	}
	return routeKey
}

func parseWindow(w http.ResponseWriter, r *http.Request) (int, bool) {
	v := r.URL.Query().Get("window")
	if v == "" {
		return departuresWindowDefault, true
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 15 || n > departuresWindowMax {
		response.Error(w, r, http.StatusBadRequest, "BAD_REQUEST", "Invalid window")
		return 0, false
	}
	return n, true
}
