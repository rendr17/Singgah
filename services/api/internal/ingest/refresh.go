// Scheduled refresh: the api process can re-run both schedule ingests on an
// interval instead of relying on a manual CLI run. A failure logs and waits
// for the next tick — the planner keeps serving the previous snapshot.
package ingest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	generated "singgah/services/api/db/generated"
	"singgah/services/api/internal/provider/commute"
	"singgah/services/api/internal/provider/gtfs"
)

// refreshProviders are the schedule sources whose freshness gates the
// startup run — last_success_at on either proves recent ingest.
var refreshProviders = []string{"tj-gtfs", "commute"}

// RefreshConfig carries what RefreshAll needs; Interval stays in the caller
// (config) — RefreshAll itself is one idempotent pass.
type RefreshConfig struct {
	FeedURL    string        // TJ GTFS zip — URL or local path
	CommuteURL string        // Commute API base
	Pace       time.Duration // delay between station timetable calls
}

// RefreshAll runs the GTFS schedule ingest and the commute timetable sweep.
// The two failure domains stay independent: both run, errors join.
func RefreshAll(ctx context.Context, pool *pgxpool.Pool, cfg RefreshConfig, log *slog.Logger) error {
	var errs []error
	if zr, err := gtfs.OpenFeed(cfg.FeedURL); err != nil {
		errs = append(errs, fmt.Errorf("gtfs feed: %w", err))
	} else if rep, err := Schedule(ctx, pool, zr, gtfs.TJProviderRegistration, "commute", "TJ:", "TJ-"); err != nil {
		errs = append(errs, fmt.Errorf("gtfs ingest: %w", err))
	} else {
		log.Info("schedule refresh: gtfs", "trips", rep.Trips,
			"unmatchedRoutes", len(rep.UnmatchedRoutes), "rejectedTrips", len(rep.RejectedTrips))
	}
	if rep, err := CommuteSchedule(ctx, pool, commute.NewClient(cfg.CommuteURL), cfg.Pace); err != nil {
		errs = append(errs, fmt.Errorf("timetable sweep: %w", err))
	} else {
		log.Info("schedule refresh: commute", "stations", rep.Stations,
			"noData", rep.StationsNoData, "failed", rep.StationsFailed,
			"trips", rep.Trips, "rejections", len(rep.Rejections))
	}
	return errors.Join(errs...)
}

// StartRefresher runs RefreshAll on an interval until ctx is done. The
// startup run is skipped when both sources were refreshed within the
// interval, so restarts don't hammer upstream. Returns a channel that
// closes when the loop exits — the owner waits on it during shutdown.
func StartRefresher(ctx context.Context, pool *pgxpool.Pool, cfg RefreshConfig, interval time.Duration, log *slog.Logger) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		run := func() {
			rctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
			defer cancel()
			if err := RefreshAll(rctx, pool, cfg, log); err != nil && !errors.Is(err, context.Canceled) {
				log.Warn("schedule refresh failed — existing data keeps serving", "error", err)
			}
		}
		fresh, err := scheduleFresh(ctx, pool, interval)
		if err != nil {
			log.Warn("schedule freshness check failed — refreshing anyway", "error", err)
		}
		if !fresh {
			run()
		}
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				run()
			}
		}
	}()
	return done
}

// scheduleFresh reports whether every schedule provider succeeded inside the
// last interval. A missing provider or timestamp counts as stale.
func scheduleFresh(ctx context.Context, pool *pgxpool.Pool, interval time.Duration) (bool, error) {
	rows, err := generated.New(pool).ListProviders(ctx)
	if err != nil {
		return false, err
	}
	cutoff := time.Now().Add(-interval)
	ok := map[string]bool{}
	for _, p := range rows {
		if p.LastSuccessAt.Valid && p.LastSuccessAt.Time.After(cutoff) {
			ok[p.Code] = true
		}
	}
	for _, code := range refreshProviders {
		if !ok[code] {
			return false, nil
		}
	}
	return true, nil
}
