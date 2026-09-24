// Typed Go call-edge enrichment (ADR-021 §3, docs/plans/callgraph-types.md).
//
// EnrichTypedEdges is an ADDITIVE pass that runs after the syntax-only
// extraction extractAndStore already performed: it loads a Go service's
// packages with golang.org/x/tools/go/packages (full type info), resolves
// each call site's target via go/types, and persists core.Edge rows through
// store.PutTypedEdges — it never touches SymbolNode.Calls / Observation.Calls,
// the syntax-derived contract callgraph.Build and internal/verify's baseline
// test_reaches already rely on.
//
// It dissolves the O9 ambiguity residue documented at
// internal/analysis/callgraph/graph.go:156-242 (the honest-refusal rules for
// an overloaded method name): a call through a concrete receiver resolves to
// EXACTLY the compiler's target (EdgeDirect — go/types already resolves
// embedded-field promotion and generic instantiation down to the underlying
// declaration for us, so no special-casing is needed for either); a call
// through an interface-typed receiver resolves to one EdgeIfaceDispatch edge
// PER in-scope implementer (never a single ambiguous edge — sum the edges
// sharing a From to get "reached via any of N"); anything else (func values,
// method values, higher-order calls) is recorded explicitly as
// EdgeUnresolved, never silently dropped.
//
// Scope (ADR-021 §3, CT-7 deferred): per-service, not cross-repo. Only edges
// whose target resolves to a symbol WITHIN the same service's own loaded
// packages are emitted — a call into an external/stdlib dependency has no
// corresponding SymbolNode in the index (goextract never extracted one), so
// emitting an edge to it would create a dangling reference the store could
// never resolve. The syntax-only Calls field already has the same scope
// discipline (callgraph.resolve simply finds nothing for such a callee).
//
// Test-variant type identity: golang.org/x/tools/go/packages, with
// Tests:true, gives a package with _test.go files up to three entries: the
// plain package, an internal test-augmented variant ("pkg [pkg.test]"), and
// an external test-augmented variant ("pkg_test [pkg.test]") — each is an
// INDEPENDENT type-checking pass, so a *types.Named gathered from one
// variant is not comparable via types.Implements against a *types.Interface
// from another. Implementer candidates are therefore gathered from the
// union of (a) every PLAIN package's own scope (the only variant every other
// package can actually import) and (b) — only when resolving a dispatch
// call site inside a test-augmented package — that same variant's own
// scope, so mock types declared in _test.go files are considered too. A
// cross-package interface declared only in one package's _test.go and
// implemented only in another package's _test.go is the one case this
// deliberately does not cover (documented, not silently wrong: it simply
// yields fewer implementer edges, never an incorrect one).
//
// Caching: per Go package (import path) content-hash — sha256 of the
// package's repo-relative file names + contents, sorted, so the cache key
// is portable across machines/worktrees (ADR-017 born-portable ethos). A
// cache hit skips the (expensive) resolve entirely for that package; its
// previously stored typed edges are left untouched. A cache miss replaces
// exactly that package's rows (store.PutTypedEdges), never the whole
// service's. After the package loop, a RECONCILE pass diffs the store's
// typed-edge package list against the packages this load produced and
// purges rows for packages that no longer exist (deleted or `git mv`-ed
// directories) — without it, a stale edge row whose From/To keys are still
// live symbols (a moved package keeps its package NAME, hence its
// SymbolKeys) would survive re-ingest forever as fabricated test_reaches /
// blast_radius evidence. The reconcile is skipped only on wholesale failure
// (every package failed to load — indistinguishable from a transient
// toolchain breakage, so the fallback rows are retained).
//
// Connector scope: EnrichTypedEdges takes the connector's allowed-file set
// (the Discover result the pipeline already holds). packages.Load("./...")
// knows nothing about .fathomignore / connector exclusions, so without the
// filter an ignored directory's Go file whose package/func names collide
// with an indexed package would emit edges keyed onto LIVE symbols. A
// package with zero allowed files is skipped entirely (and reconciled away
// if previously stored); an edge is dropped when either its call-site file
// or its target's declaring file is outside the set. A nil allowed set
// disables filtering (whole-module scope, for direct callers/tests).
//
// Failure honesty: a package that fails to load or type-check is counted
// (TypedEdgeReport.PackagesFailed) and its typed edges are left as whatever
// was last successfully stored — never overwritten with partial/wrong data,
// never blocking the rest of the service's ingest. The pipeline hook
// surfaces the counters in fusion.Report and logs a stderr line when any
// package failed, so a daemon whose typed pass always fails (e.g. `go` not
// on the daemon's PATH) is distinguishable from a non-Go service.
//
// Universe/stdlib interfaces: a dispatch call through an interface that
// stdlib or the universe scope defines (error.Error being the dominant
// case) fans out to EVERY first-party implementer of that interface — every
// custom error type in the service, for error. These edges are honest
// ("could reach any of N at runtime" is exactly the method-set truth) but
// they dominate iface-dispatch counts in server_stats, so read those
// counters as an upper bound on dispatch precision, not a defect signal.
package fusion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/tools/go/packages"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// diagWarn/diagInfo (DG-4, diagnostics sweep) route the typed-edge pass's
// own diagnostics through p.logger when SetLogger has been called
// (cmd/fathom's daemon does this at startup), preserving the ORIGINAL bare
// fmt.Fprintf(os.Stderr, format, args...) behavior byte-for-byte when it
// has not (p.logger == nil — every direct pkg/client embedder that never
// wires a logger, and every existing test in this package). format keeps
// its trailing "\n" for the os.Stderr fallback; the dlog path trims it,
// since dlog's own handler terminates each record itself.
func (p *Pipeline) diagWarn(format string, args ...any) {
	if p.logger == nil {
		fmt.Fprintf(os.Stderr, format, args...)
		return
	}
	p.logger.Warn(strings.TrimSuffix(fmt.Sprintf(format, args...), "\n"))
}

