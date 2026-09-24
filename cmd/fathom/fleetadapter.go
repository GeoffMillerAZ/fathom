package main

import (
	"context"
	"os"
	"strings"
	"sync"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/dlog"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// fleetAdapter exposes a single multi-service store client as the
// mcp.FleetBackend interface, so fleet tools (list_services,
// query_cross_service, fleet_drift, cross_repo_callers) work when one SQLite
// file holds many services — no per-service store directory required.
//
// The FleetBackend interface is synchronous and error-free by design; errors
// are reported through log (deduped per key, DG-2) and yield empty results
// rather than crashing the serve loop.
type fleetAdapter struct {
	c   *client.Client
	log *dlog.Logger

	unsupportedMu    sync.Mutex
	unsupportedCache map[string]unsupportedLangCacheEntry
}

// unsupportedLangCacheEntry is UnsupportedLanguagesFor's per-service memo
// (DG-18). langs is nil (never an empty non-nil map) when nothing is
// unsupported OR the root is missing; rootMissing distinguishes "recorded
// root no longer exists on disk" from "genuinely nothing unsupported".
type unsupportedLangCacheEntry struct {
	watermark   string
	langs       map[string]int
	rootMissing bool
}

// classifyStoreError renders a store-layer error for logging, tagging the
// SQLite "unable to open the database file" (SQLITE_CANTOPEN, errno 14)
// class distinctly from a generic failure (DG-17). ADR-020 documents an
// intentionally uncapped read-connection pool for file-backed stores, which
// is architecturally consistent with several independent open() calls
// hitting a transient external condition at once (e.g. a brief TCC-verdict
// wedge on /Volumes, per this repo's own CLAUDE.md) — a human reading the
// (now singular, deduped — DG-2) line should not have to guess whether that
// is what happened versus a permanent failure.
func classifyStoreError(err error) string {
	if err == nil {
		return ""
	}
	if strings.Contains(strings.ToUpper(err.Error()), "CANTOPEN") {
		return "store-locked (possibly transient, SQLITE_CANTOPEN): " + err.Error()
	}
	return err.Error()
}

func (f *fleetAdapter) Services() []string {
	ids, err := f.c.Services(context.Background())
	if err != nil {
		f.log.Dedupe(dlog.LevelWarn, "fleet:services", "fleet: services: %s", classifyStoreError(err))
		return nil
	}
	return ids
}

func (f *fleetAdapter) SymbolsFor(service string) []client.SymbolNode {
	nodes, err := f.c.ListAllSymbols(context.Background(), service)
	if err != nil {
		f.log.Dedupe(dlog.LevelWarn, "fleet:symbols:"+service, "fleet: symbols for %s: %s", service, classifyStoreError(err))
		return nil
	}
	return nodes
}

func (f *fleetAdapter) ObservationsFor(service string) []client.Observation {
	obs, err := f.c.Observations(context.Background(), service)
	if err != nil {
		f.log.Dedupe(dlog.LevelWarn, "fleet:observations:"+service, "fleet: observations for %s: %s", service, classifyStoreError(err))
		return nil
	}
	return obs
}

// UnsupportedLanguagesFor returns, by language display name, the count of
// files for service whose extension names a known programming language
// fathom has no registered extractor for (O69,
// connector.UnsupportedLanguageForPath) — mcp.FleetBackend's contract.
// Returns nil on a fetch error or when nothing is unsupported.
//
// It is a thin wrapper over unsupportedLanguagesCached (DG-18); see that
// method for why this no longer re-walks the service's source tree on
// every call.
func (f *fleetAdapter) UnsupportedLanguagesFor(service string) map[string]int {
	langs, _ := f.unsupportedLanguagesCached(context.Background(), service)
	return langs
}

// RootMissing reports whether service's recorded corpus root (ServiceMeta.
// Path, set at ingest time) no longer exists on disk, plus the operator
// remedy — DG-18: list_services surfaces this as a structured
// "root_missing": true field instead of an error line, since the live
// filesystem re-walk that used to discover it as a bare error is exactly
// the cost this method avoids. Satisfies the optional
// mcp.fleetRootStatusBackend interface.
func (f *fleetAdapter) RootMissing(service string) (missing bool, remedy string) {
	_, missing = f.unsupportedLanguagesCached(context.Background(), service)
	if !missing {
		return false, ""
	}
	return true, `service's recorded corpus root no longer exists on disk; drop the stale record with "fathom fleet" (the remove_service MCP tool)`
}

// unsupportedLanguagesCached is UnsupportedLanguagesFor/RootMissing's cache
// function (DG-18 — .fathom/observations.md / diagnostics sweep): list_services
// was the fleet's most-called MCP tool (~31k calls measured, p95 4.6s, max
// 182s) precisely because THIS method called client.Sources (a live
// connector.Discover filesystem walk) for every registered service on
// every single list_services call, and pkg/client.Client.Sources falls
// back to the bare service ID as its path whenever ServiceMeta has none,
// guaranteeing a permanent lstat failure for any service whose recorded
// corpus root vanished.
//
// The fix: memoize the result per (service, StoreWatermark(service)).
// StoreWatermark moves only when an ingest has replaced the service's
// stored data (a single indexed_commit/indexed_at row read — cheap, not a
// walk), so a burst of read-path calls between ingests costs exactly one
// real computation per service, never a re-walk to answer a read. A
// service whose recorded root has vanished is detected with a single
// os.Stat (not a full Discover) and cached as rootMissing=true rather than
// ever calling Sources/Discover for it at all.
func (f *fleetAdapter) unsupportedLanguagesCached(ctx context.Context, service string) (langs map[string]int, rootMissing bool) {
	watermark, wmErr := f.c.StoreWatermark(ctx, service)

	if wmErr == nil {
		f.unsupportedMu.Lock()
		if e, ok := f.unsupportedCache[service]; ok && e.watermark == watermark {
			f.unsupportedMu.Unlock()
			return e.langs, e.rootMissing
		}
		f.unsupportedMu.Unlock()
	}

	path, _, found, metaErr := f.c.ServiceMeta(ctx, service)
	if metaErr != nil {
		f.log.Dedupe(dlog.LevelWarn, "fleet:sources:"+service, "fleet: sources for %s: %s", service, classifyStoreError(metaErr))
		return nil, false
	}
	if !found || path == "" {
		// No recorded root at all (e.g. a remote/faux-mode service with no
		// filesystem corpus) — distinct from rootMissing, which names a
		// root that WAS recorded and is now gone. Nothing to walk either
		// way, so cache the negative and move on.
		f.storeUnsupportedCache(service, watermark, wmErr, nil, false)
		return nil, false
	}
	if _, statErr := os.Stat(path); statErr != nil {
		if os.IsNotExist(statErr) {
			f.log.Dedupe(dlog.LevelWarn, "fleet:root-missing:"+service, "fleet: recorded root for %s no longer exists: %s", service, path)
			f.storeUnsupportedCache(service, watermark, wmErr, nil, true)
			return nil, true
		}
		// A transient stat error (e.g. the TCC-verdict wedge this repo's
		// own CLAUDE.md documents for /Volumes) is not "missing" — fall
		// through and let Sources itself surface/dedupe it below, same as
		// before this change.
	}

	files, err := f.c.Sources(ctx, service)
	if err != nil {
		f.log.Dedupe(dlog.LevelWarn, "fleet:sources:"+service, "fleet: sources for %s: %s", service, classifyStoreError(err))
		// Cache the failure too (nil langs), keyed on the SAME watermark:
		// a persistent, non-root-missing failure (e.g. a transient
		// connector error) must not re-walk on every read either — it
		// gets one Discover attempt per watermark, same as success, and
		// the next ingest (a new watermark) is what earns it a retry.
		f.storeUnsupportedCache(service, watermark, wmErr, nil, false)
		return nil, false
	}
	var out map[string]int
	for _, sf := range files {
		if sf.Lang != core.LangUnclassified {
			continue
		}
		lang, ok := connector.UnsupportedLanguageForPath(sf.Path)
		if !ok {
			continue
		}
		if out == nil {
			out = make(map[string]int)
		}
		out[lang]++
	}
	f.storeUnsupportedCache(service, watermark, wmErr, out, false)
	return out, false
}

// storeUnsupportedCache records the computed result keyed by watermark.
// When StoreWatermark itself failed (wmErr != nil, watermark == ""), the
// result is deliberately NOT cached — an empty watermark is not a key
// (pkg/client.Client.StoreWatermark's own doc comment), so every call would
// otherwise collide on "" and one service's answer could be served for
// another's, or a transient store error could pin a stale answer forever.
func (f *fleetAdapter) storeUnsupportedCache(service, watermark string, wmErr error, langs map[string]int, rootMissing bool) {
	if wmErr != nil {
		return
	}
	f.unsupportedMu.Lock()
	defer f.unsupportedMu.Unlock()
	if f.unsupportedCache == nil {
		f.unsupportedCache = make(map[string]unsupportedLangCacheEntry)
	}
	f.unsupportedCache[service] = unsupportedLangCacheEntry{watermark: watermark, langs: langs, rootMissing: rootMissing}
}
