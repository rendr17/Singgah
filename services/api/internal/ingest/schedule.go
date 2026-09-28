package ingest

import (
	"archive/zip"
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	generated "singgah/services/api/db/generated"
	"singgah/services/api/internal/provider/gtfs"
)

// ScheduleReport summarizes one GTFS schedule ingest — written row counts,
// feed routes/trips with no canonical counterpart, and trips dropped for
// unresolvable stops. Ingest never fails silently on bad feed data.
type ScheduleReport struct {
	Services    int
	Trips       int
	StopTimes   int
	Frequencies int
	// UnmatchedRoutes are feed route_short_names with no canonical route —
	// the same "honest absence" list as the shapes ingest reports.
	UnmatchedRoutes []string
	// RejectedTrips are "trip_id: reason" — only trips left with fewer than
	// two catalog stops are dropped (a ride needs both ends).
	RejectedTrips []string
	// UnmatchedStops are feed stop_ids serving matched routes but absent
	// from the catalog (TJ feeds JakLingko feeder halte commute doesn't
	// carry). Their stop_times are skipped — times between catalog stops
	// stay real; the gaps are upstream coverage, not parse failures.
	UnmatchedStops []string
	// Resolution observability: how feed stops chained onto catalog stops.
	ResolvedViaParent int
	ResolvedViaGeo    int
	Parse             gtfs.ScheduleStats
	FetchedAt         time.Time
}

