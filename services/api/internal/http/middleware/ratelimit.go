package middleware

import (
	"encoding/json"
	"net"
	"net/http"
	"sync"
	"time"
)

// RateLimit is a small per-IP token bucket for mutation endpoints
// (docs/26: rate limit check-in/session minting). One limiter instance
// guards the routes it wraps; keyed by client IP — trusted proxy headers
// are a deployment concern, RemoteAddr is honest for direct connections.
func RateLimit(tokens float64, per time.Duration) func(http.Handler) http.Handler {
	type bucket struct {
		tokens float64
		last   time.Time
	}
	rate := tokens / per.Seconds()
	var mu sync.Mutex
	buckets := map[string]*bucket{}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			now := time.Now()
			mu.Lock()
			b, ok := buckets[ip]
			if !ok {
				// Bound the map: clients churning IPs can't grow it forever.
				if len(buckets) >= 10000 {
					buckets = map[string]*bucket{}
				}
				b = &bucket{tokens: tokens}
				buckets[ip] = b
			}
			b.tokens += rate * now.Sub(b.last).Seconds()
			if b.tokens > tokens {
				b.tokens = tokens
			}
			b.last = now
			allowed := b.tokens >= 1
			if allowed {
				b.tokens--
			}
			mu.Unlock()
			if !allowed {
				// Inline body — the response package imports this one
				// (RequestIDFrom), so middleware can't call it back.
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusTooManyRequests)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{
					"code":    "RATE_LIMITED",
					"message": "Terlalu banyak permintaan, coba lagi nanti",
				}})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
