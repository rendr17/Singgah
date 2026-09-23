// Package middleware holds leaf HTTP middleware shared by the router.
package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

// RequestIDHeader is echoed back so clients can correlate errors with logs.
const RequestIDHeader = "X-Request-ID"

type requestIDKey struct{}

// RequestID propagates the inbound X-Request-ID or mints a new one, stores it
// in the request context, and sets it on the response.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(RequestIDHeader)
		if id == "" || len(id) > 128 {
			id = newRequestID()
		}
		w.Header().Set(RequestIDHeader, id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey{}, id)))
	})
}

// RequestIDFrom returns the request ID stored by the RequestID middleware,
// or "" when absent.
func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

func newRequestID() string {
	var b [16]byte
	// rand.Read on a 16-byte buffer cannot fail on supported platforms.
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
