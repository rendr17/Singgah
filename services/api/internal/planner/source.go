package planner

import (
	"context"
	"sync"
	"time"
)

// EngineSource hands handlers a fresh-enough planner snapshot. The
// schedule is static between ingests, so a TTL guards reloads; a stale
// snapshot is rebuilt synchronously by whichever request arrives first
// while others queue on the same mutex — no background goroutines.
type EngineSource struct {
	mu   sync.Mutex
	load func(context.Context) (*Engine, error)
	ttl  time.Duration
	eng  *Engine
}

func NewEngineSource(load func(context.Context) (*Engine, error), ttl time.Duration) *EngineSource {
	return &EngineSource{load: load, ttl: ttl}
}

func (s *EngineSource) Get(ctx context.Context) (*Engine, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.eng != nil && time.Since(s.eng.BuiltAt()) < s.ttl {
		return s.eng, nil
	}
	eng, err := s.load(ctx)
	if err != nil {
		if s.eng != nil {
			return s.eng, nil // stale snapshot beats no schedule
		}
		return nil, err
	}
	s.eng = eng
	return eng, nil
}
