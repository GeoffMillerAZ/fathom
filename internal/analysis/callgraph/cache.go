package callgraph

import (
	"sync"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// defaultCacheCapacity is how many services' graphs one cache keeps. Eight
// is a fleet's worth of hot services, not a memory budget derived from
// measurement: the entry that matters is the one for the service the current
// session is working in, and the LRU exists so a fleet-wide sweep cannot
// pin every service's graph in the daemon at once.
const defaultCacheCapacity = 8

// CachedCallGraph memoizes built graphs per (service, store watermark)
// (bench-pyramid BP-28, invariant I21).
//
// The watermark is the STORE's, not the tree's: it changes on every ingest
// and only on an ingest, so a hit can never serve a graph built from
// observations the store has since replaced. A miss on a service whose
// watermark moved EVICTS the stale entry rather than keeping both — a graph
// is derived data with exactly one correct version, and holding the previous
// ingest's copy would make "which graph answered" a question the caller
// cannot see. That eviction is the whole of I21's "never across ingests".
//
// Why it exists: impact.BlastRadiusWithTypedEdges rebuilt the entire call
// graph on every call, which on this repository's ~10.6k symbols cost ~9.7s
// per blast_radius query (the L0 self oracle's first run, docs/bench/
// README.md). Nothing about that work is per-query.
//
// Safe for concurrent use. A *Graph handed out by Graph() is READ-ONLY for
// the caller: it is shared with every other holder of the same watermark, so
// AddTypedEdges on it would mutate an answer someone else is reading.
type CachedCallGraph struct {
	mu  sync.Mutex
	cap int
	// entries is ordered least-recently-used first. A linear scan over at
	// most cap entries is cheaper than the map+list bookkeeping an O(1) LRU
	// needs at this size.
	entries []cacheEntry
	hits    int
	misses  int
}

type cacheEntry struct {
	service   string
	watermark string
	graph     *Graph
}

// NewCachedCallGraph returns a cache holding at most capacity services'
// graphs. A capacity of zero or less uses the default (8).
func NewCachedCallGraph(capacity int) *CachedCallGraph {
	if capacity <= 0 {
		capacity = defaultCacheCapacity
	}
	return &CachedCallGraph{cap: capacity}
}

// Graph returns the memoized graph for (service, watermark), calling build
// exactly once per distinct pair. An EMPTY watermark is never cached: the
// caller could not name the ingest its nodes came from, so there is no key
// under which a later hit would be honest — build runs and the result is
// handed back uncached.
func (c *CachedCallGraph) Graph(service, watermark string, build func() *Graph) *Graph {
	if c == nil || watermark == "" {
		return build()
	}
	c.mu.Lock()
	for i, e := range c.entries {
		if e.service != service {
			continue
		}
		if e.watermark == watermark {
			// Hit: promote to most-recently-used.
			c.entries = append(append(c.entries[:i:i], c.entries[i+1:]...), e)
			c.hits++
			c.mu.Unlock()
			return e.graph
		}
		// Same service, different watermark: the store was re-ingested.
		// Drop the stale graph before building the new one.
		c.entries = append(c.entries[:i:i], c.entries[i+1:]...)
		break
	}
	c.misses++
	c.mu.Unlock()

	g := build()

	c.mu.Lock()
	defer c.mu.Unlock()
	// Re-check: a concurrent caller may have inserted the same key while
	// build ran. Prefer the entry already published so every holder of this
	// watermark shares one graph.
	for i, e := range c.entries {
		if e.service == service && e.watermark == watermark {
			c.entries = append(append(c.entries[:i:i], c.entries[i+1:]...), e)
			return e.graph
		}
	}
	c.entries = append(c.entries, cacheEntry{service: service, watermark: watermark, graph: g})
	if len(c.entries) > c.cap {
		c.entries = c.entries[len(c.entries)-c.cap:]
	}
	return g
}

// Invalidate drops any memoized graph for service. Callers that KNOW an
// ingest just happened need not wait for the watermark comparison.
func (c *CachedCallGraph) Invalidate(service string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	kept := c.entries[:0]
	for _, e := range c.entries {
		if e.service != service {
			kept = append(kept, e)
		}
	}
	c.entries = kept
}

// Stats reports the cache's size and its process-lifetime hit/miss counts —
// the numbers that say whether the memo is doing anything, rather than an
// assurance that it is.
func (c *CachedCallGraph) Stats() (entries, hits, misses int) {
	if c == nil {
		return 0, 0, 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries), c.hits, c.misses
}

// BuildBoth is the build function most callers want under a cache: the
// syntax graph plus the typed pass's edges, exactly what
// impact.BlastRadiusWithTypedEdges and the find_callers depth annotation
// walk today.
func BuildBoth(nodes []core.SymbolNode, typed []core.Edge) *Graph {
	g := Build(nodes)
	if len(typed) > 0 {
		AddTypedEdges(g, typed)
	}
	return g
}
