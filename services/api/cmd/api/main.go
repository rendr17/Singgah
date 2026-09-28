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
	"singgah/services/api/internal/config"
	"singgah/services/api/internal/db"
	httpapi "singgah/services/api/internal/http"
	"singgah/services/api/internal/ingest"
	"singgah/services/api/internal/journey"
	"singgah/services/api/internal/passport"
	"singgah/services/api/internal/planner"
	"singgah/services/api/internal/provider/commute"
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
		queries := generated.New(pool)
		commuteClient := commute.NewClient(cfg.CommuteBaseURL)
		// The planner snapshot reloads lazily on a TTL — schedule ingests are
		// rare, so five minutes keeps responses fresh without churning. The
		// journey planner and the station departure board share it.
		engSrc := planner.NewEngineSource(func(ctx context.Context) (*planner.Engine, error) {
			return planner.Load(ctx, queries)
		}, 5*time.Minute)
		deps.Catalog = catalog.NewHandler(queries, engSrc)
		deps.Journey = journey.NewHandler(queries, engSrc, commuteClient)
		authSvc := auth.NewService(queries)
		deps.Auth = auth.NewHandler(authSvc)
		deps.Passport = passport.NewHandler(queries, authSvc)
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
