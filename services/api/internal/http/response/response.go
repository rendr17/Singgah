// Package response is a leaf: handlers and the router share the JSON
// envelope without creating an import cycle back into the http package.
package response

import (
	"encoding/json"
	"net/http"

	"singgah/services/api/internal/http/middleware"
)

// ErrorBody matches the error envelope in docs/34_API_SPEC.md.
type ErrorBody struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	RequestID string         `json:"requestId,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
}

type errorEnvelope struct {
	Error ErrorBody `json:"error"`
}

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func Error(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	JSON(w, status, errorEnvelope{Error: ErrorBody{
		Code:      code,
		Message:   message,
		RequestID: middleware.RequestIDFrom(r.Context()),
	}})
}
