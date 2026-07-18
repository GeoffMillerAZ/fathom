package embeddingclient

import (
	"container/list"
	"context"
	"fmt"
	"sync"
	"sync/atomic"
)

// DefaultMaxEntries is the cache capacity used by NewCachingEmbedder and by
// CacheOptions.MaxEntries == 0. At typical embedding dimensions (768–3072
// float64s) a full cache costs roughly 25–100 MB, a sane ceiling for a
// long-lived process.
const DefaultMaxEntries = 4096

// CacheOptions configures a CachingEmbedder created via
// NewCachingEmbedderWithOptions.
type CacheOptions struct {
	// MaxEntries bounds the number of cached vectors:
	//
	//	 0  → DefaultMaxEntries (the recommended bounded mode)
	//	>0  → that exact cap, with least-recently-used eviction
	//	-1  → unbounded (the pre-bounded-mode behaviour; opt in explicitly
	//	      only when the key space is known to be small)
	MaxEntries int
}

// cacheEntry is the value stored in each LRU list element.
type cacheEntry struct {
	key string
	vec Vector
}

// CachingEmbedder wraps an Embedder with an in-memory, concurrency-safe LRU
// cache keyed by the raw text string. Duplicate texts within a batch are
// deduplicated before the inner Embedder is called, and the result is fanned
// out to all positions that share the same key.
//
// The cache is bounded by MaxEntries (DefaultMaxEntries unless overridden via
// NewCachingEmbedderWithOptions) and evicts the least-recently-used entry on
// overflow. Both reads and writes refresh recency.
//
// Hit and miss counts are maintained atomically and are safe to read from any
// goroutine at any time. A hit is a lookup that found a live (non-evicted)
// entry; a miss is any lookup that did not, including re-requests of evicted
// texts. Within a single EmbedBatch call, duplicate uncached texts count as
// one miss (one inner embedding is performed for them).
//
// CachingEmbedder is safe for concurrent use via an internal sync.Mutex
// guarding the map+list and atomic hit/miss counters.
type CachingEmbedder struct {
	inner      Embedder
	maxEntries int // -1 = unbounded, otherwise > 0

	mu    sync.Mutex
	cache map[string]*list.Element
	lru   *list.List // front = most recently used

	hits   atomic.Int64
	misses atomic.Int64
}

// NewCachingEmbedder wraps inner with a fresh, empty cache bounded at
// DefaultMaxEntries with LRU eviction.
//
// Compatibility note: before bounded mode existed this constructor returned an
// unbounded cache. No known caller depends on retaining more than
// DefaultMaxEntries distinct texts; callers that genuinely need unbounded
// growth must opt in via NewCachingEmbedderWithOptions with MaxEntries: -1.
func NewCachingEmbedder(inner Embedder) *CachingEmbedder {
	return NewCachingEmbedderWithOptions(inner, CacheOptions{})
}

// NewCachingEmbedderWithOptions wraps inner with a fresh, empty cache
// configured by opts. See CacheOptions.MaxEntries for capacity semantics.
func NewCachingEmbedderWithOptions(inner Embedder, opts CacheOptions) *CachingEmbedder {
	max := opts.MaxEntries
	switch {
	case max == 0:
		max = DefaultMaxEntries
	case max < 0:
		max = -1 // any negative value means unbounded
	}
	return &CachingEmbedder{
		inner:      inner,
		maxEntries: max,
		cache:      make(map[string]*list.Element),
		lru:        list.New(),
	}
}

// Hits returns the total number of cache hits since the embedder was created.
func (c *CachingEmbedder) Hits() int64 { return c.hits.Load() }

// Misses returns the total number of cache misses since the embedder was
// created. Evicted entries that are requested again count as new misses.
func (c *CachingEmbedder) Misses() int64 { return c.misses.Load() }

// Len returns the current number of cached entries.
func (c *CachingEmbedder) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.cache)
}

// MaxEntries returns the configured capacity: a positive cap, or -1 when the
// cache is unbounded.
func (c *CachingEmbedder) MaxEntries() int { return c.maxEntries }

// Dimensions reports the vector dimensionality of the wrapped Embedder when it
// implements DimensionedEmbedder, or 0 when unknown. This makes the wrapper
// transparent to DimensionsOf.
func (c *CachingEmbedder) Dimensions() int {
	if d, ok := DimensionsOf(c.inner); ok {
		return d
	}
	return 0
}

// Embed returns the cached vector for text if present; otherwise it delegates
// to the inner Embedder, stores the result, and returns it.
func (c *CachingEmbedder) Embed(ctx context.Context, text string) (Vector, error) {
	if v, ok := c.lookup(text); ok {
		c.hits.Add(1)
		return v, nil
	}
	c.misses.Add(1)

	v, err := c.inner.Embed(ctx, text)
	if err != nil {
		return nil, err
	}
	c.store(text, v)
	return v, nil
}

// EmbedBatch returns vectors for each element of texts. Texts that are already
// cached are resolved without calling the inner Embedder. Uncached texts are
// collected, deduplicated, passed to the inner EmbedBatch in a single call,
// and their results are stored before being returned.
func (c *CachingEmbedder) EmbedBatch(ctx context.Context, texts []string) ([]Vector, error) {
	results := make([]Vector, len(texts))
	filled := make([]bool, len(texts))

	// missSlot[i] is the index into missTexts that position i resolves from,
	// or -1 when position i was a cache hit. Duplicate uncached texts share a
	// single slot so the inner Embedder sees each unique text once.
	missSlot := make([]int, len(texts))
	slotOf := make(map[string]int)
	var missTexts []string

	for i, t := range texts {
		if v, ok := c.lookup(t); ok {
			c.hits.Add(1)
			results[i] = v
			filled[i] = true
			missSlot[i] = -1
			continue
		}
		s, dup := slotOf[t]
		if !dup {
			s = len(missTexts)
			slotOf[t] = s
			missTexts = append(missTexts, t)
		}
		missSlot[i] = s
	}

	if len(missTexts) == 0 {
		return results, nil
	}

	// Record one miss per unique uncached text.
	c.misses.Add(int64(len(missTexts)))

	vectors, err := c.inner.EmbedBatch(ctx, missTexts)
	if err != nil {
		return nil, err
	}
	if len(vectors) != len(missTexts) {
		return nil, fmt.Errorf("embeddingclient: inner EmbedBatch returned %d vectors for %d texts", len(vectors), len(missTexts))
	}

	for s, t := range missTexts {
		c.store(t, vectors[s])
	}
	for i := range texts {
		if filled[i] {
			continue
		}
		results[i] = vectors[missSlot[i]]
	}
	return results, nil
}

// lookup checks the cache for text, refreshing its recency on a hit.
func (c *CachingEmbedder) lookup(text string) (Vector, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.cache[text]
	if !ok {
		return nil, false
	}
	c.lru.MoveToFront(el)
	return el.Value.(*cacheEntry).vec, true
}

// store writes a result into the cache, evicting the least-recently-used
// entries while over capacity.
func (c *CachingEmbedder) store(text string, v Vector) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.cache[text]; ok {
		el.Value.(*cacheEntry).vec = v
		c.lru.MoveToFront(el)
		return
	}
	c.cache[text] = c.lru.PushFront(&cacheEntry{key: text, vec: v})
	if c.maxEntries > 0 {
		for c.lru.Len() > c.maxEntries {
			back := c.lru.Back()
			c.lru.Remove(back)
			delete(c.cache, back.Value.(*cacheEntry).key)
		}
	}
}
