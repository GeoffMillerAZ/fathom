package mcp

import (
	"context"
	"errors"
	"fmt"
	"github.com/geoffmilleraz/fathom/internal/analysis/serviceref"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/corpus"
	"github.com/geoffmilleraz/fathom/internal/dlog"
	"github.com/geoffmilleraz/fathom/internal/docindex"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/hookcache"
	"github.com/geoffmilleraz/fathom/internal/refresh"
	"github.com/geoffmilleraz/fathom/internal/scip"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// autoRefreshBackend wraps a QueryInterface and, before each per-service read,
// asks for a re-ingest when the service's git HEAD has drifted from the
// indexed commit.
//
// RT-99 (design §7.1): the re-ingest is a request to the refresh queue
// (internal/refresh), never an ingest on the request path — the read answers
// at once from the index as it stands and says so (the envelope's
// "refresh" block: queued or refreshing, the indexed commit and HEAD). The
// queue runs it under the daemon's context in its one heavy slot. A drift is
// requested once per (service, commit); only the per-service read methods
// are intercepted; fleet and metadata methods pass through unchanged.
type autoRefreshBackend struct {
	QueryInterface
	log    *dlog.Logger
	headFn func(ctx context.Context, path string) string // injectable for tests; default = currentGitHead
	now    func() time.Time                              // injectable clock for the failure backoff; nil = time.Now
	// ownerAlive reports whether the process that began an ingest (pid,
	// at started) is still running — RT-109's liveness check; nil = the
	// process-wide probe (SetIngestOwnerProbe).
	ownerAlive func(pid int, started time.Time) bool
	// queue runs the re-ingests (RT-97/RT-99): the daemon's shared queue,
	// or one this backend started for itself.
	queue *refresh.Queue

	mu       sync.Mutex
	seen     map[string]string         // service -> commit successfully refreshed to
	pending  map[string]string         // service -> the newest head requested and not yet finished
	failures map[string]refreshFailure // service -> the last failed attempt's backoff
	// resumed is the interrupted ingest_state row (owner pid and start)
	// this backend already re-ingested per service: one re-ingest per
	// interrupted row, even when the row cannot be rewritten afterwards.
	resumed map[string]ingestOwner

	// refreshTotal counts every auto-refresh re-ingest this backend has
	// requested (O108, cmd/fathom's /metrics fathom_autorefresh_total) —
	// incremented once per requested drift, regardless of whether the
	// ingest that followed succeeded (a failed attempt is retried by a read
	// after its backoff, RT-151, and counts again). Process-lifetime; read
	// via AutoRefreshTotal.
	refreshTotal int64 // atomic
}

// NewAutoRefresh returns a QueryInterface that keeps each queried service
// fresh against its git HEAD through a refresh queue of its own (started
// for the process's lifetime, with no post-start wait: that belongs to a
// daemon's start). log receives a one-line note on each refresh; pass nil
// to silence it. The daemon shares one queue with the ingest tool and its
// routes: NewAutoRefreshQueued.
func NewAutoRefresh(backend QueryInterface, log *dlog.Logger) QueryInterface {
	//fathom:tenancy-allow bg-ctx NewAutoRefresh starts a process-lifetime refresh queue; daemons use NewAutoRefreshQueued with the drain's context
	ar, _ := newAutoRefreshUntil(context.Background(), backend, log)
	return ar
}

// newAutoRefreshUntil is NewAutoRefresh with its queue's worker stopped when
// ctx ends (the queue is returned so a caller can Wait for it).
func newAutoRefreshUntil(ctx context.Context, backend QueryInterface, log *dlog.Logger) (QueryInterface, *refresh.Queue) {
	q := NewRefreshQueueLogged(backend, refresh.Config{StartWait: -1}, log)
	q.Start(ctx)
	return NewAutoRefreshQueued(backend, log, q), q
}

// NewAutoRefreshQueued is NewAutoRefresh over the process's refresh queue
// q (NewRefreshQueue, started by the caller).
func NewAutoRefreshQueued(backend QueryInterface, log *dlog.Logger, q *refresh.Queue) QueryInterface {
	return &autoRefreshBackend{
		QueryInterface: backend,
		log:            log,
		headFn:         currentGitHead,
		queue:          q,
		seen:           make(map[string]string),
		pending:        make(map[string]string),
		failures:       make(map[string]refreshFailure),
		resumed:        make(map[string]ingestOwner),
	}
}

// NewRefreshQueue returns a refresh queue whose jobs ingest through backend
// (its IngestServiceAt for a job carrying an onboarding path). cfg.Run is
// set here; the caller starts it under the daemon's lifetime context.
func NewRefreshQueue(backend QueryInterface, cfg refresh.Config) *refresh.Queue {
	return NewRefreshQueueLogged(backend, cfg, nil)
}

// NewRefreshQueueLogged is NewRefreshQueue with every job's bookkeeping
// done on the queue's worker, whoever asked and whether or not anyone waits
// (I38): SQLITE_FULL/SQLITE_IOERR is noted as store distress (RT-41), the
// CLI's service-identity cache is busted (the O15 class), and a failure is
// logged to log (nil = silent) and left on the queue (Queue.LastFailure,
// staleness_report's refresh_failure) — a post-commit /refresh-request
// answers 202 before its ingest runs, so its failure has no caller to
// return to.
func NewRefreshQueueLogged(backend QueryInterface, cfg refresh.Config, log *dlog.Logger) *refresh.Queue {
	cfg.Run = func(ctx context.Context, job refresh.Job) error {
		var ia ingestAtBackend
		if job.Path != "" {
			var ok bool
			if ia, ok = backend.(ingestAtBackend); !ok {
				return fmt.Errorf("backend does not support onboarding by path")
			}
		}
		// RT-160: every queued ingest holds tenancy's one process-wide
		// ingest slot, so a derived-table backfill (serving start-up,
		// list_services' first read) never runs beside it and writes a
		// summary with a mixed count.
		release, err := tenancy.AcquireIngestSlot(ctx)
		if err != nil {
			return err
		}
		if ia != nil {
			err = ia.IngestServiceAt(ctx, job.Service, job.Path)
		} else {
			err = backend.IngestService(ctx, job.Service)
		}
		release()
		tenancy.NoteStoreError(err, time.Now())
		hookcache.BustIdent()
		// §5: the superseded snapshot and graphs leave now, not when the
		// service is next read at its new watermark.
		evictServiceCaches(backend, job.Service)
		if err != nil && log != nil && !onlyAuto(job.Reasons) {
			// An auto-refresh failure is logged by refreshDone with its
			// backoff; every other job's failure is logged here.
			log.Warn(fmt.Sprintf("refresh: %s ingest failed (%s); reads serve the index as it stands: %v", job.Service, joinReasons(job.Reasons), err))
		}
		return err
	}
	return refresh.New(cfg)
}

// onlyAuto reports whether a job serves auto-refresh alone.
func onlyAuto(rs []refresh.Reason) bool {
	return len(rs) == 1 && rs[0] == refresh.ReasonAuto
}

func joinReasons(rs []refresh.Reason) string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = string(r)
	}
	return strings.Join(out, ", ")
}

