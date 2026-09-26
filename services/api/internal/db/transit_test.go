package db

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
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

// The alias path: a query that only matches the operator's official name —
// not the display name or code — must still find the stop. The alias lives
// in metadata.official_name, written by the adapter at ingest.
func TestSearchStopsMatchesOfficialName(t *testing.T) {
	q, ctx := testQueries(t)
	provider := upsertProvider(t, q, ctx, uniqueCode(t), "Provider E")

	stop, err := q.UpsertStop(ctx, generated.UpsertStopParams{
		ProviderID:       provider.ID,
		ProviderEntityID: "alias-1",
		Kind:             "station",
		Name:             "Dukuh Atas",
		Code:             pgtype.Text{String: "DKA", Valid: true},
		Wgs84Point:       106.8230,
		Wgs84Point_2:     -6.2050,
		Metadata:         []byte(`{"official_name":"BNI City","score":4.5}`),
	})
	if err != nil {
		t.Fatalf("UpsertStop: %v", err)
	}

	rows, err := q.SearchStops(ctx, generated.SearchStopsParams{Name: "%bni city%", Limit: 10})
	if err != nil {
		t.Fatalf("SearchStops: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != stop.ID {
		t.Fatalf("official-name alias search returned %d rows, want 1", len(rows))
	}
}

// bbox + query runs inside one query: the alias must match there too, and
// the spatial predicate must still hold.
func TestListStopsInBBoxQueryFilter(t *testing.T) {
	q, ctx := testQueries(t)
	provider := upsertProvider(t, q, ctx, uniqueCode(t), "Provider F")
	mk := func(eid, name, official string, lon, lat float64) pgtype.UUID {
		meta := []byte(`{"official_name":"` + official + `"}`)
		s, err := q.UpsertStop(ctx, generated.UpsertStopParams{
			ProviderID: provider.ID, ProviderEntityID: eid, Kind: "station",
			Name: name, Wgs84Point: lon, Wgs84Point_2: lat, Metadata: meta,
		})
		if err != nil {
			t.Fatalf("UpsertStop %s: %v", eid, err)
		}
		return s.ID
	}

	// Surabaya bbox — far from committed Jakarta fixture rows.
	want := mk("alias-in", "Tunjungan", "BNI City", 112.7400, -7.2600)
	mk("other-in", "Gubeng", "Stasiun Gubeng", 112.7520, -7.2653)
	mk("alias-out", "Luar Bbox", "BNI City", 106.8307, -6.1767)

	rows, err := q.ListStopsInBBox(ctx, generated.ListStopsInBBoxParams{
		Column1: 112.70, Column2: -7.30, Column3: 112.80, Column4: -7.20,
		Name: "%bni city%", Limit: 10,
	})
	if err != nil {
		t.Fatalf("ListStopsInBBox: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != want {
		t.Fatalf("bbox+alias returned %d rows, want only the in-bbox alias match", len(rows))
	}

	// '%%' disables the text filter — both in-bbox stops come back.
	rows, err = q.ListStopsInBBox(ctx, generated.ListStopsInBBoxParams{
		Column1: 112.70, Column2: -7.30, Column3: 112.80, Column4: -7.20,
		Name: "%%", Limit: 10,
	})
	if err != nil {
		t.Fatalf("ListStopsInBBox %%: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("bbox with %%%% filter returned %d rows, want 2", len(rows))
	}
}

// Shape picking must follow the leg's listed stops, not just the termini.
// Regression for TJ:7F — a 60 km variant sharing the termini won the old
// endpoint-only score by ~2 m while detouring ~600 m+ from the listed halte.
// Setup mirrors it: the stop-hugging shape starts/ends slightly off the
// termini (loses endpoint snap); the detour hits them exactly.
func TestSliceRouteShapePicksStopHuggingShape(t *testing.T) {
	q, ctx := testQueries(t)
	provider := upsertProvider(t, q, ctx, uniqueCode(t), "Provider Shapes")
	agency, err := q.UpsertAgency(ctx, generated.UpsertAgencyParams{
		ProviderID: provider.ID, ProviderEntityID: "ag", Name: "Agency", Timezone: "Asia/Jakarta",
	})
	if err != nil {
		t.Fatalf("UpsertAgency: %v", err)
	}
	route, err := q.UpsertRoute(ctx, generated.UpsertRouteParams{
		AgencyID: agency.ID, ProviderID: provider.ID, ProviderEntityID: "r7f", Mode: "bus",
	})
	if err != nil {
		t.Fatalf("UpsertRoute: %v", err)
	}
	for i, wkt := range []string{
		// hugs every listed stop but starts/ends ~55 m off the termini
		"LINESTRING(106.80 -6.1695,106.83 -6.169,106.87 -6.169,106.90 -6.1695)",
		// termini exact, but swings ~2 km north mid-route
		"LINESTRING(106.80 -6.170,106.85 -6.150,106.90 -6.170)",
	} {
		if _, err := q.UpsertRouteShape(ctx, generated.UpsertRouteShapeParams{
			RouteID:        route.ID,
			StGeomfromtext: wkt,
			Source:         "test",
			SourceShapeID:  "shape-" + string(rune('a'+i)),
		}); err != nil {
			t.Fatalf("UpsertRouteShape %d: %v", i, err)
		}
	}

	g, err := q.SliceRouteShape(ctx, generated.SliceRouteShapeParams{
		RouteID: route.ID,
		FromLon: 106.80, FromLat: -6.170,
		ToLon: 106.90, ToLat: -6.170,
		Lons:     []float64{106.80, 106.83, 106.87, 106.90},
		Lats:     []float64{-6.170, -6.169, -6.169, -6.170},
		MaxSnapM: 250,
	})
	if err != nil {
		t.Fatalf("SliceRouteShape: %v", err)
	}
	var geom struct {
		Coordinates [][]float64 `json:"coordinates"`
	}
	if err := json.Unmarshal([]byte(g), &geom); err != nil {
		t.Fatalf("geometry not GeoJSON: %v\n%s", err, g)
	}
	// The correct cut must pass ~on top of mid-stop (106.83,-6.169); the
	// detour's nearest vertex sits ~2.5 km away.
	near := false
	for _, c := range geom.Coordinates {
		if len(c) == 2 && abs(c[0]-106.83)+abs(c[1]+6.169) < 0.001 {
			near = true
		}
	}
	if !near {
		t.Fatalf("picked shape does not hug the listed stops: %s", g)
	}
}

// Every shape missing a listed stop is worse than no shape — the query
// returns no row so the client draws the honest stop polyline instead.
func TestSliceRouteShapeRejectsShapeFarFromStops(t *testing.T) {
	q, ctx := testQueries(t)
	provider := upsertProvider(t, q, ctx, uniqueCode(t), "Provider Far")
	agency, err := q.UpsertAgency(ctx, generated.UpsertAgencyParams{
		ProviderID: provider.ID, ProviderEntityID: "ag", Name: "Agency", Timezone: "Asia/Jakarta",
	})
	if err != nil {
		t.Fatalf("UpsertAgency: %v", err)
	}
	route, err := q.UpsertRoute(ctx, generated.UpsertRouteParams{
		AgencyID: agency.ID, ProviderID: provider.ID, ProviderEntityID: "rfar", Mode: "bus",
	})
	if err != nil {
		t.Fatalf("UpsertRoute: %v", err)
	}
	if _, err := q.UpsertRouteShape(ctx, generated.UpsertRouteShapeParams{
		RouteID:        route.ID,
		StGeomfromtext: "LINESTRING(106.80 -6.170,106.85 -6.150,106.90 -6.170)",
		Source:         "test",
		SourceShapeID:  "detour",
	}); err != nil {
		t.Fatalf("UpsertRouteShape: %v", err)
	}

	_, err = q.SliceRouteShape(ctx, generated.SliceRouteShapeParams{
		RouteID: route.ID,
		FromLon: 106.80, FromLat: -6.170,
		ToLon: 106.90, ToLat: -6.170,
		Lons:     []float64{106.80, 106.83, 106.87, 106.90},
		Lats:     []float64{-6.170, -6.169, -6.169, -6.170},
		MaxSnapM: 250,
	})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("err = %v, want ErrNoRows — a shape 1+ km from a listed stop must be refused", err)
	}
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
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
		Name: "%%", Limit: 10,
	})
	if err != nil {
		t.Fatalf("ListStopsInBBox: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != inside.ID {
		t.Fatalf("expected only the inside stop, got %d rows", len(rows))
	}
}

// Corridor alternatives: a route carrying both endpoints in descending seq
// order still serves the ride direction (seq is flattened order, not travel
// direction); duplicate stop positions order by min-hops pair first.
func TestListRouteAlternativesAndSlice(t *testing.T) {
	q, ctx := testQueries(t)
	provider := upsertProvider(t, q, ctx, uniqueCode(t), "Provider Alts")
	agency, err := q.UpsertAgency(ctx, generated.UpsertAgencyParams{
		ProviderID: provider.ID, ProviderEntityID: "ag", Name: "Agency", Timezone: "Asia/Jakarta",
	})
	if err != nil {
		t.Fatalf("UpsertAgency: %v", err)
	}
	mkStop := func(eid, name string, lon, lat float64) pgtype.UUID {
		s, err := q.UpsertStop(ctx, generated.UpsertStopParams{
			ProviderID: provider.ID, ProviderEntityID: eid, Kind: "stop", Name: name,
			Wgs84Point: lon, Wgs84Point_2: lat, Metadata: []byte("{}"),
		})
		if err != nil {
			t.Fatalf("UpsertStop %s: %v", eid, err)
		}
		return s.ID
	}
	a := mkStop("alt-a", "Halte A", 106.80, -6.170)
	m := mkStop("alt-m", "Halte M", 106.83, -6.170)
	b := mkStop("alt-b", "Halte B", 106.87, -6.170)
	mkRoute := func(eid string, stops ...pgtype.UUID) generated.Route {
		r, err := q.UpsertRoute(ctx, generated.UpsertRouteParams{
			AgencyID: agency.ID, ProviderID: provider.ID, ProviderEntityID: eid, Mode: "bus",
		})
		if err != nil {
			t.Fatalf("UpsertRoute %s: %v", eid, err)
		}
		for i, sid := range stops {
			if err := q.InsertRouteStop(ctx, generated.InsertRouteStopParams{
				RouteID: r.ID, StopID: sid, Seq: int32(i + 1),
			}); err != nil {
				t.Fatalf("InsertRouteStop %s[%d]: %v", eid, i, err)
			}
		}
		return r
	}
	// Chosen route ascending; the alternative's seq runs toward the origin —
	// like TJ:2 where Monas precedes Sumur Batu in the flattened order.
	chosen := mkRoute("tj-7f", a, m, b)
	alt := mkRoute("tj-2", b, m, a)
	mkRoute("tj-2a", a)                     // missing b — not an alternative
	mkRoute("tj-9", m, b)                   // missing a — not an alternative
	dup := mkRoute("tj-dup", a, m, b, a, b) // a×b twice → four seq pairs

	rows, err := q.ListRouteAlternatives(ctx, generated.ListRouteAlternativesParams{
		StopID: a, StopID_2: b, ID: chosen.ID,
	})
	if err != nil {
		t.Fatalf("ListRouteAlternatives: %v", err)
	}
	if len(rows) != 5 {
		t.Fatalf("alternatives = %d rows, want 5 (tj-2 once + dup's 2x2 pairs)", len(rows))
	}
	if rows[0].ID != alt.ID || rows[0].SeqFrom != 3 || rows[0].SeqTo != 1 {
		t.Fatalf("first alternative = %+v — want tj-2 descending 3->1", rows[0])
	}
	// tj-dup contributes every from×to pair, min-hops ones first
	var dupRows []generated.ListRouteAlternativesRow
	for _, r := range rows {
		if r.ID == dup.ID {
			dupRows = append(dupRows, r)
		}
	}
	if len(dupRows) != 4 || abs32(dupRows[0].SeqFrom-dupRows[0].SeqTo) != 1 {
		t.Fatalf("dup pairs not min-hops ordered: %+v", dupRows)
	}

	slice, err := q.ListRouteStopSlice(ctx, generated.ListRouteStopSliceParams{
		RouteID: alt.ID, Seq: 1, Seq_2: 3,
	})
	if err != nil {
		t.Fatalf("ListRouteStopSlice: %v", err)
	}
	if len(slice) != 3 || slice[0].ID != b || slice[2].ID != a {
		t.Fatalf("slice = %+v — ascending order, caller reverses for the ride", slice)
	}
}

func abs32(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}
