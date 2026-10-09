// Package config loads API runtime configuration from the environment.
package config

import (
	"fmt"
	"net"
	"os"
	"singgah/services/api/internal/provider/commute"
	"strconv"
	"time"
)

type Config struct {
	Env  string
	Addr string

	// DatabaseURL is optional: the API still serves non-DB endpoints when it
	// is unset, and /ready reports the database as not configured.
	DatabaseURL string

	// CommuteBaseURL is the upstream journey/fare source — overridable for
	// staging and fixture servers.
	CommuteBaseURL string

	// CORSOrigin is the single allowed cross-origin for browser clients.
	// Configure the app origin explicitly in deployments; CORS is not authorization.
	CORSOrigin string

	// ScheduleRefreshInterval enables the in-process schedule refresh loop
	// when > 0 (e.g. "24h"). The startup run is skipped when both schedule
	// providers were already refreshed inside the interval. Zero (the
	// default) keeps ingest a manual cmd/ingest-schedule run.
	ScheduleRefreshInterval time.Duration

	// GTFSFeedURL is the schedule refresh's TransJakarta feed — overridable
	// for staging and fixture servers.
	GTFSFeedURL string

	// RealtimeFeedURL enables the GTFS-RT VehiclePositions poll when set.
	// No default: the feed is only wired once its license/terms are
	// documented in docs/35_DATA_SOURCES.md — an empty value is the honest
	// "unavailable" state.
	RealtimeFeedURL      string
	RealtimePollInterval time.Duration
	// RealtimeFeedSource is the provider label stamped on normalized
	// entities; RealtimeRoutePrefix maps the feed's route_ids into the
	// catalog provider_entity_id space ("commute" uses TJ:<short_name>).
	RealtimeFeedSource  string
	RealtimeRoutePrefix string

	// RealtimeAlertsURL enables the GTFS-RT Alerts poll when set — usually a
	// dedicated endpoint; the same licensing gate applies.
	RealtimeAlertsURL      string
	RealtimeAlertsInterval time.Duration

	// RealtimeTripUpdatesURL enables the GTFS-RT TripUpdates poll when set —
	// delays annotate the departures board. Same licensing gate.
	// RealtimeTripPrefix maps the feed's trip_ids into catalog
	// provider_entity_id space; empty means match verbatim.
	RealtimeTripUpdatesURL      string
	RealtimeTripUpdatesInterval time.Duration
	RealtimeTripPrefix          string

	// AuthSessionCleanupInterval drives the dead-session sweeper (expired or
	// revoked rows can never authenticate — deleting them is pure hygiene).
	// Unlike the schedule refresh it only touches our own tables, so it
	// defaults on at 24h; AUTH_SESSION_CLEANUP_INTERVAL=0 disables it.
	AuthSessionCleanupInterval time.Duration

	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		Env:               "development",
		Addr:              ":8080",
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		ShutdownTimeout:   10 * time.Second,
	}

	if v := os.Getenv("APP_ENV"); v != "" {
		cfg.Env = v
	}

	cfg.DatabaseURL = os.Getenv("DATABASE_URL")

	cfg.CommuteBaseURL = os.Getenv("COMMUTE_BASE_URL")
	if cfg.CommuteBaseURL == "" {
		cfg.CommuteBaseURL = commute.DefaultBaseURL
	}

	if v := os.Getenv("SCHEDULE_REFRESH_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d < time.Minute {
			return Config{}, fmt.Errorf("invalid SCHEDULE_REFRESH_INTERVAL %q", v)
		}
		cfg.ScheduleRefreshInterval = d
	}

	cfg.GTFSFeedURL = os.Getenv("GTFS_FEED_URL")
	if cfg.GTFSFeedURL == "" {
		cfg.GTFSFeedURL = "https://gtfs.transjakarta.co.id/files/file_gtfs.zip"
	}

	cfg.RealtimeFeedURL = os.Getenv("REALTIME_FEED_URL")
	cfg.RealtimeFeedSource = os.Getenv("REALTIME_FEED_SOURCE")
	if cfg.RealtimeFeedSource == "" {
		cfg.RealtimeFeedSource = "gtfs-rt"
	}
	cfg.RealtimeRoutePrefix = os.Getenv("REALTIME_ROUTE_PREFIX")
	if cfg.RealtimeRoutePrefix == "" {
		cfg.RealtimeRoutePrefix = "TJ:"
	}
	cfg.RealtimePollInterval = 15 * time.Second
	if v := os.Getenv("REALTIME_POLL_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d < 5*time.Second {
			return Config{}, fmt.Errorf("invalid REALTIME_POLL_INTERVAL %q", v)
		}
		cfg.RealtimePollInterval = d
	}

	cfg.RealtimeAlertsURL = os.Getenv("REALTIME_ALERTS_URL")
	// Alerts change far slower than positions — docs/22 budgets 30–60s.
	cfg.RealtimeAlertsInterval = 45 * time.Second
	if v := os.Getenv("REALTIME_ALERTS_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d < 5*time.Second {
			return Config{}, fmt.Errorf("invalid REALTIME_ALERTS_INTERVAL %q", v)
		}
		cfg.RealtimeAlertsInterval = d
	}

	cfg.RealtimeTripUpdatesURL = os.Getenv("REALTIME_TRIP_UPDATES_URL")
	cfg.RealtimeTripPrefix = os.Getenv("REALTIME_TRIP_PREFIX")
	// Delays sit between positions and alerts — docs/22 budgets 15–60s.
	cfg.RealtimeTripUpdatesInterval = 30 * time.Second
	if v := os.Getenv("REALTIME_TRIP_UPDATES_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d < 5*time.Second {
			return Config{}, fmt.Errorf("invalid REALTIME_TRIP_UPDATES_INTERVAL %q", v)
		}
		cfg.RealtimeTripUpdatesInterval = d
	}

	cfg.AuthSessionCleanupInterval = 24 * time.Hour
	if v := os.Getenv("AUTH_SESSION_CLEANUP_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || (d > 0 && d < time.Minute) {
			return Config{}, fmt.Errorf("invalid AUTH_SESSION_CLEANUP_INTERVAL %q", v)
		}
		cfg.AuthSessionCleanupInterval = d
	}

	cfg.CORSOrigin = os.Getenv("CORS_ORIGIN")
	if cfg.CORSOrigin == "" {
		cfg.CORSOrigin = "*"
	}

	if v := os.Getenv("PORT"); v != "" {
		port, err := strconv.Atoi(v)
		if err != nil || port < 1 || port > 65535 {
			return Config{}, fmt.Errorf("invalid PORT %q", v)
		}
		cfg.Addr = net.JoinHostPort("", v)
	}

	return cfg, nil
}
