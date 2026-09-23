package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"singgah/services/api/internal/health"
	"singgah/services/api/internal/http/middleware"
)

func testRouter() http.Handler {
	return NewRouter(Deps{
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		Version: "test-123",
	})
}

func TestHealth(t *testing.T) {
	rec := httptest.NewRecorder()
	testRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("body = %v, want status ok", body)
	}
}

func TestVersion(t *testing.T) {
	rec := httptest.NewRecorder()
	testRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/version", nil))

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["version"] != "test-123" {
		t.Errorf("version = %q, want test-123", body["version"])
	}
}

func TestNotFoundUsesErrorEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()
	testRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/nope", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	var body struct {
		Error struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"requestId"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Error.Code != "NOT_FOUND" {
		t.Errorf("error.code = %q, want NOT_FOUND", body.Error.Code)
	}
	if body.Error.RequestID == "" {
		t.Error("error.requestId is empty")
	}
	if rec.Header().Get(middleware.RequestIDHeader) != body.Error.RequestID {
		t.Error("envelope requestId does not match X-Request-ID header")
	}
}

type fakePinger struct{ err error }

func (p fakePinger) Ping(context.Context) error { return p.err }

func TestReady(t *testing.T) {
	router := func(db health.Pinger) http.Handler {
		return NewRouter(Deps{
			Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
			Version: "test-123",
			DB:      db,
		})
	}

	t.Run("ok when database pings", func(t *testing.T) {
		rec := httptest.NewRecorder()
		router(fakePinger{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ready", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
	})

	t.Run("503 when database unconfigured", func(t *testing.T) {
		rec := httptest.NewRecorder()
		router(nil).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ready", nil))
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want 503", rec.Code)
		}
	})

	t.Run("503 when database unreachable", func(t *testing.T) {
		rec := httptest.NewRecorder()
		router(fakePinger{err: errors.New("conn refused")}).
			ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ready", nil))
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want 503", rec.Code)
		}
	})
}

func TestRequestIDEchoesInboundHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set(middleware.RequestIDHeader, "client-supplied-id")
	rec := httptest.NewRecorder()
	testRouter().ServeHTTP(rec, req)

	if got := rec.Header().Get(middleware.RequestIDHeader); got != "client-supplied-id" {
		t.Errorf("X-Request-ID = %q, want echoed client-supplied-id", got)
	}
}
