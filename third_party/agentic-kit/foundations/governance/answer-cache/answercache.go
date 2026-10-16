// Package answercache is a cost-saving cache for near-static LLM answers.
//
// It performs two levels of matching:
//  1. Exact match on a normalized key (lowercase, trimmed, collapsed whitespace).
//  2. Near-duplicate match via embedding cosine similarity (optional, needs Embedder).
//
// Entries expire after a configurable TTL. When the cache is full, the least
// recently used entry is evicted. The cache is concurrency-safe and compiles to
// wasm (no file I/O; clock is injected).
package answercache

import (
	"context"
	"math"
	"strings"
	"sync"
	"time"
	"unicode"

	embeddingclient "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/embedding/embedding-client"
)

// Config holds parameters for a Cache.
type Config struct {
	// TTL is the time-to-live for each cached entry.
	TTL time.Duration

	// SimThreshold is the minimum cosine similarity for a near-duplicate match.
	// Only used when an embedder is configured. Values ≤ -1.0 match everything.
	SimThreshold float64

	// MaxEntries is the maximum number of entries before LRU eviction.
	// Zero means no limit.
	MaxEntries int
}

// entry is one cached question/answer pair.
type entry struct {
	answer    string
	storedAt  time.Time
	embedding embeddingclient.Vector // nil when embedder is not configured

	// LRU bookkeeping.
	key  string
	prev *entry
	next *entry
}

// Snapshot is a plain-data representation of the cache for serialization.
// It contains no unexported types and can be safely marshalled.
type Snapshot struct {
	Entries []SnapshotEntry
}

// SnapshotEntry is one serialized cache entry.
type SnapshotEntry struct {
	Key       string
	Answer    string
	StoredAt  time.Time
	Embedding []float64
}

// Cache is a TTL + LRU answer cache. The zero value is not usable; use New.
type Cache struct {
	cfg      Config
	now      func() time.Time
	embedder embeddingclient.Embedder

	mu    sync.Mutex
	index map[string]*entry // normalized key → entry
	head  *entry            // MRU end of doubly-linked list
	tail  *entry            // LRU end
	size  int
}

// New creates a Cache with the given configuration and injected clock.
// embedder may be nil; in that case near-duplicate matching is skipped.
func New(cfg Config, now func() time.Time, embedder embeddingclient.Embedder) *Cache {
	return &Cache{
		cfg:      cfg,
		now:      now,
		embedder: embedder,
		index:    make(map[string]*entry),
	}
}

// Get returns the cached answer for question, along with a hit indicator.
// It checks exact (normalized) match first, then near-duplicate if an embedder
// is configured. Expired entries are treated as misses.
func (c *Cache) Get(ctx context.Context, question string) (string, bool) {
	key := normalize(question)
	now := c.now()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Exact match.
	if e, ok := c.index[key]; ok {
		if !c.isExpired(e, now) {
			c.promote(e)
			return e.answer, true
		}
		// Expired — remove lazily.
		c.removeEntry(e)
	}

	// Near-duplicate match (only if embedder is set).
	if c.embedder == nil {
		return "", false
	}

	// Release lock during embedding (network/compute call).
	c.mu.Unlock()
	vec, err := c.embedder.Embed(ctx, question)
	c.mu.Lock()
	if err != nil {
		return "", false
	}

	best, bestSim := (*entry)(nil), math.Inf(-1)
	for _, e := range c.index {
		if c.isExpired(e, now) {
			continue
		}
		if e.embedding == nil {
			continue
		}
		sim := cosineSimilarity(vec, e.embedding)
		if sim > bestSim {
			bestSim = sim
			best = e
		}
	}

	if best != nil && bestSim >= c.cfg.SimThreshold {
		c.promote(best)
		return best.answer, true
	}
	return "", false
}

