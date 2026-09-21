// Package mcp — memory_search (AM-8, docs/design/anchored-memory.md §4):
// anchored memories on the agent surface, every one labeled with the tier
// and state EvaluateState derived for it at read time. An unlabelled memory
// would be a claim fathom makes on its own authority; memorySearchTool
// refuses to return one rather than let it through.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/geoffmilleraz/fathom/internal/memory"
)

// memoryReaderBackend is the optional backend capability memory_search
// needs to evaluate anchor drift: the store slice memory.StoreIndex reads.
// pkg/client.Client implements it (MemoryReader); a test fake may implement
// memory.Reader directly instead. Without either the tool degrades
// honestly — a note, never an unlabelled memory.
type memoryReaderBackend interface {
	MemoryReader() memory.Reader
}

// memoryIndexFor builds a fresh anchor index for backend, or nil. Callers
// on the request path must NOT call this per tools/call — go through
// memoryIndexCache, which keeps one index per server so the StoreIndex's
// own per-(service, generation) symbol snapshot survives between calls.
func memoryIndexFor(backend QueryInterface) memory.Index {
	if backend == nil {
		return nil
	}
	if mb, ok := backend.(memoryReaderBackend); ok {
		if r := mb.MemoryReader(); r != nil {
			return memory.NewStoreIndex(r)
		}
	}
	if r, ok := backend.(memory.Reader); ok {
		return memory.NewStoreIndex(r)
	}
	return nil
}

// memoryIndexCache holds the server's one anchor index. A StoreIndex
// already caches its symbol snapshot per (service, generation) and
// rebuilds it exactly when the service's generation (its indexed
// watermark — the same notion DriftCache invalidates on) moves; building
// a fresh StoreIndex on every tools/call threw that snapshot away, so
// every file-scoped search (ForFile → AnchorTouchesFile → SymbolFile) and
// every symbol probe re-read the whole observation table. The cache keeps
// the first non-nil index for the life of the server: the backend is
// fixed at registration and pkg/client's MemoryReader is a stable store
// handle. A nil result (no reader yet) is not cached, so a backend that
// gains a reader later is not permanently degraded.
//
// Concurrency: safe for concurrent use via sync.Mutex; the StoreIndex it
// hands out is itself safe for concurrent use.
type memoryIndexCache struct {
	mu  sync.Mutex
	idx memory.Index
}

// get returns the cached index, building it from backend on first use.
func (c *memoryIndexCache) get(backend QueryInterface) memory.Index {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.idx == nil {
		c.idx = memoryIndexFor(backend)
	}
	return c.idx
}

// memorySearchDefaultLimit bounds a memory_search answer when the caller
// does not say; the surfacing order is deterministic, so a cut is always
// the lowest-ranked tail and `displaced` counts it.
const memorySearchDefaultLimit = 20

// memorySearchParams is memory_search's argument shape.
type memorySearchParams struct {
	Service string `json:"service"`
	// Query is the free-text filter: case-insensitive substring terms over
	// claim text and anchor keys. Empty matches every surfacing memory.
	Query string `json:"query"`
	// File narrows to memories whose anchors touch this repo-relative file
	// — the edit hook's injection scope (AM-6).
	File  string `json:"file"`
	Limit int    `json:"limit"`
}

// memorySearchEntry is one surfaced memory: the Evaluation (id, claim,
// tier, state, per-anchor drift, …) plus its anchor keys and the labeled
// line the edit hook renders.
type memorySearchEntry struct {
	memory.Evaluation
	Anchors []string `json:"anchors"`
	Line    string   `json:"line"`
}

// memorySearchResult is memory_search's answer.
type memorySearchResult struct {
	Service string `json:"service"`
	Query   string `json:"query,omitempty"`
	File    string `json:"file,omitempty"`
	// Total is the number of surfacing matches before Limit; Displaced is
	// how many the limit cut (present only when > 0) — the same shape the
	// obligations tool and the edit hook already speak. Elided is BA-1's
	// disclosure block for the same cut (count, of, ordering,
	// how_to_get_more); both describe one truncation.
	Total     int                 `json:"total"`
	Shown     int                 `json:"shown"`
	Displaced int                 `json:"displaced,omitempty"`
	Elided    *elided             `json:"elided,omitempty"`
	Note      string              `json:"note,omitempty"`
	Memories  []memorySearchEntry `json:"memories"`
}

// memorySearchOrdering names the surfacing order for BA-1's disclosure.
const memorySearchOrdering = "evidence tier (verified first), then anchor specificity (symbol > file > commit > claim > path), then id"

