// Command ingest-places imports named POIs from OpenStreetMap into places +
// place_transit_access (ADR-011). One bounded Overpass call at ingest time —
// OSM is never a runtime dependency. Requires DATABASE_URL.
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
	"singgah/services/api/internal/provider/osm"
)

func main() {
	south := flag.Float64("south", osm.JabodetabekBBox.South, "bbox south")
	west := flag.Float64("west", osm.JabodetabekBBox.West, "bbox west")
	north := flag.Float64("north", osm.JabodetabekBBox.North, "bbox north")
	east := flag.Float64("east", osm.JabodetabekBBox.East, "bbox east")
	overpass := flag.String("overpass", osm.DefaultBaseURL, "Overpass endpoint")
	grid := flag.Int("grid", 3, "split the bbox into grid×grid tiles — bounded per-tile responses, polite to public mirrors")
	flag.Parse()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		slog.Error("DATABASE_URL is required")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	pool, err := db.Connect(ctx, databaseURL)
	if err != nil {
		slog.Error("database connect", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	client := osm.NewClient(*overpass, osm.DefaultAPIBaseURL)
	client.SetTimeout(5 * time.Minute)

	box := osm.BBox{South: *south, West: *west, North: *north, East: *east}
	tileList := tiles(box, *grid)

	var total ingest.PlacesReport
	for i, t := range tileList {
		if i > 0 {
			select {
			case <-ctx.Done():
				slog.Error("cancelled between tiles", "error", ctx.Err())
				os.Exit(1)
			case <-time.After(5 * time.Second): // polite gap between mirror calls
			}
		}
		pois, err := client.POIs(ctx, t)
		if err != nil {
			slog.Error("overpass POI fetch failed", "tile", i, "error", err)
			os.Exit(1)
		}
		report, err := ingest.Places(ctx, pool, pois)
		if err != nil {
			slog.Error("ingest places failed", "tile", i, "error", err)
			os.Exit(1)
		}
		slog.Info("tile done", "tile", i, "of", len(tileList),
			"fetched", report.Fetched, "upserted", report.Upserted, "access", report.AccessRows)
		total.Fetched += report.Fetched
		total.Upserted += report.Upserted
		total.AccessRows += report.AccessRows
	}
	fmt.Printf("places ingest ok: %d fetched, %d upserted, %d transit-access rows across %d tiles\n",
		total.Fetched, total.Upserted, total.AccessRows, len(tileList))
}

// tiles splits box into grid×grid sub-boxes, west→east then south→north.
// Overlapping tile edges are harmless — the place upsert is idempotent.
func tiles(b osm.BBox, grid int) []osm.BBox {
	if grid < 1 {
		grid = 1
	}
	latStep := (b.North - b.South) / float64(grid)
	lonStep := (b.East - b.West) / float64(grid)
	out := make([]osm.BBox, 0, grid*grid)
	for row := 0; row < grid; row++ {
		for col := 0; col < grid; col++ {
			out = append(out, osm.BBox{
				South: b.South + float64(row)*latStep,
				North: b.South + float64(row+1)*latStep,
				West:  b.West + float64(col)*lonStep,
				East:  b.West + float64(col+1)*lonStep,
			})
		}
	}
	return out
}