func (p *Pipeline) diagInfo(format string, args ...any) {
	if p.logger == nil {
		fmt.Fprintf(os.Stderr, format, args...)
		return
	}
	p.logger.Info(strings.TrimSuffix(fmt.Sprintf(format, args...), "\n"))
}

// TypedEdgeReport summarises one EnrichTypedEdges run. Edge counts cover
// edges WRITTEN this run (cache-hit packages' stored edges are not
// recounted).
type TypedEdgeReport struct {
	PackagesTotal      int // packages returned by the go/packages load (all variants)
	PackagesLoaded     int // packages actually re-resolved this run (cache miss)
	PackagesCached     int // packages skipped: content hash unchanged since last run
	PackagesFailed     int // packages that failed to load/typecheck/persist (fallback, never blocks)
	PackagesSkipped    int // packages with zero connector-allowed files (ignored dirs; reconciled away)
	EdgesDirect        int
	EdgesIfaceDispatch int
	EdgesUnresolved    int
	EdgesFuncValue     int // VS-6: func/method referenced as a value, not called at this site
	// EdgesDroppedOutOfScope counts edges whose target's declaring file was
	// outside the connector's discovered set (targetAllowed false) — VS-9,
	// docs/plans/verify-spine.md. Never folded silently into a clean zero.
	EdgesDroppedOutOfScope int
	// DroppedOutOfScopeSample names a few (deduplicated, capped) target
	// files behind EdgesDroppedOutOfScope, across the whole run.
	DroppedOutOfScopeSample []string
	// DiscoveryGaps counts DISTINCT dropped target files that exist on disk
	// despite being excluded from the connector's allowed set — not a
	// legitimate out-of-scope target but a likely connector-discovery
	// failure (a partially wedged Discover pass, the 2026-08-23 cpp-toolchain
	// flip's likeliest cause). Printed to stderr as it's found, never a
	// clean "0 failed".
	DiscoveryGaps     int
	DiscoveryGapFiles []string // sample of those files (deduplicated, capped)
	// Fallback names the override this run applied when an AMBIENT Go
	// workspace would otherwise have governed repoPath — currently always
	// "GOWORK=off" when non-empty, since TypedEdgeEnv forces that
	// unconditionally (wave P3 / O82, .fathom/observations.md — the
	// 2026-09-16 incident: a go.work naming sibling checkouts that exist
	// only beside the operator's OWN checkout broke typed-edge loading for
	// a push clone elsewhere). Empty when no workspace was discoverable
	// from repoPath at all — never claim an override that did not happen.
	Fallback string
	Duration time.Duration
}

// hasGoSource reports whether refs contains at least one ".go" file — the
// cheap gate enrichTypedEdges uses to decide whether the typed pass (which
// shells out to the go toolchain) is worth attempting at all.
func hasGoSource(refs []core.SourceRef) bool {
	for _, r := range refs {
		if strings.HasSuffix(r.Path, ".go") {
			return true
		}
	}
	return false
}

// hasGoModule reports whether repoPath has a go.mod file at its root —
// go/packages requires a module to resolve import paths; without one, a
// Load attempt would only spend a subprocess round-trip to fail.
func hasGoModule(repoPath string) bool {
	if repoPath == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(repoPath, "go.mod"))
	return err == nil
}

