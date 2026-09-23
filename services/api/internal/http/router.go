// Package http wires the API's HTTP surface: router, middleware order, and
// the infrastructure endpoints. Domain routes are mounted here as they land.
package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"singgah/services/api/internal/health"
	"singgah/services/api/internal/http/middleware"
	"singgah/services/api/internal/http/response"
)

func NewRouter(logger *slog.Logger, version string) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Logging(logger))
	r.Use(recovery(logger))

	r.Get("/health", health.Handler)
	r.Get("/version", health.VersionHandler(version))

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		response.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Resource not found")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		response.Error(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
	})

	return r
}
