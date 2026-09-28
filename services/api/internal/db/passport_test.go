package db

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	generated "singgah/services/api/db/generated"
)

// User-data path against the real PostGIS container — skipped unless
// TEST_DATABASE_URL points at a migrated database. Each test runs inside a
// rolled-back transaction (same discipline as transit_test).

func testTxQueries(t *testing.T) (*generated.Queries, pgx.Tx, context.Context) {
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
	return generated.New(pool).WithTx(tx), tx, ctx
}

func seedUser(t *testing.T, q *generated.Queries, ctx context.Context) pgtype.UUID {
	t.Helper()
	sum := sha256.Sum256([]byte("test-token-" + uniqueCode(t)))
	sess, err := q.CreateAnonymousSession(ctx, generated.CreateAnonymousSessionParams{
		TokenHash: sum[:],
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(24 * time.Hour), Valid: true},
	})
	if err != nil {
		t.Fatalf("CreateAnonymousSession: %v", err)
	}
	return sess.UserID
}

func seedStop(t *testing.T, q *generated.Queries, ctx context.Context, code string) generated.Stop {
	t.Helper()
	p, err := q.UpsertProvider(ctx, generated.UpsertProviderParams{
		Code: uniqueCode(t),
		Name: "test provider",
	})
	if err != nil {
		t.Fatalf("UpsertProvider: %v", err)
	}
	s, err := q.UpsertStop(ctx, generated.UpsertStopParams{
		ProviderID:       p.ID,
		ProviderEntityID: code,
		Kind:             "station",
		Name:             "Test Stop " + code,
		Wgs84Point:       106.83,
		Wgs84Point_2:     -6.17,
		Metadata:         []byte("{}"),
	})
	if err != nil {
		t.Fatalf("UpsertStop: %v", err)
	}
	return s
}

func seedRouteWithStops(t *testing.T, q *generated.Queries, ctx context.Context, stops ...generated.Stop) generated.Route {
	t.Helper()
	p, err := q.UpsertProvider(ctx, generated.UpsertProviderParams{
		Code: uniqueCode(t),
		Name: "test provider",
	})
	if err != nil {
		t.Fatalf("UpsertProvider: %v", err)
	}
	a, err := q.UpsertAgency(ctx, generated.UpsertAgencyParams{
		ProviderID:       p.ID,
		ProviderEntityID: "agency-1",
		Name:             "Test Agency",
	})
	if err != nil {
		t.Fatalf("UpsertAgency: %v", err)
	}
	r, err := q.UpsertRoute(ctx, generated.UpsertRouteParams{
		AgencyID:         a.ID,
		ProviderID:       p.ID,
		ProviderEntityID: "route-1",
		Mode:             "bus",
	})
	if err != nil {
		t.Fatalf("UpsertRoute: %v", err)
	}
	for i, s := range stops {
		if err := q.InsertRouteStop(ctx, generated.InsertRouteStopParams{
			RouteID: r.ID,
			StopID:  s.ID,
			Seq:     int32(i + 1),
		}); err != nil {
			t.Fatalf("InsertRouteStop: %v", err)
		}
	}
	return r
}

func recordVisit(t *testing.T, q *generated.Queries, ctx context.Context, userID, stopID pgtype.UUID, mutationID string, observed time.Time) generated.RecordVisitEventRow {
	t.Helper()
	var mid pgtype.UUID
	if err := mid.Scan(mutationID); err != nil {
		t.Fatalf("scan mutation id: %v", err)
	}
	v, err := q.RecordVisitEvent(ctx, generated.RecordVisitEventParams{
		UserID:           userID,
		StopID:           stopID,
		ClientMutationID: mid,
		ObservedAt:       pgtype.Timestamptz{Time: observed, Valid: true},
		ValidationMethod: "manual",
		Status:           "low_confidence",
	})
	if err != nil {
		t.Fatalf("RecordVisitEvent: %v", err)
	}
	return v
}