// enrichTypedEdges is the fusion.Pipeline.Ingest call-site hook (ADR-021
// §3): invoked once per Ingest call for services whose just-discovered ref
// set contains Go source and a go.mod (and Config.DisableTypedEdges is not
// set). Best-effort — its outcome never aborts or fails the surrounding
// Ingest, but the counters are returned for fusion.Report and a stderr line
// flags per-package failures, so a daemon whose typed pass always fails is
// distinguishable from a non-Go service.
//
// The connector's Discover result (refs) becomes the typed pass's
// allowed-file set: only .go files the connector actually surfaced (i.e.
// after .fathomignore and skip-dir filtering) may contribute edges.
func (p *Pipeline) enrichTypedEdges(ctx context.Context, svc core.ServiceRef, refs []core.SourceRef) TypedEdgeReport {
	if p.cfg.DisableTypedEdges || !hasGoSource(refs) || !hasGoModule(svc.Path) {
		return TypedEdgeReport{}
	}
	atomic.AddInt64(&p.typedPassTotal, 1)
	allowed := make(map[string]bool)
	for _, r := range refs {
		if strings.HasSuffix(r.Path, ".go") {
			allowed[r.Path] = true
		}
	}

	// Serialize process-wide (W-N, docs/plans/daemon-footprint.md, O108
	// reframed): this is the one choke point every caller of the typed
	// pass goes through — explicit `ingest` and auto-refresh-on-drift both
	// call Pipeline.Ingest, which calls this method; refresh-file never
	// runs a typed pass at all (fusion.Pipeline.ReingestFile only marks
	// TypedEdgesStale). A caller that arrives while another pass holds the
	// slot waits here; ctx cancellation while waiting is honored and never
	// blocks the surrounding Ingest — it degrades exactly like any other
	// typed-pass failure (fallback rows stand).
	release, acquireErr := acquireTypedPassSlot(ctx)
	if acquireErr != nil {
		p.noteTypedEdgeStatus(svc.ID, TypedEdgeStatus{
			Failed: true,
			Reason: fmt.Sprintf("waiting for the process-wide typed-pass slot: %v", acquireErr),
		})
		p.diagWarn("fusion: typed-edge pass %q: %v while waiting for the process-wide typed-pass slot — syntax-only edges remain the baseline\n", svc.ID, acquireErr)
		return TypedEdgeReport{}
	}
	defer release()

	var rep TypedEdgeReport
	var err error
	peakHeapInuse := samplePeakHeapInuse(func() {
		rep, err = EnrichTypedEdges(ctx, p.st, svc.ID, svc.Path, allowed)
	})
	defer func() {
		packages := rep.PackagesLoaded + rep.PackagesCached
		edges := rep.EdgesDirect + rep.EdgesIfaceDispatch + rep.EdgesUnresolved + rep.EdgesFuncValue
		p.diagInfo("fusion: typed-edge pass %q: %d packages, %d edges, peak heap_inuse %.1f MiB, %s\n",
			svc.ID, packages, edges, float64(peakHeapInuse)/(1024*1024), rep.Duration)
	}()
	// BP-35: the outcome is recorded ON THE SERVICE, not only on stderr.
	// The named reason travels with it (I24) so /statusz,
	// staleness_report and doctor's typed_edges row can all say WHY the
	// graph they serve is syntax-only, instead of a silent downgrade that
	// every surface reports as healthy.
	edgesTotal := rep.EdgesDirect + rep.EdgesIfaceDispatch + rep.EdgesUnresolved + rep.EdgesFuncValue
	switch {
	case err != nil:
		p.noteTypedEdgeStatus(svc.ID, TypedEdgeStatus{Failed: true, Reason: err.Error(), Fallback: rep.Fallback})
		p.diagWarn("fusion: typed-edge pass %q failed to load: %v — syntax-only edges remain the baseline\n", svc.ID, err)
	case rep.PackagesFailed > 0:
		p.noteTypedEdgeStatus(svc.ID, TypedEdgeStatus{
			PackagesSeen: rep.PackagesTotal,
			Packages:     rep.PackagesLoaded + rep.PackagesCached,
			EdgesTotal:   edgesTotal,
			Failed:       true,
			Reason:       fmt.Sprintf("%d/%d package(s) failed typed loading", rep.PackagesFailed, rep.PackagesTotal),
			Fallback:     rep.Fallback,
		})
		p.diagWarn("fusion: typed-edge pass %q: %d/%d package(s) failed typed loading — syntax-only edges remain the baseline for those\n", svc.ID, rep.PackagesFailed, rep.PackagesTotal)
	case rep.PackagesTotal > 0 && rep.PackagesLoaded+rep.PackagesCached == 0:
		// Never-silent guard: go/packages saw packages but the connector-
		// scope filter excluded every one of them. Legitimate only when the
		// ignore rules really do exclude all Go source; otherwise it is a
		// path-mapping bug of exactly the kind the relative-repoPath
		// regression was (see TestEnrichTypedEdges_RelativeRepoPath). Also
		// the shape a Go tree with ZERO typed edges takes (wave P3 / O82):
		// disclosed as Failed here so ingest's own response can say so
		// plainly instead of silently reading "ok" with a syntax-only graph.
		p.noteTypedEdgeStatus(svc.ID, TypedEdgeStatus{
			PackagesSeen: rep.PackagesTotal,
			Failed:       true,
			Reason:       fmt.Sprintf("%d package(s) seen but 0 processed — every package excluded by the connector scope filter", rep.PackagesTotal),
			Fallback:     rep.Fallback,
		})
		p.diagWarn("fusion: typed-edge pass %q: %d package(s) seen but 0 processed — every package excluded by the connector scope filter; reconcile will purge any previously stored typed edges for this service\n", svc.ID, rep.PackagesTotal)
	default:
		p.noteTypedEdgeStatus(svc.ID, TypedEdgeStatus{
			PackagesSeen: rep.PackagesTotal,
			Packages:     rep.PackagesLoaded + rep.PackagesCached,
			EdgesTotal:   edgesTotal,
			Fallback:     rep.Fallback,
		})
	}
	return rep
}

