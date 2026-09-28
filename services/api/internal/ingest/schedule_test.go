package ingest

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"singgah/services/api/internal/db"
	"singgah/services/api/internal/provider/commute"
	"singgah/services/api/internal/provider/gtfs"
)

// scheduleZip builds a minimal in-memory GTFS zip (same shape as the
// gtfs package's feedZip helper — tests stay independent).
func scheduleZip(t *testing.T, files map[string]string) *zip.Reader {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	return zr
}

// tjFixtureSource augments the commute fixtures with a TJ line + two TJ
// halte so the schedule ingest has canonical routes/stops to attach to.
func tjFixtureSource(t *testing.T) fixtureSource {
	t.Helper()
	src := loadFixtureSource(t)
	found := false
	for _, op := range src.ops {
		if op.Code == "TJ" {
			found = true
		}
	}
	if !found {
		src.ops = append(src.ops, commute.Operator{
			Code:     "TJ",
			Name:     "Transjakarta",
			Timezone: "Asia/Jakarta",
			Lines:    []commute.Line{{Name: "Koridor 1", LineCode: "1"}},
		})
	}
	lat1, lon1 := -6.200, 106.820
	lat2, lon2 := -6.210, 106.830
	src.stations = append(src.stations,
		commute.Station{ID: "TJ-H00001P", Name: "Halte A", Code: "H00001P", Operator: "TJ", Latitude: &lat1, Longitude: &lon1},
		commute.Station{ID: "TJ-H00002P", Name: "Halte B", Code: "H00002P", Operator: "TJ", Latitude: &lat2, Longitude: &lon2},
	)
	src.lines["TJ/1"] = &commute.LineDetail{
		Segments: []commute.Segment{{
			Kind: "TRUNK",
			Stations: []commute.SegmentStation{
				{ID: "TJ-H00001P", Code: "H00001P", Name: "Halte A"},
				{ID: "TJ-H00002P", Code: "H00002P", Name: "Halte B"},
			},
		}},
	}
	return src
}

// The G-stops model the real TJ feed: trips stop at boarding points that
// reach the catalog only via parent_station or proximity, not by id.
var tjGTFSFiles = map[string]string{
	"routes.txt": `route_id,route_short_name,route_type
R1,1,3
R2,9Z,3
`,
	"calendar.txt": `service_id,monday,tuesday,wednesday,thursday,friday,saturday,sunday,start_date,end_date
SH,1,1,1,1,1,1,1,20040115,20271231
`,
	"trips.txt": `trip_id,route_id,service_id,trip_headsign,direction_id
t1,R1,SH,Pasar Baru,0
t2,R1,GH,Kota,1
t3,R2,SH,Nowhere,0
t4,R1,SH,OneStop,0
`,
	"stops.txt": `stop_id,stop_name,stop_lat,stop_lon,location_type,parent_station
G00001,Halte A Sisi,-6.2001,106.8201,0,H00001P
G00002,Halte B Sisi,-6.2101,106.8301,0,H00002P
GGEOB,Halte B Seberang,-6.2097,106.8302,0,
GFAR,Jauh Sekali,-6.800,107.300,0,
`,
	"stop_times.txt": `trip_id,stop_sequence,stop_id,arrival_time,departure_time
t1,0,G00001,05:00:00,05:00:00
t1,1,G00002,05:10:00,05:10:10
t1,2,GGEOB,05:15:00,05:15:10
t1,3,GFAR,05:40:00,05:40:10
t2,0,G00001,08:00:00,08:00:00
t2,1,G00002,08:10:00,08:10:00
t3,0,G00001,09:00:00,09:00:00
t3,1,G00002,09:10:00,09:10:00
t4,0,GFAR,10:00:00,10:00:00
`,
	"frequencies.txt": `trip_id,start_time,end_time,headway_secs,exact_times
t1,05:00:00,22:00:00,600,0
`,
}

