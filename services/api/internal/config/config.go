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

	if v := os.Getenv("PORT"); v != "" {
		port, err := strconv.Atoi(v)
		if err != nil || port < 1 || port > 65535 {
			return Config{}, fmt.Errorf("invalid PORT %q", v)
		}
		cfg.Addr = net.JoinHostPort("", v)
	}

	return cfg, nil
}
