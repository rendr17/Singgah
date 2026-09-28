package auth

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"singgah/services/api/db/generated"
)

type fakeStore struct {
	sessions map[string]generated.AuthSession
	users    int
	touched  int
}

func newFakeStore() *fakeStore {
	return &fakeStore{sessions: map[string]generated.AuthSession{}}
}

func (f *fakeStore) CreateUser(ctx context.Context) (generated.User, error) {
	f.users++
	var id pgtype.UUID
	id.Bytes[15] = byte(f.users)
	id.Valid = true
	return generated.User{ID: id}, nil
}

func (f *fakeStore) CreateAuthSession(ctx context.Context, arg generated.CreateAuthSessionParams) (generated.AuthSession, error) {
	sess := generated.AuthSession{UserID: arg.UserID, TokenHash: arg.TokenHash, ExpiresAt: arg.ExpiresAt}
	f.sessions[string(arg.TokenHash)] = sess
	return sess, nil
}

func (f *fakeStore) GetAuthSessionByTokenHash(ctx context.Context, h []byte) (generated.AuthSession, error) {
	sess, ok := f.sessions[string(h)]
	if !ok || sess.RevokedAt.Valid || sess.ExpiresAt.Time.Before(time.Now()) {
		return generated.AuthSession{}, pgx.ErrNoRows
	}
	return sess, nil
}

func (f *fakeStore) TouchUserLastSeen(ctx context.Context, id pgtype.UUID) error {
	f.touched++
	return nil
}

func (f *fakeStore) RevokeAuthSession(ctx context.Context, h []byte) error {
	sess, ok := f.sessions[string(h)]
	if !ok {
		return pgx.ErrNoRows
	}
	sess.RevokedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	f.sessions[string(h)] = sess
	return nil
}

func TestIssueThenAuthenticate(t *testing.T) {
	svc := NewService(newFakeStore())
	token, userID, expiresAt, err := svc.Issue(context.Background())
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != tokenBytes {
		t.Fatalf("token must be %d random bytes base64url, got len=%d err=%v", tokenBytes, len(raw), err)
	}
	if !expiresAt.After(time.Now().Add(SessionTTL - time.Minute)) {
		t.Fatalf("expiresAt %v not ~SessionTTL ahead", expiresAt)
	}
	got, err := svc.Authenticate(context.Background(), token)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if got != userID {
		t.Fatalf("userID mismatch: got %v want %v", got.Bytes, userID.Bytes)
	}
}

func TestAuthenticateRejects(t *testing.T) {
	svc := NewService(newFakeStore())
	token, _, _, _ := svc.Issue(context.Background())

	for name, tok := range map[string]string{
		"empty":      "",
		"not-base64": "%%%invalid%%%",
		"short":      base64.RawURLEncoding.EncodeToString([]byte("tiny")),
		"unknown":    base64.RawURLEncoding.EncodeToString(make([]byte, tokenBytes)),
	} {
		if _, err := svc.Authenticate(context.Background(), tok); !errors.Is(err, ErrUnauthenticated) {
			t.Fatalf("%s: want ErrUnauthenticated, got %v", name, err)
		}
	}
	if _, err := svc.Authenticate(context.Background(), token); err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}
}

func TestRevokedSessionFails(t *testing.T) {
	svc := NewService(newFakeStore())
	token, _, _, _ := svc.Issue(context.Background())
	if err := svc.Revoke(context.Background(), token); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if _, err := svc.Authenticate(context.Background(), token); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("revoked token must fail, got %v", err)
	}
}

func TestRequireUser(t *testing.T) {
	svc := NewService(newFakeStore())
	token, userID, _, _ := svc.Issue(context.Background())

	var got pgtype.UUID
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = UserIDFrom(r.Context())
		w.WriteHeader(http.StatusNoContent)
	})
	h := svc.RequireUser(inner)

	// No header → 401.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no header: got %d want 401", rec.Code)
	}

	// Valid → handler runs with userID in ctx.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent || got != userID {
		t.Fatalf("valid: code=%d userID match=%v", rec.Code, got == userID)
	}

	// Garbage → 401.
	req.Header.Set("Authorization", "Bearer garbage")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("garbage: got %d want 401", rec.Code)
	}
}