func TestVisitEventReplayIsIdempotent(t *testing.T) {
	q, _, ctx := testTxQueries(t)
	user := seedUser(t, q, ctx)
	stop := seedStop(t, q, ctx, "stop-a")

	mut := "aaaaaaaa-0000-0000-0000-000000000001"
	first := recordVisit(t, q, ctx, user, stop.ID, mut, time.Now())
	if !first.WasInserted {
		t.Fatal("first insert should report was_inserted=true")
	}
	second := recordVisit(t, q, ctx, user, stop.ID, mut, time.Now().Add(time.Hour))
	if second.WasInserted {
		t.Fatal("replay must not insert a second row")
	}
	if second.ID != first.ID {
		t.Fatalf("replay returned different row: %v vs %v", first.ID, second.ID)
	}
	list, err := q.ListVisitEvents(ctx, generated.ListVisitEventsParams{UserID: user, Limit: 50})
	if err != nil {
		t.Fatalf("ListVisitEvents: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("replay created duplicates: %d rows", len(list))
	}
}

func TestListVisitEventsKeysetPagination(t *testing.T) {
	q, _, ctx := testTxQueries(t)
	user := seedUser(t, q, ctx)
	stop := seedStop(t, q, ctx, "stop-b")

	base := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		mut := fmt.Sprintf("bbbbbbbb-0000-0000-0000-%012x", i+1)
		recordVisit(t, q, ctx, user, stop.ID, mut, base.Add(-time.Duration(i)*time.Hour))
	}

	seen := map[string]bool{}
	before := generated.ListVisitEventsParams{UserID: user, Limit: 3}
	var sizes []int
	for page := 0; page < 10; page++ {
		rows, err := q.ListVisitEvents(ctx, before)
		if err != nil {
			t.Fatalf("page %d: %v", page, err)
		}
		sizes = append(sizes, len(rows))
		if len(rows) == 0 {
			break
		}
		var prev time.Time
		for i, v := range rows {
			k := v.ObservedAt.Time.Format(time.RFC3339Nano)
			if seen[k] {
				t.Fatalf("duplicate row across pages: %v", k)
			}
			seen[k] = true
			if i > 0 && v.ObservedAt.Time.After(prev) {
				t.Fatal("ordering broken: observed_at not descending")
			}
			prev = v.ObservedAt.Time
		}
		last := rows[len(rows)-1]
		before.BeforeAt = last.ObservedAt
		before.BeforeID = last.ID
	}
	if len(seen) != 5 {
		t.Fatalf("walked %d distinct rows want 5", len(seen))
	}
	if len(sizes) < 2 || sizes[0] != 3 || sizes[1] != 2 {
		t.Fatalf("page sizes %v want [3 2 ...]", sizes)
	}
}

func TestPassportProgressCountsDistinctVisits(t *testing.T) {
	q, _, ctx := testTxQueries(t)
	user := seedUser(t, q, ctx)
	other := seedUser(t, q, ctx)
	stop1 := seedStop(t, q, ctx, "prog-1")
	stop2 := seedStop(t, q, ctx, "prog-2")
	seedRouteWithStops(t, q, ctx, stop1, stop2)

	recordVisit(t, q, ctx, user, stop1.ID, "cccccccc-0000-0000-0000-000000000001", time.Now())

	total, err := q.PassportProgressTotal(ctx, user)
	if err != nil {
		t.Fatalf("PassportProgressTotal: %v", err)
	}
	if total.VisitedStops != 1 || total.TotalStops < 2 {
		t.Fatalf("total: got %d/%d want visited=1 over >=2", total.VisitedStops, total.TotalStops)
	}
	// Note: TotalStops counts every route-served stop in the test DB — the
	// invariant that matters is our two seeded stops are included and the
	// visited set is exactly ours.
	otherTotal, err := q.PassportProgressTotal(ctx, other)
	if err != nil {
		t.Fatalf("other user total: %v", err)
	}
	if otherTotal.VisitedStops != 0 {
		t.Fatalf("other user saw %d visited — ownership leak", otherTotal.VisitedStops)
	}
	byColl, err := q.PassportProgressByCollection(ctx, user)
	if err != nil {
		t.Fatalf("byCollection: %v", err)
	}
	// Seeded stops don't belong to any collection — the join must not
	// invent rows for them.
	for _, c := range byColl {
		if c.VisitedStops > 0 && c.TotalStops == 0 {
			t.Fatalf("collection %s: visited without members", c.Slug)
		}
	}
}

func TestDeleteDeadAuthSessionsSweeps(t *testing.T) {
	q, tx, ctx := testTxQueries(t)
	user := seedUser(t, q, ctx)

	// Dead: revoked + expired. Alive: fresh.
	sum := sha256.Sum256([]byte("revoked-" + uniqueCode(t)))
	if _, err := q.CreateAnonymousSession(ctx, generated.CreateAnonymousSessionParams{
		TokenHash: sum[:],
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(24 * time.Hour), Valid: true},
	}); err != nil {
		t.Fatalf("seed revoked session: %v", err)
	}
	if err := q.RevokeAuthSession(ctx, sum[:]); err != nil {
		t.Fatalf("RevokeAuthSession: %v", err)
	}
	exp := sha256.Sum256([]byte("expired-" + uniqueCode(t)))
	if _, err := tx.Exec(ctx, `
		INSERT INTO auth_sessions (user_id, token_hash, expires_at)
		VALUES ($1, $2, now() - interval '1 day')`, user, exp[:]); err != nil {
		t.Fatalf("seed expired session: %v", err)
	}

	n, err := q.DeleteDeadAuthSessions(ctx)
	if err != nil {
		t.Fatalf("DeleteDeadAuthSessions: %v", err)
	}
	if n != 2 {
		t.Fatalf("deleted %d rows want 2 (revoked + expired)", n)
	}
	// The live session survives; the revoked one is gone for real.
	if _, err := q.GetAuthSessionByTokenHash(ctx, sum[:]); err == nil {
		t.Fatal("revoked session still resolves after sweep")
	}
}
