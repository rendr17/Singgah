// Departures board: GET /stations/{id}/departures. The source is the
// provider's static timetable, so the board is always labelled "scheduled" —
// never live (docs/09 transit truth rules).
package catalog

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	generated "singgah/services/api/db/generated"
	"singgah/services/api/internal/http/response"
	"singgah/services/api/internal/provider/commute"
)

const providerCode = "commute"

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
	op, code := splitProviderRef(stop.ProviderEntityID)
	if stop.ProviderCode != providerCode || op == "" || h.tt == nil {
		response.Error(w, r, http.StatusUnprocessableEntity, "PROVIDER_UNSUPPORTED",
			"Jadwal tidak tersedia untuk penyedia data stasiun ini")
		return
	}

	anchor := h.now().In(jakarta)
	from, to := timetableRange(anchor, window)
	entries, err := h.tt.Timetable(r.Context(), op, code, from, to)
	if errors.Is(err, commute.ErrStationUnknown) {
		// The station exists in our catalog but upstream knows no timetable —
		// an honest empty board, not a failure.
		entries = nil
	} else if err != nil {
		response.Error(w, r, http.StatusBadGateway, "PROVIDER_UNAVAILABLE",
			"Jadwal keberangkatan sedang tidak tersedia")
		return
	}

	board := h.buildBoard(r.Context(), stop, op, entries, anchor, window)
	response.JSON(w, http.StatusOK, map[string]any{"departures": board})
}

// timetableRange picks the upstream HH:MM bounds. Once lookback+window spans
// a day, ask for the whole service day instead of a wrapped range.
func timetableRange(anchor time.Time, window int) (from, to string) {
	if departuresLookback+window >= departuresWindowMax {
		return "00:00", "23:59"
	}
	return anchor.Add(-departuresLookback * time.Minute).Format("15:04"),
		anchor.Add(time.Duration(window) * time.Minute).Format("15:04")
}

// buildBoard groups entries by line and direction, filters them against the
// real minute window (upstream rounds bounds to whole hours), and orders
// groups by soonest upcoming departure.
func (h *Handler) buildBoard(
	ctx context.Context,
	stop generated.GetStopRow,
	op string,
	entries []commute.TimetableEntry,
	anchor time.Time,
	window int,
) StationDepartures {
	anchorMin := anchor.Hour()*60 + anchor.Minute()

	type cand struct {
		d Departure
		m int // minutes from anchor
	}
	type dirAgg struct {
		next    []cand
		prev    *Departure
		prevMin int // minutes since the past boarding
	}
	lineOrder := []string{}
	lines := map[string]map[string]*dirAgg{}
	for _, e := range entries {
		depMin, ok := minutesOfDay(e.EstimatedDeparture)
		if !ok || e.LineCode == "" {
			continue
		}
		d := (depMin - anchorMin + 1440) % 1440
		dirs := lines[e.LineCode]
		if dirs == nil {
			dirs = map[string]*dirAgg{}
			lines[e.LineCode] = dirs
			lineOrder = append(lineOrder, e.LineCode)
		}
		dir := dirs[e.BoundFor]
		if dir == nil {
			dir = &dirAgg{prevMin: -1}
			dirs[e.BoundFor] = dir
		}
		dep := Departure{Time: e.EstimatedDeparture[:5], TripNumber: e.TripNumber, BoundFor: e.BoundFor}
		if d <= window {
			dir.next = append(dir.next, cand{d: dep, m: d})
		} else if past := 1440 - d; past <= departuresLookback && (dir.prevMin < 0 || past < dir.prevMin) {
			dir.prevMin = past
			dir.prev = &dep
		}
	}

	// nextMin is minutes until the group's soonest upcoming departure; groups
	// with none sort last, alphabetically.
	nextMin := func(dir *dirAgg) int {
		if len(dir.next) == 0 {
			return 1 << 30
		}
		m := dir.next[0].m
		for _, c := range dir.next[1:] {
			if c.m < m {
				m = c.m
			}
		}
		return m
	}
	board := StationDepartures{
		Station:       stopRef(stop.ID, stop.Name, stop.Code, stop.Lon, stop.Lat),
		Status:        "scheduled",
		WindowMinutes: window,
		Lines:         make([]DepartureLine, 0, len(lines)),
		Source:        DepartureSource{Provider: providerCode, RequestedAt: h.now().UTC()},
	}
	routeCache := map[string]*RouteRef{}
	lineMin := map[string]int{}
	for _, lc := range lineOrder {
		dirs := lines[lc]
		names := make([]string, 0, len(dirs))
		for bf := range dirs {
			names = append(names, bf)
		}
		sort.Slice(names, func(a, b int) bool {
			if ma, mb := nextMin(dirs[names[a]]), nextMin(dirs[names[b]]); ma != mb {
				return ma < mb
			}
			return names[a] < names[b]
		})
		line := DepartureLine{
			LineCode:   lc,
			Route:      h.routeFor(ctx, op, lc, routeCache),
			Directions: make([]DepartureDirection, 0, len(names)),
		}
		for _, bf := range names {
			dir := dirs[bf]
			sort.Slice(dir.next, func(a, b int) bool { return dir.next[a].m < dir.next[b].m })
			out := DepartureDirection{
				BoundFor:          bf,
				Departures:        make([]Departure, 0, len(dir.next)),
				PreviousDeparture: dir.prev,
			}
			for _, c := range dir.next {
				out.Departures = append(out.Departures, c.d)
			}
			line.Directions = append(line.Directions, out)
		}
		lineMin[lc] = nextMin(dirs[names[0]])
		board.Lines = append(board.Lines, line)
	}
	sort.Slice(board.Lines, func(a, b int) bool {
		la, lb := board.Lines[a], board.Lines[b]
		if ma, mb := lineMin[la.LineCode], lineMin[lb.LineCode]; ma != mb {
			return ma < mb
		}
		return la.LineCode < lb.LineCode
	})
	return board
}

// routeFor resolves a provider line ("B" under operator "KCI") to the
// canonical route; a miss stays nil rather than failing the board.
func (h *Handler) routeFor(ctx context.Context, op, lineCode string, cache map[string]*RouteRef) *RouteRef {
	if r, seen := cache[lineCode]; seen {
		return r
	}
	var out *RouteRef
	id, err := h.store.GetRouteByProviderEntityID(ctx, generated.GetRouteByProviderEntityIDParams{
		Code:             providerCode,
		ProviderEntityID: op + ":" + lineCode,
	})
	if err == nil {
		if row, err := h.store.GetRoute(ctx, id); err == nil {
			r := routeRef(row.ID, row.ShortName, row.LongName, row.Mode, row.Color, row.AgencyCode, row.AgencyName)
			out = &r
		}
	}
	cache[lineCode] = out
	return out
}

// splitProviderRef turns "KCI-SW" into operator "KCI" and station code "SW" —
// the provider's composite station id format.
func splitProviderRef(id string) (op, code string) {
	i := strings.IndexByte(id, '-')
	if i <= 0 || i == len(id)-1 {
		return "", ""
	}
	return id[:i], id[i+1:]
}

// minutesOfDay parses provider "HH:MM:SS" wall-clock strings.
func minutesOfDay(s string) (int, bool) {
	t, err := time.Parse("15:04:05", s)
	if err != nil {
		return 0, false
	}
	return t.Hour()*60 + t.Minute(), true
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
