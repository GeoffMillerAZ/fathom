// Package resultcache provides generic, exact-key memoization with TTL,
// LRU eviction, and single-flight computation for expensive results such
// as LLM calls, query evaluations, or tool invocations.
//
// Purpose: callers memoize a computation keyed by an exact string. Cache[V]
// stores values of any type V, expires entries after a configurable TTL
// (via an injected clock — never time.Now()), and evicts the least recently
// used entry when MaxEntries is reached. GetOrCompute owns single-flight
// semantics: concurrent callers for the same key share exactly one compute
// invocation; losers wait for the leader's result. A compute error is
// delivered to every caller of that flight and is never cached, so the next
// call recomputes.
//
// Boundary vs foundations/governance/answer-cache: answercache is a
// semantic Q&A cache — it normalizes natural-language questions and matches
// near-duplicates by embedding cosine similarity. resultcache is exact-key
// memoization with no normalization and no semantic matching; the key is an
// opaque string and only byte-identical keys hit. Use answercache to dedupe
// near-static LLM answers across phrasings; use resultcache to memoize
// deterministic computations behind a precise key.
//
// Composition: stdlib only (container/list, sync, time). Behavioral
// package — no schema.cue, no data contract.
//
// Concurrency contract: safe for concurrent use. Internal state is guarded
// by a sync.Mutex; compute functions run without the lock held, so they may
// block or call back into the cache under different keys (re-entrant calls
// with the same key from inside its own compute would deadlock the flight
// and must be avoided).
package resultcache

import (
	"container/list"
	"context"
	"sync"
	"time"
)

// DefaultMaxEntries is the capacity used when Config.MaxEntries is zero.
const DefaultMaxEntries = 1024

// Config holds parameters for a Cache.
type Config struct {
	// TTL is the time-to-live for each cached entry, measured by the
	// injected clock. Zero (or negative) means entries never expire.
	TTL time.Duration

	// MaxEntries is the maximum number of entries before LRU eviction.
	// Zero (or negative) means DefaultMaxEntries.
	MaxEntries int
}

// Stats is a snapshot of cache counters.
type Stats struct {
	// Hits counts lookups that returned a live cached value.
	Hits int64

	// Misses counts lookups that found no live value, including expired
	// entries and GetOrCompute callers that joined an in-flight compute.
	Misses int64

	// Evictions counts entries removed by LRU capacity pressure.
	// Lazily removed expired entries are counted as misses, not evictions.
	Evictions int64
}

// entry is one cached key/value pair.
type entry[V any] struct {
	key       string
	val       V
	expiresAt time.Time // zero means no expiry
}

// flight is one in-progress compute shared by concurrent callers of a key.
type flight[V any] struct {
	done chan struct{} // closed when the leader's compute finishes
	val  V
	err  error
}

// Cache is a concurrency-safe memoization store keyed by exact string.
// The zero value is not usable; construct via New.
type Cache[V any] struct {
	mu      sync.Mutex
	ttl     time.Duration
	max     int
	now     func() time.Time
	order   *list.List               // front = most recently used; values are *entry[V]
	index   map[string]*list.Element // key -> element in order
	flights map[string]*flight[V]    // key -> in-progress compute
	stats   Stats
}

// New returns an initialised, empty Cache configured by cfg.
// now is the injected clock used for TTL decisions; if nil, New panics —
// this package requires an explicit clock and never falls back to
// time.Now() (pass time.Now explicitly at the composition root if
// wall-clock behavior is wanted).
func New[V any](cfg Config, now func() time.Time) *Cache[V] {
	if now == nil {
		panic("resultcache.New: nil now clock")
	}
	max := cfg.MaxEntries
	if max <= 0 {
		max = DefaultMaxEntries
	}
	ttl := cfg.TTL
	if ttl < 0 {
		ttl = 0
	}
	return &Cache[V]{
		ttl:     ttl,
		max:     max,
		now:     now,
		order:   list.New(),
		index:   make(map[string]*list.Element),
		flights: make(map[string]*flight[V]),
	}
}

