// Package auth owns anonymous-first session identity (ADR-010): a bare
// `users` row plus a 256-bit opaque bearer token. Only the token's SHA-256
// hash is ever stored, so a database leak does not leak usable credentials.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"singgah/services/api/db/generated"
	"singgah/services/api/internal/http/response"
)

// SessionTTL is a fixed long-lived expiry: anonymous device identity has no
// credential to rotate, so short-lived tokens would add churn without gain.
const SessionTTL = 365 * 24 * time.Hour

const tokenBytes = 32 // 256-bit

// ErrUnauthenticated covers every way a token can fail — malformed, unknown,
// expired, or revoked. Callers get a single 401 either way.
var ErrUnauthenticated = errors.New("unauthenticated")

// Store is the persistence seam the service needs — generated.Queries satisfies it.
type Store interface {
	CreateUser(ctx context.Context) (generated.User, error)
	CreateAuthSession(ctx context.Context, arg generated.CreateAuthSessionParams) (generated.AuthSession, error)
	GetAuthSessionByTokenHash(ctx context.Context, tokenHash []byte) (generated.AuthSession, error)
	TouchUserLastSeen(ctx context.Context, id pgtype.UUID) error
	RevokeAuthSession(ctx context.Context, tokenHash []byte) error
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

// Issue creates a fresh user + session and returns the raw token — the only
// place the unhashed token ever exists server-side.
func (s *Service) Issue(ctx context.Context) (token string, userID pgtype.UUID, expiresAt time.Time, err error) {
	var raw [tokenBytes]byte
	if _, err = rand.Read(raw[:]); err != nil {
		return "", pgtype.UUID{}, time.Time{}, err
	}
	user, err := s.store.CreateUser(ctx)
	if err != nil {
		return "", pgtype.UUID{}, time.Time{}, err
	}
	expiresAt = time.Now().Add(SessionTTL)
	_, err = s.store.CreateAuthSession(ctx, generated.CreateAuthSessionParams{
		UserID:    user.ID,
		TokenHash: tokenHash(raw[:]),
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		return "", pgtype.UUID{}, time.Time{}, err
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), user.ID, expiresAt, nil
}

// Authenticate resolves a bearer token to its user ID.
func (s *Service) Authenticate(ctx context.Context, token string) (pgtype.UUID, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != tokenBytes {
		return pgtype.UUID{}, ErrUnauthenticated
	}
	sess, err := s.store.GetAuthSessionByTokenHash(ctx, tokenHash(raw))
	if err != nil {
		return pgtype.UUID{}, ErrUnauthenticated
	}
	// Hourly-throttled inside the query — a write at most once per user per hour.
	_ = s.store.TouchUserLastSeen(ctx, sess.UserID)
	return sess.UserID, nil
}

// Revoke ends a session early ("logout" for an anonymous identity).
func (s *Service) Revoke(ctx context.Context, token string) error {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != tokenBytes {
		return ErrUnauthenticated
	}
	return s.store.RevokeAuthSession(ctx, tokenHash(raw))
}

type userIDKey struct{}

// UserIDFrom returns the authenticated user ID set by RequireUser.
func UserIDFrom(ctx context.Context) (pgtype.UUID, bool) {
	id, ok := ctx.Value(userIDKey{}).(pgtype.UUID)
	return id, ok
}

// RequireUser gates a route on a valid Authorization: Bearer token.
func (s *Service) RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok {
			response.Error(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Token sesi diperlukan")
			return
		}
		userID, err := s.Authenticate(r.Context(), token)
		if err != nil {
			response.Error(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Sesi tidak valid atau kedaluwarsa")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userIDKey{}, userID)))
	})
}

func tokenHash(raw []byte) []byte {
	sum := sha256.Sum256(raw)
	return sum[:]
}
