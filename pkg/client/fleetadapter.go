package client

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/dlog"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// FleetAdapter exposes a single multi-service store client as the
// mcp.FleetBackend interface, so fleet tools (list_services,
// query_cross_service, fleet_drift, cross_repo_callers) work when one SQLite
// file holds many services — no per-service store directory required.
//
// The FleetBackend interface is synchronous and error-free by design; errors
// are reported through log (deduped per key, DG-2) and yield empty results
// rather than crashing the serve loop.
//
// It lives beside the client (moved from cmd/fathom, RT-93) so the daemon
// and the cost harness (internal/mcp/costtest) measure the same adapter.
type FleetAdapter struct {
	c   *Client
	log *dlog.Logger
}

// NewFleetAdapter adapts c, logging store errors (deduped) to log.
func NewFleetAdapter(c *Client, log *dlog.Logger) *FleetAdapter {
	return &FleetAdapter{c: c, log: log}
}

// SetLog replaces the adapter's logger.
func (f *FleetAdapter) SetLog(log *dlog.Logger) { f.log = log }

// ClassifyStoreError renders a store-layer error for logging, tagging the
// SQLite "unable to open the database file" (SQLITE_CANTOPEN, errno 14)
// class distinctly from a generic failure (DG-17). ADR-020 documents an
// intentionally uncapped read-connection pool for file-backed stores, which
// is architecturally consistent with several independent open() calls
// hitting a transient external condition at once (e.g. a brief TCC-verdict
// wedge on /Volumes, per this repo's own CLAUDE.md) — a human reading the
// (now singular, deduped — DG-2) line should not have to guess whether that
// is what happened versus a permanent failure.
//
// RT-41 (design §2.1): SQLITE_FULL and SQLITE_IOERR are store distress —
// recorded (tenancy.NoteStoreDistress), so ingest and writes are refused
// with the named reason — and a read that failed on one is rendered as that
// named error by its caller, never as a zero count (I38).
func ClassifyStoreError(err error) string {
	if err == nil {
		return ""
	}
	if kind := tenancy.StoreDistressKind(err); kind != "" {
		tenancy.NoteStoreDistress(kind, time.Now())
		return "store distress (" + kind + "): " + err.Error()
	}
	if strings.Contains(strings.ToUpper(err.Error()), "CANTOPEN") {
		return "store-locked (possibly transient, SQLITE_CANTOPEN): " + err.Error()
	}
	return err.Error()
}

func (f *FleetAdapter) Services() []string {
	ids, err := f.c.Services(context.Background())
	if err != nil {
		f.log.Dedupe(dlog.LevelWarn, "fleet:services", "fleet: services: %s", ClassifyStoreError(err))
		return nil
	}
	return ids
}

func (f *FleetAdapter) SymbolsFor(service string) []SymbolNode {
	nodes, err := f.c.ListAllSymbols(context.Background(), service)
	if err != nil {
		f.log.Dedupe(dlog.LevelWarn, "fleet:symbols:"+service, "fleet: symbols for %s: %s", service, ClassifyStoreError(err))
		return nil
	}
	return nodes
}

// SymbolAggregatesFor answers list_services' symbol count and languages
// from client.ServiceSymbolAggregates — an aggregate cached per watermark
// and single-flighted, never a symbol load (RT-144). ok is false on a store
// error (logged, deduped), which list_services reports as zero symbols,
// exactly as a failed SymbolsFor always read.
func (f *FleetAdapter) SymbolAggregatesFor(service string) (int, []core.SourceKind, bool) {
	n, kinds, err := f.SymbolAggregatesErr(service)
	return n, kinds, err == nil
}

// SymbolAggregatesErr is SymbolAggregatesFor with the store error named
// (RT-41): list_services renders it instead of a zero count (I38).
func (f *FleetAdapter) SymbolAggregatesErr(service string) (int, []core.SourceKind, error) {
	agg, err := f.c.ServiceSymbolAggregates(context.Background(), service)
	if err != nil {
		named := ClassifyStoreError(err)
		f.log.Dedupe(dlog.LevelWarn, "fleet:aggregates:"+service, "fleet: symbol aggregates for %s: %s", service, named)
		return 0, nil, fmt.Errorf("%s", named)
	}
	return agg.Count, agg.Kinds, nil
}

func (f *FleetAdapter) ObservationsFor(service string) []Observation {
	obs, err := f.c.Observations(context.Background(), service)
	if err != nil {
		f.log.Dedupe(dlog.LevelWarn, "fleet:observations:"+service, "fleet: observations for %s: %s", service, ClassifyStoreError(err))
		return nil
	}
	return obs
}

