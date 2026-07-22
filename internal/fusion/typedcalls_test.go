package fusion_test

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// typedFixtureDir returns the absolute path to the small, self-contained Go
// module (its own go.mod) used to exercise EnrichTypedEdges's resolution
// rules: direct calls, interface dispatch (with the three ADR-021 §3 O9
// residue shapes — adapter Complete, Store contract-suite dispatch, Ping),
// func/method values, embedded promotion, generics, and multi-init skip.
func typedFixtureDir(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("testdata", "typedfixture"))
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}
	if _, err := os.Stat(filepath.Join(abs, "go.mod")); err != nil {
		t.Fatalf("typed fixture module missing go.mod at %s: %v", abs, err)
	}
	return abs
}

// edgeSet indexes edges by (FromSymbol, Kind) -> sorted ToSymbols, for
// order-independent assertions on interface-dispatch fan-out.
func edgeSet(edges []core.Edge) map[string][]string {
	m := map[string][]string{}
	for _, e := range edges {
		k := e.FromSymbol + "|" + string(e.Kind)
		m[k] = append(m[k], e.ToSymbol)
	}
	for k := range m {
		sort.Strings(m[k])
	}
	return m
}

func key(pkg, name string) string { return core.SymbolKey("typedfixture", pkg, name) }

// TestEnrichTypedEdges_Resolution is the CT-3 fixture matrix: direct call,
// interface dispatch (2 impls, three residue shapes), func value, method
// value, embedded promotion, generics instantiation, and the multi-init
// documented skip — table-tested against the fixture module by calling
// EnrichTypedEdges directly (CT-2's entry point).
func TestEnrichTypedEdges_Resolution(t *testing.T) {
	dir := typedFixtureDir(t)
	ctx := context.Background()
	st := store.NewMemoryStore()

	report, err := fusion.EnrichTypedEdges(ctx, st, "typedfixture", dir)
	if err != nil {
		t.Fatalf("EnrichTypedEdges: %v", err)
	}
	if report.PackagesFailed != 0 {
		t.Errorf("PackagesFailed = %d, want 0 (fixture module must build clean)", report.PackagesFailed)
	}
	if report.PackagesLoaded == 0 {
		t.Fatal("PackagesLoaded = 0, want at least the fixture's own package(s)")
	}

	edges, err := st.TypedEdges(ctx, "typedfixture")
	if err != nil {
		t.Fatalf("TypedEdges: %v", err)
	}
	if len(edges) == 0 {
		t.Fatal("TypedEdges returned no edges")
	}
	set := edgeSet(edges)

	// --- Direct call: Direct -> helper ---
	direct := set[key("typedfixture", "Direct")+"|"+string(core.EdgeDirect)]
	if !contains(direct, key("typedfixture", "helper")) {
		t.Errorf("Direct's direct edges = %v, want to contain helper", direct)
	}

	// --- Interface dispatch: RunAdapter -> {AdapterA.Complete, AdapterB.Complete} ---
	adapterDispatch := set[key("typedfixture", "RunAdapter")+"|"+string(core.EdgeIfaceDispatch)]
	wantAdapter := []string{key("AdapterA", "AdapterA.Complete"), key("AdapterB", "AdapterB.Complete")}
	sort.Strings(wantAdapter)
	if !equalStrSlices(adapterDispatch, wantAdapter) {
		t.Errorf("RunAdapter dispatch edges = %v, want %v", adapterDispatch, wantAdapter)
	}

	// --- Interface dispatch: RunContract -> {MemStore.PutClaims, DiskStore.PutClaims} ---
	storeDispatch := set[key("typedfixture", "RunContract")+"|"+string(core.EdgeIfaceDispatch)]
	wantStore := []string{key("MemStore", "MemStore.PutClaims"), key("DiskStore", "DiskStore.PutClaims")}
	sort.Strings(wantStore)
	if !equalStrSlices(storeDispatch, wantStore) {
		t.Errorf("RunContract dispatch edges = %v, want %v", storeDispatch, wantStore)
	}

	// --- Interface dispatch: CheckPing -> {LocalPing.Ping, RemotePing.Ping} ---
	pingDispatch := set[key("typedfixture", "CheckPing")+"|"+string(core.EdgeIfaceDispatch)]
	wantPing := []string{key("LocalPing", "LocalPing.Ping"), key("RemotePing", "RemotePing.Ping")}
	sort.Strings(wantPing)
	if !equalStrSlices(pingDispatch, wantPing) {
		t.Errorf("CheckPing dispatch edges = %v, want %v", pingDispatch, wantPing)
	}

	// --- Func value / method value: unresolved, never silently dropped ---
	for _, caller := range []string{"CallFuncValue", "UseFuncValue", "UseMethodValue"} {
		unresolved := set[key("typedfixture", caller)+"|"+string(core.EdgeUnresolved)]
		if len(unresolved) == 0 {
			t.Errorf("%s: no EdgeUnresolved recorded, want at least one", caller)
		}
		for _, to := range unresolved {
			if !strings.HasPrefix(to, "<unresolved:") {
				t.Errorf("%s unresolved ToSymbol = %q, want the <unresolved:LINE> callsite marker", caller, to)
			}
		}
	}

	// --- Embedded promotion: CallPromoted -> Base.Hello (the declaring type) ---
	promoted := set[key("typedfixture", "CallPromoted")+"|"+string(core.EdgeDirect)]
	if !contains(promoted, key("Base", "Base.Hello")) {
		t.Errorf("CallPromoted direct edges = %v, want to contain Base.Hello", promoted)
	}

	// --- Generics: CallGeneric -> Stack.Push (the generic definition, not a
	// per-instantiation symbol like "Stack[int].Push") ---
	generic := set[key("typedfixture", "CallGeneric")+"|"+string(core.EdgeDirect)]
	if !contains(generic, key("Stack", "Stack.Push")) {
		t.Errorf("CallGeneric direct edges = %v, want to contain Stack.Push", generic)
	}

	// --- Multi-init: no edges recorded FROM the ambiguous "init" caller key ---
	for _, e := range edges {
		if strings.HasSuffix(e.FromSymbol, ".init") || strings.HasSuffix(e.FromSymbol, "::init") {
			t.Errorf("found an edge from an init() caller (documented skip): %+v", e)
		}
	}

	// --- Test* callers reach through to typed edges too (CT-4 prerequisite):
	// TestRunAdapter -EdgeDirect-> RunAdapter -EdgeIfaceDispatch-> {AdapterA,AdapterB}.Complete
	testDirect := set[key("typedfixture", "TestRunAdapter")+"|"+string(core.EdgeDirect)]
	if !contains(testDirect, key("typedfixture", "RunAdapter")) {
		t.Errorf("TestRunAdapter direct edges = %v, want to contain RunAdapter", testDirect)
	}
}