// RefreshQueue is the queue this backend's re-ingests run on.
func (a *autoRefreshBackend) RefreshQueue() *refresh.Queue { return a.queue }

// refreshFailure is the backoff after a failed inline ingest of one
// (service, head): the next attempt waits until next. The wait doubles per
// consecutive failure from autoRefreshBackoffBase to autoRefreshBackoffMax.
type refreshFailure struct {
	head     string
	failures int
	next     time.Time
	err      string // the last attempt's error, for the stale read's note
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

// freshnessProber is the request path's freshness capability (RT-110,
// I48): pkg/client's ServiceFreshnessProbe, which never reads the corpus.
// Preferred over serviceWatermarker when the backend has both.
type freshnessProber interface {
	ServiceFreshnessProbe(ctx context.Context, service string) (string, error)
}

// ensure asks for a re-ingest of service when its corpus has drifted from
// the indexed watermark, and returns at once (RT-99, design §7.1): the read
// is answered from the index as it stands, and the envelope's "refresh"
// block says the service is queued or refreshing, with the indexed commit
// and HEAD. Git-backed corpora compare HEAD SHAs (read from the ref files,
// no fork — RT-101); non-git corpora ask the connector's watermark (a
// capped stat manifest, RT-110). Best-effort: any failure to determine
// freshness leaves the index as-is.
//
// The re-ingest is a request to the refresh queue, which runs it under the
// daemon's context in its one heavy slot (RT-97): a requester's disconnect
// never cancels it, and no read waits for it. A drift is requested once per
// (service, head); a newer head while one is pending becomes the queue's
// one follow-up. seen[service] is recorded only after a successful ingest
// (RT-151); a failed (service, head) is retried after a backoff, not by
// every read.
func (a *autoRefreshBackend) ensure(ctx context.Context, service string) {
	if service == "" {
		return
	}
	// RT-150 (design §7.1, §2.1, §2.3): no inline ingest at shed and
	// above, in safe mode, during warm-up after a start, or for a
	// quarantined service (never re-queued until an explicit ingest). The
	// read answers from the index as it stands and the envelope says why
	// (refreshnotes.go). Decided before any drift check, so a gated read
	// costs no git or corpus read.
	if reason := tenancy.RefreshDeferralNow(service, a.clock()); reason != "" {
		noteRefreshDeferred(ctx, service, reason)
		return
	}
	// RT-39/RT-40/RT-41 (§2.3, §2.1): no inline ingest in safe mode,
	// during warm-up or in store distress (RT-150 adds the stale answer's
	// disclosure).
	if ok, _ := tenancy.InlineIngestAllowed(a.clock()); !ok {
		return
	}
	// RT-61 (design §3.3): a console-origin read — the page over HTTP or
	// the daemon's own in-process call — never triggers an auto-refresh.
	if IsConsoleRead(originFrom(ctx)) {
		return
	}
	path, indexed, found, err := a.QueryInterface.ServiceMeta(ctx, service)
	if err != nil {
		return
	}
	// RT-109 (design §6): an ingest another process began and did not
	// finish — killed or halted, its owner no longer running — means the
	// index is not the one its watermark describes: re-ingest even with no
	// drift, and even at a HEAD this backend already refreshed to (the
	// seen debounce below describes this process's ingests, not another's).
	// One point read of the row per ensure.
	interrupted, owner, rowPath := a.interruptedIngest(ctx, service)
	if interrupted {
		a.mu.Lock()
		if a.resumed[service] == owner {
			interrupted = false // already re-ingested once
		}
		a.mu.Unlock()
	}
	if !found || path == "" {
		// A first ingest killed before its final commit recorded no
		// service_meta; its row names the corpus root.
		if !interrupted || rowPath == "" {
			return
		}
		path, indexed = rowPath, ""
	}
	head := a.headFn(ctx, path)
	if head == "" {
		// RT-110/I48: the request path probes (stats, cached) — never the
		// content manifest, which reads the whole corpus.
		if p, ok := a.QueryInterface.(freshnessProber); ok {
			head, _ = p.ServiceFreshnessProbe(ctx, service)
		} else if w, ok := a.QueryInterface.(serviceWatermarker); ok {
			head, _ = w.ServiceWatermark(ctx, service)
		}
	}
	if head == "" || (head == indexed && !interrupted) {
		return
	}
	a.queue.NoteQueried(service)
	a.mu.Lock()
	if !interrupted && a.seen[service] == head {
		a.mu.Unlock()
		return
	}
	if p, ok := a.pending[service]; ok && p == head && !interrupted {
		// Already requested: this read serves the index as it stands. A
		// newer head than the pending one is requested again below — the
		// queue coalesces it into the queued job or its one follow-up.
		a.mu.Unlock()
		noteRefreshQueued(ctx, service, a.queue.StateOf(service), indexed, head)
		return
	}
	if f, failed := a.failures[service]; failed && f.head == head && a.clock().Before(f.next) {
		a.mu.Unlock()
		// I38/I4: served from the stale index like any drifted read, so
		// the envelope says so — and that the refresh failed.
		noteRefreshDeferred(ctx, service, fmt.Sprintf("index_stale — served from the index at %s; HEAD is %s; the last refresh failed (%s) and is retried after %s",
			shortSHA(indexed), shortSHA(head), f.err, f.next.UTC().Format(time.RFC3339)))
		return
	}
	a.pending[service] = head
	if interrupted {
		if a.resumed == nil {
			a.resumed = make(map[string]ingestOwner)
		}
		a.resumed[service] = owner
	}
	atomic.AddInt64(&a.refreshTotal, 1)
	a.mu.Unlock()
	if a.log != nil {
		a.log.Info(fmt.Sprintf("auto-refresh: %s drifted %s→%s, refresh requested", service, shortSHA(indexed), shortSHA(head)))
	}
	st := a.queue.RequestRefresh(ctx, refresh.Request{
		Service: service,
		Reason:  refresh.ReasonAuto,
		Done:    func(err error) { a.refreshDone(service, head, err) },
	}, false)
	if st.State == refresh.StateRefused {
		noteRefreshDeferred(ctx, service, "deferred ("+st.Refused+") — served from the index as it stands")
		return
	}
	state := st.State
	if st.Paused != "" {
		noteRefreshDeferred(ctx, service, "queued, "+st.Paused+" — served from the index at "+shortSHA(indexed)+" (HEAD "+shortSHA(head)+")")
		return
	}
	noteRefreshQueued(ctx, service, state, indexed, head)
}

// ingestStateBackend is the optional backend capability behind RT-109:
// the service's ingest_state row (pkg/client implements it).
type ingestStateBackend interface {
	IngestState(ctx context.Context, service string) (row store.IngestState, found bool, err error)
}

// interruptedIngest reports whether service's last ingest was left
// in_progress by another process that is no longer running — killed or
// halted before its final commit — and the corpus root that ingest read.
// A row this process wrote is its own ingest's (a failure here is retried
// by the drift and backoff rules), and a row whose owner is still running
// is a live concurrent ingest (a CLI ingest into this store): re-ingesting
// beside it would double the memory this program bounds (review finding
// 3), so both are not interruptions.
func (a *autoRefreshBackend) interruptedIngest(ctx context.Context, service string) (bool, ingestOwner, string) {
	isb, ok := a.QueryInterface.(ingestStateBackend)
	if !ok {
		return false, ingestOwner{}, ""
	}
	row, found, err := isb.IngestState(ctx, service)
	if err != nil || !found || row.State != store.IngestInProgress || row.PID == os.Getpid() {
		return false, ingestOwner{}, ""
	}
	alive := a.ownerAlive
	if alive == nil {
		alive = loadIngestOwnerProbe()
	}
	if alive(row.PID, time.Unix(0, row.StartedAt)) {
		return false, ingestOwner{}, ""
	}
	return true, ingestOwner{pid: row.PID, started: row.StartedAt}, row.Path
}

// ingestOwner identifies one ingest_state row's ingest: its pid and start.
type ingestOwner struct {
	pid     int
	started int64
}

// ingestOwnerProbe is the process-wide liveness check behind
// interruptedIngest. The default is kill(pid, 0) alone; cmd/fathom
// installs one that also rejects a reused pid (a process that started
// after the row was written).
var ingestOwnerProbe atomic.Pointer[func(pid int, started time.Time) bool]

// SetIngestOwnerProbe installs the liveness check RT-109's interrupted-
// ingest rule uses: whether pid, recorded as starting an ingest at
// started, is that same process and still running. nil restores the
// default (kill(pid, 0)).
func SetIngestOwnerProbe(fn func(pid int, started time.Time) bool) {
	if fn == nil {
		ingestOwnerProbe.Store(nil)
		return
	}
	ingestOwnerProbe.Store(&fn)
}

func loadIngestOwnerProbe() func(int, time.Time) bool {
	if p := ingestOwnerProbe.Load(); p != nil {
		return *p
	}
	return pidRunning
}

// pidRunning is kill(pid, 0): nil or EPERM means a process holds the pid.
func pidRunning(pid int, _ time.Time) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// refreshDone records the result of the queued re-ingest requested for
// (service, head): seen[service] only after success (RT-151), a doubling
// backoff after a failure, never for a refusal (the gate said "not now",
// not "failed"). Called on the queue's worker.
func (a *autoRefreshBackend) refreshDone(service, head string, err error) {
	// Store distress (RT-41) and the identity-cache bust (critic finding
	// F2) are the queue's own per-job bookkeeping (NewRefreshQueueLogged),
	// done for every job whoever requested it.

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.pending[service] == head {
		delete(a.pending, service)
	}
	switch {
	case err == nil:
		a.seen[service] = head
		delete(a.failures, service)
	case errors.Is(err, refresh.ErrRefused):
		// Not a failure: the next read after the gate lifts asks again.
	default:
		f := a.failures[service]
		if f.head != head {
			f = refreshFailure{head: head}
		}
		f.failures++
		f.err = err.Error()
		if len(f.err) > 200 {
			f.err = f.err[:200] + "…"
		}
		wait := autoRefreshBackoff(f.failures)
		f.next = a.clock().Add(wait)
		a.failures[service] = f
		if a.log != nil {
			a.log.Warn(fmt.Sprintf("auto-refresh: %s ingest failed (attempt %d at %s; reads serve the index as it stands, next attempt in %s): %v",
				service, f.failures, shortSHA(head), wait, err))
		}
	}
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
	//fathom:tenancy-allow sources autorefresh's QueryInterface pass-through, which only forwards the call; the memo loader is the one caller
	return a.QueryInterface.ListAllSymbols(ctx, serviceID)
}

func (a *autoRefreshBackend) ScanSources(ctx context.Context, service string, b corpus.Budget, fn func(core.SourceFile) bool) (corpus.Coverage, error) {
	a.ensure(ctx, service)
	return a.QueryInterface.ScanSources(ctx, service, b, fn)
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

func (a *autoRefreshBackend) ImportSCIP(ctx context.Context, serviceID, path string) (scip.Import, error) {
	sb, ok := a.QueryInterface.(scipImportBackend)
	if !ok {
		return scip.Import{}, fmt.Errorf("backend does not import SCIP indexes")
	}
	return sb.ImportSCIP(ctx, serviceID, path)
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

// CompileLayerState forwards the compile_layer state (GL-27, see
// compileStateBackend in meta.go) — same O7 forwarding class. A wrapped
// backend without the capability answers all zero, which status omits.
func (a *autoRefreshBackend) CompileLayerState() (enabled, configured bool, reason string) {
	cs, ok := a.QueryInterface.(compileStateBackend)
	if !ok {
		return false, false, ""
	}
	return cs.CompileLayerState()
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

// ReviewObligations forwards GL-5's review-audience serve, degrading to
// plain Obligations (review rules absent) for an inner backend without it —
// the same graceful degrade ObligationsLayer keeps above.
func (a *autoRefreshBackend) ReviewObligations(ctx context.Context, service, file string, maxRules int) (semantic.Obligations, error) {
	a.ensure(ctx, service)
	if rb, ok := a.QueryInterface.(reviewObligationsBackend); ok {
		return rb.ReviewObligations(ctx, service, file, maxRules)
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

// ClaimsForService forwards the service's claims with their validity windows
// (GL-17): findings_near, review_diff's certifications and every dated
// `as_of` answer read them through claimsListBackend/findingsBackend/
// certificationsBackend, and without this forward the wrapper (serve-http's
// default) stripped the capability — the same O7 class as every other
// optional capability in this file — so those verbs degraded on a real
// daemon. A service-wide read, never ensure()d, like ClaimsSummary.
func (a *autoRefreshBackend) ClaimsForService(ctx context.Context, serviceID, layer string) ([]core.SemanticClaim, error) {
	cb, ok := a.QueryInterface.(claimsListBackend)
	if !ok {
		return nil, errSemanticUnsupported
	}
	return cb.ClaimsForService(ctx, serviceID, layer)
}

// The review findings bridge's claim writes (EW-80, O1072): review_diff
// persists its findings through reviewClaimsBackend, and without these
// forwards the wrapper (every daemon's default) stripped the capability —
// the O7 class again — so every daemon answered findings_persisted:false.
// Writes, never ensure()d; a wrapped backend without the capability
// refuses by name.
func (a *autoRefreshBackend) reviewClaims() (reviewClaimsBackend, error) {
	rcb, ok := a.QueryInterface.(reviewClaimsBackend)
	if !ok {
		return nil, fmt.Errorf("backend does not persist review claims")
	}
	return rcb, nil
}

func (a *autoRefreshBackend) PutClaims(ctx context.Context, claims []core.SemanticClaim, links []core.ClaimLink) error {
	rcb, err := a.reviewClaims()
	if err != nil {
		return err
	}
	return rcb.PutClaims(ctx, claims, links)
}

func (a *autoRefreshBackend) SetClaimStatus(ctx context.Context, serviceID, claimID string, status core.ClaimStatus) error {
	rcb, err := a.reviewClaims()
	if err != nil {
		return err
	}
	return rcb.SetClaimStatus(ctx, serviceID, claimID, status)
}

func (a *autoRefreshBackend) SourceFingerprint(ctx context.Context, serviceID, path string) (string, bool, error) {
	rcb, err := a.reviewClaims()
	if err != nil {
		return "", false, err
	}
	return rcb.SourceFingerprint(ctx, serviceID, path)
}

func (a *autoRefreshBackend) SetClaimValidTo(ctx context.Context, serviceID, claimID string, t time.Time) error {
	rcb, err := a.reviewClaims()
	if err != nil {
		return err
	}
	return rcb.SetClaimValidTo(ctx, serviceID, claimID, t)
}

func (a *autoRefreshBackend) WriteSupersedes(ctx context.Context, serviceID, successorID, predecessorID, why string) error {
	rcb, err := a.reviewClaims()
	if err != nil {
		return err
	}
	return rcb.WriteSupersedes(ctx, serviceID, successorID, predecessorID, why)
}

// SearchDocs, SearchPkgDocs and DocLinkDrift forward the doc index's reads
// (EW-80's class scan found them stripped the same way): search kind
// "docs" answered "not available on this backend" on every daemon, and
// staleness_report left doc_link_drift out. SearchDocs is a per-service
// read and ensure()s like QuerySymbols; the package-docs and drift reads
// are not, like DocLinksAsOf.
func (a *autoRefreshBackend) SearchDocs(ctx context.Context, service, query string, concernSymbols []string, limit int) ([]docindex.RankedDoc, error) {
	a.ensure(ctx, service)
	db, ok := a.QueryInterface.(docsSearchBackend)
	if !ok {
		return nil, fmt.Errorf("backend does not search docs")
	}
	return db.SearchDocs(ctx, service, query, concernSymbols, limit)
}

func (a *autoRefreshBackend) SearchPkgDocs(ctx context.Context, pkgKey, query string, limit int) (docindex.PkgDocsResult, error) {
	db, ok := a.QueryInterface.(pkgDocsSearchBackend)
	if !ok {
		return docindex.PkgDocsResult{}, fmt.Errorf("backend does not search package docs")
	}
	return db.SearchPkgDocs(ctx, pkgKey, query, limit)
}

func (a *autoRefreshBackend) DocLinkDrift(ctx context.Context, service string) ([]docindex.DriftedDocLink, error) {
	db, ok := a.QueryInterface.(docDriftBackend)
	if !ok {
		return nil, errSemanticUnsupported
	}
	return db.DocLinkDrift(ctx, service)
}

// DocLinksAsOf forwards GL-16's dated doc-link read for staleness_report's
// `as_of`: without it the wrapper would strip the capability on a real
// daemon and every dated staleness answer would name doc links unavailable.
func (a *autoRefreshBackend) DocLinksAsOf(ctx context.Context, service string, at time.Time) ([]store.DocLinkView, error) {
	lb, ok := a.QueryInterface.(docLinksAsOfBackend)
	if !ok {
		return nil, errSemanticUnsupported
	}
	return lb.DocLinksAsOf(ctx, service, at)
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
// Store forwards the wrapped backend's store (conserveBackend, O1068).
// Every daemon serves through this wrapper, so without the forward each
// store-backed capability read "no store" there while `--store` read the
// same index: verify_plan's runtime_pass and gone: rows, review_diff's
// conservation and integrity dimensions, test_impact's risk rank. nil when
// the wrapped backend holds no store; storeOf reads that as absent.
func (a *autoRefreshBackend) Store() store.Store {
	if sb, ok := a.QueryInterface.(conserveBackend); ok {
		return sb.Store()
	}
	return nil
}

func (a *autoRefreshBackend) StoreWatermark(ctx context.Context, serviceID string) (string, error) {
	w, ok := a.QueryInterface.(storeWatermarker)
	if !ok {
		return "", fmt.Errorf("backend does not support store watermarks")
	}
	return w.StoreWatermark(ctx, serviceID)
}

// EnsureFresh runs the auto-refresh freshness check for service — what a
// symbol load through this backend runs first — for the graph store's
// snapshot path (resolved_nav.go navScope), whose cache hits never reach
// ListAllSymbols.
func (a *autoRefreshBackend) EnsureFresh(ctx context.Context, service string) {
	a.ensure(ctx, service)
}

// SuppressionToken forwards the wrapped backend's suppression token
// (RT-75): the snapshot key's suppression segment.
func (a *autoRefreshBackend) SuppressionToken(service string) string {
	if t, ok := a.QueryInterface.(suppressionTokener); ok {
		return t.SuppressionToken(service)
	}
	return ""
}

// StoreID forwards the wrapped backend's store identity (RT-74): without
// it the MCP tools and the console would key the one graph store under
// two identities for one store. "" (a wrapped backend that names no
// store) is never cached.
func (a *autoRefreshBackend) StoreID() string {
	if id, ok := a.QueryInterface.(storeIdentifier); ok {
		return id.StoreID()
	}
	return ""
}

// SourcePaths forwards the wrapped backend's path lister (RT-71): without
// it a paths-only question would fall back to a budgeted content scan.
func (a *autoRefreshBackend) SourcePaths(ctx context.Context, service string) ([]string, error) {
	a.ensure(ctx, service)
	return sourcePaths(ctx, a.QueryInterface, service)
}

// ReadSourceFiles forwards the wrapped backend's named-file reader
// (RT-71, RT-86), falling back to a budgeted scan keeping those paths.
func (a *autoRefreshBackend) ReadSourceFiles(ctx context.Context, service string, paths []string) ([]core.SourceFile, error) {
	a.ensure(ctx, service)
	return sourceFiles(ctx, a.QueryInterface, service, paths), nil
}

// ServiceRefsFor forwards the wrapped backend's ingest-extracted routes
// and client calls (RT-89); a backend without them has none extracted.
func (a *autoRefreshBackend) ServiceRefsFor(ctx context.Context, service string) (store.ServiceRefs, bool, error) {
	if rr, ok := a.QueryInterface.(serviceref.RefReader); ok {
		a.ensure(ctx, service)
		return rr.ServiceRefsFor(ctx, service)
	}
	return store.ServiceRefs{}, false, nil
}