// Schedule ingest must write trips/stop_times/frequencies resolved onto
// canonical stops, report honest absence (unmatched routes/stops), reject
// trips that can't be served, and stay idempotent on re-run.
func TestScheduleIngestResolvesAndIsIdempotent(t *testing.T) {
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

	// Catalog tests share the database — start clean so counts are exact.
	if _, err := pool.Exec(ctx,
		"TRUNCATE frequencies, stop_times, trips, services, route_stops, transfers, stops, routes, agencies CASCADE"); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	if _, err := Commute(ctx, pool, tjFixtureSource(t)); err != nil {
		t.Fatalf("seed catalog: %v", err)
	}

	zr := scheduleZip(t, tjGTFSFiles)
	report, err := Schedule(ctx, pool, zr, gtfs.TJProviderRegistration, "commute", "TJ:", "TJ-")
	if err != nil {
		t.Fatalf("schedule ingest: %v", err)
	}

	if report.Services != 1 || report.Trips != 1 || report.Frequencies != 1 {
		t.Fatalf("report counts: %+v", report)
	}
	// t1 keeps only catalog-resolved stop_times: G00001+G00002 via parent,
	// GGEOB via geo; GFAR stays out.
	if report.StopTimes != 3 || report.ResolvedViaParent != 2 || report.ResolvedViaGeo != 1 {
		t.Fatalf("resolution: %+v", report)
	}
	if len(report.UnmatchedRoutes) != 1 || report.UnmatchedRoutes[0] != "9Z" {
		t.Fatalf("unmatched routes: %+v", report.UnmatchedRoutes)
	}
	if len(report.UnmatchedStops) != 1 || report.UnmatchedStops[0] != "GFAR" {
		t.Fatalf("unmatched stops: %+v", report.UnmatchedStops)
	}
	wantRejected := map[string]bool{"t2: unknown service GH": true, "t4: fewer than 2 catalog stops": true}
	for _, r := range report.RejectedTrips {
		delete(wantRejected, r)
	}
	if len(wantRejected) != 0 {
		t.Fatalf("missing rejections: %v in %+v", wantRejected, report.RejectedTrips)
	}

	// The written trip must bind the canonical TJ:1 route and catalog stops.
	var routeEntity, headsign string
	var stopCount, freqCount int
	if err := pool.QueryRow(ctx, `
		SELECT r.provider_entity_id, t.headsign,
			(SELECT count(*) FROM stop_times st WHERE st.trip_id = t.id),
			(SELECT count(*) FROM frequencies f WHERE f.trip_id = t.id)
		FROM trips t JOIN routes r ON r.id = t.route_id
		WHERE t.provider_entity_id = 't1'`).Scan(&routeEntity, &headsign, &stopCount, &freqCount); err != nil {
		t.Fatalf("trip row: %v", err)
	}
	if routeEntity != "TJ:1" || headsign != "Pasar Baru" || stopCount != 3 || freqCount != 1 {
		t.Fatalf("trip = %s %q stops=%d freqs=%d", routeEntity, headsign, stopCount, freqCount)
	}
	var firstStop string
	if err := pool.QueryRow(ctx, `
		SELECT s.provider_entity_id FROM stop_times st JOIN stops s ON s.id = st.stop_id
		WHERE st.trip_id = (SELECT id FROM trips WHERE provider_entity_id='t1')
		ORDER BY st.seq LIMIT 1`).Scan(&firstStop); err != nil {
		t.Fatalf("first stop: %v", err)
	}
	if firstStop != "TJ-H00001P" {
		t.Fatalf("first stop resolved to %q, want TJ-H00001P", firstStop)
	}

	// Re-run: services upsert, trips replace — no duplicates.
	report2, err := Schedule(ctx, pool, zr, gtfs.TJProviderRegistration, "commute", "TJ:", "TJ-")
	if err != nil {
		t.Fatalf("re-ingest: %v", err)
	}
	var trips, stopTimes int
	_ = pool.QueryRow(ctx, "SELECT count(*) FROM trips").Scan(&trips)
	_ = pool.QueryRow(ctx, "SELECT count(*) FROM stop_times").Scan(&stopTimes)
	if trips != 1 || stopTimes != 3 || report2.StopTimes != 3 {
		t.Fatalf("re-ingest duplicated rows: trips=%d stop_times=%d report=%+v", trips, stopTimes, report2)
	}
}
