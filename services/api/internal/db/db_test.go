package db

import (
	"context"
	"os"
	"testing"
	"time"

	generated "singgah/services/api/db/generated"
)

// Integration test against the real PostGIS container — skipped unless
// TEST_DATABASE_URL points at a migrated database (compose or CI service).
func TestPostGISVersionQuery(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set — start infrastructure/local compose and run migrations")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := Connect(ctx, url)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer pool.Close()

	version, err := generated.New(pool).PostGISVersion(ctx)
	if err != nil {
		t.Fatalf("PostGISVersion: %v", err)
	}
	if version == "" {
		t.Error("postgis extension reported empty version")
	}
}
