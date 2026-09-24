// Package http wires the API's HTTP surface: router, middleware order, and
// the infrastructure endpoints. Domain routes are mounted here as they land.
package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"singgah/services/api/internal/catalog"
	"singgah/services/api/internal/health"
	"singgah/services/api/internal/http/middleware"
	"singgah/services/api/internal/http/response"
)

// Deps carries the router's runtime dependencies — constructed once in main.
type Deps struct {
	Logger  *slog.Logger
	Version string
	// DB may be nil when DATABASE_URL is unset; /ready reports that honestly.
	DB health.Pinger
	// Catalog is nil without a database; its routes then report unavailable.
	Catalog *catalog.Handler
}

func NewRouter(deps Deps) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Logging(deps.Logger))
	r.Use(recovery(deps.Logger))

	r.Get("/health", health.Handler)
	r.Get("/ready", health.ReadyHandler(deps.DB))
	r.Get("/version", health.VersionHandler(deps.Version))

	if deps.Catalog != nil {
		r.Mount("/api/v1", deps.Catalog.Routes())
	}

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		response.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Resource not found")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		response.Error(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
	})

	return r
}
