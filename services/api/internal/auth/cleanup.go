package auth

import (
	"context"
	"log/slog"
	"time"
)

// CleanupStore is the persistence seam the session sweeper needs —
// generated.Queries satisfies it.
type CleanupStore interface {
	DeleteDeadAuthSessions(ctx context.Context) (int64, error)
}

// StartSessionCleanup deletes expired/revoked auth sessions on an interval
// until ctx is done — dead sessions can never authenticate again and would
// otherwise grow the table forever. One immediate pass runs on start so a
// long-off instance still cleans up. Returns a channel that closes when the
// loop exits — the owner waits on it during shutdown.
func StartSessionCleanup(ctx context.Context, store CleanupStore, interval time.Duration, log *slog.Logger) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		sweep := func() {
			sctx, cancel := context.WithTimeout(ctx, time.Minute)
			defer cancel()
			n, err := store.DeleteDeadAuthSessions(sctx)
			if err != nil {
				log.Warn("auth session cleanup failed — retrying next tick", "error", err)
			} else if n > 0 {
				log.Info("auth session cleanup", "deleted", n)
			}
		}
		sweep()
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				sweep()
			}
		}
	}()
	return done
}
