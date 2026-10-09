package planner

import (
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// EstimatedPosition is a schedule-derived guess of where a trip's vehicle
// is right now. It is never live data — consumers must label it estimated.
// Docs/40 calls this "line-progress representation": the honest answer for
// modes without a GPS feed (rail today), interpolating between the two
// stop_times the schedule says the vehicle sits between.
type EstimatedPosition struct {
	ID      string // unique per trip instance — "<tripKey>" or "<tripKey>~<k>"
	TripKey string // provider trip_id — dedupe key against live feed trip_ids
	RouteID pgtype.UUID
	Mode    string
	Lon     float64
	Lat     float64
}

// EstimatedPositions returns one position per trip instance the schedule
// says is in service at t, restricted to modes. Frequency templates expand
// into every overlapping instance — a headway line genuinely has several
// trains out at once.
func (e *Engine) EstimatedPositions(t time.Time, modes map[string]bool) []EstimatedPosition {
	local := t.In(e.loc)
	midnight := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, e.loc)

	var out []EstimatedPosition
	// Yesterday's service day catches post-midnight trips (arr > 24h).
	for _, base := range []time.Time{midnight, midnight.AddDate(0, 0, -1)} {
		T := t.Unix() - base.Unix()
		if T < 0 {
			continue
		}
		weekday := int(base.Weekday())
		mask := int16(1 << ((weekday + 6) % 7)) // GTFS bit0=monday
		for i := range e.trips {
			tr := &e.trips[i]
			if len(modes) > 0 && !modes[tr.mode] {
				continue
			}
			s := tr.service
			if s.dayMask&mask == 0 || base.Unix() < s.start.Unix() || base.Unix() > s.end.Unix() {
				continue
			}
			n := len(tr.stops)
			if n == 0 {
				continue
			}
			if tr.freq == nil {
				if int64(tr.dep[0]) <= T && T <= int64(tr.arr[n-1]) {
					if p, ok := e.positionAt(tr, T, 0); ok {
						p.ID = tr.key
						out = append(out, p)
					}
				}
				continue
			}
			// Instance k shifts the template by k*headway; the frequency
			// window bounds the first stop's departure (search.go semantics).
			h := int64(tr.freq.headway)
			kmax := int64(tr.freq.end-tr.dep[0]) / h
			for k := int64(0); k <= kmax; k++ {
				shift := k * h
				if int64(tr.dep[0])+shift <= T && T <= int64(tr.arr[n-1])+shift {
					if p, ok := e.positionAt(tr, T, shift); ok {
						p.ID = fmt.Sprintf("%s~%d", tr.key, k)
						out = append(out, p)
					}
				}
			}
		}
	}
	return out
}

// positionAt interpolates the vehicle's spot at service-second T (template
// times + shift). Dwell windows land on the stop; moving windows lerp
// straight between stop coordinates — shapes add nothing at estimated
// precision.
func (e *Engine) positionAt(tr *tripRec, T, shift int64) (EstimatedPosition, bool) {
	p := EstimatedPosition{TripKey: tr.key, RouteID: tr.routeID, Mode: tr.mode}
	for i := 0; i < len(tr.stops); i++ {
		at, ok1 := e.stops[tr.stops[i]]
		if T <= int64(tr.dep[i])+shift {
			if !ok1 {
				return p, false
			}
			p.Lon, p.Lat = at.Lon, at.Lat
			return p, true
		}
		if i+1 < len(tr.stops) && T <= int64(tr.arr[i+1])+shift {
			nx, ok2 := e.stops[tr.stops[i+1]]
			if !ok1 || !ok2 {
				return p, false
			}
			span := int64(tr.arr[i+1]) - int64(tr.dep[i])
			if span <= 0 {
				p.Lon, p.Lat = nx.Lon, nx.Lat
				return p, true
			}
			f := float64(T-int64(tr.dep[i])-shift) / float64(span)
			p.Lon = at.Lon + f*(nx.Lon-at.Lon)
			p.Lat = at.Lat + f*(nx.Lat-at.Lat)
			return p, true
		}
	}
	return p, false
}
