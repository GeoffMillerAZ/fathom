package mcp

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/dlog"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/hookcache"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
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
	log    *dlog.Logger
	headFn func(path string) string // injectable for tests; default = currentGitHead
	now    func() time.Time         // injectable clock for the failure backoff; nil = time.Now

	mu       sync.Mutex
	seen     map[string]string         // service -> commit successfully refreshed to
	inflight map[string]*inlineIngest  // service -> the detached ingest running now
	failures map[string]refreshFailure // service -> the last failed attempt's backoff

	// refreshTotal counts every auto-refresh re-ingest this backend has
	// triggered (O108, cmd/fathom's /metrics fathom_autorefresh_total) —
	// incremented once per ingest attempt for a debounced drift, regardless
	// of whether the ingest that followed succeeded (a failed attempt is
	// retried by a read after its backoff, RT-151, and counts again). Process-lifetime; read via
	// AutoRefreshTotal.
	refreshTotal int64 // atomic
}

// NewAutoRefresh returns a QueryInterface that transparently keeps each queried
// service fresh against its git HEAD. log receives a one-line note on each
// refresh; pass nil to silence it.
func NewAutoRefresh(backend QueryInterface, log *dlog.Logger) QueryInterface {
	return &autoRefreshBackend{
		QueryInterface: backend,
		log:            log,
		headFn:         currentGitHead,
		seen:           make(map[string]string),
		inflight:       make(map[string]*inlineIngest),
		failures:       make(map[string]refreshFailure),
	}
}

// refreshFailure is the backoff after a failed inline ingest of one
// (service, head): the next attempt waits until next. The wait doubles per
// consecutive failure from autoRefreshBackoffBase to autoRefreshBackoffMax.
type refreshFailure struct {
	head     string
	failures int
	next     time.Time
}

// autoRefreshBackoff is the wait after the n-th consecutive failure (n ≥ 1).
func autoRefreshBackoff(n int) time.Duration {
	d := autoRefreshBackoffBase
	for i := 1; i < n && d < autoRefreshBackoffMax; i++ {
		d *= 2
	}
	if d > autoRefreshBackoffMax {
		d = autoRefreshBackoffMax
	}
	return d
}

func (a *autoRefreshBackend) clock() time.Time {
	if a.now != nil {
		return a.now()
	}
	return time.Now()
}