// memorySearchTool answers a memory_search over ms, evaluating drift
// against idx at read time with the injected clock. Every returned entry
// carries a non-empty tier and state; a retired memory never appears
// (memory.Search drops it before its anchors are even probed). Order is
// tier, then specificity, then id.
func memorySearchTool(ctx context.Context, ms memory.Store, idx memory.Index, cache *memory.DriftCache, now func() time.Time, p memorySearchParams) (memorySearchResult, error) {
	if idx == nil {
		return memorySearchResult{}, fmt.Errorf("memory_search: no anchor index — memory state cannot be evaluated")
	}
	limit := p.Limit
	if limit <= 0 {
		limit = memorySearchDefaultLimit
	}
	evs, err := memory.Search(ctx, ms, idx, cache, now, memory.SearchQuery{Service: p.Service, Query: p.Query, File: p.File})
	if err != nil {
		return memorySearchResult{}, fmt.Errorf("memory_search: %w", err)
	}
	res := memorySearchResult{Service: p.Service, Query: p.Query, File: p.File, Total: len(evs), Memories: []memorySearchEntry{}}
	for _, e := range evs {
		if e.Tier == "" || e.State == "" {
			// §9: fathom does not have the authority to state a memory
			// without saying how much to trust it.
			return memorySearchResult{}, fmt.Errorf("memory_search: refusing to surface memory %s without a tier and state", e.ID)
		}
		if e.State == memory.StateRetired {
			return memorySearchResult{}, fmt.Errorf("memory_search: refusing to surface retired memory %s", e.ID)
		}
		if len(res.Memories) >= limit {
			res.Displaced++
			continue
		}
		anchors := make([]string, 0, len(e.Drift))
		for _, d := range e.Drift {
			anchors = append(anchors, d.Anchor.Key())
		}
		res.Memories = append(res.Memories, memorySearchEntry{Evaluation: e, Anchors: anchors, Line: e.Line()})
	}
	res.Shown = len(res.Memories)
	res.Elided = elisionFor(res.Shown, res.Total, memorySearchOrdering,
		"re-call with a larger limit, or narrow query/file")
	if res.Total == 0 {
		res.Note = "no surfacing memories match"
	}
	return res, nil
}

// registerMemoryTools registers memory_search. The memories file is the
// service's own `<root>/.fathom/memories.jsonl` (root = the recorded
// corpus path, law 13: project scope governs); the anchor index comes from
// the backend's store slice. One drift cache AND one anchor index per
// server, both keyed by index generation, so a session that searches
// forty times hashes each anchor once and loads each service's symbol
// snapshot once per generation (§3.1).
func registerMemoryTools(s *Server, backend QueryInterface) {
	cache := memory.NewDriftCache()
	indexes := &memoryIndexCache{}
	s.registerTool(toolDescriptor{
		Name: "memory_search",
		// Every byte of this schema is paid by every connected session
		// (BA-7's schema tax): keep it terse; docs/mcp-tools.md carries
		// the full contract.
		Description: "Anchored memories bound to symbols/files/globs/commits, each with its evidence tier. Reach for it when you want what an earlier session recorded. Required: service.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{"type": "string", "description": "Service / repo identifier"},
				"query":   map[string]any{"type": "string", "description": "Substring terms over claim/anchor keys; empty = all"},
				"file":    map[string]any{"type": "string", "description": "Repo-relative file to scope to"},
				"limit":   map[string]any{"type": "integer", "description": "Max results (default 20)"},
			},
			"required": []string{"service"},
		},
		group: GroupShared,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p memorySearchParams
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "memory_search: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "memory_search: service required"}
		}
		root, _, found, err := backend.ServiceMeta(ctx, p.Service)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "memory_search: " + err.Error()}
		}
		if !found || root == "" {
			return memorySearchResult{
				Service: p.Service, Query: p.Query, File: p.File,
				Note:     "not measurable: service has no recorded corpus path (not indexed?) — memories live in <root>/.fathom/memories.jsonl",
				Memories: []memorySearchEntry{},
			}, nil
		}
		idx := indexes.get(backend)
		if idx == nil {
			return memorySearchResult{
				Service: p.Service, Query: p.Query, File: p.File,
				Note:     "not measurable: backend does not expose the anchor index, so memory state cannot be evaluated and no memory is surfaced unlabelled",
				Memories: []memorySearchEntry{},
			}, nil
		}
		res, err := memorySearchTool(ctx, memory.NewFileStore(memory.Path(root)), idx, cache, time.Now, p)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		return res, nil
	})
}
