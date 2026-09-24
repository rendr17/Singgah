package db

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	generated "singgah/services/api/db/generated"
)

// Schema-invariant tests against the real PostGIS container — skipped unless
// TEST_DATABASE_URL points at a migrated database (compose or CI service).
// Each run mints a unique provider code, so re-runs are safe on a dirty DB.

// Each test runs inside a rolled-back transaction: full isolation between
// tests and no residue in the shared test database.
func testQueries(t *testing.T) (*generated.Queries, context.Context) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set — start infrastructure/local compose and run migrations")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	pool, err := Connect(ctx, url)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(pool.Close)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	return generated.New(pool).WithTx(tx), ctx
}

func uniqueCode(t *testing.T) string {
	t.Helper()
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatalf("rand: %v", err)
	}
	return "test-" + hex.EncodeToString(b[:])
}

func upsertProvider(t *testing.T, q *generated.Queries, ctx context.Context, code, name string) generated.Provider {
	t.Helper()
	p, err := q.UpsertProvider(ctx, generated.UpsertProviderParams{
		Code:           code,
		Name:           name,
		LicenseName:    pgtype.Text{String: "test license", Valid: true},
		RefreshCadence: pgtype.Text{String: "daily", Valid: true},
	})
	if err != nil {
		t.Fatalf("UpsertProvider: %v", err)
	}
	return p
}

// Ingest re-runs must keep the canonical UUID stable: upserting the same
// (provider_id, provider_entity_id) updates the row instead of duplicating it.
func TestTransitUpsertsAreIdempotent(t *testing.T) {
	q, ctx := testQueries(t)

	provider := upsertProvider(t, q, ctx, uniqueCode(t), "Provider A")
	again := upsertProvider(t, q, ctx, provider.Code, "Provider A v2")
	if again.ID != provider.ID {
		t.Error("provider upsert changed canonical ID")
	}
	if again.Name != "Provider A v2" {
		t.Errorf("provider name not updated, got %q", again.Name)
	}

	agency, err := q.UpsertAgency(ctx, generated.UpsertAgencyParams{
		ProviderID:       provider.ID,
		ProviderEntityID: "ag-1",
		Name:             "Agency A",
		Timezone:         "Asia/Jakarta",
	})
	if err != nil {
		t.Fatalf("UpsertAgency: %v", err)
	}
	agency2, err := q.UpsertAgency(ctx, generated.UpsertAgencyParams{
		ProviderID:       provider.ID,
		ProviderEntityID: "ag-1",
		Name:             "Agency A renamed",
		Timezone:         "Asia/Jakarta",
	})
	if err != nil {
		t.Fatalf("UpsertAgency re-run: %v", err)
	}
	if agency2.ID != agency.ID || agency2.Name != "Agency A renamed" {
		t.Error("agency upsert not idempotent")
	}

	// Near Gambir station, Jakarta.
	stop, err := q.UpsertStop(ctx, generated.UpsertStopParams{
		ProviderID:       provider.ID,
		ProviderEntityID: "stop-1",
		Kind:             "station",
		Name:             "Gambir",
		Wgs84Point:       106.8307,
		Wgs84Point_2:     -6.1767,
		Metadata:         []byte("{}"),
	})
	if err != nil {
		t.Fatalf("UpsertStop: %v", err)
	}
	stop2, err := q.UpsertStop(ctx, generated.UpsertStopParams{
		ProviderID:       provider.ID,
		ProviderEntityID: "stop-1",
		Kind:             "station",
		Name:             "Stasiun Gambir",
		Wgs84Point:       106.8307,
		Wgs84Point_2:     -6.1767,
		Metadata:         []byte("{}"),
	})
	if err != nil {
		t.Fatalf("UpsertStop re-run: %v", err)
	}
	if stop2.ID != stop.ID || stop2.Name != "Stasiun Gambir" {
		t.Error("stop upsert not idempotent")
	}

	route, err := q.UpsertRoute(ctx, generated.UpsertRouteParams{
		AgencyID:         agency.ID,
		ProviderID:       provider.ID,
		ProviderEntityID: "route-1",
		ShortName:        pgtype.Text{String: "CK", Valid: true},
		Mode:             "rail",
	})
	if err != nil {
		t.Fatalf("UpsertRoute: %v", err)
	}
	if route.Mode != "rail" {
		t.Errorf("route mode = %q", route.Mode)
	}
}

// geography(point,4326) rejects out-of-range coordinates at the type level —
// a broken provider payload cannot land in the catalog.
func TestUpsertStopRejectsBrokenCoordinate(t *testing.T) {
	q, ctx := testQueries(t)
	provider := upsertProvider(t, q, ctx, uniqueCode(t), "Provider B")

	_, err := q.UpsertStop(ctx, generated.UpsertStopParams{
		ProviderID:       provider.ID,
		ProviderEntityID: "bad-1",
		Kind:             "stop",
		Name:             "Nowhere",
		Wgs84Point:       106.8,
		Wgs84Point_2:     -91.0,
		Metadata:         []byte("{}"),
	})
	if err == nil {
		t.Error("expected latitude -91 to be rejected")
	}
}

// A transfer must reference real stops — no dangling edges in the network.
func TestUpsertTransferRequiresExistingStops(t *testing.T) {
	q, ctx := testQueries(t)

	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		t.Fatalf("rand: %v", err)
	}
	missing := pgtype.UUID{Bytes: raw, Valid: true}

	_, err := q.UpsertTransfer(ctx, generated.UpsertTransferParams{
		FromStopID:    missing,
		ToStopID:      missing,
		Accessibility: []byte("{}"),
		FareContext:   []byte("{}"),
	})
	if err == nil {
		t.Error("expected FK violation for missing stops")
	}
}