// EnrichTypedEdges loads every Go package under repoPath (pattern "./...",
// Tests:true so Test* callers are covered — goextract's own model treats
// _test.go source as ordinary symbols), resolves each call site, and
// persists typed core.Edge rows through st.PutTypedEdges, one call per Go
// package (the cache/replace granularity). See the package doc for the full
// design.
//
// allowed is the connector-scope filter (repo-relative forward-slash .go
// paths, as SourceRef.Path records them): a package with zero allowed
// files is skipped; an edge is dropped when its call-site file OR its
// target's declaring file is outside the set. nil disables filtering
// (whole-module scope) — the pipeline always passes the real Discover set;
// nil is for direct callers/tests that operate on self-contained fixture
// modules.
//
// The returned error is non-nil only when NO package could even be
// attempted (e.g. the go toolchain itself failed to run for repoPath); a
// per-package failure is counted in the report, never returned as an error
// — the caller (fusion's Ingest hook) treats both cases identically: never
// block, never fail the ingest.
func EnrichTypedEdges(ctx context.Context, st store.Store, serviceID, repoPath string, allowed map[string]bool) (TypedEdgeReport, error) {
	start := time.Now()
	var report TypedEdgeReport

	// Normalize repoPath to absolute FIRST: allowed-set membership rel-maps
	// go/packages' absolute file positions against repoPath, and
	// filepath.Rel errors on a relative base — before this normalization a
	// relative repoPath (the CLI's --path "." default, or a ServiceMeta
	// path recorded as ".") made every package silently fail the filter:
	// zero edges, zero failures, zero stderr.
	if abs, absErr := filepath.Abs(repoPath); absErr == nil {
		repoPath = abs
	}

	// Fallback disclosure (wave P3 / O82): detect, best-effort and BEFORE
	// forcing GOWORK=off below, whether an ambient Go workspace would have
	// governed this load — `go env GOWORK` run from repoPath's own
	// directory with the UN-forced environment auto-discovers a go.work
	// exactly as an un-forced `go build` would (upward search, or GOWORK
	// pinned globally via `go env -w`). Never blocking: any failure (no
	// toolchain, a broken environment) leaves Fallback empty rather than
	// guessing.
	if ws := detectWorkspaceOverride(repoPath); ws != "" {
		report.Fallback = "GOWORK=off"
	}

	fset := token.NewFileSet()
	cfg := &packages.Config{
		Context: ctx,
		Dir:     repoPath,
		Fset:    fset,
		Tests:   true,
		Env:     TypedEdgeEnv(),
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedTypes | packages.NeedSyntax | packages.NeedTypesInfo,
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		report.Duration = time.Since(start)
		return report, fmt.Errorf("fusion: typed-edge load %q: %w", repoPath, err)
	}
	// Sort by ID, NOT PkgPath: go/packages gives a test-augmented variant
	// ("pkg [pkg.test]") the SAME PkgPath as its plain package (PkgPath is
	// the logical Go import path, shared across variants) — ID is the
	// actual unique-per-variant identifier and the only field a stable,
	// tie-free sort (and therefore the processedFiles dedup below) can rely
	// on. This also means PkgPath is ALREADY variant-agnostic — no
	// stripping needed for first-party membership.
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].ID < pkgs[j].ID })
	report.PackagesTotal = len(pkgs)

	// First-party import paths: only a call resolving into one of these has
	// a corresponding SymbolNode in the index (see the package doc's Scope
	// note).
	firstParty := make(map[string]bool, len(pkgs))
	for _, pkg := range pkgs {
		firstParty[pkg.PkgPath] = true
	}

	// Global (plain-variant) implementer candidate pool — see the package
	// doc's note on test-variant type-object identity.
	var globalCandidates []*types.Named
	for _, pkg := range pkgs {
		if isTestVariant(pkg.ID) {
			continue
		}
		globalCandidates = append(globalCandidates, gatherCandidates(pkg.Types)...)
	}

	// Each real source file is resolved exactly once, under whichever
	// package variant visits it first (plain sorts before its test-augmented
	// forms — see isTestVariant) — this is what keeps Tests:true from
	// double-emitting edges for every non-test file that a test-augmented
	// variant re-compiles alongside its _test.go additions. A CACHE-HIT
	// package must mark its files too: skipping the marking made a
	// _test.go-only change re-resolve the (cache-hit) plain package's files
	// under the test-variant ID — exact-duplicate rows.
	processedFiles := make(map[string]bool)

	// keep is the reconcile keep-set: every pkg.ID this load produced,
	// EXCEPT connector-ignored packages (their stored rows must be purged).
	// Failed packages ARE kept — fallback semantics retain their last
	// successful rows.
	keep := make(map[string]bool, len(pkgs))

	// VS-9 (docs/plans/verify-spine.md): run-level dedup for the
	// out-of-scope-drop sample and the discovery-gap file list, so a target
	// hit from more than one call site or package is only sampled/printed
	// once.
	droppedSampleSeen := map[string]bool{}
	discoveryGapSeen := map[string]bool{}

	for _, pkg := range pkgs {
		// Connector scope (see the package doc): a package with zero
		// allowed files contributes nothing — and is deliberately NOT in
		// the keep set, so previously stored rows for a now-ignored
		// package are reconciled away below.
		if allowed != nil && !anyFileAllowed(repoPath, allowed, pkg.CompiledGoFiles) {
			report.PackagesSkipped++
			continue
		}
		keep[pkg.ID] = true

		if len(pkg.Errors) > 0 {
			report.PackagesFailed++
			continue
		}
		hash, hashErr := packageContentHash(repoPath, pkg)
		if hashErr != nil {
			report.PackagesFailed++
			continue
		}
		// Cache/replace granularity is pkg.ID (unique per variant), never
		// pkg.PkgPath (shared between a package and its test-augmented
		// variant — using it here was the bug that made a test-variant's
		// PutTypedEdges silently wipe its plain sibling's just-stored rows).
		if cached, found, cacheErr := st.TypedEdgePackageFingerprint(ctx, serviceID, pkg.ID); cacheErr == nil && found && cached == hash {
			report.PackagesCached++
			for _, f := range pkg.CompiledGoFiles {
				processedFiles[f] = true
			}
			continue
		}

		candidates := globalCandidates
		if isTestVariant(pkg.ID) {
			candidates = append(append([]*types.Named(nil), globalCandidates...), gatherCandidates(pkg.Types)...)
		}

		var edges []core.Edge
		for i, file := range pkg.Syntax {
			var filename string
			if i < len(pkg.CompiledGoFiles) {
				filename = pkg.CompiledGoFiles[i]
			}
			if filename != "" {
				if processedFiles[filename] {
					continue
				}
				processedFiles[filename] = true
				if allowed != nil && !allowed[repoRelSlash(repoPath, filename)] {
					continue // ignored file within a partially allowed package
				}
			}
			fr := resolveFileEdges(serviceID, repoPath, fset, file, pkg, firstParty, candidates, allowed)
			edges = append(edges, fr.edges...)
			report.EdgesDroppedOutOfScope += fr.droppedOutOfScope
			for _, f := range fr.droppedSample {
				if !droppedSampleSeen[f] {
					droppedSampleSeen[f] = true
					if len(report.DroppedOutOfScopeSample) < 5 {
						report.DroppedOutOfScopeSample = append(report.DroppedOutOfScopeSample, f)
					}
				}
			}
			for _, f := range fr.discoveryGapFiles {
				if !discoveryGapSeen[f] {
					discoveryGapSeen[f] = true
					report.DiscoveryGaps++
					if len(report.DiscoveryGapFiles) < 5 {
						report.DiscoveryGapFiles = append(report.DiscoveryGapFiles, f)
					}
					// NOT routed through diagWarn/dlog (DG-4 scope note):
					// EnrichTypedEdges is a package-level exported function
					// with ~20 direct test callers (typedcalls_test.go and
					// siblings) that construct no Pipeline at all — adding a
					// *dlog.Logger parameter here would ripple into every
					// one of them for a single diagnostic line. p.diagWarn
					// (this file, enrichTypedEdges above) already covers the
					// method callers that DO have a Pipeline; this line
					// keeps its original bare stderr write until a session
					// takes on that broader signature change deliberately.
					fmt.Fprintf(os.Stderr, "fusion: typed-edge pass %q: discovery gap — %s exists on disk but was excluded from the connector's allowed set; a typed edge into it was dropped as out-of-scope\n", serviceID, f)
				}
			}
		}
		sortEdges(edges)

		if putErr := st.PutTypedEdges(ctx, serviceID, pkg.ID, edges); putErr != nil {
			report.PackagesFailed++
			continue
		}
		if fpErr := st.SetTypedEdgePackageFingerprint(ctx, serviceID, pkg.ID, hash); fpErr != nil {
			report.PackagesFailed++
			continue
		}
		report.PackagesLoaded++
		for _, e := range edges {
			switch e.Kind {
			case core.EdgeDirect:
				report.EdgesDirect++
			case core.EdgeIfaceDispatch:
				report.EdgesIfaceDispatch++
			case core.EdgeUnresolved:
				report.EdgesUnresolved++
			case core.EdgeFuncValue:
				report.EdgesFuncValue++
			}
		}
	}

	// Reconcile (see the package doc): purge stored typed-edge packages that
	// this load did not produce — deleted/moved directories, or packages the
	// connector now ignores. Skipped only on wholesale failure (every
	// attempted package failed): that is indistinguishable from a transient
	// toolchain breakage, and deleting everything on it would destroy the
	// documented fallback rows.
	wholesaleFailure := report.PackagesFailed > 0 && report.PackagesLoaded+report.PackagesCached == 0
	if !wholesaleFailure {
		if stored, listErr := st.ListTypedEdgePackages(ctx, serviceID); listErr == nil {
			for _, id := range stored {
				if keep[id] {
					continue
				}
				if delErr := st.DeleteTypedEdgePackage(ctx, serviceID, id); delErr != nil {
					report.PackagesFailed++ // stale rows remain; next ingest retries
				}
			}
		}
	}

	report.Duration = time.Since(start)
	return report, nil
}

