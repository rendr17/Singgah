package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimit(t *testing.T) {
	calls := 0
	h := RateLimit(3, time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusNoContent)
	}))
	req := func() *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/", nil)
		r.RemoteAddr = "1.2.3.4:9999"
		return r
	}
	for i := 0; i < 3; i++ {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req())
		if rec.Code != http.StatusNoContent {
			t.Fatalf("request %d: got %d want allowed", i+1, rec.Code)
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req())
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("4th request: got %d want 429", rec.Code)
	}
	if calls != 3 {
		t.Fatalf("handler ran %d times, want 3", calls)
	}
	// A different IP has its own bucket.
	r2 := httptest.NewRequest(http.MethodPost, "/", nil)
	r2.RemoteAddr = "5.6.7.8:1111"
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, r2)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("other IP: got %d want allowed", rec.Code)
	}
}
