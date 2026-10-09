// Command api is the Singgah HTTP API server.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	generated "singgah/services/api/db/generated"
	"singgah/services/api/internal/auth"
	"singgah/services/api/internal/catalog"
	"singgah/services/api/internal/collections"
	"singgah/services/api/internal/config"
	"singgah/services/api/internal/db"
	httpapi "singgah/services/api/internal/http"
	"singgah/services/api/internal/ingest"
	"singgah/services/api/internal/journey"
	"singgah/services/api/internal/passport"
	"singgah/services/api/internal/places"
	"singgah/services/api/internal/planner"
	"singgah/services/api/internal/provider/commute"
	"singgah/services/api/internal/provider/gtfsrt"
	"singgah/services/api/internal/realtime"
	"singgah/services/api/internal/trails"
)

// version is injected at build time via -ldflags "-X main.version=<ver>".
var version = "dev"

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	logger := newLogger(cfg.Env)
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	deps := httpapi.Deps{Logger: logger, Version: version, CORSOrigin: cfg.CORSOrigin}

	var refreshDone <-chan struct{}
	var cleanupDone <-chan struct{}
	var realtimeDone <-chan struct{}
	var alertsDone <-chan struct{}
	var updatesDone <-chan struct{}
	var queries *generated.Queries

	// Realtime mounts unconditionally: with no poller the handler reports
	// feedStatus=unavailable rather than 404ing or claiming live.
	rtCache := realtime.NewCache()
	rtAlerts := realtime.NewAlertStore()
	rtUpdates := realtime.NewTripUpdateStore()
	var rtPoller, rtAlertPoller, rtUpdatesPoller *realtime.Poller
	var rtEstimator realtime.Estimator

	if cfg.DatabaseURL != "" {
		connectCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		pool, err := db.Connect(connectCtx, cfg.DatabaseURL)
		cancel()
		if err != nil {
			logger.Error("database connect", "error", err)
			os.Exit(1)
		}
		defer pool.Close()
		deps.DB = pool
		queries = generated.New(pool)
		commuteClient := commute.NewClient(cfg.CommuteBaseURL)
		// The planner snapshot reloads lazily on a TTL — schedule ingests are
		// rare, so five minutes keeps responses fresh without churning. The
		// journey planner and the station departure board share it.
		engSrc := planner.NewEngineSource(func(ctx context.Context) (*planner.Engine, error) {
			return planner.Load(ctx, queries)
		}, 5*time.Minute)
		// Rail modes have no licensed position feed — their honest answer is
		// a timetable-derived estimate, labeled "estimated", never "live".
		rtEstimator = &realtime.ScheduledEstimator{
			Engine: engSrc,
			Modes:  map[string]bool{"rail": true, "subway": true, "tram": true},
		}
		deps.Catalog = catalog.NewHandler(queries, engSrc)
		deps.Journey = journey.NewHandler(queries, engSrc, commuteClient)
		authSvc := auth.NewService(queries)
		deps.Auth = auth.NewHandler(authSvc)
		deps.Passport = passport.NewHandler(queries, authSvc)
		deps.Collections = collections.NewHandler(queries)
		deps.Places = places.NewHandler(queries, authSvc.RequireUser)
		deps.Trails = trails.NewHandler(queries)
		if cfg.ScheduleRefreshInterval > 0 {
			refreshDone = ingest.StartRefresher(ctx, pool, ingest.RefreshConfig{
				FeedURL:    cfg.GTFSFeedURL,
				CommuteURL: cfg.CommuteBaseURL,
				Pace:       150 * time.Millisecond,
			}, cfg.ScheduleRefreshInterval, logger)
			logger.Info("schedule refresh enabled", "interval", cfg.ScheduleRefreshInterval)
		}
		if cfg.AuthSessionCleanupInterval > 0 {
			cleanupDone = auth.StartSessionCleanup(ctx, queries, cfg.AuthSessionCleanupInterval, logger)
			logger.Info("auth session cleanup enabled", "interval", cfg.AuthSessionCleanupInterval)
		}
		logger.Info("database connected")
	} else {
		logger.Warn("DATABASE_URL unset — database endpoints report unavailable")
	}

	// A licensed GTFS-RT feed turns the cache into a live pipeline. Route
	// resolution needs the DB; without it RouteID stays empty, which the
	// response shape already supports.
	var resolve gtfsrt.RouteResolver
	if queries != nil && (cfg.RealtimeFeedURL != "" || cfg.RealtimeAlertsURL != "" || cfg.RealtimeTripUpdatesURL != "") {
		resolved := map[string]string{}
		resolve = func(ctx context.Context, rid string) (string, bool) {
			if id, ok := resolved[rid]; ok {
				return id, true
			}
			u, err := queries.GetRouteByProviderEntityID(ctx, generated.GetRouteByProviderEntityIDParams{
				Code: "commute", ProviderEntityID: cfg.RealtimeRoutePrefix + rid,
			})
			if err != nil || !u.Valid {
				return "", false
			}
			resolved[rid] = u.String()
			return resolved[rid], true
		}
	}
	// TripUpdates join on canonical trip ids — the resolver maps the feed's
	// trip_id (with the configured prefix) through trips.provider_entity_id.
	var resolveTrip gtfsrt.TripResolver
	if queries != nil && cfg.RealtimeTripUpdatesURL != "" {
		resolved := map[string]string{}
		resolveTrip = func(ctx context.Context, tid string) (string, bool) {
			if id, ok := resolved[tid]; ok {
				return id, true
			}
			u, err := queries.GetTripByProviderEntityID(ctx, generated.GetTripByProviderEntityIDParams{
				Code: "commute", ProviderEntityID: cfg.RealtimeTripPrefix + tid,
			})
			if err != nil || !u.Valid {
				return "", false
			}
			resolved[tid] = u.String()
			return resolved[tid], true
		}
	}
	if cfg.RealtimeFeedURL != "" {
		rtPoller = realtime.NewPoller(
			gtfsrt.NewClient(cfg.RealtimeFeedURL, cfg.RealtimeFeedSource, resolve),
			cfg.RealtimeFeedSource, rtCache, cfg.RealtimePollInterval, logger)
		realtimeDone = rtPoller.Run(ctx)
		logger.Info("realtime poll enabled", "source", cfg.RealtimeFeedSource, "interval", cfg.RealtimePollInterval)
	}
	if cfg.RealtimeAlertsURL != "" {
		rtAlertPoller = realtime.NewAlertPoller(
			gtfsrt.NewClient(cfg.RealtimeAlertsURL, cfg.RealtimeFeedSource, resolve),
			cfg.RealtimeFeedSource, rtAlerts, cfg.RealtimeAlertsInterval, logger)
		alertsDone = rtAlertPoller.Run(ctx)
		logger.Info("alerts poll enabled", "source", cfg.RealtimeFeedSource, "interval", cfg.RealtimeAlertsInterval)
	}
	if cfg.RealtimeTripUpdatesURL != "" {
		rtUpdatesPoller = realtime.NewTripUpdatePoller(
			gtfsrt.NewClient(cfg.RealtimeTripUpdatesURL, cfg.RealtimeFeedSource, resolve).
				WithTripResolver(resolveTrip),
			cfg.RealtimeFeedSource, rtUpdates, cfg.RealtimeTripUpdatesInterval, logger)
		updatesDone = rtUpdatesPoller.Run(ctx)
		logger.Info("trip-updates poll enabled", "source", cfg.RealtimeFeedSource, "interval", cfg.RealtimeTripUpdatesInterval)
	}
	if deps.Catalog != nil && rtUpdatesPoller != nil {
		deps.Catalog.WithTripUpdates(rtUpdates, rtUpdatesPoller)
	}
	deps.Realtime = realtime.NewHandler(rtCache, rtPoller, rtEstimator, rtAlerts, rtAlertPoller).
		WithDone(ctx.Done())

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           httpapi.NewRouter(deps),
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("listening", "addr", srv.Addr, "env", cfg.Env, "version", version)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("graceful shutdown failed", "error", err)
			os.Exit(1)
		}
		if refreshDone != nil {
			// The refresher's ctx is already done — the loop exits promptly;
			// an in-flight ingest aborts through ctx itself.
			select {
			case <-refreshDone:
			case <-time.After(5 * time.Second):
			}
		}
		if cleanupDone != nil {
			select {
			case <-cleanupDone:
			case <-time.After(5 * time.Second):
			}
		}
		if realtimeDone != nil {
			select {
			case <-realtimeDone:
			case <-time.After(5 * time.Second):
			}
		}
		if alertsDone != nil {
			select {
			case <-alertsDone:
			case <-time.After(5 * time.Second):
			}
		}
		if updatesDone != nil {
			select {
			case <-updatesDone:
			case <-time.After(5 * time.Second):
			}
		}
	}
}

// Text output stays readable in development; production emits JSON for
// whatever log pipeline collects stdout.
func newLogger(env string) *slog.Logger {
	if env == "development" {
		return slog.New(slog.NewTextHandler(os.Stdout, nil))
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, nil))
}
