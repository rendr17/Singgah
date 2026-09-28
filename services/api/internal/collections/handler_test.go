package collections

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	generated "singgah/services/api/db/generated"
)

type fakeStore struct {
	rows []generated.ListCollectionsRow
	err  error
}

func (f *fakeStore) ListCollections(ctx context.Context) ([]generated.ListCollectionsRow, error) {
	return f.rows, f.err
}

func TestList(t *testing.T) {
	store := &fakeStore{rows: []generated.ListCollectionsRow{
		{
			Slug:        "mrt-jakarta",
			Title:       "MRT Jakarta",
			Description: pgtype.Text{String: "Semua stasiun MRT.", Valid: true},
			Kind:        "curated",
			ItemCount:   13,
		},
		{Slug: "no-desc", Title: "Tanpa deskripsi", Kind: "curated", ItemCount: 0},
	}}
	h := NewHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/collections", nil)
	rec := httptest.NewRecorder()
	h.list(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Collections []struct {
			Slug        string `json:"slug"`
			Title       string `json:"title"`
			Description string `json:"description"`
			Kind        string `json:"kind"`
			ItemCount   int64  `json:"itemCount"`
		} `json:"collections"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Collections) != 2 {
		t.Fatalf("collections: %+v", resp.Collections)
	}
	if resp.Collections[0].Slug != "mrt-jakarta" || resp.Collections[0].ItemCount != 13 ||
		resp.Collections[0].Description != "Semua stasiun MRT." {
		t.Fatalf("first collection: %+v", resp.Collections[0])
	}
	if resp.Collections[1].Description != "" {
		t.Fatalf("null description should be omitted, got %q", resp.Collections[1].Description)
	}
}
