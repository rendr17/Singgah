package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"singgah/services/api/internal/db"
	"singgah/services/api/internal/provider/commute"
)

// Integration test: fixture source -> pipeline -> real PostGIS, twice, to
// prove idempotency. Requires TEST_DATABASE_URL (compose or CI service).

type fixtureSource struct {
	ops       []commute.Operator
	stations  []commute.Station
	transfers map[string][]commute.Transfer
	lines     map[string]*commute.LineDetail
}

func (f fixtureSource) Operators(context.Context) ([]commute.Operator, error) { return f.ops, nil }
func (f fixtureSource) Stations(context.Context) ([]commute.Station, error)   { return f.stations, nil }
func (f fixtureSource) Transfers(_ context.Context, op, code string) ([]commute.Transfer, error) {
	return f.transfers[op+"/"+code], nil
}
func (f fixtureSource) LineDetail(_ context.Context, op, code string) (*commute.LineDetail, error) {
	return f.lines[op+"/"+code], nil
}

func loadFixtureSource(t *testing.T) fixtureSource {
	t.Helper()
	dir := "../provider/commute/testdata/"
	read := func(name string, out any) {
		raw, err := os.ReadFile(dir + name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		var env struct {
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(raw, &env); err != nil {
			t.Fatalf("decode %s: %v", name, err)
		}
		if err := json.Unmarshal(env.Data, out); err != nil {
			t.Fatalf("decode data %s: %v", name, err)
		}
	}

	var src fixtureSource
	read("operators.json", &src.ops)
	read("stations.json", &src.stations)
	var tr []commute.Transfer
	read("transfers-KCI-SUD.json", &tr)
	src.transfers = map[string][]commute.Transfer{"KCI/SUD": tr}
	src.lines = map[string]*commute.LineDetail{}
	for _, key := range []string{"KCI-C", "KCI-TP", "MRTJ-M"} {
		var ld commute.LineDetail
		read("line-detail-"+key+".json", &ld)
		src.lines[strings.Replace(key, "-", "/", 1)] = &ld
	}
	return src
}

func TestCommuteIngestIsIdempotent(t *testing.T) {
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

	src := loadFixtureSource(t)

	report, err := Commute(ctx, pool, src)
	if err != nil {
		t.Fatalf("ingest run 1: %v", err)
	}
	// RouteStops: KCI:C 2 + KCI:TP 1 (ghost rejected) + MRTJ:M 1.
	if report.Operators != 2 || report.Stops != 4 || report.Routes != 3 ||
		report.Transfers != 2 || report.RouteStops != 4 {
		t.Errorf("counts: %+v", report)
	}
	// The EXTERNAL transfer and the ghost segment station must be reported,
	// not silently dropped.
	if len(report.Rejections) != 2 {
		t.Fatalf("rejections: %+v", report.Rejections)
	}
	seen := map[string]bool{}
	for _, r := range report.Rejections {
		seen[r.EntityID] = true
	}
	if !seen["T-KCI-SUD-EXT-1"] || !seen["KCI-GHOST"] {
		t.Errorf("rejections: %+v", report.Rejections)
	}

	var stopsBefore, transfersBefore, routeStopsBefore int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM stops").Scan(&stopsBefore); err != nil {
		t.Fatalf("count stops: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM transfers").Scan(&transfersBefore); err != nil {
		t.Fatalf("count transfers: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM route_stops").Scan(&routeStopsBefore); err != nil {
		t.Fatalf("count route_stops: %v", err)
	}

	// Re-ingest: upserts must keep canonical IDs and not duplicate rows.
	report2, err := Commute(ctx, pool, src)
	if err != nil {
		t.Fatalf("ingest run 2: %v", err)
	}
	var stopsAfter, transfersAfter, routeStopsAfter int
	_ = pool.QueryRow(ctx, "SELECT count(*) FROM stops").Scan(&stopsAfter)
	_ = pool.QueryRow(ctx, "SELECT count(*) FROM transfers").Scan(&transfersAfter)
	_ = pool.QueryRow(ctx, "SELECT count(*) FROM route_stops").Scan(&routeStopsAfter)
	if stopsAfter != stopsBefore || transfersAfter != transfersBefore || routeStopsAfter != routeStopsBefore {
		t.Errorf("re-ingest duplicated rows: stops %d->%d transfers %d->%d route_stops %d->%d",
			stopsBefore, stopsAfter, transfersBefore, transfersAfter, routeStopsBefore, routeStopsAfter)
	}

	// Ordered sequence must survive: KCI:C serves SUD at seq 1 then AC.
	var seqName string
	if err := pool.QueryRow(ctx,
		`SELECT s.name FROM route_stops rs JOIN stops s ON s.id = rs.stop_id
		 JOIN routes r ON r.id = rs.route_id
		 WHERE r.provider_entity_id = 'KCI:C' ORDER BY rs.seq LIMIT 1`,
	).Scan(&seqName); err != nil {
		t.Fatalf("route_stops order: %v", err)
	}
	if seqName != "Sudirman" {
		t.Errorf("first KCI:C stop = %q, want Sudirman", seqName)
	}

	// Provenance: provider row carries the ODbL-1.0 registry fields and a
	// fresh last_success_at after the run.
	var license, attribution string
	var lastSuccess *time.Time
	err = pool.QueryRow(ctx,
		"SELECT license_name, attribution_text, last_success_at FROM providers WHERE code=$1",
		commute.ProviderCode).Scan(&license, &attribution, &lastSuccess)
	if err != nil {
		t.Fatalf("provider row: %v", err)
	}
	if license != "ODbL-1.0" || attribution == "" {
		t.Errorf("registry fields: license=%q attribution=%q", license, attribution)
	}
	if lastSuccess == nil {
		t.Error("last_success_at not stamped")
	}
	fmt.Printf("run2: %+v\n", report2)
}

// failingSource dies on the stations fetch — the run fails after the provider
// row and attempt stamp were already committed.
type failingSource struct{ fixtureSource }

func (f failingSource) Stations(context.Context) ([]commute.Station, error) {
	return nil, fmt.Errorf("upstream down")
}

// A run that dies mid-pipeline must still leave last_attempt_at — otherwise
// "ingest keeps failing" is indistinguishable from "never ran" on the health
// surface.
func TestFailedIngestStillStampsAttempt(t *testing.T) {
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

	if _, err := Commute(ctx, pool, failingSource{loadFixtureSource(t)}); err == nil {
		t.Fatal("expected ingest to fail")
	}
	var attempt *time.Time
	if err := pool.QueryRow(ctx,
		"SELECT last_attempt_at FROM providers WHERE code=$1",
		commute.ProviderCode).Scan(&attempt); err != nil {
		t.Fatalf("provider row: %v", err)
	}
	if attempt == nil {
		t.Error("last_attempt_at not stamped for a failed run")
	}
}

// A station that leaves the provider feed is tombstoned (removed_at), not
// deleted — and resurrected if it reappears in a later run.
func TestIngestTombstonesDroppedStops(t *testing.T) {
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

	// Catalog tests share this database — start from an empty catalog so the
	// tombstone counts are deterministic regardless of earlier runs.
	if _, err := pool.Exec(ctx,
		"TRUNCATE route_stops, transfers, stops, routes, agencies CASCADE"); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	src := loadFixtureSource(t)
	if _, err := Commute(ctx, pool, src); err != nil {
		t.Fatalf("ingest run 1: %v", err)
	}

	kept := src.stations[:0]
	for _, s := range src.stations {
		if s.ID != "MRTJ-DKA" {
			kept = append(kept, s)
		}
	}
	src.stations = kept

	report, err := Commute(ctx, pool, src)
	if err != nil {
		t.Fatalf("ingest run 2: %v", err)
	}
	if report.RemovedStops != 1 {
		t.Errorf("RemovedStops = %d, want 1", report.RemovedStops)
	}
	var removedAt *time.Time
	if err := pool.QueryRow(ctx,
		"SELECT removed_at FROM stops WHERE provider_entity_id='MRTJ-DKA'").Scan(&removedAt); err != nil {
		t.Fatalf("removed_at: %v", err)
	}
	if removedAt == nil {
		t.Error("MRTJ-DKA was not tombstoned")
	}
	var active int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM stops WHERE removed_at IS NULL").Scan(&active); err != nil {
		t.Fatalf("active count: %v", err)
	}
	if active != 3 {
		t.Errorf("active stops = %d, want 3", active)
	}

	// Reappearing in a later feed resurrects the row — upsert clears the flag.
	src.stations = append(src.stations, commute.Station{
		ID: "MRTJ-DKA", Name: "Dukuh Atas BNI", Operator: "MRTJ",
		Latitude: ptr(-6.202), Longitude: ptr(106.823),
	})
	if _, err := Commute(ctx, pool, src); err != nil {
		t.Fatalf("ingest run 3: %v", err)
	}
	if err := pool.QueryRow(ctx,
		"SELECT removed_at FROM stops WHERE provider_entity_id='MRTJ-DKA'").Scan(&removedAt); err != nil {
		t.Fatalf("removed_at after resurrect: %v", err)
	}
	if removedAt != nil {
		t.Error("resurrected stop kept removed_at")
	}
}

func ptr(f float64) *float64 { return &f }

// A station without an upstream code is valid data (code is optional), but
// its transfers cannot be fetched — that edge is rejected, not run-fatal.
func TestIngestSkipsTransfersForCodelessStation(t *testing.T) {
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

	src := loadFixtureSource(t)
	lat, lon := -6.2, 106.8
	src.stations = append(src.stations, commute.Station{
		ID: "KCI-NOCODE", Name: "Tanpa Kode", Operator: "KCI",
		Latitude: &lat, Longitude: &lon,
	})

	report, err := Commute(ctx, pool, src)
	if err != nil {
		t.Fatalf("a codeless station must not fail the run: %v", err)
	}
	found := false
	for _, r := range report.Rejections {
		if r.EntityID == "KCI-NOCODE" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a missing-code rejection, got %+v", report.Rejections)
	}
}