// Row-local integrity: a stop cannot parent itself, a transfer cannot loop
// back to the same stop or carry negative physical quantities.
func TestIntegrityConstraints(t *testing.T) {
	q, ctx := testQueries(t)
	provider := upsertProvider(t, q, ctx, uniqueCode(t), "Provider D")

	stop, err := q.UpsertStop(ctx, generated.UpsertStopParams{
		ProviderID:       provider.ID,
		ProviderEntityID: "s1",
		Kind:             "station",
		Name:             "Juanda",
		Wgs84Point:       106.8505,
		Wgs84Point_2:     -6.1768,
		Metadata:         []byte("{}"),
	})
	if err != nil {
		t.Fatalf("UpsertStop: %v", err)
	}

	// Self-parent can't be expressed on insert (the id doesn't exist yet), so
	// create the row first, then upsert it pointing at itself.
	self, err := q.UpsertStop(ctx, generated.UpsertStopParams{
		ProviderID:       provider.ID,
		ProviderEntityID: "s3",
		Kind:             "platform",
		Name:             "Temp",
		Wgs84Point:       106.8505,
		Wgs84Point_2:     -6.1768,
		Metadata:         []byte("{}"),
	})
	if err != nil {
		t.Fatalf("UpsertStop s3: %v", err)
	}
	if _, err := q.UpsertStop(ctx, generated.UpsertStopParams{
		ProviderID:       provider.ID,
		ProviderEntityID: "s3",
		ParentStationID:  self.ID,
		Kind:             "platform",
		Name:             "Self parent",
		Wgs84Point:       106.8505,
		Wgs84Point_2:     -6.1768,
		Metadata:         []byte("{}"),
	}); err == nil {
		t.Error("expected self-parent to be rejected")
	}

	if _, err := q.UpsertTransfer(ctx, generated.UpsertTransferParams{
		FromStopID:    stop.ID,
		ToStopID:      stop.ID,
		Accessibility: []byte("{}"),
		FareContext:   []byte("{}"),
	}); err == nil {
		t.Error("expected self-transfer to be rejected")
	}

	if _, err := q.UpsertTransfer(ctx, generated.UpsertTransferParams{
		FromStopID:    stop.ID,
		ToStopID:      self.ID,
		WalkDistanceM: pgtype.Int4{Int32: -5, Valid: true},
		Accessibility: []byte("{}"),
		FareContext:   []byte("{}"),
	}); err == nil {
		t.Error("expected negative walk_distance_m to be rejected")
	}
}

// The spatial index path: radius query returns real meter distances.
// The anchor is Surabaya — far from the Jakarta fixture data that the ingest
// test commits to the shared test database.
func TestListStopsWithin(t *testing.T) {
	q, ctx := testQueries(t)
	provider := upsertProvider(t, q, ctx, uniqueCode(t), "Provider C")

	near, err := q.UpsertStop(ctx, generated.UpsertStopParams{
		ProviderID:       provider.ID,
		ProviderEntityID: "near",
		Kind:             "station",
		Name:             "Surabaya Gubeng",
		Wgs84Point:       112.7521,
		Wgs84Point_2:     -7.2653,
		Metadata:         []byte("{}"),
	})
	if err != nil {
		t.Fatalf("UpsertStop near: %v", err)
	}
	if _, err := q.UpsertStop(ctx, generated.UpsertStopParams{
		ProviderID:       provider.ID,
		ProviderEntityID: "far",
		Kind:             "station",
		Name:             "Gambir",
		Wgs84Point:       106.8307,
		Wgs84Point_2:     -6.1767,
		Metadata:         []byte("{}"),
	}); err != nil {
		t.Fatalf("UpsertStop far: %v", err)
	}

	rows, err := q.ListStopsWithin(ctx, generated.ListStopsWithinParams{
		Wgs84Point:   112.7520,
		Wgs84Point_2: -7.2650,
		StDwithin:    5000,
		Limit:        10,
	})
	if err != nil {
		t.Fatalf("ListStopsWithin: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != near.ID {
		t.Fatalf("expected only the near stop, got %d rows", len(rows))
	}
	if rows[0].DistanceM <= 0 || rows[0].DistanceM > 5000 {
		t.Errorf("distance_m = %v, want within (0, 5000]", rows[0].DistanceM)
	}
}

func TestListStopsInBBox(t *testing.T) {
	q, ctx := testQueries(t)
	provider := upsertProvider(t, q, ctx, uniqueCode(t), "Provider D")

	inside, err := q.UpsertStop(ctx, generated.UpsertStopParams{
		ProviderID:       provider.ID,
		ProviderEntityID: "inside",
		Kind:             "station",
		Name:             "Surabaya Gubeng",
		Wgs84Point:       112.7521,
		Wgs84Point_2:     -7.2653,
		Metadata:         []byte("{}"),
	})
	if err != nil {
		t.Fatalf("UpsertStop inside: %v", err)
	}
	if _, err := q.UpsertStop(ctx, generated.UpsertStopParams{
		ProviderID:       provider.ID,
		ProviderEntityID: "outside",
		Kind:             "station",
		Name:             "Gambir",
		Wgs84Point:       106.8307,
		Wgs84Point_2:     -6.1767,
		Metadata:         []byte("{}"),
	}); err != nil {
		t.Fatalf("UpsertStop outside: %v", err)
	}

	// Surabaya viewport — far from the Jakarta fixture/ingest rows.
	rows, err := q.ListStopsInBBox(ctx, generated.ListStopsInBBoxParams{
		Column1: 112.70, Column2: -7.30, Column3: 112.80, Column4: -7.20,
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("ListStopsInBBox: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != inside.ID {
		t.Fatalf("expected only the inside stop, got %d rows", len(rows))
	}
}
