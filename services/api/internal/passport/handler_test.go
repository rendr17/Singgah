package passport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"singgah/services/api/db/generated"
	"singgah/services/api/internal/auth"
)

var testUser = pgtype.UUID{Bytes: [16]byte{9}, Valid: true}
var testStop = pgtype.UUID{Bytes: [16]byte{7}, Valid: true}

type fakeStore struct {
	stopExists bool
	distanceM  float64
	distErr    error
	rows       []generated.VisitEvent
	inserted   int
	totalStops int64
	byMode     []generated.PassportProgressByModeRow
	byRoute    []generated.PassportProgressByRouteRow
	entries    []generated.JournalEntry
	jseq       int
}

func (f *fakeStore) GetStop(ctx context.Context, id pgtype.UUID) (generated.GetStopRow, error) {
	if !f.stopExists {
		return generated.GetStopRow{}, pgx.ErrNoRows
	}
	return generated.GetStopRow{ID: id}, nil
}

func (f *fakeStore) StopDistanceM(ctx context.Context, arg generated.StopDistanceMParams) (float64, error) {
	return f.distanceM, f.distErr
}

func (f *fakeStore) RecordVisitEvent(ctx context.Context, arg generated.RecordVisitEventParams) (generated.RecordVisitEventRow, error) {
	for _, v := range f.rows {
		if v.ClientMutationID == arg.ClientMutationID {
			return row(v, false), nil
		}
	}
	f.inserted++
	v := generated.VisitEvent{
		ID:               pgtype.UUID{Bytes: [16]byte{byte(f.inserted)}, Valid: true},
		UserID:           arg.UserID,
		StopID:           arg.StopID,
		ClientMutationID: arg.ClientMutationID,
		ObservedAt:       arg.ObservedAt,
		ValidationMethod: arg.ValidationMethod,
		DistanceM:        arg.DistanceM,
		Status:           arg.Status,
		CreatedAt:        pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	f.rows = append(f.rows, v)
	return row(v, true), nil
}

func row(v generated.VisitEvent, inserted bool) generated.RecordVisitEventRow {
	return generated.RecordVisitEventRow{
		ID: v.ID, UserID: v.UserID, StopID: v.StopID,
		ClientMutationID: v.ClientMutationID, ObservedAt: v.ObservedAt,
		ValidationMethod: v.ValidationMethod, DistanceM: v.DistanceM,
		Status: v.Status, CreatedAt: v.CreatedAt, WasInserted: inserted,
	}
}

func (f *fakeStore) ListVisitEvents(ctx context.Context, userID pgtype.UUID) ([]generated.VisitEvent, error) {
	var out []generated.VisitEvent
	for _, v := range f.rows {
		if v.UserID == userID {
			out = append(out, v)
		}
	}
	return out, nil
}

func (f *fakeStore) PassportProgressTotal(ctx context.Context, userID pgtype.UUID) (generated.PassportProgressTotalRow, error) {
	seen := map[pgtype.UUID]bool{}
	for _, v := range f.rows {
		if v.UserID == userID {
			seen[v.StopID] = true
		}
	}
	return generated.PassportProgressTotalRow{TotalStops: f.totalStops, VisitedStops: int64(len(seen))}, nil
}

func (f *fakeStore) PassportProgressByMode(ctx context.Context, userID pgtype.UUID) ([]generated.PassportProgressByModeRow, error) {
	return f.byMode, nil
}

func (f *fakeStore) PassportProgressByRoute(ctx context.Context, userID pgtype.UUID) ([]generated.PassportProgressByRouteRow, error) {
	return f.byRoute, nil
}

// --- journal ---

func (f *fakeStore) CreateJournalEntry(ctx context.Context, arg generated.CreateJournalEntryParams) (generated.JournalEntry, error) {
	f.jseq++
	e := generated.JournalEntry{
		ID:           pgtype.UUID{Bytes: [16]byte{0xA0, byte(f.jseq)}, Valid: true},
		UserID:       arg.UserID,
		StopID:       arg.StopID,
		VisitEventID: arg.VisitEventID,
		Body:         arg.Body,
		Visibility:   "private",
		CreatedAt:    pgtype.Timestamptz{Time: time.Now(), Valid: true},
		UpdatedAt:    pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	f.entries = append(f.entries, e)
	return e, nil
}

func (f *fakeStore) GetJournalEntry(ctx context.Context, arg generated.GetJournalEntryParams) (generated.JournalEntry, error) {
	for _, e := range f.entries {
		if e.ID == arg.ID && e.UserID == arg.UserID {
			return e, nil
		}
	}
	return generated.JournalEntry{}, pgx.ErrNoRows
}

func (f *fakeStore) ListJournalEntries(ctx context.Context, arg generated.ListJournalEntriesParams) ([]generated.JournalEntry, error) {
	var out []generated.JournalEntry
	for i := len(f.entries) - 1; i >= 0; i-- {
		if f.entries[i].UserID == arg.UserID {
			out = append(out, f.entries[i])
		}
	}
	if int32(len(out)) > arg.Limit {
		out = out[:arg.Limit]
	}
	return out, nil
}

func (f *fakeStore) UpdateJournalEntry(ctx context.Context, arg generated.UpdateJournalEntryParams) (generated.JournalEntry, error) {
	for i, e := range f.entries {
		if e.ID == arg.ID && e.UserID == arg.UserID {
			e.Body = arg.Body
			e.UpdatedAt = pgtype.Timestamptz{Time: time.Now().Add(time.Second), Valid: true}
			f.entries[i] = e
			return e, nil
		}
	}
	return generated.JournalEntry{}, pgx.ErrNoRows
}

func (f *fakeStore) DeleteJournalEntry(ctx context.Context, arg generated.DeleteJournalEntryParams) (int64, error) {
	for i, e := range f.entries {
		if e.ID == arg.ID && e.UserID == arg.UserID {
			f.entries = append(f.entries[:i], f.entries[i+1:]...)
			return 1, nil
		}
	}
	return 0, nil
}

func (f *fakeStore) GetVisitEventOwner(ctx context.Context, id pgtype.UUID) (pgtype.UUID, error) {
	for _, v := range f.rows {
		if v.ID == id {
			return v.UserID, nil
		}
	}
	return pgtype.UUID{}, pgx.ErrNoRows
}

func newHandler(f *fakeStore) *Handler {
	return NewHandler(f, auth.NewService(nil))
}

func post(t *testing.T, h *Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/visits", bytes.NewBufferString(body))
	req = req.WithContext(auth.ContextWithUserID(req.Context(), testUser))
	rec := httptest.NewRecorder()
	h.createVisit(rec, req)
	return rec
}

func mutationID() string {
	return fmt.Sprintf("11111111-2222-3333-4444-%012d", time.Now().UnixNano()%1e12)
}

func TestCheckinGeofenceConfirmed(t *testing.T) {
	h := newHandler(&fakeStore{stopExists: true, distanceM: 80})
	rec := post(t, h, fmt.Sprintf(`{
		"stopId": "%s", "clientMutationId": "%s",
		"observedAt": "%s", "lat": -6.2, "lon": 106.8
	}`, uuidStr(testStop), mutationID(), time.Now().UTC().Format(time.RFC3339)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("got %d want 201: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Visit    visitDTO `json:"visit"`
		Replayed bool     `json:"replayed"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Visit.Status != "confirmed" || resp.Visit.ValidationMethod != "geofence" {
		t.Fatalf("want confirmed/geofence, got %s/%s", resp.Visit.Status, resp.Visit.ValidationMethod)
	}
	if resp.Visit.DistanceM == nil || *resp.Visit.DistanceM != 80 {
		t.Fatalf("distanceM want 80, got %v", resp.Visit.DistanceM)
	}
	if resp.Replayed {
		t.Fatal("fresh insert must not report replayed")
	}
}

func TestCheckinFarAndManualAreLowConfidence(t *testing.T) {
	for name, tc := range map[string]struct {
		dist float64
		body func(stop, mut, at string) string
	}{
		"outside geofence": {300, func(stop, mut, at string) string {
			return fmt.Sprintf(`{"stopId":%q,"clientMutationId":%q,"observedAt":%q,"lat":-6.2,"lon":106.8}`, stop, mut, at)
		}},
		"no fix → manual": {0, func(stop, mut, at string) string {
			return fmt.Sprintf(`{"stopId":%q,"clientMutationId":%q,"observedAt":%q}`, stop, mut, at)
		}},
	} {
		t.Run(name, func(t *testing.T) {
			h := newHandler(&fakeStore{stopExists: true, distanceM: tc.dist})
			rec := post(t, h, tc.body(uuidStr(testStop), mutationID(), time.Now().UTC().Format(time.RFC3339)))
			if rec.Code != http.StatusCreated {
				t.Fatalf("got %d: %s", rec.Code, rec.Body.String())
			}
			var resp struct {
				Visit visitDTO `json:"visit"`
			}
			_ = json.NewDecoder(rec.Body).Decode(&resp)
			if resp.Visit.Status != "low_confidence" {
				t.Fatalf("want low_confidence, got %s", resp.Visit.Status)
			}
		})
	}
}

func TestCheckinReplayIsIdempotent(t *testing.T) {
	store := &fakeStore{stopExists: true, distanceM: 50}
	h := newHandler(store)
	mut := mutationID()
	body := fmt.Sprintf(`{"stopId":%q,"clientMutationId":%q,"observedAt":%q,"lat":-6.2,"lon":106.8}`,
		uuidStr(testStop), mut, time.Now().UTC().Format(time.RFC3339))
	if rec := post(t, h, body); rec.Code != http.StatusCreated {
		t.Fatalf("first: %d", rec.Code)
	}
	rec := post(t, h, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("replay: got %d want 200", rec.Code)
	}
	var resp struct {
		Replayed bool `json:"replayed"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if !resp.Replayed {
		t.Fatal("replay must report replayed=true")
	}
	if store.inserted != 1 {
		t.Fatalf("replay must not insert again, inserted=%d", store.inserted)
	}
}

func TestCheckinValidation(t *testing.T) {
	h := newHandler(&fakeStore{stopExists: true})
	stop, at := uuidStr(testStop), time.Now().UTC().Format(time.RFC3339)
	for name, body := range map[string]string{
		"bad json":           `{`,
		"bad stop":           fmt.Sprintf(`{"stopId":"nope","clientMutationId":%q,"observedAt":%q}`, mutationID(), at),
		"missing mutation":   fmt.Sprintf(`{"stopId":%q,"observedAt":%q}`, stop, at),
		"missing observedAt": fmt.Sprintf(`{"stopId":%q,"clientMutationId":%q}`, stop, mutationID()),
		"future observedAt":  fmt.Sprintf(`{"stopId":%q,"clientMutationId":%q,"observedAt":%q}`, stop, mutationID(), time.Now().Add(2*time.Hour).UTC().Format(time.RFC3339)),
		"lat only":           fmt.Sprintf(`{"stopId":%q,"clientMutationId":%q,"observedAt":%q,"lat":-6.2}`, stop, mutationID(), at),
		"lat range":          fmt.Sprintf(`{"stopId":%q,"clientMutationId":%q,"observedAt":%q,"lat":-91,"lon":0}`, stop, mutationID(), at),
	} {
		if rec := post(t, h, body); rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: got %d want 400 (%s)", name, rec.Code, rec.Body.String())
		}
	}
}

func TestCheckinReplayDifferentPayloadConflicts(t *testing.T) {
	store := &fakeStore{stopExists: true, distanceM: 50}
	h := newHandler(store)
	mut := mutationID()
	at := time.Now().UTC().Format(time.RFC3339)
	stop := uuidStr(testStop)
	post(t, h, fmt.Sprintf(`{"stopId":%q,"clientMutationId":%q,"observedAt":%q}`, stop, mut, at))
	// Same key, different stop → conflict, not silent replay.
	otherStop := uuidStr(pgtype.UUID{Bytes: [16]byte{6}, Valid: true})
	rec := post(t, h, fmt.Sprintf(`{"stopId":%q,"clientMutationId":%q,"observedAt":%q}`, otherStop, mut, at))
	if rec.Code != http.StatusConflict {
		t.Fatalf("stop mismatch: got %d want 409", rec.Code)
	}
	// Same key, different observedAt → conflict too.
	rec = post(t, h, fmt.Sprintf(`{"stopId":%q,"clientMutationId":%q,"observedAt":%q}`,
		stop, mut, time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)))
	if rec.Code != http.StatusConflict {
		t.Fatalf("observedAt mismatch: got %d want 409", rec.Code)
	}
}

func TestCheckinUnknownStop(t *testing.T) {
	h := newHandler(&fakeStore{stopExists: false})
	rec := post(t, h, fmt.Sprintf(`{"stopId":%q,"clientMutationId":%q,"observedAt":%q}`,
		uuidStr(testStop), mutationID(), time.Now().UTC().Format(time.RFC3339)))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("got %d want 404", rec.Code)
	}
}

func TestListVisitsOnlyOwn(t *testing.T) {
	store := &fakeStore{stopExists: true, distanceM: 10}
	h := newHandler(store)
	post(t, h, fmt.Sprintf(`{"stopId":%q,"clientMutationId":%q,"observedAt":%q}`,
		uuidStr(testStop), mutationID(), time.Now().UTC().Format(time.RFC3339)))
	// Someone else's visit must not leak.
	other := pgtype.UUID{Bytes: [16]byte{8}, Valid: true}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/visits", nil)
	req = req.WithContext(auth.ContextWithUserID(req.Context(), other))
	rec := httptest.NewRecorder()
	h.listVisits(rec, req)
	var resp struct {
		Visits []visitDTO `json:"visits"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if len(resp.Visits) != 0 {
		t.Fatalf("ownership leak: other user saw %d visits", len(resp.Visits))
	}
	// Own list sees it.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/visits", nil)
	req = req.WithContext(auth.ContextWithUserID(req.Context(), testUser))
	rec = httptest.NewRecorder()
	h.listVisits(rec, req)
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if len(resp.Visits) != 1 {
		t.Fatalf("own list: got %d want 1", len(resp.Visits))
	}
}

func TestProgress(t *testing.T) {
	color := "25B8EB"
	store := &fakeStore{
		stopExists: true,
		distanceM:  50,
		totalStops: 400,
		byMode: []generated.PassportProgressByModeRow{
			{Mode: "bus", TotalStops: 245, VisitedStops: 1},
			{Mode: "rail", TotalStops: 158, VisitedStops: 0},
		},
		byRoute: []generated.PassportProgressByRouteRow{
			{RouteKey: "TJ:4B", Mode: "bus", Color: pgtype.Text{String: color, Valid: true}, TotalStops: 12, VisitedStops: 1},
		},
	}
	h := newHandler(store)
	post(t, h, fmt.Sprintf(`{"stopId":%q,"clientMutationId":%q,"observedAt":%q}`,
		uuidStr(testStop), mutationID(), time.Now().UTC().Format(time.RFC3339)))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/passport/progress", nil)
	req = req.WithContext(auth.ContextWithUserID(req.Context(), testUser))
	rec := httptest.NewRecorder()
	h.progress(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		VisitedStops int64 `json:"visitedStops"`
		TotalStops   int64 `json:"totalStops"`
		ByMode       []struct {
			Mode         string `json:"mode"`
			VisitedStops int64  `json:"visitedStops"`
			TotalStops   int64  `json:"totalStops"`
		} `json:"byMode"`
		ByRoute []struct {
			RouteKey string `json:"routeKey"`
			Color    string `json:"color"`
		} `json:"byRoute"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.VisitedStops != 1 || resp.TotalStops != 400 {
		t.Fatalf("total: got %d/%d want 1/400", resp.VisitedStops, resp.TotalStops)
	}
	if len(resp.ByMode) != 2 || resp.ByMode[0].Mode != "bus" {
		t.Fatalf("byMode: %+v", resp.ByMode)
	}
	if len(resp.ByRoute) != 1 || resp.ByRoute[0].RouteKey != "TJ:4B" || resp.ByRoute[0].Color != color {
		t.Fatalf("byRoute: %+v", resp.ByRoute)
	}
}
