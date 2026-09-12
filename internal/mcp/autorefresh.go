package mcp

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/hookcache"
	"github.com/geoffmilleraz/fathom/internal/semantic"
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
	// Critic finding F2 (BLOCKING, phase-P fix round): bust the CLI's
	// service-identity cache (cmd/fathom/identcache.go) on every
	// auto-refresh re-ingest too — a git-HEAD-drift re-ingest changes the
	// same answer a stale cache entry could otherwise keep serving for
	// up to the cache's TTL. Best-effort regardless of whether the
	// ingest above succeeded or failed (a failed ingest may still have
	// changed on-disk state worth invalidating against).
	hookcache.BustIdent()
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

func (a *autoRefreshBackend) TypedEdges(ctx context.Context, serviceID string) ([]core.Edge, error) {
	a.ensure(ctx, serviceID)
	return a.QueryInterface.TypedEdges(ctx, serviceID)
}

// ─── forwarded optional capabilities ─────────────────────────────────────────
//
// Optional-capability interfaces (semantic tools, claims summary) are detected
// by type assertion on the SERVED backend — and interface embedding does not
// forward methods outside QueryInterface's method set, so without these
// explicit forwards, wrapping the client in auto-refresh silently stripped
// every semantic capability from the daemon (dogfood O7). Obligations and
// compile additionally get ensure() so claims are checked against a fresh
// index and a fresh citation sweep.

var errSemanticUnsupported = fmt.Errorf("backend does not serve semantic layers")

// ingestAtBackend is the optional backend capability for onboarding a new
// service from an explicit corpus root (the ingest tool's `path` argument).
type ingestAtBackend interface {
	IngestServiceAt(ctx context.Context, service, path string) error
}

func (a *autoRefreshBackend) IngestServiceAt(ctx context.Context, service, path string) error {
	ia, ok := a.QueryInterface.(ingestAtBackend)
	if !ok {
		return fmt.Errorf("backend does not support onboarding by path")
	}
	return ia.IngestServiceAt(ctx, service, path)
}

// registrySurgeryBackend is the optional capability behind the
// remove_service / rename_service verbs (IO-3): both were asked for
// verbatim on 2026-06-16 and every interim surgery was hand-run sqlite
// under the daemon lock.
type registrySurgeryBackend interface {
	RemoveService(ctx context.Context, serviceID string) (bool, error)
	RenameService(ctx context.Context, oldID, newID string) error
}

func (a *autoRefreshBackend) RemoveService(ctx context.Context, serviceID string) (bool, error) {
	rs, ok := a.QueryInterface.(registrySurgeryBackend)
	if !ok {
		return false, fmt.Errorf("backend does not support registry surgery")
	}
	return rs.RemoveService(ctx, serviceID)
}

func (a *autoRefreshBackend) RenameService(ctx context.Context, oldID, newID string) error {
	rs, ok := a.QueryInterface.(registrySurgeryBackend)
	if !ok {
		return fmt.Errorf("backend does not support registry surgery")
	}
	return rs.RenameService(ctx, oldID, newID)
}

// IngestStats forwards the ADR-021 §2 / TI-6 process-lifetime
// unclassified/oversize counters (see statsBackend in meta.go). Without this
// explicit forward, wrapping the client in auto-refresh would silently hide
// the counters from server_stats/ingest — the same class of bug ingestAtBackend
// documents above (dogfood O7).
func (a *autoRefreshBackend) IngestStats() (unclassified, oversize int64) {
	sb, ok := a.QueryInterface.(statsBackend)
	if !ok {
		return 0, 0
	}
	return sb.IngestStats()
}

// ClassifiedZeroStats forwards the per-tool classified-but-zero counters
// (see classifiedZeroBackend in meta.go) — same O7 forwarding class.
func (a *autoRefreshBackend) ClassifiedZeroStats() map[string]int64 {
	cz, ok := a.QueryInterface.(classifiedZeroBackend)
	if !ok {
		return nil
	}
	return cz.ClassifiedZeroStats()
}

// TypedEdgeStatus forwards the per-service typed-edge pass outcome (BP-35,
// see typedEdgeStatusBackend in meta.go) — the same O7 forwarding class:
// without it, wrapping the client in auto-refresh would make every
// staleness_report say "unknown" on a daemon that knows perfectly well.
func (a *autoRefreshBackend) TypedEdgeStatus(service string) (fusion.TypedEdgeStatus, bool) {
	teb, ok := a.QueryInterface.(typedEdgeStatusBackend)
	if !ok {
		return fusion.TypedEdgeStatus{}, false
	}
	return teb.TypedEdgeStatus(service)
}

func (a *autoRefreshBackend) Obligations(ctx context.Context, service, file string, maxRules int) (semantic.Obligations, error) {
	a.ensure(ctx, service)
	ob, ok := a.QueryInterface.(obligationsBackend)
	if !ok {
		return semantic.Obligations{}, errSemanticUnsupported
	}
	return ob.Obligations(ctx, service, file, maxRules)
}

func (a *autoRefreshBackend) ObligationsIndexStale(ctx context.Context, service, file string) bool {
	ob, ok := a.QueryInterface.(obligationsBackend)
	if !ok {
		return false
	}
	return ob.ObligationsIndexStale(ctx, service, file)
}

func (a *autoRefreshBackend) CompileLayer(ctx context.Context, service, layer string, dryRun bool) (core.CompileRun, error) {
	a.ensure(ctx, service)
	cb, ok := a.QueryInterface.(compileBackend)
	if !ok {
		return core.CompileRun{}, errSemanticUnsupported
	}
	return cb.CompileLayer(ctx, service, layer, dryRun)
}

func (a *autoRefreshBackend) ClaimsSummary(ctx context.Context, service string) (map[string]any, error) {
	cs, ok := a.QueryInterface.(claimsSummaryBackend)
	if !ok {
		return nil, errSemanticUnsupported
	}
	return cs.ClaimsSummary(ctx, service)
}
