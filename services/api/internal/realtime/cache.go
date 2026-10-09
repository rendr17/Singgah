package realtime

import (
	"sort"
	"sync"
	"time"
)

// Cache is the current live-state store — in-memory per the architecture
// ("current live cache"), scoped reads only. It never persists history;
// selected-event storage is a later phase task.
type Cache struct {
	mu       sync.RWMutex
	vehicles map[string]Vehicle
	// lastWriteAt is when a poll last delivered entities — the feed-level
	// "still alive" signal independent of individual entity freshness.
	lastWriteAt time.Time
}

func NewCache() *Cache {
	return &Cache{vehicles: map[string]Vehicle{}}
}

func vehicleKey(v Vehicle) string {
	// Vehicle IDs are only unique per provider.
	return v.Source + "/" + v.ID
}

// Upsert replaces each entity under its provider-scoped key and records the
// write time. Callers pass already-normalized vehicles; zero positions are
// filtered upstream by the adapter, not silently dropped here.
func (c *Cache) Upsert(vs []Vehicle, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, v := range vs {
		c.vehicles[vehicleKey(v)] = v
	}
	c.lastWriteAt = now
	c.evictLocked(now)
}

// Snapshot returns the entities inside bbox (nil bbox = unscoped, kept for
// the poller/tests — the HTTP handler always requires a viewport). routeID
// additionally narrows to one corridor. Entities past the retain window are
// evicted instead of served.
func (c *Cache) Snapshot(now time.Time, bbox *[4]float64, routeID string) []Vehicle {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.evictLocked(now)
	out := make([]Vehicle, 0, len(c.vehicles))
	for _, v := range c.vehicles {
		if bbox != nil && !v.inBBox(*bbox) {
			continue
		}
		if routeID != "" && v.RouteID != routeID {
			continue
		}
		out = append(out, v)
	}
	// Deterministic order so a stable viewport yields a stable response.
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		return a.ID < b.ID
	})
	return out
}

// LastWriteAt reports the most recent successful upsert.
func (c *Cache) LastWriteAt() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastWriteAt
}

func (c *Cache) evictLocked(now time.Time) {
	for k, v := range c.vehicles {
		if now.Sub(v.ObservedAt) > VehicleRetainWindow {
			delete(c.vehicles, k)
		}
	}
}
