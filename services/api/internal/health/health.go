// Package health exposes the infrastructure endpoints. Readiness checks
// (database ping) join Handler when the database lands.
package health

import (
	"net/http"

	"singgah/services/api/internal/http/response"
)

// Handler reports that the process is serving requests.
func Handler(w http.ResponseWriter, _ *http.Request) {
	response.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// VersionHandler reports the build version injected via ldflags.
func VersionHandler(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		response.JSON(w, http.StatusOK, map[string]string{"version": version})
	}
}
