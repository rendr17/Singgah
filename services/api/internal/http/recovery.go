package http

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"singgah/services/api/internal/http/middleware"
	"singgah/services/api/internal/http/response"
)

// recovery converts a handler panic into the standard 500 error envelope and
// logs it with the request ID. It lives beside the router rather than in
// middleware/ because it needs the response writer, which would create a
// middleware -> http import cycle.
func recovery(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("panic recovered",
						slog.Any("panic", rec),
						slog.String("request_id", middleware.RequestIDFrom(r.Context())),
						slog.String("path", r.URL.Path),
						slog.String("stack", string(debug.Stack())),
					)
					response.Error(w, r, http.StatusInternalServerError, "INTERNAL", "Internal server error")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
