// Package resultcache provides a generic, mutex-guarded memoization cache
// for expensive computations such as LLM or query results.
package resultcache

import "sync"

// Cache is a concurrency-safe memoization store keyed by string.
// The zero value is not usable; construct via New.
type Cache struct {
	mu     sync.Mutex
	store  map[string]any
	hits   int
	misses int
}

// New returns an initialised, empty Cache.
func New() *Cache {
	return &Cache{store: make(map[string]any)}
}

// Get returns the cached value and true if present, otherwise (nil, false).
func (c *Cache) Get(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.store[key]
	if ok {
		c.hits++
	} else {
		c.misses++
	}
	return v, ok
}

// GetOrCompute returns the cached value for key if present; otherwise it
// invokes fn, stores the result (on nil error), and returns it.
// fn is called without the lock held, so it may itself be expensive or
// blocking. A concurrent call with the same key that arrives while fn is
// running will re-invoke fn; the last writer wins. This is intentionally
// simple: for workloads that need single-flight semantics, callers should
// layer singleflight on top.
func (c *Cache) GetOrCompute(key string, fn func() (any, error)) (any, error) {
	c.mu.Lock()
	if v, ok := c.store[key]; ok {
		c.hits++
		c.mu.Unlock()
		return v, nil
	}
	c.misses++
	c.mu.Unlock()

	v, err := fn()
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.store[key] = v
	c.mu.Unlock()
	return v, nil
}

// Invalidate removes all cached entries and resets statistics.
// It should be called after a corpus ingest so stale results are evicted.
func (c *Cache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store = make(map[string]any)
	c.hits = 0
	c.misses = 0
}

// Stats holds cache access counters.
type Stats struct {
	Hits   int
	Misses int
}

// Stats returns a snapshot of current hit/miss counters.
func (c *Cache) Stats() Stats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return Stats{Hits: c.hits, Misses: c.misses}
}