// Schedule imports services/trips/stop_times/frequencies from a GTFS feed.
// Routes match via routePrefix+route_short_name and stops via
// stopPrefix+stop_id against the catalog provider's provider_entity_id —
// feed-local ids never become canonical ids. The provider's previous trips
// are replaced atomically inside the ingest transaction.
func Schedule(ctx context.Context, pool *pgxpool.Pool, zr *zip.Reader,
	provider generated.UpsertProviderParams, matchProviderCode, routePrefix, stopPrefix string) (ScheduleReport, error) {
	fetchedAt := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	report := ScheduleReport{FetchedAt: fetchedAt.Time}

	feed, stats, err := gtfs.ParseSchedule(zr)
	if err != nil {
		return report, fmt.Errorf("ingest schedule: parse feed: %w", err)
	}
	report.Parse = stats

	q0 := generated.New(pool)
	reg, err := q0.UpsertProvider(ctx, provider)
	if err != nil {
		return report, fmt.Errorf("ingest schedule: provider registration: %w", err)
	}
	if err := q0.TouchProviderLastAttempt(ctx, provider.Code); err != nil {
		return report, fmt.Errorf("ingest schedule: last_attempt_at: %w", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return report, fmt.Errorf("ingest schedule: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := q0.WithTx(tx)

	serviceIDs := make(map[string]pgtype.UUID, len(feed.Services))
	for _, s := range feed.Services {
		start, err1 := parseGtfsDate(s.StartDate)
		end, err2 := parseGtfsDate(s.EndDate)
		if err1 != nil || err2 != nil {
			report.RejectedTrips = append(report.RejectedTrips,
				fmt.Sprintf("service %s: bad calendar dates", s.ID))
			continue
		}
		row, err := q.UpsertService(ctx, generated.UpsertServiceParams{
			ProviderID:       reg.ID,
			ProviderEntityID: s.ID,
			DayMask:          int16(s.DayMask),
			StartDate:        start,
			EndDate:          end,
			FetchedAt:        fetchedAt,
		})
		if err != nil {
			return report, fmt.Errorf("ingest schedule: service %s: %w", s.ID, err)
		}
		serviceIDs[s.ID] = row.ID
		report.Services++
	}

	// Old trips die first — inside this tx the catalog never goes empty.
	if _, err := q.DeleteProviderTrips(ctx, reg.ID); err != nil {
		return report, fmt.Errorf("ingest schedule: clear trips: %w", err)
	}

	routeIDs := map[string]pgtype.UUID{}   // route_short_name -> canonical id (cache)
	unmatched := map[string]struct{}{}     // dedupe feed names before reporting
	stopEntityIDs := map[string]struct{}{} // stops needed by matched trips
	matched := make([]gtfs.ScheduleTrip, 0, len(feed.Trips))
	for _, t := range feed.Trips {
		routeID, ok := routeIDs[t.RouteShortName]
		if !ok {
			var err error
			routeID, err = q.GetRouteByProviderEntityID(ctx, generated.GetRouteByProviderEntityIDParams{
				Code:             matchProviderCode,
				ProviderEntityID: routePrefix + t.RouteShortName,
			})
			if err != nil {
				unmatched[t.RouteShortName] = struct{}{}
				continue
			}
			routeIDs[t.RouteShortName] = routeID
		}
		matched = append(matched, t)
		for _, st := range feed.StopTimes[t.ID] {
			stopEntityIDs[stopPrefix+st.StopID] = struct{}{}
			if fs, ok := feed.Stops[st.StopID]; ok && fs.ParentStation != "" {
				stopEntityIDs[stopPrefix+fs.ParentStation] = struct{}{}
			}
		}
	}
	report.UnmatchedRoutes = make([]string, 0, len(unmatched))
	for name := range unmatched {
		report.UnmatchedRoutes = append(report.UnmatchedRoutes, name)
	}
	sort.Strings(report.UnmatchedRoutes)

	entityIDs := make([]string, 0, len(stopEntityIDs))
	for id := range stopEntityIDs {
		entityIDs = append(entityIDs, id)
	}
	stopIDs := map[string]pgtype.UUID{}
	if len(entityIDs) > 0 {
		rows, err := q.ListStopIDsByProviderEntityIDs(ctx, generated.ListStopIDsByProviderEntityIDsParams{
			Code:      matchProviderCode,
			EntityIds: entityIDs,
		})
		if err != nil {
			return report, fmt.Errorf("ingest schedule: resolve stops: %w", err)
		}
		for _, r := range rows {
			stopIDs[r.ProviderEntityID] = r.ID
		}
	}

	// Geo-match reference: all live stops of the catalog provider. Feed
	// stops with no parent_station and no direct id resolve to the nearest
	// catalog stop inside geoMatchM — a boarding point sitting on a catalog
	// halte is the same place for our station-level graph.
	geoStops := []generated.ListStopCoordsByProviderRow{}
	if cr, err := q.ListStopCoordsByProvider(ctx, matchProviderCode); err != nil {
		return report, fmt.Errorf("ingest schedule: load stop coords: %w", err)
	} else {
		geoStops = cr
	}
	resolveStop := func(feedID string) (pgtype.UUID, bool, string) {
		if id, ok := stopIDs[stopPrefix+feedID]; ok {
			return id, true, "direct"
		}
		fs, ok := feed.Stops[feedID]
		if !ok {
			return pgtype.UUID{}, false, ""
		}
		if id, ok := stopIDs[stopPrefix+fs.ParentStation]; ok && fs.ParentStation != "" {
			return id, true, "parent"
		}
		best := -1.0
		var bestID pgtype.UUID
		for _, gs := range geoStops {
			d := haversineM(fs.Lat, fs.Lon, gs.Lat, gs.Lon)
			if best < 0 || d < best {
				best = d
				bestID = gs.ID
			}
		}
		if best >= 0 && best <= geoMatchM {
			return bestID, true, "geo"
		}
		return pgtype.UUID{}, false, ""
	}

	freqByTrip := map[string][]gtfs.ScheduleFrequency{}
	for _, f := range feed.Frequencies {
		freqByTrip[f.TripID] = append(freqByTrip[f.TripID], f)
	}

	unmatchedStops := map[string]struct{}{}
	for _, t := range matched {
		serviceID, ok := serviceIDs[t.ServiceID]
		if !ok {
			report.RejectedTrips = append(report.RejectedTrips, t.ID+": unknown service "+t.ServiceID)
			continue
		}
		// Keep only stop_times at catalog stops. A feeder halte the catalog
		// doesn't carry can't be boarded anyway — dropping its row leaves
		// the real times between the remaining catalog stops intact.
		type resolved struct {
			st gtfs.ScheduleStopTime
			id pgtype.UUID
		}
		sts := make([]resolved, 0, len(feed.StopTimes[t.ID]))
		for _, st := range feed.StopTimes[t.ID] {
			if id, ok, how := resolveStop(st.StopID); ok {
				sts = append(sts, resolved{st, id})
				switch how {
				case "parent":
					report.ResolvedViaParent++
				case "geo":
					report.ResolvedViaGeo++
				}
			} else {
				unmatchedStops[st.StopID] = struct{}{}
			}
		}
		if len(sts) < 2 {
			report.RejectedTrips = append(report.RejectedTrips, t.ID+": fewer than 2 catalog stops")
			continue
		}
		tripID, err := q.InsertTrip(ctx, generated.InsertTripParams{
			RouteID:          routeIDs[t.RouteShortName],
			ServiceID:        serviceID,
			ProviderID:       reg.ID,
			ProviderEntityID: t.ID,
			Headsign:         pgtype.Text{String: t.Headsign, Valid: t.Headsign != ""},
			DirectionID:      pgtype.Int2{Int16: int16(t.DirectionID), Valid: t.DirectionID >= 0},
			FetchedAt:        fetchedAt,
		})
		if err != nil {
			return report, fmt.Errorf("ingest schedule: trip %s: %w", t.ID, err)
		}
		for _, st := range sts {
			if err := q.InsertStopTime(ctx, generated.InsertStopTimeParams{
				TripID:           tripID,
				StopID:           st.id,
				Seq:              int32(st.st.Seq),
				ArrivalSeconds:   int32(st.st.ArrivalSeconds),
				DepartureSeconds: int32(st.st.DepartureSeconds),
				Derived:          false, // GTFS stop_times are published times
			}); err != nil {
				return report, fmt.Errorf("ingest schedule: stop_time %s seq %d: %w", t.ID, st.st.Seq, err)
			}
			report.StopTimes++
		}
		for _, f := range freqByTrip[t.ID] {
			if err := q.InsertFrequency(ctx, generated.InsertFrequencyParams{
				TripID:         tripID,
				StartSeconds:   int32(f.StartSeconds),
				EndSeconds:     int32(f.EndSeconds),
				HeadwaySeconds: int32(f.HeadwaySeconds),
				ExactTimes:     f.ExactTimes,
			}); err != nil {
				return report, fmt.Errorf("ingest schedule: frequency %s: %w", t.ID, err)
			}
			report.Frequencies++
		}
		report.Trips++
	}
	report.UnmatchedStops = make([]string, 0, len(unmatchedStops))
	for id := range unmatchedStops {
		report.UnmatchedStops = append(report.UnmatchedStops, id)
	}
	sort.Strings(report.UnmatchedStops)

	if err := q.TouchProviderLastSuccess(ctx, provider.Code); err != nil {
		return report, fmt.Errorf("ingest schedule: last_success_at: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return report, fmt.Errorf("ingest schedule: commit: %w", err)
	}
	return report, nil
}

// parseGtfsDate reads YYYYMMDD into a date — GTFS writes dates without
// separators while the column is a real SQL date.
func parseGtfsDate(s string) (pgtype.Date, error) {
	t, err := time.Parse("20060102", s)
	if err != nil {
		return pgtype.Date{}, fmt.Errorf("bad gtfs date %q", s)
	}
	return pgtype.Date{Time: t, Valid: true}, nil
}

// geoMatchM is the radius inside which a parentless feed stop counts as the
// same place as its nearest catalog stop. Halte pairs across a street sit
// ~30-80 m apart; distinct halte start ~300 m — 150 m separates the two
// populations without inventing stations (measured against the TJ feed,
// Sep 2026).
const geoMatchM = 150

// haversineM returns the great-circle distance in meters.
func haversineM(lat1, lon1, lat2, lon2 float64) float64 {
	const r = 6371000.0
	toRad := func(d float64) float64 { return d * math.Pi / 180 }
	p1, p2 := toRad(lat1), toRad(lat2)
	dp, dl := toRad(lat2-lat1), toRad(lon2-lon1)
	a := math.Sin(dp/2)*math.Sin(dp/2) + math.Cos(p1)*math.Cos(p2)*math.Sin(dl/2)*math.Sin(dl/2)
	return 2 * r * math.Asin(math.Sqrt(a))
}