// anyFileAllowed reports whether at least one of files (absolute paths)
// maps to a repo-relative path in the allowed set.
func anyFileAllowed(repoPath string, allowed map[string]bool, files []string) bool {
	for _, f := range files {
		if allowed[repoRelSlash(repoPath, f)] {
			return true
		}
	}
	return false
}

// sortEdges sorts edges by (FromSymbol, ToSymbol, Kind) for deterministic
// storage order.
func sortEdges(edges []core.Edge) {
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].FromSymbol != edges[j].FromSymbol {
			return edges[i].FromSymbol < edges[j].FromSymbol
		}
		if edges[i].ToSymbol != edges[j].ToSymbol {
			return edges[i].ToSymbol < edges[j].ToSymbol
		}
		return edges[i].Kind < edges[j].Kind
	})
}

// fileEdgeResult is resolveFileEdges' return: the edges it resolved, plus
// VS-9 (docs/plans/verify-spine.md) accounting for targets it had to drop
// because the connector-scope filter (targetAllowed) excluded their
// declaring file — never folded silently into the edge count.
type fileEdgeResult struct {
	edges []core.Edge
	// droppedOutOfScope counts every edge (call, func-value, or dispatch
	// implementer) dropped because targetAllowed(fn) was false — one per
	// drop, not deduplicated by file.
	droppedOutOfScope int
	// droppedSample names a few (deduplicated, capped) target files whose
	// edges were dropped this call — a debugging sample, not exhaustive.
	droppedSample []string
	// discoveryGapFiles names the (deduplicated) subset of dropped target
	// files that EXIST on disk despite being excluded from `allowed`: not a
	// legitimate out-of-scope target but a likely connector-discovery gap
	// (see EnrichTypedEdges' DiscoveryGaps counter).
	discoveryGapFiles []string
}