// CallersOfKeys answers cross-service callers from the store's call index
// (RT-84): one indexed query, never a whole-service load. A store error is
// logged (deduped) and returned; fleetCallers then falls back to the scan.
func (f *FleetAdapter) CallersOfKeys(ctx context.Context, keys []string) ([]store.CallerRow, error) {
	rows, err := f.c.CallersOfKeys(ctx, keys)
	if err != nil {
		f.log.Dedupe(dlog.LevelWarn, "fleet:callers-of-keys", "fleet: callers of keys: %s", ClassifyStoreError(err))
	}
	return rows, err
}

// CallsIndexed reports whether service's call index is complete
// (DerivedState.CallsIndexed); a service that is not keeps the scan.
func (f *FleetAdapter) CallsIndexed(ctx context.Context, service string) bool {
	st, err := f.c.DerivedState(ctx, service)
	return err == nil && st.CallsIndexed
}

// SymbolSourcesForNames resolves declarations by exact name over the
// (service_id, symbol) index (RT-84's cross-service callees).
func (f *FleetAdapter) SymbolSourcesForNames(ctx context.Context, service string, names []string) ([]store.SymbolSource, error) {
	return f.c.SymbolSourcesForNames(ctx, service, names)
}

// StoredSummary answers list_services from the service's service_summary
// row (RT-79, design §4.4): written at ingest, backfilled by SQL at first
// use, never a symbol load or a walk. A row not yet derived is a named
// absence; a store error is named too (RT-41, I38) — the count is unknown,
// never zero.
func (f *FleetAdapter) StoredSummary(ctx context.Context, service string) (store.ServiceSummary, error) {
	sum, err := f.c.ServiceSummary(ctx, service)
	if err == nil || errors.Is(err, ErrSummaryPending) {
		return sum, err
	}
	named := ClassifyStoreError(err)
	f.log.Dedupe(dlog.LevelWarn, "fleet:summary:"+service, "fleet: summary for %s: %s", service, named)
	return store.ServiceSummary{}, fmt.Errorf("%s", named)
}

// UnsupportedLanguagesFor returns, by language display name, the count of
// files for service whose extension names a known programming language
// fathom has no registered extractor for (O69) — mcp.FleetBackend's
// contract. RT-79: read from the service's summary row (classified by file
// NAME at ingest, or at backfill from the fingerprinted paths), never a
// walk. nil when nothing is unsupported or the summary is not yet derived.
func (f *FleetAdapter) UnsupportedLanguagesFor(service string) map[string]int {
	return f.UnsupportedLanguagesForCtx(context.Background(), service)
}

// UnsupportedLanguagesForCtx is UnsupportedLanguagesFor under the
// request's ctx. Satisfies mcp's optional fleetCtxBackend.
func (f *FleetAdapter) UnsupportedLanguagesForCtx(ctx context.Context, service string) map[string]int {
	sum, err := f.StoredSummary(ctx, service)
	if err != nil {
		return nil
	}
	return sum.UnsupportedLangs
}

// RootMissing reports whether service's recorded corpus root (ServiceMeta.
// Path, set at ingest time) no longer exists on disk, plus the operator
// remedy — DG-18: list_services surfaces this as a structured
// "root_missing": true field. One os.Stat of the recorded root (RT-79: no
// walk). Satisfies the optional mcp.fleetRootStatusBackend interface.
func (f *FleetAdapter) RootMissing(service string) (missing bool, remedy string) {
	return f.RootMissingCtx(context.Background(), service)
}

// RootMissingCtx is RootMissing under the request's ctx.
func (f *FleetAdapter) RootMissingCtx(ctx context.Context, service string) (missing bool, remedy string) {
	path, _, found, err := f.c.ServiceMeta(ctx, service)
	if err != nil || !found || path == "" {
		// No recorded root (a remote/faux corpus), or an unreadable meta
		// row: not "missing" — nothing was recorded to go missing.
		return false, ""
	}
	if _, statErr := os.Stat(path); statErr == nil || !os.IsNotExist(statErr) {
		// A transient stat error (the /Volumes TCC wedge) is not "missing".
		return false, ""
	}
	f.log.Dedupe(dlog.LevelWarn, "fleet:root-missing:"+service, "fleet: recorded root for %s no longer exists: %s", service, path)
	return true, `service's recorded corpus root no longer exists on disk; drop the stale record with "fathom fleet" (the remove_service MCP tool)`
}

// Client is the client the adapter reads.
func (f *FleetAdapter) Client() *Client { return f.c }
