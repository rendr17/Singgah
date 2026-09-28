package ingest

import (
	"context"
	"os"
	"testing"
	"time"

	"singgah/services/api/internal/db"
	"singgah/services/api/internal/provider/commute"
)

// fakeTimetables serves canned entries per station — the sweep contract is
// one full-day call per catalog stop.
type fakeTimetables struct {
	byStation map[string][]commute.TimetableEntry // "op/code"
	errs      map[string]error
	calls     []string
}

func (f *fakeTimetables) Timetable(_ context.Context, op, code, from, to string) ([]commute.TimetableEntry, error) {
	f.calls = append(f.calls, op+"/"+code)
	if err := f.errs[op+"/"+code]; err != nil {
		return nil, err
	}
	return f.byStation[op+"/"+code], nil
}

func ttEntry(id, stationID, dep, arr, boundFor, line, trip string) commute.TimetableEntry {
	var tn *string
	if trip != "" {
		tn = &trip
	}
	return commute.TimetableEntry{
		ID: id, StationID: stationID, TripNumber: tn,
		EstimatedDeparture: dep, EstimatedArrival: arr,
		BoundFor: boundFor, LineCode: line,
	}
}

// The sweep must write reconstructed trips onto canonical stops, count
// honest absence (404 upstream) separately from failures, and stay
// idempotent on re-run.
func TestCommuteScheduleSweepAndPersist(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set — start infrastructure/local compose and run migrations")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.Connect(ctx, url)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer pool.Close()

	if _, err := pool.Exec(ctx,
		"TRUNCATE frequencies, stop_times, trips, services, route_stops, transfers, stops, routes, agencies CASCADE"); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	if _, err := Commute(ctx, pool, loadFixtureSource(t)); err != nil {
		t.Fatalf("seed catalog: %v", err)
	}

	src := &fakeTimetables{
		byStation: map[string][]commute.TimetableEntry{
			"KCI/SUD": {
				// T1: KCI-style — terminus arrival published upstream.
				ttEntry("KCI-SUD-T1", "KCI-SUD", "08:00:00", "08:12:00", "Ancol", "C", "T1"),
				// T4: boundFor a station the catalog doesn't carry.
				ttEntry("KCI-SUD-T4", "KCI-SUD", "11:00:00", "11:00:00", "Ghost", "TP", "T4"),
			},
			"LRTJBDB/DKA": {
				// LRTJBDB has no topology fixture — entries count as skipped.
				ttEntry("LRTJBDB-DKA-1", "LRTJBDB-DKA", "07:00:00", "07:00:00", "Jatimulya", "BK", "L1"),
				ttEntry("LRTJBDB-DKA-2", "LRTJBDB-DKA", "07:10:00", "07:00:00", "Jatimulya", "BK", "L2"),
			},
		},
		errs: map[string]error{
			"MRTJ/DKA": commute.ErrStationUnknown, // published absence
		},
	}

	report, err := CommuteSchedule(ctx, pool, src, 0)
	if err != nil {
		t.Fatalf("CommuteSchedule: %v", err)
	}
	if report.Stations != 4 || report.StationsNoData != 2 || report.StationsFailed != 0 {
		t.Fatalf("station counts: %+v", report) // AC empty + MRTJ 404
	}
	if len(src.calls) != 4 {
		t.Fatalf("calls=%v, want one per catalog stop", src.calls)
	}
	if report.Entries != 4 || report.SkippedStops != 2 {
		t.Fatalf("entries=%d skipped=%d, want 4/2", report.Entries, report.SkippedStops)
	}
	// T1 gets its published terminus appended (SUD->AC); T4 dies with one
	// published stop and an unresolvable boundFor.
	if report.Trips != 1 || report.StopTimes != 2 || report.DerivedStopTimes != 0 {
		t.Fatalf("report counts: %+v", report)
	}
	if len(report.Rejections) != 1 {
		t.Fatalf("rejections: %v", report.Rejections)
	}

	var routeEntity, headsign string
	var stops int
	var firstArr, firstDep, lastArr int
	var lastDerived bool
	if err := pool.QueryRow(ctx, `
		SELECT r.provider_entity_id, t.headsign,
			(SELECT count(*) FROM stop_times st WHERE st.trip_id = t.id),
			(SELECT arrival_seconds FROM stop_times st WHERE st.trip_id = t.id ORDER BY seq LIMIT 1),
			(SELECT departure_seconds FROM stop_times st WHERE st.trip_id = t.id ORDER BY seq LIMIT 1),
			(SELECT arrival_seconds FROM stop_times st WHERE st.trip_id = t.id ORDER BY seq DESC LIMIT 1),
			(SELECT derived FROM stop_times st WHERE st.trip_id = t.id ORDER BY seq DESC LIMIT 1)
		FROM trips t JOIN routes r ON r.id = t.route_id
		WHERE t.provider_entity_id LIKE '%T1%'`).Scan(
		&routeEntity, &headsign, &stops, &firstArr, &firstDep, &lastArr, &lastDerived); err != nil {
		t.Fatalf("trip row: %v", err)
	}
	if routeEntity != "KCI:C" || headsign != "Ancol" || stops != 2 {
		t.Fatalf("trip = %s %q stops=%d", routeEntity, headsign, stops)
	}
	if firstArr != 8*3600 || firstDep != 8*3600 || lastArr != 8*3600+12*60 || lastDerived {
		t.Fatalf("times = dep %d arr %d lastArr %d derived=%v", firstArr, firstDep, lastArr, lastDerived)
	}

	// Service row: one daily service for the operator, all-days mask.
	var mask int16
	if err := pool.QueryRow(ctx, `
		SELECT s.day_mask FROM services s
		JOIN trips t ON t.service_id = s.id
		WHERE t.provider_entity_id LIKE '%T1%'`).Scan(&mask); err != nil {
		t.Fatalf("service row: %v", err)
	}
	if mask != 127 {
		t.Fatalf("day_mask=%d, want 127 (daily — upstream dayMask is undocumented)", mask)
	}

	// Re-run: trips replace atomically — no duplicates.
	report2, err := CommuteSchedule(ctx, pool, src, 0)
	if err != nil {
		t.Fatalf("re-sweep: %v", err)
	}
	var trips, stopTimes int
	_ = pool.QueryRow(ctx, "SELECT count(*) FROM trips").Scan(&trips)
	_ = pool.QueryRow(ctx, "SELECT count(*) FROM stop_times").Scan(&stopTimes)
	if trips != 1 || stopTimes != 2 || report2.Trips != 1 {
		t.Fatalf("re-sweep duplicated: trips=%d stop_times=%d", trips, stopTimes)
	}
}
