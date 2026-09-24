// Command ingest refreshes the canonical transit catalog from the Commute
// Data Platform. Run manually or on a schedule; it requires DATABASE_URL.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"singgah/services/api/internal/db"
	"singgah/services/api/internal/ingest"
	"singgah/services/api/internal/provider/commute"
)

func main() {
	baseURL := flag.String("base-url", commute.DefaultBaseURL, "provider API base URL (override for fixtures/staging)")
	flag.Parse()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		slog.Error("DATABASE_URL is required")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	pool, err := db.Connect(ctx, databaseURL)
	if err != nil {
		slog.Error("database connect", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	report, err := ingest.Commute(ctx, pool, commute.NewClient(*baseURL))
	if err != nil {
		slog.Error("ingest failed", "error", err)
		os.Exit(1)
	}

	fmt.Printf("ingest ok: %d operators, %d stops, %d routes, %d transfers; %d rejected\n",
		report.Operators, report.Stops, report.Routes, report.Transfers, len(report.Rejections))
	for _, r := range report.Rejections {
		fmt.Printf("  rejected %s: %s\n", r.EntityID, r.Reason)
	}
}