// TestEnrichTypedEdges_Caching is CT-5's correctness half: a second run over
// unchanged content must be a full cache hit (no packages reloaded) and must
// leave the previously stored edges completely untouched; touching one
// file's content must invalidate only that file's package.
func TestEnrichTypedEdges_Caching(t *testing.T) {
	dir := typedFixtureDir(t)
	ctx := context.Background()
	st := store.NewMemoryStore()

	first, err := fusion.EnrichTypedEdges(ctx, st, "typedfixture", dir)
	if err != nil {
		t.Fatalf("first EnrichTypedEdges: %v", err)
	}
	if first.PackagesLoaded == 0 {
		t.Fatal("first run: PackagesLoaded = 0, want > 0")
	}
	firstEdges, err := st.TypedEdges(ctx, "typedfixture")
	if err != nil {
		t.Fatalf("TypedEdges after first run: %v", err)
	}

	second, err := fusion.EnrichTypedEdges(ctx, st, "typedfixture", dir)
	if err != nil {
		t.Fatalf("second EnrichTypedEdges: %v", err)
	}
	if second.PackagesLoaded != 0 {
		t.Errorf("second run: PackagesLoaded = %d, want 0 (full cache hit)", second.PackagesLoaded)
	}
	if second.PackagesCached != first.PackagesTotal-first.PackagesFailed {
		t.Errorf("second run: PackagesCached = %d, want %d (every package that succeeded)", second.PackagesCached, first.PackagesTotal-first.PackagesFailed)
	}

	secondEdges, err := st.TypedEdges(ctx, "typedfixture")
	if err != nil {
		t.Fatalf("TypedEdges after second run: %v", err)
	}
	if len(firstEdges) != len(secondEdges) {
		t.Fatalf("edge count changed across cached run: %d -> %d", len(firstEdges), len(secondEdges))
	}
	if second.Duration >= first.Duration {
		t.Logf("cached run (%s) was not faster than first run (%s) — logged, not failed: timing can be noisy on a loaded machine", second.Duration, first.Duration)
	}

	// Touch exactly one file: only its OWN package should reload.
	target := filepath.Join(dir, "direct.go")
	orig, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read %s: %v", target, err)
	}
	if err := os.WriteFile(target, append(orig, []byte("\n// touched\n")...), 0o644); err != nil {
		t.Fatalf("write %s: %v", target, err)
	}
	t.Cleanup(func() { _ = os.WriteFile(target, orig, 0o644) })

	third, err := fusion.EnrichTypedEdges(ctx, st, "typedfixture", dir)
	if err != nil {
		t.Fatalf("third EnrichTypedEdges: %v", err)
	}
	// direct.go is compiled into BOTH the plain package and its
	// test-augmented variant (Tests:true re-includes every non-test file
	// alongside the _test.go additions), so touching it invalidates both
	// package entries' content hash — the synthetic ".test" main entry
	// (which doesn't reference direct.go at all) must stay cached.
	if third.PackagesLoaded != 2 {
		t.Errorf("third run (one file touched): PackagesLoaded = %d, want 2 (plain + its test-augmented variant)", third.PackagesLoaded)
	}
	if third.PackagesCached != third.PackagesTotal-2 {
		t.Errorf("third run: PackagesCached = %d, want %d (everything except the 2 invalidated packages)", third.PackagesCached, third.PackagesTotal-2)
	}
}