// resolveFileEdges walks every top-level *ast.FuncDecl in file (mirroring
// GoExtractor.Extract's own top-level-only walk) and resolves each call
// site reachable from its body (mirroring collectCalls's descent into
// nested closures — their calls are attributed to the enclosing named
// function, exactly like the syntax pass) PLUS every *ast.Ident/
// *ast.SelectorExpr in non-call-fun VALUE position that resolves to a
// *types.Func (VS-6, docs/plans/verify-spine.md): mux.HandleFunc(p, h), a
// route-table literal {p, h}, an http.HandlerFunc(h) conversion,
// middleware(h), a method value s.handleX. These are recorded as
// core.EdgeFuncValue, additive alongside the call edges — an enclosing
// function that only ever registers a handler this way now reaches it.
func resolveFileEdges(
	serviceID, repoPath string,
	fset *token.FileSet,
	file *ast.File,
	pkg *packages.Package,
	firstParty map[string]bool,
	candidates []*types.Named,
	allowed map[string]bool,
) fileEdgeResult {
	var edges []core.Edge
	var droppedOutOfScope int
	var droppedSample []string
	var discoveryGapFiles []string
	seenDroppedFile := map[string]bool{}
	info := pkg.TypesInfo

	// targetAllowed applies the connector-scope filter to a resolved target:
	// an edge may not point at a symbol whose declaring file the connector
	// excluded (goextract never indexed it, and its package/receiver names
	// could collide with a live symbol's key — the fabrication path). Every
	// refusal is counted (VS-9): a small deduplicated sample of the target
	// files is kept, and a target file that EXISTS ON DISK despite being
	// excluded is flagged as a discovery gap, never a clean silent drop.
	targetAllowed := func(fn *types.Func) bool {
		if allowed == nil {
			return true
		}
		pos := fset.Position(fn.Pos())
		if pos.Filename == "" {
			return false // no declaring position → cannot prove it's in scope
		}
		rel := repoRelSlash(repoPath, pos.Filename)
		if allowed[rel] {
			return true
		}
		droppedOutOfScope++
		if !seenDroppedFile[rel] {
			seenDroppedFile[rel] = true
			if len(droppedSample) < 5 {
				droppedSample = append(droppedSample, rel)
			}
			if _, statErr := os.Stat(pos.Filename); statErr == nil {
				discoveryGapFiles = append(discoveryGapFiles, rel)
			}
		}
		return false
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name == nil || fn.Body == nil {
			continue
		}
		if fn.Name.Name == "init" {
			// Documented skip: goextract records every init() under the
			// bare name "init", so a package with more than one init has an
			// ambiguous SymbolKey the store can hold multiple rows under —
			// attaching typed edges to it could silently point at the
			// wrong one. See the package doc.
			continue
		}
		callerFn, ok := info.Defs[fn.Name].(*types.Func)
		if !ok {
			continue
		}
		callerKey, ok := symbolKeyForFunc(serviceID, repoPath, fset, callerFn)
		if !ok {
			continue
		}
		fromFile := repoRelSlash(repoPath, fset.Position(fn.Pos()).Filename)

		// addDirect/addDispatch are shared by the call pass (EdgeDirect/
		// EdgeIfaceDispatch below) and the value-ref pass (EdgeFuncValue):
		// same firstParty/targetAllowed/symbolKeyForFunc discipline, only
		// the emitted Kind differs.
		addDirect := func(fn *types.Func, kind core.EdgeKind) {
			if fn.Pkg() == nil || !firstParty[fn.Pkg().Path()] {
				return
			}
			if !targetAllowed(fn) {
				return
			}
			toKey, ok := symbolKeyForFunc(serviceID, repoPath, fset, fn)
			if !ok {
				return
			}
			edges = append(edges, core.Edge{
				FromService: serviceID, FromFile: fromFile,
				FromSymbol: callerKey, ToSymbol: toKey, Kind: kind,
			})
		}
		addDispatch := func(iface *types.Interface, methodName string, kind core.EdgeKind) {
			seen := map[string]bool{}
			for _, cand := range candidates {
				implFn, ok := lookupImplementerMethod(cand, iface, methodName, pkg.Types)
				if !ok || implFn.Pkg() == nil || !firstParty[implFn.Pkg().Path()] {
					continue
				}
				if !targetAllowed(implFn) {
					continue
				}
				toKey, ok := symbolKeyForFunc(serviceID, repoPath, fset, implFn)
				if !ok || seen[toKey] {
					continue
				}
				seen[toKey] = true
				edges = append(edges, core.Edge{
					FromService: serviceID, FromFile: fromFile,
					FromSymbol: callerKey, ToSymbol: toKey, Kind: kind,
				})
			}
		}

		// skip marks AST nodes the value-ref pass must not classify on its
		// own: a CallExpr's own Fun (already classified by the call pass
		// below), a composite-literal KEY (a struct field NAME, never a
		// value), and the left side of an assignment (an ASSIGNED-TO
		// variable, never a reference). Registered at the parent node's
		// visit — ast.Inspect is pre-order, so the child is marked before
		// Inspect itself reaches it.
		skip := map[ast.Expr]bool{}

		ast.Inspect(fn.Body, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.CallExpr:
				funExpr := node.Fun
				for {
					p, ok := funExpr.(*ast.ParenExpr)
					if !ok {
						break
					}
					funExpr = p.X
				}
				skip[funExpr] = true

				target := classifyCall(info, node.Fun)
				switch target.kind {
				case targetDirect:
					addDirect(target.fn, core.EdgeDirect)
				case targetIfaceDispatch:
					addDispatch(target.iface, target.fn.Name(), core.EdgeIfaceDispatch)
				case targetUnresolved:
					line := fset.Position(node.Pos()).Line
					edges = append(edges, core.Edge{
						FromService: serviceID, FromFile: fromFile,
						FromSymbol: callerKey,
						ToSymbol:   fmt.Sprintf("<unresolved:%d>", line),
						Kind:       core.EdgeUnresolved,
					})
				}
				return true

			case *ast.CompositeLit:
				for _, elt := range node.Elts {
					if kv, ok := elt.(*ast.KeyValueExpr); ok {
						if keyIdent, ok := kv.Key.(*ast.Ident); ok {
							skip[keyIdent] = true
						}
					}
				}
				return true

			case *ast.AssignStmt:
				for _, lhs := range node.Lhs {
					skip[lhs] = true
				}
				return true

			case *ast.Ident:
				if skip[node] {
					return true
				}
				target := classifyCall(info, node)
				switch target.kind {
				case targetDirect:
					addDirect(target.fn, core.EdgeFuncValue)
				case targetIfaceDispatch:
					addDispatch(target.iface, target.fn.Name(), core.EdgeFuncValue)
				}
				return true

			case *ast.SelectorExpr:
				// A SelectorExpr's own Sel identifier is never an independent
				// value reference — go/types also resolves it via info.Uses
				// (a method selection included), and the bare-Ident branch of
				// classifyCall has no interface/concrete distinction, so
				// letting it through here would misclassify an interface
				// method value as a direct edge to the interface's own
				// (phantom) method. Always route through the SelectorExpr
				// itself, which uses classifySelection's precise resolution.
				skip[node.Sel] = true
				if skip[node] {
					return true
				}
				target := classifyCall(info, node)
				switch target.kind {
				case targetDirect:
					addDirect(target.fn, core.EdgeFuncValue)
				case targetIfaceDispatch:
					addDispatch(target.iface, target.fn.Name(), core.EdgeFuncValue)
				}
				return true
			}
			return true
		})
	}
	return fileEdgeResult{
		edges:             edges,
		droppedOutOfScope: droppedOutOfScope,
		droppedSample:     droppedSample,
		discoveryGapFiles: discoveryGapFiles,
	}
}

