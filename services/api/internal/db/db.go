// Package db owns the PostgreSQL connection pool. pgxpool is the single
// connection API; sqlc-generated code accepts it via pgx/v5's DBTX interface.
package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect opens a pool and verifies it with a ping so a bad DATABASE_URL or
// an unreachable server fails fast at startup instead of at first request.
func Connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("db: parse url: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}
	return pool, nil
}
