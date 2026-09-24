package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
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
}

func (f fixtureSource) Operators(context.Context) ([]commute.Operator, error) { return f.ops, nil }
func (f fixtureSource) Stations(context.Context) ([]commute.Station, error)   { return f.stations, nil }
func (f fixtureSource) Transfers(_ context.Context, op, code string) ([]commute.Transfer, error) {
	return f.transfers[op+"/"+code], nil
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
	if report.Operators != 2 || report.Stops != 4 || report.Routes != 3 || report.Transfers != 2 {
		t.Errorf("counts: %+v", report)
	}
	// The EXTERNAL transfer must be reported, not silently dropped.
	if len(report.Rejections) != 1 || report.Rejections[0].EntityID != "T-KCI-SUD-EXT-1" {
		t.Errorf("rejections: %+v", report.Rejections)
	}

	var stopsBefore, transfersBefore int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM stops").Scan(&stopsBefore); err != nil {
		t.Fatalf("count stops: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM transfers").Scan(&transfersBefore); err != nil {
		t.Fatalf("count transfers: %v", err)
	}

	// Re-ingest: upserts must keep canonical IDs and not duplicate rows.
	report2, err := Commute(ctx, pool, src)
	if err != nil {
		t.Fatalf("ingest run 2: %v", err)
	}
	var stopsAfter, transfersAfter int
	_ = pool.QueryRow(ctx, "SELECT count(*) FROM stops").Scan(&stopsAfter)
	_ = pool.QueryRow(ctx, "SELECT count(*) FROM transfers").Scan(&transfersAfter)
	if stopsAfter != stopsBefore || transfersAfter != transfersBefore {
		t.Errorf("re-ingest duplicated rows: stops %d->%d transfers %d->%d",
			stopsBefore, stopsAfter, transfersBefore, transfersAfter)
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