// Get returns the cached value for key and true on a live hit.
// Expired entries are removed lazily and reported as misses.
// A hit refreshes the entry's LRU recency.
func (c *Cache[V]) Get(key string) (V, bool) {
	now := c.now()
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.getLocked(key, now)
}

// Put stores v under key, refreshing its TTL and LRU recency.
// If the cache is at capacity and key is new, the least recently used
// entry is evicted.
func (c *Cache[V]) Put(key string, v V) {
	now := c.now()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.putLocked(key, v, now)
}

// GetOrCompute returns the cached value for key if live; otherwise it
// computes it with single-flight semantics: the first caller (the leader)
// invokes compute exactly once, and concurrent callers for the same key
// wait for that result instead of recomputing. On success the value is
// cached and returned to every caller of the flight. On error nothing is
// cached, the error is returned to every caller of the flight, and a
// subsequent call recomputes.
//
// compute receives the leader's ctx. A waiter whose own ctx is cancelled
// stops waiting and returns ctx.Err(); the leader's compute is unaffected
// and its result (if successful) is still cached for future callers.
func (c *Cache[V]) GetOrCompute(ctx context.Context, key string, compute func(context.Context) (V, error)) (V, error) {
	now := c.now()
	c.mu.Lock()
	if v, ok := c.getLocked(key, now); ok {
		c.mu.Unlock()
		return v, nil
	}
	// Miss already counted by getLocked. Join an in-flight compute if any.
	if f, ok := c.flights[key]; ok {
		c.mu.Unlock()
		select {
		case <-f.done:
			return f.val, f.err
		case <-ctx.Done():
			var zero V
			return zero, ctx.Err()
		}
	}
	// Become the leader.
	f := &flight[V]{done: make(chan struct{})}
	c.flights[key] = f
	c.mu.Unlock()

	v, err := compute(ctx)

	c.mu.Lock()
	delete(c.flights, key)
	if err == nil {
		c.putLocked(key, v, c.now())
	}
	c.mu.Unlock()

	if err != nil {
		var zero V
		v = zero // every caller of a failed flight gets the zero value
	}
	f.val, f.err = v, err
	close(f.done)
	return v, err
}

// Stats returns a snapshot of current counters.
func (c *Cache[V]) Stats() Stats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stats
}

// Len returns the number of entries currently stored (live or expired but
// not yet lazily removed).
func (c *Cache[V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}

// getLocked looks up key at time now, counting a hit or miss.
// Callers must hold c.mu.
func (c *Cache[V]) getLocked(key string, now time.Time) (V, bool) {
	if el, ok := c.index[key]; ok {
		e := el.Value.(*entry[V])
		if e.expiresAt.IsZero() || now.Before(e.expiresAt) {
			c.stats.Hits++
			c.order.MoveToFront(el)
			return e.val, true
		}
		// Expired — remove lazily; counts as a miss, not an eviction.
		c.order.Remove(el)
		delete(c.index, key)
	}
	c.stats.Misses++
	var zero V
	return zero, false
}

// putLocked inserts or refreshes key at time now, evicting LRU entries
// past capacity. Callers must hold c.mu.
func (c *Cache[V]) putLocked(key string, v V, now time.Time) {
	var expiresAt time.Time
	if c.ttl > 0 {
		expiresAt = now.Add(c.ttl)
	}
	if el, ok := c.index[key]; ok {
		e := el.Value.(*entry[V])
		e.val = v
		e.expiresAt = expiresAt
		c.order.MoveToFront(el)
		return
	}
	c.index[key] = c.order.PushFront(&entry[V]{key: key, val: v, expiresAt: expiresAt})
	for c.order.Len() > c.max {
		oldest := c.order.Back()
		if oldest == nil {
			break
		}
		e := oldest.Value.(*entry[V])
		c.order.Remove(oldest)
		delete(c.index, e.key)
		c.stats.Evictions++
	}
}