// TestEnrichTypedEdges_LoadFailure is the honesty-boundary test: a broken
// Go module must never fail the caller or panic — it must be counted and
// otherwise ignored (fallback), matching the "never blocks ingest" contract.
func TestEnrichTypedEdges_LoadFailure(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/broken\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "broken.go"), []byte("package broken\n\nfunc Broken( {\n"), 0o644); err != nil {
		t.Fatalf("write broken.go: %v", err)
	}

	ctx := context.Background()
	st := store.NewMemoryStore()
	report, err := fusion.EnrichTypedEdges(ctx, st, "broken-svc", dir)
	if err != nil {
		t.Fatalf("EnrichTypedEdges returned an error for a broken package; want counted failure, not a returned error: %v", err)
	}
	if report.PackagesFailed == 0 {
		t.Error("PackagesFailed = 0, want at least 1 for the syntax-broken package")
	}
	edges, err := st.TypedEdges(ctx, "broken-svc")
	if err != nil {
		t.Fatalf("TypedEdges: %v", err)
	}
	if len(edges) != 0 {
		t.Errorf("TypedEdges = %d, want 0 (a failed package must not persist partial edges)", len(edges))
	}
}

// TestEnrichTypedEdges_NoGoModule documents what EnrichTypedEdges itself does
// against a directory with no go.mod and no .go files: go/packages returns
// one synthetic, failing package placeholder rather than an empty set or a
// top-level error — counted as a failure, never returned as an error and
// never persisted. (The cheap, subprocess-free skip for this case lives one
// layer up, in the unexported Pipeline.enrichTypedEdges Ingest hook, gated
// on hasGoSource/hasGoModule before EnrichTypedEdges is ever called — this
// test only pins EnrichTypedEdges's own honesty contract when it IS called
// against such a directory.)
func TestEnrichTypedEdges_NoGoModule(t *testing.T) {
	dir := t.TempDir() // empty: no go.mod, no .go files
	ctx := context.Background()
	st := store.NewMemoryStore()

	start := time.Now()
	report, err := fusion.EnrichTypedEdges(ctx, st, "empty-svc", dir)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("EnrichTypedEdges returned an error for an empty dir; want a counted failure, not an error: %v", err)
	}
	if report.PackagesLoaded != 0 {
		t.Errorf("PackagesLoaded = %d, want 0", report.PackagesLoaded)
	}
	edges, edgeErr := st.TypedEdges(ctx, "empty-svc")
	if edgeErr != nil {
		t.Fatalf("TypedEdges: %v", edgeErr)
	}
	if len(edges) != 0 {
		t.Errorf("TypedEdges = %d, want 0", len(edges))
	}
	// Not asserted strictly (machine-dependent), but should be well under a
	// few seconds — go/packages fails fast on a dir with no module.
	if elapsed > 10*time.Second {
		t.Errorf("EnrichTypedEdges on an empty dir took %s, suspiciously slow", elapsed)
	}
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

func equalStrSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
