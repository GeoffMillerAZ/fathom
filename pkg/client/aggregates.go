package client

// aggregates.go — RT-144 (docs/design/responsible-tenant.md §4.4): the
// symbol count and source kinds list_services reports per service, from a
// SQL aggregate (store.SourceKindCounts) instead of a whole-service symbol
// load — cached per store watermark and single-flighted, so a burst of cold
// callers costs one aggregate per service per ingest. RT-79 replaces the
// aggregate with a service_summary row.

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// SymbolAggregates is one service's symbol count and the distinct source
// kinds its symbols carry (sorted).
type SymbolAggregates struct {
	Count int
	Kinds []core.SourceKind
}

type aggregateEntry struct {
	watermark string
	agg       SymbolAggregates
}

type aggregateCall struct {
	done chan struct{}
	agg  SymbolAggregates
	err  error
}

// aggregateMemo is the per-Client cache ServiceSymbolAggregates keeps.
type aggregateMemo struct {
	mu     sync.Mutex
	cache  map[string]aggregateEntry
	flight map[string]*aggregateCall
}

// DropHotPathMemos drops the obligations hot path's memo — the layer
// corpus's paths and guide contents, and what was derived from each edited
// file (hash, behaviour tokens) — at the tenancy ladder's refuse rung
// (RT-34: refuse drops every cache). In-flight walks keep their flight
// entries and store into the fresh maps; the next call re-reads.
func (c *Client) DropHotPathMemos() {
	m := &c.oblMemo
	m.mu.Lock()
	m.paths = nil
	m.edited = nil
	m.mu.Unlock()
}

// DropCaches drops the client's fleet memo (the per-service symbol
// aggregates) at the tenancy ladder's shed rung (RT-34).
func (c *Client) DropCaches() {
	c.aggregates.mu.Lock()
	c.aggregates.cache = nil
	c.aggregates.mu.Unlock()
}

// ServiceSymbolAggregates returns service's symbol count and source kinds
// from an aggregate query — never loading an observation row. The answer
// is memoized per StoreWatermark(service); concurrent callers that miss
// the cache share one query. A service whose watermark cannot be read is
// answered uncached (an empty watermark is not a key).
func (c *Client) ServiceSymbolAggregates(ctx context.Context, service string) (SymbolAggregates, error) {
	watermark, wmErr := c.StoreWatermark(ctx, service)
	m := &c.aggregates

	m.mu.Lock()
	if wmErr == nil {
		if e, ok := m.cache[service]; ok && e.watermark == watermark {
			m.mu.Unlock()
			return e.agg, nil
		}
	}
	if call, ok := m.flight[service]; ok {
		m.mu.Unlock()
		select {
		case <-call.done:
			return call.agg, call.err
		case <-ctx.Done():
			return SymbolAggregates{}, ctx.Err()
		}
	}
	if m.flight == nil {
		m.flight = make(map[string]*aggregateCall)
	}
	call := &aggregateCall{done: make(chan struct{})}
	m.flight[service] = call
	m.mu.Unlock()

	call.agg, call.err = c.computeSymbolAggregates(ctx, service)

	m.mu.Lock()
	delete(m.flight, service)
	if call.err == nil && wmErr == nil {
		if m.cache == nil {
			m.cache = make(map[string]aggregateEntry)
		}
		m.cache[service] = aggregateEntry{watermark: watermark, agg: call.agg}
	}
	m.mu.Unlock()
	close(call.done)
	return call.agg, call.err
}

func (c *Client) computeSymbolAggregates(ctx context.Context, service string) (SymbolAggregates, error) {
	rows, err := c.st.SourceKindCounts(ctx, service)
	if err != nil {
		return SymbolAggregates{}, fmt.Errorf("client.ServiceSymbolAggregates: %w", err)
	}
	var agg SymbolAggregates
	kinds := map[core.SourceKind]bool{}
	for _, r := range rows {
		agg.Count += r.Count
		kinds[r.Kind] = true
	}
	for k := range kinds {
		agg.Kinds = append(agg.Kinds, k)
	}
	sort.Slice(agg.Kinds, func(i, j int) bool { return agg.Kinds[i] < agg.Kinds[j] })
	return agg, nil
}