// The failure backoff for inline auto-refresh ingests (RT-151 review fix): a
// failed (service, head) is retried after 30 s, then 60 s, 120 s …, capped
// at 30 min, never by every read. Until wave B's quarantine (RT-150) this is
// what keeps a persistent failure — the /Volumes TCC wedge fails every
// service at once — from turning hook traffic into a continuous ingest loop.
const (
	autoRefreshBackoffBase = 30 * time.Second
	autoRefreshBackoffMax  = 30 * time.Minute
)

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
//
// The ingest itself runs detached from the caller under one process-wide
// ticket (RT-149): only the read that starts it waits, and only while its own
// ctx lives, so a requester that disconnects returns at once and the ingest
// still completes. Every other read of the service while an ingest is in
// flight answers at once from the index as it stands (a stale answer, which
// staleness_report discloses as index_stale) — never one parked handler per
// concurrent read. seen[service] is recorded only after a successful ingest
// (RT-151); a failed (service, head) is retried after a backoff, not by
// every read.
func (a *autoRefreshBackend) ensure(ctx context.Context, service string) {
	if service == "" {
		return
	}
	// RT-34/RT-155 (design §2.1, §2.3): background work pauses at shed and
	// above, and a quarantined service is never re-queued until an
	// explicit ingest — the read answers from the index as it stands.
	if tenancy.CurrentLevel() >= tenancy.LevelShed || tenancy.IsQuarantined(service) {
		return
	}
	// RT-61 (design §3.3): a console-origin read — the page over HTTP or
	// the daemon's own in-process call — never triggers an auto-refresh.
	if IsConsoleRead(originFrom(ctx)) {
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
	if _, running := a.inflight[service]; running {
		// One inline ingest per service at a time; this read serves the
		// index as it stands. A newer head than the running ingest's is
		// picked up by the first read after that ingest finishes.
		a.mu.Unlock()
		return
	}
	if f, failed := a.failures[service]; failed && f.head == head && a.clock().Before(f.next) {
		a.mu.Unlock()
		return
	}
	fl := &inlineIngest{head: head, done: make(chan struct{})}
	a.inflight[service] = fl
	atomic.AddInt64(&a.refreshTotal, 1)
	if a.log != nil {
		a.log.Info(fmt.Sprintf("auto-refresh: %s drifted %s→%s, re-ingesting", service, shortSHA(indexed), shortSHA(head)))
	}
	// RT-149 (design §3.5): detached from the requester, so a disconnect
	// never cancels an ingest midway under a watermark that claims HEAD.
	go a.runInlineIngest(context.WithoutCancel(ctx), service, fl)
	a.mu.Unlock()
	select {
	case <-fl.done:
	case <-ctx.Done():
	}
}

// inlineIngest is one detached auto-refresh ingest in flight for a service.
type inlineIngest struct {
	head string
	done chan struct{} // closed when the ingest has finished, either way
}

// inlineIngestTicket is the one process-wide ingest ticket (RT-149, design
// §3.5/§7.1): every autoRefreshBackend's inline ingest holds it, so two
// services never ingest inline at once. It stands in for wave D's refresh
// queue heavy slot, which replaces the inline ingest entirely (RT-99).
var inlineIngestTicket = make(chan struct{}, 1)

// runInlineIngest waits for the process-wide ticket, ingests, and records
// seen[service] only after a successful ingest (RT-151): a failed ingest is
// retried by the first read after its backoff instead of being debounced
// forever.
func (a *autoRefreshBackend) runInlineIngest(ctx context.Context, service string, fl *inlineIngest) {
	inlineIngestTicket <- struct{}{}
	err := a.QueryInterface.IngestService(ctx, service)
	<-inlineIngestTicket

	// Critic finding F2 (BLOCKING, phase-P fix round): bust the CLI's
	// service-identity cache (cmd/fathom/identcache.go) on every
	// auto-refresh re-ingest too — a git-HEAD-drift re-ingest changes the
	// same answer a stale cache entry could otherwise keep serving for
	// up to the cache's TTL. Best-effort regardless of whether the
	// ingest above succeeded or failed (a failed ingest may still have
	// changed on-disk state worth invalidating against).
	hookcache.BustIdent()

	a.mu.Lock()
	if err == nil {
		a.seen[service] = fl.head
		delete(a.failures, service)
	} else {
		f := a.failures[service]
		if f.head != fl.head {
			f = refreshFailure{head: fl.head}
		}
		f.failures++
		wait := autoRefreshBackoff(f.failures)
		f.next = a.clock().Add(wait)
		a.failures[service] = f
		if a.log != nil {
			a.log.Warn(fmt.Sprintf("auto-refresh: %s ingest failed (attempt %d at %s; reads serve the index as it stands, next attempt in %s): %v",
				service, f.failures, shortSHA(fl.head), wait, err))
		}
	}
	if a.inflight[service] == fl {
		delete(a.inflight, service)
	}
	a.mu.Unlock()
	close(fl.done)
}

// AutoRefreshTotal returns the process-lifetime count of auto-refresh
// re-ingests this backend has triggered (O108). Consumed by cmd/fathom's
// /metrics as fathom_autorefresh_total via the optional
// autoRefreshStatsBackend capability (meta.go).
func (a *autoRefreshBackend) AutoRefreshTotal() int64 {
	return atomic.LoadInt64(&a.refreshTotal)
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

// ClaimsAnchoredTo forwards SU-21's governance-on-rename receipt (ADR-041
// decision 7) — same O7 forwarding class as every other optional
// capability in this file: without it, wrapping the client in auto-refresh
// would silently drop claims_anchored from rename_impact on a daemon
// running with auto-refresh on (serve-http's own default).
func (a *autoRefreshBackend) ClaimsAnchoredTo(ctx context.Context, serviceID, symbolKey string) ([]string, error) {
	cab, ok := a.QueryInterface.(claimsAnchoredBackend)
	if !ok {
		return nil, nil
	}
	return cab.ClaimsAnchoredTo(ctx, serviceID, symbolKey)
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
// DefaultIgnoredDirs forwards exclusionsBackend (RT-145) through the
// auto-refresh wrapper, the same forwarding class as IngestStats.
func (a *autoRefreshBackend) DefaultIgnoredDirs(service string) []string {
	if eb, ok := a.QueryInterface.(exclusionsBackend); ok {
		return eb.DefaultIgnoredDirs(service)
	}
	return nil
}

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

// ObligationsIndexCheck forwards RT-26's named-skip freshness check — the
// same O7 forwarding class as every optional capability in this file:
// without it the wrapper would strip index_unchecked on every real daemon.
func (a *autoRefreshBackend) ObligationsIndexCheck(ctx context.Context, service, file string) (bool, string) {
	if cb, ok := a.QueryInterface.(obligationsIndexCheckBackend); ok {
		return cb.ObligationsIndexCheck(ctx, service, file)
	}
	if ob, ok := a.QueryInterface.(obligationsBackend); ok {
		return ob.ObligationsIndexStale(ctx, service, file), ""
	}
	return false, ""
}

// ObligationsAsOf forwards SU-19's bi-temporal read (ADR-041 decision 7) —
// same O7 forwarding class as every other optional capability in this file:
// without it, wrapping the client in auto-refresh would silently make the
// obligations tool refuse every as_of argument on a daemon running with
// auto-refresh on (serve-http's own default). ensure()d like Obligations
// itself: an as_of answer should still run against a fresh index.
func (a *autoRefreshBackend) ObligationsAsOf(ctx context.Context, service, file string, maxRules int, asOf time.Time) (semantic.Obligations, error) {
	a.ensure(ctx, service)
	aob, ok := a.QueryInterface.(obligationsAsOfBackend)
	if !ok {
		return semantic.Obligations{}, errSemanticUnsupported
	}
	return aob.ObligationsAsOf(ctx, service, file, maxRules, asOf)
}

func (a *autoRefreshBackend) RootObligations(ctx context.Context, service string, maxRules int) (semantic.Obligations, error) {
	a.ensure(ctx, service)
	rb, ok := a.QueryInterface.(rootObligationsBackend)
	if !ok {
		return semantic.Obligations{}, errSemanticUnsupported
	}
	return rb.RootObligations(ctx, service, maxRules)
}

// ObligationsLayer forwards VB-9's layer-split file-scoped read (ADR-035
// decision 2) — same O7 forwarding class as Obligations/RootObligations
// above, and ensure()d the same way (a fresh index before reading a
// specific layer). Without this explicit forward, wrapping the client in
// auto-refresh would silently degrade every ObligationsLayer call back to
// obligationLayers' own pre-VB-9 fallback (the "all" union regardless of
// which layer the caller asked for), on any daemon running with
// auto-refresh on — serve-http's own default.
//
// Unlike every OTHER forward in this file, this method's own PRESENCE on
// the wrapper changes what obligationLayers (semantic_tools.go) sees:
// backend.(obligationLayerBackend) is a TYPE assertion on the SERVED
// backend, which is this wrapper when auto-refresh is on — so once this
// method exists at all, that assertion always succeeds, regardless of
// whether the INNER backend actually implements the layer split.
// obligationLayers treats a non-nil error from a backend it believes
// supports layers as a hard failure (never a fallback — a backend that
// predates the split is supposed to degrade silently, never error), so
// this method must replicate that SAME degrade here rather than ever
// return errSemanticUnsupported: an inner backend with no
// obligationLayerBackend falls back to plain Obligations (the pre-VB-9
// union), exactly as obligationLayers' own top-level fallback already
// does for an UNWRAPPED old backend — auto-refresh being on must never
// turn a graceful degrade into a hard error.
func (a *autoRefreshBackend) ObligationsLayer(ctx context.Context, service, file string, maxRules int, layer semantic.ObligationLayerFilter) (semantic.Obligations, error) {
	a.ensure(ctx, service)
	if lb, ok := a.QueryInterface.(obligationLayerBackend); ok {
		return lb.ObligationsLayer(ctx, service, file, maxRules, layer)
	}
	ob, ok := a.QueryInterface.(obligationsBackend)
	if !ok {
		return semantic.Obligations{}, errSemanticUnsupported
	}
	return ob.Obligations(ctx, service, file, maxRules)
}

// RootObligationsLayer is ObligationsLayer's root-scope sibling — same
// forward, same ensure(), and the SAME reason it must degrade to
// rootObligations (never errSemanticUnsupported) when the inner backend
// predates the layer split: see ObligationsLayer's own doc comment.
func (a *autoRefreshBackend) RootObligationsLayer(ctx context.Context, service string, maxRules int, layer semantic.ObligationLayerFilter) (semantic.Obligations, error) {
	a.ensure(ctx, service)
	if rb, ok := a.QueryInterface.(rootObligationsLayerBackend); ok {
		return rb.RootObligationsLayer(ctx, service, maxRules, layer)
	}
	ob, ok := a.QueryInterface.(obligationsBackend)
	if !ok {
		return semantic.Obligations{}, errSemanticUnsupported
	}
	return rootObligations(ctx, a.QueryInterface, ob, service, maxRules)
}

// ObligationsLayerCoverage forwards O54 deliverable 2's coverage
// disclosure (the compiled layer's latest compile-run completeness) —
// wave N15 item 5's own fix: THIS is the forward that was missing when
// ADR-037's Revisit-when trigger fired (wave O1's raw-section fallback
// shipped, but the operator daemon — auto-refresh on by default — never
// forwarded this capability at all, so it never emitted layer_coverage
// in the first place, regardless of the raw-fallback work). No ensure():
// mirrors ClaimsSummary below, a service-wide summary read, not a
// specific file's compiled content.
func (a *autoRefreshBackend) ObligationsLayerCoverage(ctx context.Context, service string) (semantic.LayerCoverage, bool, error) {
	cb, ok := a.QueryInterface.(layerCoverageBackend)
	if !ok {
		return semantic.LayerCoverage{}, false, nil
	}
	return cb.ObligationsLayerCoverage(ctx, service)
}

// ClaimFeedback forwards D2's refute/corroborate write path (the
// obligations tool's feedback action) — same O7 forwarding class as
// RemoveService/RenameService below: a write, never ensure()d (a verdict
// on an existing claim needs no fresh re-ingest first).
func (a *autoRefreshBackend) ClaimFeedback(ctx context.Context, service, claimID, action, note, author string) error {
	fb, ok := a.QueryInterface.(claimFeedbackBackend)
	if !ok {
		return fmt.Errorf("backend does not support claim feedback")
	}
	return fb.ClaimFeedback(ctx, service, claimID, action, note, author)
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

// RawFallbackObligations forwards wave O1's raw-section fallback capability
// (ADR-037) — same O7 forwarding class as every other optional semantic
// capability above: without this explicit forward, wrapping the client in
// auto-refresh (serve-http's default configuration) would silently strip
// the capability and obligations would degrade back to the layer_coverage
// note-only disclosure on every real daemon, exactly the class of bug this
// file's own doc comment names.
func (a *autoRefreshBackend) RawFallbackObligations(ctx context.Context, service, file string, max int) ([]semantic.Obligation, string, bool, error) {
	a.ensure(ctx, service)
	rfb, ok := a.QueryInterface.(rawFallbackBackend)
	if !ok {
		return nil, "", false, nil
	}
	return rfb.RawFallbackObligations(ctx, service, file, max)
}

// StoreWatermark forwards the optional storeWatermarker capability
// (resolved_nav.go) — same O7 forwarding class as IngestStats/TypedEdgeStatus
// above. Without it, serve-http's default configuration (auto-refresh ON,
// daemonAutoRefreshDefault) wraps every backend in autoRefreshBackend, whose
// embedded QueryInterface never promotes a method outside that interface's
// own set; serviceWatermark's type assertion then always fails, mark is
// always "", and the BP-28 watermark-keyed call-graph memo silently never
// caches anything — for the nav tools AND verify_plan (O33) alike, no matter
// how correctly their own callers are wired to the cache.
func (a *autoRefreshBackend) StoreWatermark(ctx context.Context, serviceID string) (string, error) {
	w, ok := a.QueryInterface.(storeWatermarker)
	if !ok {
		return "", fmt.Errorf("backend does not support store watermarks")
	}
	return w.StoreWatermark(ctx, serviceID)
}