// Put stores question→answer in the cache.
// If an embedder is configured, the question embedding is also stored.
func (c *Cache) Put(ctx context.Context, question, answer string) {
	key := normalize(question)
	now := c.now()

	// Compute embedding outside the lock when possible.
	var vec embeddingclient.Vector
	if c.embedder != nil {
		v, err := c.embedder.Embed(ctx, question)
		if err == nil {
			vec = v
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if existing, ok := c.index[key]; ok {
		// Update in place.
		existing.answer = answer
		existing.storedAt = now
		existing.embedding = vec
		c.promote(existing)
		return
	}

	e := &entry{
		answer:    answer,
		storedAt:  now,
		embedding: vec,
		key:       key,
	}
	c.index[key] = e
	c.pushFront(e)
	c.size++

	// Evict LRU entry if over capacity.
	if c.cfg.MaxEntries > 0 {
		for c.size > c.cfg.MaxEntries {
			c.evictTail()
		}
	}
}

// Snapshot returns a serialisable copy of all live (non-expired) cache entries.
func (c *Cache) Snapshot() Snapshot {
	now := c.now()
	c.mu.Lock()
	defer c.mu.Unlock()

	var entries []SnapshotEntry
	for _, e := range c.index {
		if c.isExpired(e, now) {
			continue
		}
		se := SnapshotEntry{
			Key:      e.key,
			Answer:   e.answer,
			StoredAt: e.storedAt,
		}
		if e.embedding != nil {
			cp := make([]float64, len(e.embedding))
			copy(cp, e.embedding)
			se.Embedding = cp
		}
		entries = append(entries, se)
	}
	return Snapshot{Entries: entries}
}

// Restore replaces the cache contents with the entries from snap.
// It discards all existing entries first.
func (c *Cache) Restore(snap Snapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Clear existing.
	c.index = make(map[string]*entry)
	c.head = nil
	c.tail = nil
	c.size = 0

	for _, se := range snap.Entries {
		e := &entry{
			answer:   se.Answer,
			storedAt: se.StoredAt,
			key:      se.Key,
		}
		if len(se.Embedding) > 0 {
			cp := make(embeddingclient.Vector, len(se.Embedding))
			copy(cp, se.Embedding)
			e.embedding = cp
		}
		c.index[se.Key] = e
		c.pushFront(e)
		c.size++
		if c.cfg.MaxEntries > 0 && c.size > c.cfg.MaxEntries {
			c.evictTail()
		}
	}
}

// --- internal helpers ---

func (c *Cache) isExpired(e *entry, now time.Time) bool {
	if c.cfg.TTL <= 0 {
		return false
	}
	return now.Sub(e.storedAt) >= c.cfg.TTL
}

// pushFront inserts e at the MRU (head) end.
func (c *Cache) pushFront(e *entry) {
	e.prev = nil
	e.next = c.head
	if c.head != nil {
		c.head.prev = e
	}
	c.head = e
	if c.tail == nil {
		c.tail = e
	}
}

// promote moves e to the MRU (head) position.
func (c *Cache) promote(e *entry) {
	if c.head == e {
		return
	}
	c.unlink(e)
	e.prev = nil
	e.next = c.head
	if c.head != nil {
		c.head.prev = e
	}
	c.head = e
}

// unlink removes e from the doubly-linked list without touching c.index.
func (c *Cache) unlink(e *entry) {
	if e.prev != nil {
		e.prev.next = e.next
	} else {
		c.head = e.next
	}
	if e.next != nil {
		e.next.prev = e.prev
	} else {
		c.tail = e.prev
	}
	e.prev = nil
	e.next = nil
}

// removeEntry removes e from both the list and the index.
func (c *Cache) removeEntry(e *entry) {
	c.unlink(e)
	delete(c.index, e.key)
	c.size--
}

// evictTail removes the LRU (tail) entry.
func (c *Cache) evictTail() {
	if c.tail == nil {
		return
	}
	c.removeEntry(c.tail)
}

// normalize lower-cases, trims, and collapses internal whitespace.
func normalize(s string) string {
	s = strings.ToLower(s)
	// Collapse whitespace (including tabs, newlines) to single space.
	var b strings.Builder
	b.Grow(len(s))
	prevSpace := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !prevSpace {
				b.WriteByte(' ')
			}
			prevSpace = true
		} else {
			b.WriteRune(r)
			prevSpace = false
		}
	}
	return strings.TrimSpace(b.String())
}

// cosineSimilarity returns the cosine similarity of two vectors.
func cosineSimilarity(a, b embeddingclient.Vector) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