// targetKind classifies a call site's resolution outcome.
type targetKind int

const (
	// targetNone: not a call to any indexable symbol at all (a builtin call
	// like len()/make(), or a type conversion) — no edge is emitted.
	targetNone targetKind = iota
	targetDirect
	targetIfaceDispatch
	targetUnresolved
)

// callTarget is classifyCall's result.
type callTarget struct {
	kind  targetKind
	fn    *types.Func      // set for targetDirect / targetIfaceDispatch
	iface *types.Interface // set for targetIfaceDispatch
}

// classifyCall resolves a CallExpr's Fun expression to its call target using
// info (the package's *types.Info from NeedTypesInfo).
func classifyCall(info *types.Info, fun ast.Expr) callTarget {
	for {
		p, ok := fun.(*ast.ParenExpr)
		if !ok {
			break
		}
		fun = p.X
	}
	switch f := fun.(type) {
	case *ast.Ident:
		switch obj := info.Uses[f].(type) {
		case *types.Func:
			return callTarget{kind: targetDirect, fn: obj}
		case *types.Builtin, *types.TypeName:
			// A builtin call (len, make, append, …) or a type conversion —
			// not a call to any indexable symbol; no edge, not "unresolved".
			return callTarget{kind: targetNone}
		default:
			// A local func-typed variable/parameter — a func value.
			return callTarget{kind: targetUnresolved}
		}
	case *ast.SelectorExpr:
		if sel, ok := info.Selections[f]; ok {
			return classifySelection(sel)
		}
		// Not a method selection: a package-qualified reference (pkg.Func
		// or pkg.Var). info.Uses on the selector's Sel identifies it
		// directly, dot-imports included.
		switch obj := info.Uses[f.Sel].(type) {
		case *types.Func:
			return callTarget{kind: targetDirect, fn: obj}
		case *types.Builtin, *types.TypeName:
			return callTarget{kind: targetNone}
		default:
			return callTarget{kind: targetUnresolved}
		}
	default:
		// Function literal immediately invoked, or an index/slice/composite
		// expression producing a func value.
		return callTarget{kind: targetUnresolved}
	}
}

// classifySelection resolves a method-shaped selector expression (the
// SelectorExpr appeared in info.Selections, so it denotes a field or
// method, never a package-qualified reference).
func classifySelection(sel *types.Selection) callTarget {
	if sel.Kind() != types.MethodVal && sel.Kind() != types.MethodExpr {
		return callTarget{kind: targetUnresolved} // FieldVal: a struct field of func type
	}
	fn, ok := sel.Obj().(*types.Func)
	if !ok {
		return callTarget{kind: targetUnresolved}
	}
	// sel.Obj() already resolves embedded-field promotion AND generic
	// instantiation down to the underlying declaration for us — the direct
	// case needs no further unwrapping (see the package doc).
	//
	// Dispatch is detected on the RESOLVED METHOD's receiver, not on
	// sel.Recv(): when a STRUCT embeds an interface, a promoted call
	// (w.Complete() with w a struct) has a concrete sel.Recv() but resolves
	// to the interface's own *types.Func — dynamic dispatch through the
	// embedded interface value. Classifying by sel.Recv() alone emitted
	// that as an EdgeDirect to a phantom "Iface.Iface.Method" key goextract
	// never creates; classifying by the method's receiver treats it as the
	// interface dispatch it really is (fan-out to real implementers). A
	// plain interface-typed receiver call resolves to the same branch.
	if iface, ok := receiverInterface(fn); ok {
		return callTarget{kind: targetIfaceDispatch, fn: fn, iface: iface}
	}
	return callTarget{kind: targetDirect, fn: fn}
}

// receiverInterface returns the *types.Interface underlying fn's receiver
// type when fn is an interface's own method declaration (an "interface
// method"); ok is false for a concrete method or a plain function. An
// interface method is never an emittable edge target: goextract creates no
// method node for it (interfaces index as a single KindGoInterface symbol),
// so a key built from it would be a phantom.
func receiverInterface(fn *types.Func) (*types.Interface, bool) {
	sig, ok := fn.Type().(*types.Signature)
	if !ok {
		return nil, false
	}
	recv := sig.Recv()
	if recv == nil {
		return nil, false
	}
	iface, ok := recv.Type().Underlying().(*types.Interface)
	return iface, ok
}

