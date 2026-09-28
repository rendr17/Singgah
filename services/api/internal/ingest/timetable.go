package ingest

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	generated "singgah/services/api/db/generated"
	"singgah/services/api/internal/provider/commute"
)

// TimetableSource is the sweep's view of the provider — one full-day
// timetable call per catalog station.
type TimetableSource interface {
	Timetable(ctx context.Context, operatorCode, stationCode, from, to string) ([]commute.TimetableEntry, error)
}

// CommuteScheduleReport summarizes one timetable sweep — coverage, honest
// absence (operators with no published timetable), per-station failures,
// and every reconstruction reject.
type CommuteScheduleReport struct {
	Stations         int
	StationsNoData   int // upstream 404 — timetable simply isn't published
	StationsFailed   int
	FailedStations   []string
	Entries          int
	Trips            int
	StopTimes        int
	DerivedStopTimes int
	SkippedStops     int
	Rejections       []string
	FetchedAt        time.Time
}

// CommuteSchedule sweeps every catalog station's full-day timetable and
// reconstructs runs via commute.BuildTrips. Reconstruction and persistence
// semantics:
//   - run identity: tripNumber groups entries into trips; LRTJ-style
//     per-entry ids chain greedily and every stop lands Derived;
//   - the commute timetable is one static per-operator board — dayMask is
//     undocumented upstream, so all trips attach to a "runs daily" service
//     (mask 127) bounded to a 90-day assumed-stability window. Freshness
//     lives in fetched_at; docs/35 notes the limitation;
//   - a station's failed call shrinks this run's coverage loudly — it is
//     counted and listed, never retried silently nor fatal for the rest.
func CommuteSchedule(ctx context.Context, pool *pgxpool.Pool, src TimetableSource, pace time.Duration) (CommuteScheduleReport, error) {
	fetchedAt := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	report := CommuteScheduleReport{FetchedAt: fetchedAt.Time}

	q0 := generated.New(pool)
	reg, err := q0.GetProviderByCode(ctx, "commute")
	if err != nil {
		return report, fmt.Errorf("ingest timetables: provider row: %w", err)
	}
	if err := q0.TouchProviderLastAttempt(ctx, "commute"); err != nil {
		return report, fmt.Errorf("ingest timetables: last_attempt_at: %w", err)
	}

	stops, err := q0.ListCatalogStops(ctx, "commute")
	if err != nil {
		return report, fmt.Errorf("ingest timetables: catalog stops: %w", err)
	}
	topoRows, err := q0.ListRouteStopTopology(ctx, "commute")
	if err != nil {
		return report, fmt.Errorf("ingest timetables: topology: %w", err)
	}
	topo := map[string]*commute.LineTopology{}
	for _, r := range topoRows {
		t := topo[r.RouteKey]
		if t == nil {
			t = &commute.LineTopology{RouteKey: r.RouteKey}
			topo[r.RouteKey] = t
		}
		t.Stops = append(t.Stops, commute.TopoStop{EntityID: r.StopKey, Name: r.Name})
	}
	lineTopo := make(map[string]commute.LineTopology, len(topo))
	for k, t := range topo {
		lineTopo[k] = *t
	}

	var entries []commute.TimetableEntry
	for _, s := range stops {
		op, _, ok := commute.SplitStationID(s.ProviderEntityID)
		if !ok || !s.Code.Valid || s.Code.String == "" {
			report.StationsFailed++
			report.FailedStations = append(report.FailedStations, s.ProviderEntityID)
			continue
		}
		report.Stations++
		es, err := src.Timetable(ctx, op, s.Code.String, "00:00", "23:59")
		switch {
		case errors.Is(err, commute.ErrStationUnknown):
			report.StationsNoData++
		case err != nil:
			report.StationsFailed++
			report.FailedStations = append(report.FailedStations,
				fmt.Sprintf("%s: %v", s.ProviderEntityID, err))
		default:
			if len(es) == 0 {
				report.StationsNoData++
				continue
			}
			entries = append(entries, es...)
		}
		if pace > 0 {
			select {
			case <-ctx.Done():
				return report, ctx.Err()
			case <-time.After(pace):
			}
		}
	}
	report.Entries = len(entries)

	built := commute.BuildTrips(entries, lineTopo)
	report.Rejections = built.Rejections
	report.SkippedStops = built.SkippedStops

	// Collect every referenced catalog stop + route for one bulk resolve.
	stopKeys := map[string]struct{}{}
	routeKeys := map[string]struct{}{}
	for _, t := range built.Trips {
		routeKeys[t.RouteKey] = struct{}{}
		for _, st := range t.Stops {
			stopKeys[st.StopEntityID] = struct{}{}
		}
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return report, fmt.Errorf("ingest timetables: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := q0.WithTx(tx)

	stopIDs, err := resolveStopIDs(ctx, q, "commute", stopKeys)
	if err != nil {
		return report, fmt.Errorf("ingest timetables: resolve stops: %w", err)
	}
	routeIDs := map[string]pgtype.UUID{}
	unmatchedRoutes := map[string]struct{}{}
	for rk := range routeKeys {
		routeID, err := q.GetRouteByProviderEntityID(ctx, generated.GetRouteByProviderEntityIDParams{
			Code:             "commute",
			ProviderEntityID: rk,
		})
		if err != nil {
			unmatchedRoutes[rk] = struct{}{}
			continue
		}
		routeIDs[rk] = routeID
	}

	// One "runs daily" service per operator — the upstream timetable carries
	// no calendar semantics of its own.
	start := pgtype.Date{Time: fetchedAt.Time.Truncate(24 * time.Hour), Valid: true}
	end := pgtype.Date{Time: start.Time.AddDate(0, 0, 90), Valid: true}
	serviceIDs := map[string]pgtype.UUID{}
	for _, t := range built.Trips {
		op, _, _ := commute.SplitStationID(t.Stops[0].StopEntityID)
		if _, ok := serviceIDs[op]; ok {
			continue
		}
		row, err := q.UpsertService(ctx, generated.UpsertServiceParams{
			ProviderID:       reg.ID,
			ProviderEntityID: op + ":daily",
			DayMask:          127,
			StartDate:        start,
			EndDate:          end,
			FetchedAt:        fetchedAt,
		})
		if err != nil {
			return report, fmt.Errorf("ingest timetables: service %s: %w", op, err)
		}
		serviceIDs[op] = row.ID
	}

	if _, err := q.DeleteProviderTrips(ctx, reg.ID); err != nil {
		return report, fmt.Errorf("ingest timetables: clear trips: %w", err)
	}

	for _, t := range built.Trips {
		routeID, ok := routeIDs[t.RouteKey]
		if !ok {
			continue // already counted in unmatchedRoutes
		}
		op, _, _ := commute.SplitStationID(t.Stops[0].StopEntityID)
		serviceID, ok := serviceIDs[op]
		if !ok {
			continue
		}
		tripID, err := q.InsertTrip(ctx, generated.InsertTripParams{
			RouteID:          routeID,
			ServiceID:        serviceID,
			ProviderID:       reg.ID,
			ProviderEntityID: t.EntityKey,
			Headsign:         pgtype.Text{String: t.Headsign, Valid: t.Headsign != ""},
			DirectionID:      pgtype.Int2{Int16: int16(t.Direction), Valid: t.Direction >= 0},
			FetchedAt:        fetchedAt,
		})
		if err != nil {
			return report, fmt.Errorf("ingest timetables: trip %s: %w", t.EntityKey, err)
		}
		for i, st := range t.Stops {
			stopID, ok := stopIDs[st.StopEntityID]
			if !ok {
				report.SkippedStops++
				continue
			}
			if err := q.InsertStopTime(ctx, generated.InsertStopTimeParams{
				TripID:           tripID,
				StopID:           stopID,
				Seq:              int32(i),
				ArrivalSeconds:   int32(st.ArrivalSec),
				DepartureSeconds: int32(st.DepartureSec),
				Derived:          st.Derived,
			}); err != nil {
				return report, fmt.Errorf("ingest timetables: stop_time %s seq %d: %w", t.EntityKey, i, err)
			}
			report.StopTimes++
			if st.Derived {
				report.DerivedStopTimes++
			}
		}
		report.Trips++
	}
	if len(unmatchedRoutes) > 0 {
		rs := make([]string, 0, len(unmatchedRoutes))
		for rk := range unmatchedRoutes {
			rs = append(rs, rk)
		}
		sort.Strings(rs)
		report.Rejections = append(report.Rejections, "unmatched routes: "+fmt.Sprint(rs))
	}

	if err := q.TouchProviderLastSuccess(ctx, "commute"); err != nil {
		return report, fmt.Errorf("ingest timetables: last_success_at: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return report, fmt.Errorf("ingest timetables: commit: %w", err)
	}
	return report, nil
}

func resolveStopIDs(ctx context.Context, q *generated.Queries, providerCode string, keys map[string]struct{}) (map[string]pgtype.UUID, error) {
	ids := make([]string, 0, len(keys))
	for k := range keys {
		ids = append(ids, k)
	}
	out := map[string]pgtype.UUID{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := q.ListStopIDsByProviderEntityIDs(ctx, generated.ListStopIDsByProviderEntityIDsParams{
		Code:      providerCode,
		EntityIds: ids,
	})
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.ProviderEntityID] = r.ID
	}
	return out, nil
}
