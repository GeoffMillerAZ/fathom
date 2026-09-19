package scip

import (
	"context"
	"os"
	"sort"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/store"
)

const fixtureService = "tsfixture-svc"

// extractFixtureNodes runs the REAL TypeScript extractor
// (internal/extractor.NewTSExtractor("javascript") — the same extractor
// fusion registers for .ts) over testdata/tsfixture/*.ts, exactly as an
// ingest would. It is the independent oracle TestImportIndex_
// MapsOccurrencesToTypedEdges joins ImportIndex's output against: SU-9
// requires every edge endpoint to be a symbol key the language's own
// extractor actually mints, not a key this package merely asserts matches.
func extractFixtureNodes(t *testing.T) []core.SymbolNode {
	t.Helper()
	e, err := extractor.NewTSExtractor("javascript")
	if err != nil {
		t.Fatalf("NewTSExtractor: %v", err)
	}
	files := []string{"math.ts", "greeter.ts", "main.ts"}
	var all []core.SymbolNode
	for _, f := range files {
		src, err := os.ReadFile("testdata/tsfixture/" + f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		nodes, err := e.Extract(fixtureService, f, src)
		if err != nil {
			t.Fatalf("extract %s: %v", f, err)
		}
		all = append(all, nodes...)
	}
	return all
}

func loadFixtureIndexBytes(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/tsfixture/index.scip")
	if err != nil {
		t.Fatalf("read testdata/tsfixture/index.scip: %v", err)
	}
	// The committed file must be exactly what BuildTSFixtureIndex produces
	// right now — never a stale binary a source edit silently outran.
	want, err := proto.Marshal(BuildTSFixtureIndex())
	if err != nil {
		t.Fatalf("marshal BuildTSFixtureIndex: %v", err)
	}
	if string(data) != string(want) {
		t.Fatalf("testdata/tsfixture/index.scip is stale — regenerate it from BuildTSFixtureIndex")
	}
	return data
}

// TestImportIndex_MapsOccurrencesToTypedEdges is SU-9's pinning test
// (docs/plans/substrate.md): decodes the committed fixture SCIP index,
// checks the exact edge set (from, to, kind) ImportIndex derives, and
// joins every endpoint against core.SymbolNode.Key() values the REAL
// tsextract.go extractor mints for the same fixture source — proving
// ImportIndex's package/qualifier convention is the extractor's, not a
// private one only this package agrees with itself about.
func TestImportIndex_MapsOccurrencesToTypedEdges(t *testing.T) {
	raw := loadFixtureIndexBytes(t)

	imp, err := ImportIndex(fixtureService, raw)
	if err != nil {
		t.Fatalf("ImportIndex: %v", err)
	}

	extracted := extractFixtureNodes(t)
	extractedKeys := map[string]bool{}
	for _, n := range extracted {
		extractedKeys[n.Key()] = true
	}

	addKey := core.SymbolKey(fixtureService, "math", "add")
	greeterKey := core.SymbolKey(fixtureService, "greeter", "Greeter")
	greetKey := core.SymbolKey(fixtureService, "greeter", "Greeter.greet")
	runKey := core.SymbolKey(fixtureService, "main", "run")

	for _, want := range []string{addKey, greeterKey, greetKey, runKey} {
		if !extractedKeys[want] {
			t.Fatalf("test setup: tsextract never minted %q — fixture/extractor convention mismatch", want)
		}
	}

	type pair struct {
		from, to string
		kind     core.EdgeKind
	}
	var got []pair
	for _, pkg := range imp.Packages {
		for _, e := range pkg.Edges {
			got = append(got, pair{e.FromSymbol, e.ToSymbol, e.Kind})
			if e.FromService != fixtureService {
				t.Errorf("edge %+v: FromService = %q, want %q", e, e.FromService, fixtureService)
			}
		}
	}
	sort.Slice(got, func(i, j int) bool {
		if got[i].from != got[j].from {
			return got[i].from < got[j].from
		}
		return got[i].to < got[j].to
	})

	want := []pair{
		{greetKey, addKey, core.EdgeDirect},
		{greetKey, unresolvedMarker("scip-typescript npm typescript 5.0.0 lib/console.log()."), core.EdgeUnresolved},
		{runKey, greetKey, core.EdgeDirect},
		{runKey, greeterKey, core.EdgeDirect},
	}
	sort.Slice(want, func(i, j int) bool {
		if want[i].from != want[j].from {
			return want[i].from < want[j].from
		}
		return want[i].to < want[j].to
	})

	if len(got) != len(want) {
		t.Fatalf("edges = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("edge[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}

	// Every RESOLVED edge's from/to must be a real, extracted symbol key —
	// SU-9's own join requirement. The one unresolved edge's To is a
	// synthetic marker by design and is exempt.
	for _, e := range got {
		if !extractedKeys[e.from] {
			t.Errorf("edge from %q is not an extracted symbol key", e.from)
		}
		if e.kind == core.EdgeDirect && !extractedKeys[e.to] {
			t.Errorf("resolved edge to %q is not an extracted symbol key", e.to)
		}
	}

	if imp.Report.Documents != 3 {
		t.Errorf("Report.Documents = %d, want 3", imp.Report.Documents)
	}
	if imp.Report.Direct != 3 || imp.Report.Unresolved != 1 || imp.Report.Edges != 4 {
		t.Errorf("Report = %+v, want Direct=3 Unresolved=1 Edges=4", imp.Report)
	}
	if imp.Report.Skipped["no-enclosing-definition"] != 2 {
		t.Errorf("Report.Skipped[no-enclosing-definition] = %d, want 2 (the two top-level import references)", imp.Report.Skipped["no-enclosing-definition"])
	}
	if imp.Report.Language != "typescript" {
		t.Errorf("Report.Language = %q, want %q", imp.Report.Language, "typescript")
	}
	if got, want := imp.Report.Producer.String(), "scip-typescript 0.3.16"; got != want {
		t.Errorf("Report.Producer.String() = %q, want %q", got, want)
	}

	// Package-granular grouping: "greeter" carries the two edges FROM
	// Greeter.greet (one direct, one unresolved); "main" carries the two
	// edges FROM run; "math" contributes no edges (add is only ever a
	// target here, never a caller).
	pkgEdgeCount := map[string]int{}
	for _, pkg := range imp.Packages {
		pkgEdgeCount[pkg.Package] = len(pkg.Edges)
	}
	if pkgEdgeCount["greeter"] != 2 {
		t.Errorf("package greeter edges = %d, want 2", pkgEdgeCount["greeter"])
	}
	if pkgEdgeCount["main"] != 2 {
		t.Errorf("package main edges = %d, want 2", pkgEdgeCount["main"])
	}
	if _, ok := pkgEdgeCount["math"]; ok {
		t.Errorf("package math should contribute no edges (nothing calls FROM it), got %d", pkgEdgeCount["math"])
	}
}

// TestImportIndex_PackageGranularReplacement is SU-9's second required
// test: importing an index that covers ONLY the "greeter" package (a
// stand-in for "greeter.ts changed, re-run scip-typescript, re-import")
// must replace exactly the "greeter" rows in the store — "math" and
// "main"'s previously-imported edges must survive untouched, the same
// contract internal/fusion/typedcalls.go's own per-package
// store.PutTypedEdges already guarantees for the native Go pass.
func TestImportIndex_PackageGranularReplacement(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()

	full, err := ImportIndex(fixtureService, loadFixtureIndexBytes(t))
	if err != nil {
		t.Fatalf("ImportIndex(full): %v", err)
	}
	for _, pkg := range full.Packages {
		if err := st.PutTypedEdges(ctx, fixtureService, pkg.Package, pkg.Edges); err != nil {
			t.Fatalf("PutTypedEdges(%s): %v", pkg.Package, err)
		}
	}
	before, err := st.TypedEdges(ctx, fixtureService)
	if err != nil {
		t.Fatalf("TypedEdges: %v", err)
	}
	if len(before) != 4 {
		t.Fatalf("edges after full import = %d, want 4", len(before))
	}

	// A greeter-only index: the same greeter.ts document, decoded alone —
	// simulating a re-import scoped to one changed file.
	greeterIdx := BuildTSFixtureIndex()
	greeterIdx.Documents = greeterIdx.Documents[1:2] // greeter.ts only
	data, err := proto.Marshal(greeterIdx)
	if err != nil {
		t.Fatalf("marshal greeter-only index: %v", err)
	}
	partial, err := ImportIndex(fixtureService, data)
	if err != nil {
		t.Fatalf("ImportIndex(partial): %v", err)
	}
	for _, pkg := range partial.Packages {
		if pkg.Package != "greeter" {
			t.Fatalf("partial import produced package %q, want only greeter", pkg.Package)
		}
		if err := st.PutTypedEdges(ctx, fixtureService, pkg.Package, pkg.Edges); err != nil {
			t.Fatalf("PutTypedEdges(%s): %v", pkg.Package, err)
		}
	}

	after, err := st.TypedEdges(ctx, fixtureService)
	if err != nil {
		t.Fatalf("TypedEdges: %v", err)
	}
	// The greeter-only reimport carries no reference to console.log's
	// import statement handling differences — same two edges as before,
	// so the total is unchanged; what matters is that "main"'s edges
	// (run -> Greeter, run -> Greeter.greet) are STILL present, proving
	// they were never touched by the greeter-scoped replace.
	if len(after) != 4 {
		t.Fatalf("edges after partial reimport = %d, want 4 (main's rows must survive untouched)", len(after))
	}
	runKey := core.SymbolKey(fixtureService, "main", "run")
	var mainEdges int
	for _, e := range after {
		if e.FromSymbol == runKey {
			mainEdges++
		}
	}
	if mainEdges != 2 {
		t.Errorf("edges FROM %q after partial reimport = %d, want 2 (untouched by the greeter-only PutTypedEdges)", runKey, mainEdges)
	}
}