// symbolKeyForFunc maps a *types.Func to the SAME canonical SymbolKey
// goextract's node would carry (core.SymbolNode.Key): a plain function's
// Package component is its declared package NAME (not import path); a
// method's Package component is its receiver's bare type name
// (pointer-stripped; a generic receiver's type-parameter list is stripped
// too — types.Named.Obj().Name() already excludes it, matching goextract's
// receiverTypeName unwrapping of *ast.IndexExpr/*ast.IndexListExpr). Either
// component is qualified by the DIRECTORY of fn's declaring file, relative
// to repoPath, exactly as core.QualifiedPackage qualifies the node's key
// from its FilePath (PC-32) — so a `func run` in cmd/a and one in cmd/b get
// distinct keys here just as they do in the syntax pass. ok is false when
// fn has no usable package/receiver.
func symbolKeyForFunc(serviceID, repoPath string, fset *token.FileSet, fn *types.Func) (string, bool) {
	sig, ok := fn.Type().(*types.Signature)
	if !ok {
		return "", false
	}
	var relFile string
	if pos := fset.Position(fn.Pos()); pos.Filename != "" {
		relFile = repoRelSlash(repoPath, pos.Filename)
	}
	if recv := sig.Recv(); recv != nil {
		recvName := namedTypeName(recv.Type())
		if recvName == "" {
			return "", false
		}
		return core.SymbolKey(serviceID, core.QualifiedPackage(relFile, recvName), recvName+"."+fn.Name()), true
	}
	if fn.Pkg() == nil {
		return "", false
	}
	return core.SymbolKey(serviceID, core.QualifiedPackage(relFile, fn.Pkg().Name()), fn.Name()), true
}

// namedTypeName returns the bare declared name of t's named type,
// unwrapping one level of pointer indirection. Returns "" for anything else
// (defensive — should not be reached for a method's receiver type).
func namedTypeName(t types.Type) string {
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	named, ok := t.(*types.Named)
	if !ok {
		return ""
	}
	return named.Obj().Name()
}

// isTestVariant reports whether pkgID (a *packages.Package.ID — NOT its
// PkgPath, which a test-augmented variant shares with its plain package) is
// a go/packages test-augmented variant ("pkg [pkg.test]" / "pkg_test [pkg.test]").
func isTestVariant(pkgID string) bool {
	return strings.Contains(pkgID, " [")
}

// gatherCandidates collects the concrete (non-interface) named types
// declared directly in pkg's own package scope — implementer candidates for
// interface-dispatch resolution.
func gatherCandidates(pkg *types.Package) []*types.Named {
	if pkg == nil {
		return nil
	}
	scope := pkg.Scope()
	var out []*types.Named
	for _, name := range scope.Names() {
		tn, ok := scope.Lookup(name).(*types.TypeName)
		if !ok || tn.IsAlias() {
			continue
		}
		named, ok := tn.Type().(*types.Named)
		if !ok {
			continue
		}
		if _, isIface := named.Underlying().(*types.Interface); isIface {
			continue
		}
		out = append(out, named)
	}
	return out
}

// lookupImplementerMethod reports whether named (or *named) implements
// iface, and if so returns the concrete *types.Func satisfying methodName —
// types.NewMethodSet handles embedded-field promotion for us, so a method
// inherited from an embedded STRUCT field resolves to its actual declaring
// type. A candidate satisfying iface only via an embedded INTERFACE field
// resolves to the interface's own method (see receiverInterface) — a
// phantom key, and the true runtime target (whatever the field holds) is
// statically unknowable — so it is skipped, not emitted.
func lookupImplementerMethod(named *types.Named, iface *types.Interface, methodName string, from *types.Package) (*types.Func, bool) {
	if types.Implements(named, iface) {
		if fn, ok := concreteMethod(types.NewMethodSet(named), from, methodName); ok {
			return fn, true
		}
	}
	ptr := types.NewPointer(named)
	if types.Implements(ptr, iface) {
		if fn, ok := concreteMethod(types.NewMethodSet(ptr), from, methodName); ok {
			return fn, true
		}
	}
	return nil, false
}

// concreteMethod looks methodName up in mset and returns it only when it is
// a CONCRETE method — declared on (or struct-embedded into) the type, never
// an interface's own method surfacing through an embedded interface field.
func concreteMethod(mset *types.MethodSet, from *types.Package, methodName string) (*types.Func, bool) {
	sel := mset.Lookup(from, methodName)
	if sel == nil {
		return nil, false
	}
	fn, ok := sel.Obj().(*types.Func)
	if !ok {
		return nil, false
	}
	if _, isIfaceMethod := receiverInterface(fn); isIfaceMethod {
		return nil, false
	}
	return fn, true
}

// TypedEdgeEpoch salts every typed-edge package fingerprint (the per-package
// cache key below), the way ExtractorEpoch salts the file hash-gate. Bump it
// in any commit that changes the KEYS or KINDS the typed pass stores: a
// cache hit keeps the stored rows verbatim, so without the bump every
// unchanged package would keep edges written in the old key form — silently
// dropped by callgraph.AddTypedEdges (an endpoint that is no longer a node)
// and invisible until a file in that package happened to change.
// Epoch 2 = PC-32: from/to keys carry the package directory
// (core.QualifiedPackage); every row written before it named the package
// by declared name only.
const TypedEdgeEpoch = "2"

// packageContentHash hashes pkg's compiled Go files by repo-relative path +
// content, sorted, under the TypedEdgeEpoch salt — the per-package cache
// key. Repo-relative (not absolute) paths keep the cache portable across
// machines/worktrees (ADR-017).
func packageContentHash(repoPath string, pkg *packages.Package) (string, error) {
	files := append([]string(nil), pkg.CompiledGoFiles...)
	sort.Strings(files)
	h := sha256.New()
	h.Write([]byte("typed-edge-epoch:" + TypedEdgeEpoch))
	h.Write([]byte{0})
	h.Write([]byte(pkg.PkgPath))
	h.Write([]byte{0})
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return "", err
		}
		h.Write([]byte(repoRelSlash(repoPath, f)))
		h.Write([]byte{0})
		h.Write(data)
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// repoRelSlash converts absPath to a forward-slash path relative to
// repoPath, matching the connector's SourceRef.Path convention. Falls back
// to the slash-converted absolute path if it isn't under repoPath.
func repoRelSlash(repoPath, absPath string) string {
	rel, err := filepath.Rel(repoPath, absPath)
	if err != nil {
		return filepath.ToSlash(absPath)
	}
	return filepath.ToSlash(rel)
}
