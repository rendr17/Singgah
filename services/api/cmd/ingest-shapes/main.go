// Command ingest-shapes imports route path geometry from a GTFS feed
// (routes.txt + trips.txt + shapes.txt) into route_shapes. It is separate
// from the catalog ingest: different source, different cadence, different
// failure domain. Requires DATABASE_URL; -feed accepts a URL or a local zip.
package main

import (
	"archive/zip"
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"singgah/services/api/internal/db"
	"singgah/services/api/internal/ingest"
	"singgah/services/api/internal/provider/osm"

	generated "singgah/services/api/db/generated"
)

// tjGTFS registers the TransJakarta GTFS feed (docs/35_DATA_SOURCES.md) —
// official open data published by PT Transportasi Jakarta via PPID, served
// from gtfs.transjakarta.co.id. License field stays honest: the feed ships
// no feed_info.txt and the PPID page states no formal license.
var tjGTFS = generated.UpsertProviderParams{
	Code:            "tj-gtfs",
	Name:            "TransJakarta GTFS",
	SourceUrl:       pgtype.Text{String: "https://gtfs.transjakarta.co.id/files/file_gtfs.zip", Valid: true},
	TermsUrl:        pgtype.Text{String: "https://ppid.transjakarta.co.id/pusat-data/data-terbuka/transjakarta-gtfs-feed", Valid: true},
	LicenseName:     pgtype.Text{String: "official open data — no formal license stated", Valid: true},
	AttributionText: pgtype.Text{String: "Geometri rute oleh PT Transportasi Jakarta (GTFS feed)", Valid: true},
	AllowedUse:      pgtype.Text{String: "route path geometry for map display", Valid: true},
	RefreshCadence:  pgtype.Text{String: "monthly", Valid: true},
	Owner:           pgtype.Text{String: "PT Transportasi Jakarta", Valid: true},
	KnownLimitations: pgtype.Text{
		String: "path geometry only — no schedules/fares; ~6 corridor codes in the catalog have no GTFS counterpart",
		Valid:  true,
	},
}

func openFeed(spec string) (*zip.Reader, error) {
	if strings.HasPrefix(spec, "http://") || strings.HasPrefix(spec, "https://") {
		resp, err := http.Get(spec) //nolint:gosec — operator-provided URL is the point
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("feed download: HTTP %d", resp.StatusCode)
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return zip.NewReader(bytes.NewReader(body), int64(len(body)))
	}
	body, err := os.ReadFile(spec)
	if err != nil {
		return nil, err
	}
	return zip.NewReader(bytes.NewReader(body), int64(len(body)))
}

func main() {
	source := flag.String("source", "gtfs", "shape source: gtfs | osm")
	feed := flag.String("feed", "https://gtfs.transjakarta.co.id/files/file_gtfs.zip", "GTFS zip URL or local file (source=gtfs)")
	prefix := flag.String("prefix", "TJ:", "canonical provider_entity_id prefix used to match route_short_name (source=gtfs)")
	overpass := flag.String("overpass", osm.DefaultBaseURL, "Overpass endpoint (source=osm)")
	matchProvider := flag.String("match-provider", "commute", "provider code whose routes the shapes attach to")
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

	switch *source {
	case "gtfs":
		zr, err := openFeed(*feed)
		if err != nil {
			slog.Error("open feed", "error", err)
			os.Exit(1)
		}
		report, err := ingest.Shapes(ctx, pool, zr, tjGTFS, *matchProvider, *prefix)
		if err != nil {
			slog.Error("ingest shapes failed", "error", err)
			os.Exit(1)
		}
		fmt.Printf("shapes ingest ok: %d parsed, %d matched, %d unmatched; %d skipped rows, %d orphans, %d unbound\n",
			report.Parsed, report.Matched, len(report.Unmatched),
			report.Parse.SkippedRows, report.Parse.OrphanShape, report.Parse.UnboundShape)
		for _, name := range report.Unmatched {
			fmt.Printf("  no canonical route for GTFS route %q\n", name)
		}
	case "osm":
		// All relation ids in one Overpass call; mirrors that rate-limit or
		// block the caller fall back to the main API's per-relation /full.
		var ids []int64
		for _, rels := range osm.RouteRelations {
			ids = append(ids, rels...)
		}
		client := osm.NewClient(*overpass, osm.DefaultAPIBaseURL)
		geoms, err := client.RelationGeoms(ctx, ids)
		if err != nil {
			slog.Warn("overpass failed — falling back to api.openstreetmap.org /full", "error", err)
			geoms, err = client.RelationGeomsFull(ctx, ids)
			if err != nil {
				slog.Error("ingest osm failed", "error", err)
				os.Exit(1)
			}
		}
		report, err := ingest.OSMShapes(ctx, pool, geoms, osm.RouteRelations, *matchProvider)
		if err != nil {
			slog.Error("ingest osm failed", "error", err)
			os.Exit(1)
		}
		fmt.Printf("osm ingest ok: %d relations, %d components written; %d routes matched, %d unmatched\n",
			report.Relations, report.Components, report.Matched, len(report.Unmatched))
		for _, name := range report.Unmatched {
			fmt.Printf("  no canonical route / missing relation for %q\n", name)
		}
	default:
		slog.Error("unknown -source", "value", *source)
		os.Exit(1)
	}
}
