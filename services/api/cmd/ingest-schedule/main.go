// Command ingest-schedule imports scheduled services (services, trips,
// stop_times, frequencies) — either from a GTFS feed (-source gtfs) or by
// sweeping Commute station timetables (-source commute). It is separate
// from the catalog ingest and the shapes ingest: different cadence,
// different failure domain. Requires DATABASE_URL.
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
	"singgah/services/api/internal/provider/gtfs"
)

func main() {
	source := flag.String("source", "gtfs", "schedule source: gtfs | commute")
	feed := flag.String("feed", "https://gtfs.transjakarta.co.id/files/file_gtfs.zip", "GTFS zip URL or local file")
	matchProvider := flag.String("match-provider", "commute", "provider code whose routes/stops the schedule attaches to")
	routePrefix := flag.String("route-prefix", "TJ:", "canonical route provider_entity_id prefix for route_short_name")
	stopPrefix := flag.String("stop-prefix", "TJ-", "canonical stop provider_entity_id prefix for GTFS stop_id")
	baseURL := flag.String("base-url", commute.DefaultBaseURL, "Commute API base URL (commute source)")
	pace := flag.Duration("pace", 150*time.Millisecond, "delay between station timetable calls (commute source)")
	flag.Parse()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		slog.Error("DATABASE_URL is required")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	pool, err := db.Connect(ctx, databaseURL)
	if err != nil {
		slog.Error("database connect", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	switch *source {
	case "gtfs":
		zr, err := gtfs.OpenFeed(*feed)
		if err != nil {
			slog.Error("open feed", "error", err)
			os.Exit(1)
		}
		report, err := ingest.Schedule(ctx, pool, zr, gtfs.TJProviderRegistration, *matchProvider, *routePrefix, *stopPrefix)
		if err != nil {
			slog.Error("ingest schedule failed", "error", err)
			os.Exit(1)
		}
		fmt.Printf("schedule ingest ok: %d services, %d trips (%d stop_times, %d frequencies); %d unmatched routes, %d unmatched stops, %d rejected trips; parse skipped %d rows (%d orphan stop_times, %d orphan freqs)\n",
			report.Services, report.Trips, report.StopTimes, report.Frequencies,
			len(report.UnmatchedRoutes), len(report.UnmatchedStops), len(report.RejectedTrips),
			report.Parse.SkippedRows, report.Parse.OrphanStopTimes, report.Parse.OrphanFreqs)
		for _, name := range report.UnmatchedRoutes {
			fmt.Printf("  no canonical route for GTFS route %q\n", name)
		}
		for _, rej := range report.RejectedTrips {
			fmt.Printf("  rejected trip %s\n", rej)
		}
	case "commute":
		report, err := ingest.CommuteSchedule(ctx, pool, commute.NewClient(*baseURL), *pace)
		if err != nil {
			slog.Error("ingest timetables failed", "error", err)
			os.Exit(1)
		}
		fmt.Printf("timetable sweep ok: %d stations (%d no timetable, %d failed), %d entries -> %d trips (%d stop_times, %d derived); %d skipped, %d rejections\n",
			report.Stations, report.StationsNoData, report.StationsFailed,
			report.Entries, report.Trips, report.StopTimes, report.DerivedStopTimes,
			report.SkippedStops, len(report.Rejections))
		for _, f := range report.FailedStations {
			fmt.Printf("  station failed %s\n", f)
		}
		for _, rej := range report.Rejections {
			fmt.Printf("  rejected %s\n", rej)
		}
	default:
		slog.Error("unknown -source", "source", *source)
		os.Exit(2)
	}
}
