package realtime

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Source is the seam a real feed adapter implements. Returning an error
// means the provider failed; returning an empty slice means the provider
// answered and had nothing — only the first opens the breaker.
type Source interface {
	FetchVehicles(ctx context.Context) ([]Vehicle, error)
}

// breakerFailures is how many consecutive fetch errors open the circuit.
// Open means: stop hammering a dead provider, keep serving last-known STALE.
const breakerFailures = 3

// FeedStatus is the feed-level health surfaced on /vehicles and
// /realtime/health. It is feed truth, not entity freshness — a "live" feed
// can still serve zero vehicles.
type FeedStatus struct {
	Status              string     `json:"status"` // live | degraded | unavailable
	Source              string     `json:"source,omitempty"`
	LastSuccessAt       *time.Time `json:"lastSuccessAt,omitempty"`
	LastErrorAt         *time.Time `json:"lastErrorAt,omitempty"`
	ConsecutiveFailures int        `json:"consecutiveFailures,omitempty"`
}

// Poller owns a provider fetch loop and its circuit breaker. Construct it
// only when a source is configured; without a Poller the handler reports
// "unavailable" rather than pretending an empty cache is a healthy feed.
type Poller struct {
	fetch    func(ctx context.Context) error
	sourceID string
	interval time.Duration
	log      *slog.Logger

	mu          sync.Mutex
	failures    int
	lastSuccess time.Time
	lastError   time.Time
	openUntil   time.Time // breaker open until this instant
}

func newPoller(fetch func(ctx context.Context) error, sourceID string, interval time.Duration, log *slog.Logger) *Poller {
	return &Poller{fetch: fetch, sourceID: sourceID, interval: interval, log: log}
}

func NewPoller(src Source, sourceID string, cache *Cache, interval time.Duration, log *slog.Logger) *Poller {
	return newPoller(func(ctx context.Context) error {
		vs, err := src.FetchVehicles(ctx)
		if err != nil {
			return err
		}
		now := time.Now()
		for i := range vs {
			if vs[i].ReceivedAt.IsZero() {
				vs[i].ReceivedAt = now
			}
			if vs[i].Source == "" {
				vs[i].Source = sourceID
			}
		}
		cache.Upsert(vs, now)
		return nil
	}, sourceID, interval, log)
}

// Run polls on interval until ctx is done. The returned channel closes when
// the loop exits so the owner can wait during shutdown — same contract as
// ingest.StartRefresher.
func (p *Poller) Run(ctx context.Context) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		p.tick(ctx)
		t := time.NewTicker(p.interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				p.tick(ctx)
			}
		}
	}()
	return done
}

func (p *Poller) tick(ctx context.Context) {
	p.mu.Lock()
	if now := time.Now(); now.Before(p.openUntil) {
		p.mu.Unlock()
		return
	}
	p.mu.Unlock()

	fctx, cancel := context.WithTimeout(ctx, p.interval/2)
	err := p.fetch(fctx)
	cancel()

	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	if err != nil {
		p.failures++
		p.lastError = now
		if p.failures >= breakerFailures {
			// Skip a bounded number of cycles, then half-open; each failed
			// retry pushes the window out again so a dead provider is not
			// hammered every interval.
			p.openUntil = now.Add(p.interval * 4)
		}
		p.log.Warn("realtime fetch failed", "source", p.sourceID, "error", err, "failures", p.failures)
		return
	}
	p.failures = 0
	p.openUntil = time.Time{}
	p.lastSuccess = now
}

// Status snapshots the feed state. "degraded" covers both an open breaker
// and a feed that has never succeeded — both mean "don't trust silence".
func (p *Poller) Status() FeedStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	s := FeedStatus{Status: "live", Source: p.sourceID}
	if !p.lastSuccess.IsZero() {
		t := p.lastSuccess
		s.LastSuccessAt = &t
	}
	if !p.lastError.IsZero() {
		t := p.lastError
		s.LastErrorAt = &t
	}
	if p.failures > 0 {
		s.ConsecutiveFailures = p.failures
	}
	if p.failures > 0 || p.lastSuccess.IsZero() {
		s.Status = "degraded"
	}
	return s
}
