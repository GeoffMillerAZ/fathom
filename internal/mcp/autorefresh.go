package mcp

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// autoRefreshBackend wraps a QueryInterface and, before each per-service read,
// re-ingests the service when its git HEAD has drifted from the indexed commit.
//
// This removes the manual staleness→ingest loop that dominates real sessions:
// queries are always answered against the current commit. Re-ingest is
// hash-gated (near-free when the tree is clean) and debounced per (service,
// commit), so a given drift triggers at most one ingest no matter how many
// queries follow. Only the per-service read methods are intercepted; fleet and
// metadata methods pass through unchanged.
type autoRefreshBackend struct {
	QueryInterface
	log    io.Writer
	headFn func(path string) string // injectable for tests; default = currentGitHead

	mu   sync.Mutex
	seen map[string]string // service -> commit already refreshed to
}

// NewAutoRefresh returns a QueryInterface that transparently keeps each queried
// service fresh against its git HEAD. log receives a one-line note on each
// refresh; pass nil to silence it.
func NewAutoRefresh(backend QueryInterface, log io.Writer) QueryInterface {
	return &autoRefreshBackend{
		QueryInterface: backend,
		log:            log,
		headFn:         currentGitHead,
		seen:           make(map[string]string),
	}
}

// serviceWatermarker is the optional backend capability for domain-neutral
// freshness (ADR-017): the connector's current corpus watermark for a service.
// pkg/client implements it; test fakes may omit it.
type serviceWatermarker interface {
	ServiceWatermark(ctx context.Context, service string) (string, error)
}

// ensure re-ingests service when its corpus has drifted from the indexed
// watermark. Git-backed corpora compare HEAD SHAs (cheap, no corpus read);
// non-git corpora fall back to the connector's content-addressed watermark —
// costlier (one corpus read), acceptable because non-git corpora (docs
// folders, findings exports) are small by nature. Best-effort: any failure to
// determine freshness leaves the index as-is (a stale answer is strictly
// better than a failed query).
func (a *autoRefreshBackend) ensure(ctx context.Context, service string) {
	if service == "" {
		return
	}
	path, indexed, found, err := a.QueryInterface.ServiceMeta(ctx, service)
	if err != nil || !found || path == "" {
		return
	}
	head := a.headFn(path)
	if head == "" {
		if w, ok := a.QueryInterface.(serviceWatermarker); ok {
			head, _ = w.ServiceWatermark(ctx, service)
		}
	}
	if head == "" || head == indexed {
		return
	}
	a.mu.Lock()
	if a.seen[service] == head {
		a.mu.Unlock()
		return
	}
	a.seen[service] = head
	a.mu.Unlock()

	if a.log != nil {
		fmt.Fprintf(a.log, "auto-refresh: %s drifted %s→%s, re-ingesting\n", service, shortSHA(indexed), shortSHA(head))
	}
	if err := a.QueryInterface.IngestService(ctx, service); err != nil && a.log != nil {
		fmt.Fprintf(a.log, "auto-refresh: %s ingest failed: %v\n", service, err)
	}
}

func shortSHA(s string) string {
	if s == "" {
		return "(none)"
	}
	if len(s) > 7 {
		return s[:7]
	}
	return s
}

// ─── intercepted per-service reads ───────────────────────────────────────────

func (a *autoRefreshBackend) QuerySymbols(ctx context.Context, q core.Query) (core.QueryResult, error) {
	a.ensure(ctx, q.ServiceID)
	return a.QueryInterface.QuerySymbols(ctx, q)
}

func (a *autoRefreshBackend) FindCallers(ctx context.Context, serviceID, symbol string) ([]core.SymbolNode, error) {
	a.ensure(ctx, serviceID)
	return a.QueryInterface.FindCallers(ctx, serviceID, symbol)
}

func (a *autoRefreshBackend) FindSymbol(ctx context.Context, serviceID, name string) ([]core.SymbolNode, error) {
	a.ensure(ctx, serviceID)
	return a.QueryInterface.FindSymbol(ctx, serviceID, name)
}

func (a *autoRefreshBackend) ListAllSymbols(ctx context.Context, serviceID string) ([]core.SymbolNode, error) {
	a.ensure(ctx, serviceID)
	return a.QueryInterface.ListAllSymbols(ctx, serviceID)
}

func (a *autoRefreshBackend) Sources(ctx context.Context, service string) ([]core.SourceFile, error) {
	a.ensure(ctx, service)
	return a.QueryInterface.Sources(ctx, service)
}
