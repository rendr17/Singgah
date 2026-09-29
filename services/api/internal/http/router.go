// Package http wires the API's HTTP surface: router, middleware order, and
// the infrastructure endpoints. Domain routes are mounted here as they land.
package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"singgah/services/api/internal/auth"
	"singgah/services/api/internal/catalog"
	"singgah/services/api/internal/collections"
	"singgah/services/api/internal/health"
	"singgah/services/api/internal/http/middleware"
	"singgah/services/api/internal/http/response"
	"singgah/services/api/internal/journey"
	"singgah/services/api/internal/passport"
	"singgah/services/api/internal/places"
)

// Deps carries the router's runtime dependencies — constructed once in main.
type Deps struct {
	Logger  *slog.Logger
	Version string
	// DB may be nil when DATABASE_URL is unset; /ready reports that honestly.
	DB health.Pinger
	// Catalog and Journey are nil without a database; their routes are then
	// simply unmounted (404) — /ready still reports the dependency honestly.
	Catalog     *catalog.Handler
	Journey     *journey.Handler
	Auth        *auth.Handler
	Passport    *passport.Handler
	Collections *collections.Handler
	Places      *places.Handler
	// CORSOrigin is the Access-Control-Allow-Origin value ("*" for local dev).
	CORSOrigin string
}

func NewRouter(deps Deps) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.CORS(deps.CORSOrigin))
	r.Use(middleware.RequestID)
	r.Use(middleware.Logging(deps.Logger))
	r.Use(recovery(deps.Logger))

	r.Get("/health", health.Handler)
	r.Get("/ready", health.ReadyHandler(deps.DB))
	r.Get("/version", health.VersionHandler(deps.Version))

	r.Route("/api/v1", func(v1 chi.Router) {
		if deps.Catalog != nil {
			deps.Catalog.RegisterRoutes(v1)
		}
		if deps.Journey != nil {
			deps.Journey.RegisterRoutes(v1)
		}
		if deps.Auth != nil {
			deps.Auth.RegisterRoutes(v1)
		}
		if deps.Passport != nil {
			deps.Passport.RegisterRoutes(v1)
		}
		if deps.Collections != nil {
			deps.Collections.RegisterRoutes(v1)
		}
		if deps.Places != nil {
			deps.Places.RegisterRoutes(v1)
		}
	})

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		response.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Resource not found")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		response.Error(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
	})

	return r
}
