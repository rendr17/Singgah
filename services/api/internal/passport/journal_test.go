package passport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"singgah/services/api/db/generated"
	"singgah/services/api/internal/auth"
)

// withChiID injects a chi URL param so handler tests can call methods that
// read chi.URLParam without standing up a router.
func withChiID(r *http.Request, key, val string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, val)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func authed(method, path, body string) *http.Request {
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, bytes.NewBufferString(body))
	}
	return r.WithContext(auth.ContextWithUserID(r.Context(), testUser))
}

func TestJournalCreateAndList(t *testing.T) {
	h := newHandler(&fakeStore{stopExists: true})
	rec := httptest.NewRecorder()
	h.createEntry(rec, authed(http.MethodPost, "/api/v1/journal",
		fmt.Sprintf(`{"stopId":%q,"body":" catatan exit utara "}`, uuidStr(testStop))))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		Entry journalDTO `json:"entry"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&created)
	if created.Entry.Body != "catatan exit utara" || created.Entry.StopID == nil {
		t.Fatalf("entry: %+v", created.Entry)
	}
	if created.Entry.Visibility != "private" {
		t.Fatalf("visibility must be private, got %s", created.Entry.Visibility)
	}

	rec = httptest.NewRecorder()
	h.listEntries(rec, authed(http.MethodGet, "/api/v1/journal", ""))
	var list struct {
		Entries []journalDTO `json:"entries"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&list)
	if len(list.Entries) != 1 {
		t.Fatalf("list: got %d want 1", len(list.Entries))
	}
}

func TestJournalCreateValidation(t *testing.T) {
	h := newHandler(&fakeStore{stopExists: false})
	long := strings.Repeat("x", journalBodyMaxLen+1)
	for name, body := range map[string]string{
		"empty body":    `{"body":""}`,
		"space body":    `{"body":"   "}`,
		"too long":      fmt.Sprintf(`{"body":%q}`, long),
		"bad stop":      `{"stopId":"x","body":"ok"}`,
		"unknown stop":  fmt.Sprintf(`{"stopId":%q,"body":"ok"}`, uuidStr(testStop)),
		"bad visit":     `{"visitEventId":"x","body":"ok"}`,
		"unknown visit": fmt.Sprintf(`{"visitEventId":%q,"body":"ok"}`, uuidStr(testStop)),
	} {
		rec := httptest.NewRecorder()
		h.createEntry(rec, authed(http.MethodPost, "/api/v1/journal", body))
		want := http.StatusBadRequest
		if name == "unknown stop" || name == "unknown visit" {
			want = http.StatusNotFound
		}
		if rec.Code != want {
			t.Fatalf("%s: got %d want %d (%s)", name, rec.Code, want, rec.Body.String())
		}
	}
}

func TestJournalVisitLinkOwnership(t *testing.T) {
	// A visit owned by someone else must not be linkable.
	otherUser := pgtype.UUID{Bytes: [16]byte{0x08}, Valid: true}
	foreignVisit := pgtype.UUID{Bytes: [16]byte{0xB0}, Valid: true}
	store := &fakeStore{stopExists: true, rows: []generated.VisitEvent{
		{ID: foreignVisit, UserID: otherUser, StopID: testStop},
	}}
	h := newHandler(store)
	rec := httptest.NewRecorder()
	h.createEntry(rec, authed(http.MethodPost, "/api/v1/journal",
		fmt.Sprintf(`{"visitEventId":%q,"body":"ok"}`, uuidStr(foreignVisit))))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("foreign visit: got %d want 404", rec.Code)
	}
}

func TestJournalUpdateAndDelete(t *testing.T) {
	h := newHandler(&fakeStore{})
	rec := httptest.NewRecorder()
	h.createEntry(rec, authed(http.MethodPost, "/api/v1/journal", `{"body":"v1"}`))
	var created struct {
		Entry journalDTO `json:"entry"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&created)
	eid := created.Entry.ID

	// PATCH without base → last write wins.
	rec = httptest.NewRecorder()
	req := authed(http.MethodPatch, "/api/v1/journal/"+eid, `{"body":"v2"}`)
	req = withChiID(req, "entryID", eid)
	h.updateEntry(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch: %d %s", rec.Code, rec.Body.String())
	}

	// PATCH with stale base → 409.
	rec = httptest.NewRecorder()
	stale := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano)
	req = authed(http.MethodPatch, "/api/v1/journal/"+eid,
		fmt.Sprintf(`{"body":"v3","baseUpdatedAt":%q}`, stale))
	req = withChiID(req, "entryID", eid)
	h.updateEntry(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("stale patch: got %d want 409", rec.Code)
	}

	// DELETE own entry → 204; repeat → 404.
	rec = httptest.NewRecorder()
	req = authed(http.MethodDelete, "/api/v1/journal/"+eid, "")
	req = withChiID(req, "entryID", eid)
	h.deleteEntry(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: got %d want 204", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.deleteEntry(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("delete twice: got %d want 404", rec.Code)
	}
}
