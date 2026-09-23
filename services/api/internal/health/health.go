// Package health exposes the infrastructure endpoints.
package health

import (
	"context"
	"net/http"

	"singgah/services/api/internal/http/response"
)

// Pinger is the minimal readiness dependency — *pgxpool.Pool satisfies it.
type Pinger interface {
	Ping(context.Context) error
}

// Handler reports that the process is serving requests.
func Handler(w http.ResponseWriter, _ *http.Request) {
	response.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ReadyHandler reports whether dependencies are usable — today that is the
// database. 503 when the database is unconfigured or unreachable.
func ReadyHandler(db Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if db == nil {
			response.Error(w, r, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Database not configured")
			return
		}
		if err := db.Ping(r.Context()); err != nil {
			response.Error(w, r, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Database unreachable")
			return
		}
		response.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// VersionHandler reports the build version injected via ldflags.
func VersionHandler(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		response.JSON(w, http.StatusOK, map[string]string{"version": version})
	}
}
