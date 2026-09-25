package scip

import (
	"context"
	"os"
	"sort"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	scippb "github.com/geoffmilleraz/fathom/third_party/scip"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/store"
)

const fixtureService = "tsfixture-svc"

// extractFixtureNodes runs the REAL TypeScript extractor
// (internal/extractor.NewTSExtractor("javascript") — the same extractor
// fusion registers for .ts) over testdata/tsfixture/*.ts, exactly as an
// ingest would. It is the independent oracle TestImportIndex_
// MapsOccurrencesToTypedEdges joins ImportIndex's output against, and (O115)
// the very input ImportIndex now maps every SCIP definition against — SU-9
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

// loadNoDisplayFixtureIndexBytes loads the O115 regression fixture: the
// SAME occurrences and symbol strings as index.scip, but with every
// SymbolInformation.DisplayName cleared — the shape scip-typescript 0.4.0
// and scip-python 0.6.6 actually produce, confirmed 0 of 23,583 / 5,551
// definitions carrying a DisplayName (.fathom/observations.md O115).
func loadNoDisplayFixtureIndexBytes(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/tsfixture/index-no-display.scip")
	if err != nil {
		t.Fatalf("read testdata/tsfixture/index-no-display.scip: %v", err)
	}
	want, err := proto.Marshal(BuildTSFixtureIndexNoDisplayNames())
	if err != nil {
		t.Fatalf("marshal BuildTSFixtureIndexNoDisplayNames: %v", err)
	}
	if string(data) != string(want) {
		t.Fatalf("testdata/tsfixture/index-no-display.scip is stale — regenerate it from BuildTSFixtureIndexNoDisplayNames")
	}
	return data
}

type edgePair struct {
	from, to string
	kind     core.EdgeKind
}

func edgePairs(imp Import) []edgePair {
	var got []edgePair
	for _, pkg := range imp.Packages {
		for _, e := range pkg.Edges {
			got = append(got, edgePair{e.FromSymbol, e.ToSymbol, e.Kind})
		}
	}
	sort.Slice(got, func(i, j int) bool {
		if got[i].from != got[j].from {
			return got[i].from < got[j].from
		}
		return got[i].to < got[j].to
	})
	return got
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
	extracted := extractFixtureNodes(t)

	imp, err := ImportIndex(fixtureService, raw, extracted)
	if err != nil {
		t.Fatalf("ImportIndex: %v", err)
	}

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

	got := edgePairs(imp)

	want := []edgePair{
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
	if imp.Report.Unmapped != 0 {
		t.Errorf("Report.Unmapped = %d, want 0 (every fixture definition sits at a real extracted node's line)", imp.Report.Unmapped)
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
	// O116 (.fathom/observations.md): "math" has a mapped definition (add)
	// but contributes no edges of its own (nothing in this fixture calls
	// FROM it) — it must still be LISTED, with zero edges, not absent. A
	// package present in Import.Packages is exactly what tells a caller
	// "this import knows about this package's edges, and they are these";
	// absence must never be how "the current count is zero" is spelled, or
	// a re-import that corrects a package from some edges down to none can
	// never clear the stale rows an earlier import left behind (see
	// TestImportIndex_O116_ReimportClearsPackageThatDropsToZeroEdges).
	mathEdges, ok := pkgEdgeCount["math"]
	if !ok {
		t.Errorf("package math is absent from Import.Packages, want present with 0 edges (O116)")
	} else if mathEdges != 0 {
		t.Errorf("package math edges = %d, want 0", mathEdges)
	}
}

// TestImportIndex_O116_ZeroEdgePackageStillListed is O116's own regression
// test (.fathom/observations.md): a package that has at least one mapped
// definition in this import must appear in Import.Packages even when it
// contributes zero edges — "math" here, whose sole definition (add) is
// never called FROM anywhere in this fixture. Before the fix,
// Import.Packages only ever listed the keys of the internal edgesByPkg map
// (populated only when at least one edge landed in a package), so a
// package genuinely part of this import's package set but computing to
// zero edges this run was indistinguishable, to every caller, from a
// package the import never touched at all.
func TestImportIndex_O116_ZeroEdgePackageStillListed(t *testing.T) {
	raw := loadFixtureIndexBytes(t)
	extracted := extractFixtureNodes(t)

	imp, err := ImportIndex(fixtureService, raw, extracted)
	if err != nil {
		t.Fatalf("ImportIndex: %v", err)
	}

	var mathPkg *PackageEdges
	for i := range imp.Packages {
		if imp.Packages[i].Package == "math" {
			mathPkg = &imp.Packages[i]
		}
	}
	if mathPkg == nil {
		t.Fatalf(`Import.Packages has no entry for "math" — a package with a mapped definition and zero edges must still be listed (O116)`)
	}
	if len(mathPkg.Edges) != 0 {
		t.Errorf("package math edges = %d, want 0", len(mathPkg.Edges))
	}
}

// TestImportIndex_O116_ReimportClearsPackageThatDropsToZeroEdges reproduces
// O116's exact shape end to end, against BOTH store backends
// (.fathom/observations.md): a full import writes 2 edges FROM
// Greeter.greet under package "greeter"; a later, CORRECTED re-import of
// the same document — its definitions (Greeter, Greeter.greet) still
// mapped, still "in the import's package set", but every REFERENCE
// occurrence gone, so it computes to zero edges this run — must still be
// looped through store.PutTypedEdges (because it is still present in
// Import.Packages, now with an empty Edges slice per the fix above),
// clearing "greeter"'s stale rows rather than leaving the first import's
// now-wrong edges standing forever. store.PutTypedEdges itself already
// clears a package's rows correctly the instant it is CALLED with an empty
// slice (internal/store/contract_test.go's
// "typed_edges_roundtrip_and_replace") — O116's gap was entirely that
// nothing ever called it for this package once it dropped to zero edges.
func TestImportIndex_O116_ReimportClearsPackageThatDropsToZeroEdges(t *testing.T) {
	backends := []struct {
		name  string
		store func(t *testing.T) store.Store
	}{
		{"memory", func(t *testing.T) store.Store { return store.NewMemoryStore() }},
		{"sqlite", func(t *testing.T) store.Store {
			s, err := store.NewSQLiteStore("")
			if err != nil {
				t.Fatalf("NewSQLiteStore: %v", err)
			}
			t.Cleanup(func() { _ = s.Close() })
			return s
		}},
	}

	for _, b := range backends {
		t.Run(b.name, func(t *testing.T) {
			ctx := context.Background()
			st := b.store(t)
			nodes := extractFixtureNodes(t)

			full, err := ImportIndex(fixtureService, loadFixtureIndexBytes(t), nodes)
			if err != nil {
				t.Fatalf("ImportIndex(full): %v", err)
			}
			for _, pkg := range full.Packages {
				if err := st.PutTypedEdges(ctx, fixtureService, pkg.Package, pkg.Edges); err != nil {
					t.Fatalf("PutTypedEdges(%s): %v", pkg.Package, err)
				}
			}

			greeterKey := core.SymbolKey(fixtureService, "greeter", "Greeter.greet")
			before, err := st.TypedEdges(ctx, fixtureService)
			if err != nil {
				t.Fatalf("TypedEdges: %v", err)
			}
			var beforeFromGreeter int
			for _, e := range before {
				if e.FromSymbol == greeterKey {
					beforeFromGreeter++
				}
			}
			if beforeFromGreeter != 2 {
				t.Fatalf("edges FROM %q before reimport = %d, want 2 (test setup)", greeterKey, beforeFromGreeter)
			}

			// The "corrected" re-import: greeter.ts's definitions survive
			// unchanged (still mapped), but every reference occurrence is
			// gone — the shape a stricter mapper produces when it decides
			// every one of Greeter.greet's former occurrences was actually
			// unmapped or external, the same kind of correction O115's real
			// fix made in production for some packages.
			greeterIdx := BuildTSFixtureIndex()
			greeterDoc := greeterIdx.Documents[1]
			if greeterDoc.RelativePath != "greeter.ts" {
				t.Fatalf("test setup: Documents[1] = %q, want greeter.ts", greeterDoc.RelativePath)
			}
			var defsOnly []*scippb.Occurrence
			for _, occ := range greeterDoc.Occurrences {
				if isDefinition(occ) {
					defsOnly = append(defsOnly, occ)
				}
			}
			greeterIdx.Documents = []*scippb.Document{{
				RelativePath: greeterDoc.RelativePath,
				Language:     greeterDoc.Language,
				Occurrences:  defsOnly,
				Symbols:      greeterDoc.Symbols,
			}}
			data, err := proto.Marshal(greeterIdx)
			if err != nil {
				t.Fatalf("marshal corrected greeter index: %v", err)
			}
			corrected, err := ImportIndex(fixtureService, data, nodes)
			if err != nil {
				t.Fatalf("ImportIndex(corrected): %v", err)
			}

			var greeterPkg *PackageEdges
			for i := range corrected.Packages {
				if corrected.Packages[i].Package == "greeter" {
					greeterPkg = &corrected.Packages[i]
				}
			}
			if greeterPkg == nil {
				t.Fatalf(`corrected import's Packages has no entry for "greeter" — O116 regressed (a package with mapped definitions and zero edges must still be listed)`)
			}
			if len(greeterPkg.Edges) != 0 {
				t.Fatalf("corrected greeter edges = %d, want 0 (test setup)", len(greeterPkg.Edges))
			}

			for _, pkg := range corrected.Packages {
				if err := st.PutTypedEdges(ctx, fixtureService, pkg.Package, pkg.Edges); err != nil {
					t.Fatalf("PutTypedEdges(%s): %v", pkg.Package, err)
				}
			}

			after, err := st.TypedEdges(ctx, fixtureService)
			if err != nil {
				t.Fatalf("TypedEdges after corrected reimport: %v", err)
			}
			for _, e := range after {
				if e.FromSymbol == greeterKey {
					t.Errorf("stale edge from %q survived the corrected reimport: %+v (O116)", greeterKey, e)
				}
			}
		})
	}
}

// TestImportIndex_O115_ResolvesWithoutDisplayName is O115's own regression
// test: scip-typescript 0.4.0 and scip-python 0.6.6 — the only published
// releases of each, confirmed live against real calibration clones — never
// set SymbolInformation.DisplayName. Decoding the SAME fixture with every
// DisplayName cleared (index-no-display.scip) must derive the EXACT SAME
// edge set as the DisplayName-populated fixture, entirely from definition
// POSITION against the real tsextract nodes — proving the fix does not
// depend on a producer ever setting DisplayName at all.
func TestImportIndex_O115_ResolvesWithoutDisplayName(t *testing.T) {
	extracted := extractFixtureNodes(t)

	withNames, err := ImportIndex(fixtureService, loadFixtureIndexBytes(t), extracted)
	if err != nil {
		t.Fatalf("ImportIndex(with display names): %v", err)
	}
	withoutNames, err := ImportIndex(fixtureService, loadNoDisplayFixtureIndexBytes(t), extracted)
	if err != nil {
		t.Fatalf("ImportIndex(without display names): %v", err)
	}

	gotWith := edgePairs(withNames)
	gotWithout := edgePairs(withoutNames)
	if len(gotWithout) == 0 {
		t.Fatalf("no edges resolved without DisplayName — O115 regressed")
	}
	if len(gotWith) != len(gotWithout) {
		t.Fatalf("edge count with DisplayName = %d, without = %d, want equal", len(gotWith), len(gotWithout))
	}
	for i := range gotWith {
		if gotWith[i] != gotWithout[i] {
			t.Errorf("edge[%d] with DisplayName = %+v, without = %+v — position-based mapping must agree", i, gotWith[i], gotWithout[i])
		}
	}

	if withoutNames.Report.Unmapped != 0 {
		t.Errorf("Report.Unmapped = %d, want 0 — every definition sits at a real extracted node's line even with no DisplayName", withoutNames.Report.Unmapped)
	}
	if withoutNames.Report.ResolvedViaFallback != 0 {
		t.Errorf("Report.ResolvedViaFallback = %d, want 0 — this fixture's positions are all unambiguous, the descriptor fallback should never fire", withoutNames.Report.ResolvedViaFallback)
	}

	// The join requirement holds without DisplayName too: every resolved
	// edge's endpoints are real extracted symbol keys.
	extractedKeys := map[string]bool{}
	for _, n := range extracted {
		extractedKeys[n.Key()] = true
	}
	for _, e := range gotWithout {
		if !extractedKeys[e.from] {
			t.Errorf("edge from %q is not an extracted symbol key", e.from)
		}
		if e.kind == core.EdgeDirect && !extractedKeys[e.to] {
			t.Errorf("resolved edge to %q is not an extracted symbol key", e.to)
		}
	}
}

// TestImportIndex_O115_UnmappedEndpointNeverEmitsEdge builds a tiny
// synthetic index (no committed fixture needed — this exercises the
// resolver's refusal path, not the fixture join) with one definition whose
// position matches no real node AND whose descriptor suffix names a leaf
// no node in that file carries either. It must never be guessed into an
// edge: a reference TO it, and every reference FROM it, are dropped and
// counted under Report.Skipped["unmapped-endpoint"] and Report.Unmapped —
// never fabricated as core.EdgeDirect (that would mistype a namespace
// collision as a real edge) and never mistyped core.EdgeUnresolved either
// (that means "external to this index", which an internally-defined but
// unmapped symbol is not).
func TestImportIndex_O115_UnmappedEndpointNeverEmitsEdge(t *testing.T) {
	const (
		symCaller  = "scip-test npm fixture 0.0.1 mod/caller()."
		symMystery = "scip-test npm fixture 0.0.1 mod/`weird thing`#"
	)
	doc := &scippb.Document{
		RelativePath: "mod.ts",
		Language:     "typescript",
		Occurrences: []*scippb.Occurrence{
			{Range: []int32{0, 9, 0, 15}, EnclosingRange: []int32{0, 0, 2, 1}, Symbol: symCaller, SymbolRoles: int32(scippb.SymbolRole_Definition)},
			// A definition whose position (line 10) matches no supplied
			// node, and whose descriptor leaf ("weird thing", backtick
			// quoted) matches no node's name in this file either.
			{Range: []int32{10, 0, 10, 4}, Symbol: symMystery, SymbolRoles: int32(scippb.SymbolRole_Definition)},
			// caller's body references the unmapped definition.
			{Range: []int32{1, 4, 1, 8}, Symbol: symMystery},
		},
		Symbols: []*scippb.SymbolInformation{
			{Symbol: symCaller, DisplayName: ""},
			{Symbol: symMystery, DisplayName: ""},
		},
	}
	idx := &scippb.Index{
		Metadata:  &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-test", Version: "0.0.1"}},
		Documents: []*scippb.Document{doc},
	}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	nodes := []core.SymbolNode{
		{Service: "svc", Package: "mod", Name: "caller", FilePath: "mod.ts", Line: 1},
	}
	imp, err := ImportIndex("svc", raw, nodes)
	if err != nil {
		t.Fatalf("ImportIndex: %v", err)
	}

	if got := edgePairs(imp); len(got) != 0 {
		t.Fatalf("edges = %+v, want none — an edge to/from an unmapped definition must never be emitted", got)
	}
	if imp.Report.Unmapped != 1 {
		t.Errorf("Report.Unmapped = %d, want 1 (symMystery's own definition)", imp.Report.Unmapped)
	}
	if imp.Report.Skipped["unmapped-endpoint"] != 1 {
		t.Errorf(`Report.Skipped["unmapped-endpoint"] = %d, want 1 (the reference to symMystery)`, imp.Report.Skipped["unmapped-endpoint"])
	}
	if imp.Report.Direct != 0 || imp.Report.Unresolved != 0 {
		t.Errorf("Report = %+v, want Direct=0 Unresolved=0 — never mistype an unmapped-but-internal reference as either", imp.Report)
	}
}

// TestImportIndex_O115_DescriptorFallback_AmbiguousPosition proves the
// SECOND half of O115's fix: when two supplied nodes share a definition's
// exact file+line (the position match is ambiguous, not merely absent),
// ImportIndex falls back to the SCIP symbol's own descriptor suffix and
// resolves against the leaf name — never picking arbitrarily between the
// tied candidates.
func TestImportIndex_O115_DescriptorFallback_AmbiguousPosition(t *testing.T) {
	const symGreet = "scip-test npm fixture 0.0.1 mod/Widget#greet()."
	doc := &scippb.Document{
		RelativePath: "mod.ts",
		Language:     "typescript",
		Occurrences: []*scippb.Occurrence{
			{Range: []int32{4, 4, 4, 9}, EnclosingRange: []int32{4, 4, 6, 5}, Symbol: symGreet, SymbolRoles: int32(scippb.SymbolRole_Definition)},
		},
		Symbols: []*scippb.SymbolInformation{{Symbol: symGreet, DisplayName: ""}},
	}
	idx := &scippb.Index{Documents: []*scippb.Document{doc}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Two nodes tied at the exact same file+line (a real case: a
	// decorator or an overload signature can leave two extractor-minted
	// symbols at one line) — the position match must be refused as
	// ambiguous, and the descriptor suffix's leaf ("greet") must pick out
	// the ONE candidate whose own name ends in ".greet".
	nodes := []core.SymbolNode{
		{Service: "svc", Package: "mod", Name: "Widget.greet", FilePath: "mod.ts", Line: 5},
		{Service: "svc", Package: "mod", Name: "Widget.greetOverload", FilePath: "mod.ts", Line: 5},
	}
	imp, err := ImportIndex("svc", raw, nodes)
	if err != nil {
		t.Fatalf("ImportIndex: %v", err)
	}
	if imp.Report.Unmapped != 0 {
		t.Fatalf("Report.Unmapped = %d, want 0 — the descriptor fallback should have resolved it", imp.Report.Unmapped)
	}
	if imp.Report.ResolvedViaFallback != 1 {
		t.Errorf("Report.ResolvedViaFallback = %d, want 1", imp.Report.ResolvedViaFallback)
	}
}

// TestImportIndex_O115_ParameterNeverCollapsesOntoEnclosingFunction is a
// hand-verified real-world regression: a scip-python import of flask's
// `Flask.raise_routing_exception` (which never calls itself) produced a
// spurious `raise_routing_exception -> raise_routing_exception` edge for
// every reference to its OWN `self`/`request` parameters, because a
// parameter's SCIP definition sits on the EXACT SAME LINE as its
// enclosing function's own signature, and fathom's own extractors never
// mint a SymbolNode for a parameter — so a naive position match silently
// joined "this parameter" to "the function that declares it". This
// fixture reproduces that shape minimally: one function definition and
// its "request" parameter's OWN definition, both on line 0, plus a
// reference to the parameter inside the function's body.
func TestImportIndex_O115_ParameterNeverCollapsesOntoEnclosingFunction(t *testing.T) {
	const (
		symFoo     = "scip-python python . v1 mod/foo()."
		symFooReq  = "scip-python python . v1 mod/foo().(request)"
		symUnrelat = "scip-python python . v1 mod/helper()."
	)
	doc := &scippb.Document{
		RelativePath: "mod.py",
		Language:     "python",
		Occurrences: []*scippb.Occurrence{
			// def foo(request):  <- both foo's own def and request's own
			// param-def sit on line 0, exactly like a real Python/TS
			// signature.
			{Range: []int32{0, 4, 0, 7}, EnclosingRange: []int32{0, 0, 2, 0}, Symbol: symFoo, SymbolRoles: int32(scippb.SymbolRole_Definition)},
			{Range: []int32{0, 8, 0, 15}, Symbol: symFooReq, SymbolRoles: int32(scippb.SymbolRole_Definition)},
			// A separate function, defined elsewhere in the file, so the
			// genuine foo->helper call has a real, mapped target to
			// resolve to.
			{Range: []int32{4, 4, 4, 10}, EnclosingRange: []int32{4, 0, 4, 20}, Symbol: symUnrelat, SymbolRoles: int32(scippb.SymbolRole_Definition)},
			// return helper(request) — a genuine call, plus a reference
			// to request itself (the exact shape that produced the real
			// flask self-loop: `self.debug`/`request.routing_exception`).
			{Range: []int32{1, 11, 1, 17}, Symbol: symUnrelat},
			{Range: []int32{1, 18, 1, 25}, Symbol: symFooReq},
			{Range: []int32{2, 0, 2, 4}, Symbol: symFooReq},
		},
	}
	idx := &scippb.Index{Documents: []*scippb.Document{doc}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	nodes := []core.SymbolNode{
		{Service: "svc", Package: "mod", Name: "foo", FilePath: "mod.py", Line: 1},
		{Service: "svc", Package: "mod", Name: "helper", FilePath: "mod.py", Line: 5},
	}
	imp, err := ImportIndex("svc", raw, nodes)
	if err != nil {
		t.Fatalf("ImportIndex: %v", err)
	}

	fooKey := core.SymbolKey("svc", "mod", "foo")
	helperKey := core.SymbolKey("svc", "mod", "helper")
	got := edgePairs(imp)

	for _, e := range got {
		if e.from == fooKey && e.to == fooKey {
			t.Fatalf("edges = %+v: foo -> foo self-loop must never be emitted — a parameter reference collapsed onto its enclosing function", got)
		}
	}
	want := []edgePair{{fooKey, helperKey, core.EdgeDirect}}
	if len(got) != len(want) || got[0] != want[0] {
		t.Errorf("edges = %+v, want %+v (only the genuine foo->helper call; every reference to `request` must be dropped, not mapped)", got, want)
	}
	// The parameter's own definition must count as unmapped, never as a
	// silent match.
	if imp.Report.Unmapped < 1 {
		t.Errorf("Report.Unmapped = %d, want at least 1 (request's own parameter definition)", imp.Report.Unmapped)
	}
}

// TestImportIndex_O115_LocalSymbolsAreDocumentScoped is a hand-verified
// real-world regression: SCIP's own spec reserves "local <id>" for a
// symbol unique only WITHIN its enclosing document (freely reused,
// unrelated, by every other document — a file's first local is commonly
// "local 0" everywhere). A real scip-typescript import wired a method
// that only ever referenced its own destructured local variable to an
// UNRELATED type in a different test file, because that other file's
// definition of the identically-numbered local happened to map (by
// position) to a real fathom symbol, and a flat, document-unaware
// scipSymbol->docDef table let the two collide. This fixture reproduces
// that shape minimally: doc b defines "local 5" (mapped, by contrivance,
// onto a real "target" symbol); doc a's own function references "local 5"
// too, but NEVER defines it itself — the reference must resolve as
// EXTERNAL (unresolved), never as a direct edge to b's unrelated target.
func TestImportIndex_O115_LocalSymbolsAreDocumentScoped(t *testing.T) {
	docA := &scippb.Document{
		RelativePath: "a.ts",
		Language:     "typescript",
		Occurrences: []*scippb.Occurrence{
			{Range: []int32{0, 9, 0, 12}, EnclosingRange: []int32{0, 0, 2, 1}, Symbol: "scip-test npm fixture 0.0.1 a/aFn().", SymbolRoles: int32(scippb.SymbolRole_Definition)},
			// aFn references "local 5" — but a.ts itself never DEFINES a
			// "local 5" of its own in this fixture.
			{Range: []int32{1, 4, 1, 11}, Symbol: "local 5"},
		},
	}
	docB := &scippb.Document{
		RelativePath: "b.ts",
		Language:     "typescript",
		Occurrences: []*scippb.Occurrence{
			// b.ts's OWN "local 5", positioned to map (by position) onto
			// a real fathom node named "target" — standing in for the
			// real-world case where a producer's local numbering happens
			// to land a local definition on a line fathom's own extractor
			// also minted a symbol at.
			{Range: []int32{4, 0, 4, 6}, Symbol: "local 5", SymbolRoles: int32(scippb.SymbolRole_Definition)},
		},
	}
	idx := &scippb.Index{Documents: []*scippb.Document{docA, docB}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	nodes := []core.SymbolNode{
		{Service: "svc", Package: "a", Name: "aFn", FilePath: "a.ts", Line: 1},
		{Service: "svc", Package: "b", Name: "target", FilePath: "b.ts", Line: 5},
	}
	imp, err := ImportIndex("svc", raw, nodes)
	if err != nil {
		t.Fatalf("ImportIndex: %v", err)
	}

	aFnKey := core.SymbolKey("svc", "a", "aFn")
	targetKey := core.SymbolKey("svc", "b", "target")
	for _, pkg := range imp.Packages {
		for _, e := range pkg.Edges {
			if e.ToSymbol == targetKey {
				t.Fatalf("edge %+v: a.ts's reference to its own \"local 5\" must never resolve to b.ts's unrelated %q — local symbols are document-scoped", e, targetKey)
			}
		}
	}
	// The reference must instead be honestly external (unresolved) — a.ts
	// never defines a "local 5" of its own.
	found := false
	for _, pkg := range imp.Packages {
		for _, e := range pkg.Edges {
			if e.FromSymbol == aFnKey && e.Kind == core.EdgeUnresolved && e.ToSymbol == unresolvedMarker("local 5") {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("expected an unresolved edge aFn -> <unresolved:local 5>, got packages=%+v", imp.Packages)
	}
}

// TestDescriptorIsParameterLike exercises the Parameter/TypeParameter
// guard directly against real descriptor shapes (a Method's mandatory
// trailing '.' is what distinguishes it from a bare Parameter).
func TestDescriptorIsParameterLike(t *testing.T) {
	cases := []struct {
		sym  string
		want bool
	}{
		{"scip-python python . v1 mod/foo().(request)", true},                     // Parameter
		{"scip-typescript npm tsfixture 0.0.1 greeter/Greeter#greet().[T]", true}, // TypeParameter
		{"scip-typescript npm tsfixture 0.0.1 math/add().", false},                // Method
		{"scip-typescript npm tsfixture 0.0.1 greeter/Greeter#", false},           // Type
		{"scip-python python . v1 mod/CONST.", false},                             // Term
		{"local 3", false},
	}
	for _, c := range cases {
		if got := descriptorIsParameterLike(c.sym); got != c.want {
			t.Errorf("descriptorIsParameterLike(%q) = %v, want %v", c.sym, got, c.want)
		}
	}
}

// TestDescriptorLeafName exercises the SCIP descriptor-suffix parser
// (O115's fallback) against real symbol strings: the fixture's own, and
// the exact real-producer string .fathom/observations.md O115 records from
// scip-typescript 0.4.0 against zod.
func TestDescriptorLeafName(t *testing.T) {
	cases := []struct {
		sym      string
		wantLeaf string
		wantOK   bool
	}{
		{"scip-typescript npm tsfixture 0.0.1 math/add().", "add", true},
		{"scip-typescript npm tsfixture 0.0.1 greeter/Greeter#", "Greeter", true},
		{"scip-typescript npm tsfixture 0.0.1 greeter/Greeter#greet().", "greet", true},
		{"scip-typescript npm tsfixture 0.0.1 main/run().", "run", true},
		{"scip-typescript npm typescript 5.0.0 lib/console.log().", "log", true},
		// The real string O115 recorded from scip-typescript 0.4.0 against
		// zod: a backtick-quoted file-name namespace segment must not be
		// split on the '.' inside it.
		{"scip-typescript npm zod 4.4.3 src/v4/core/`registries.ts`/$output#", "$output", true},
		// A "local" symbol carries no package/descriptor path at all —
		// there is no name to recover.
		{"local 3", "", false},
	}
	for _, c := range cases {
		leaf, ok := descriptorLeafName(c.sym)
		if leaf != c.wantLeaf || ok != c.wantOK {
			t.Errorf("descriptorLeafName(%q) = (%q, %v), want (%q, %v)", c.sym, leaf, ok, c.wantLeaf, c.wantOK)
		}
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
	nodes := extractFixtureNodes(t)

	full, err := ImportIndex(fixtureService, loadFixtureIndexBytes(t), nodes)
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
	partial, err := ImportIndex(fixtureService, data, nodes)
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

// TestImportIndex_FindCallers_ResolvesThroughImportedEdges is the
// end-to-end O115 regression the plan asks for: with NO DisplayName
// anywhere in the index (the real-producer shape), the edges ImportIndex
// derives and a store persists must let a find_callers-style reverse
// lookup (internal/analysis/callgraph, the exact graph
// internal/mcp.callerGraph builds for the find_callers tool) resolve a
// caller through the imported edge — proving the fix restores the
// capability O115 found silently broken, not merely that the edge
// structs look right in isolation.
func TestImportIndex_FindCallers_ResolvesThroughImportedEdges(t *testing.T) {
	nodes := extractFixtureNodes(t)
	imp, err := ImportIndex(fixtureService, loadNoDisplayFixtureIndexBytes(t), nodes)
	if err != nil {
		t.Fatalf("ImportIndex: %v", err)
	}

	ctx := context.Background()
	st := store.NewMemoryStore()
	// Observations are how the store's ListAllSymbols/callerGraph path
	// (internal/mcp/tools.go's own callerGraph) recovers SymbolNodes —
	// round-trip each extracted node through it exactly like a real
	// ingest would.
	epoch := time.Unix(0, 0).UTC()
	obsIn := make([]core.Observation, 0, len(nodes))
	for _, n := range nodes {
		obsIn = append(obsIn, core.Observation{
			ServiceID:    n.Service,
			SourceRef:    n.FilePath,
			SourceKind:   n.Kind,
			EvidenceTier: n.Tier,
			Symbol:       n.Name,
			Package:      n.Package,
			Claim:        n.Signature,
			Calls:        n.Calls,
			Line:         n.Line,
			EndLine:      n.EndLine,
			IngestedAt:   epoch,
		})
	}
	if err := st.PutObservations(ctx, obsIn); err != nil {
		t.Fatalf("PutObservations: %v", err)
	}
	for _, pkg := range imp.Packages {
		if err := st.PutTypedEdges(ctx, fixtureService, pkg.Package, pkg.Edges); err != nil {
			t.Fatalf("PutTypedEdges(%s): %v", pkg.Package, err)
		}
	}

	obs, err := st.QueryObservations(ctx, fixtureService, "")
	if err != nil {
		t.Fatalf("QueryObservations: %v", err)
	}
	storedNodes := make([]core.SymbolNode, 0, len(obs))
	for _, o := range obs {
		storedNodes = append(storedNodes, o.ToSymbolNode())
	}
	typed, err := st.TypedEdges(ctx, fixtureService)
	if err != nil {
		t.Fatalf("TypedEdges: %v", err)
	}

	// The exact graph internal/mcp/tools.go's callerGraph builds for the
	// find_callers tool: syntax edges from every indexed symbol, plus the
	// typed pass's edges (here, the SCIP-imported ones) layered on top.
	g := callgraph.Build(storedNodes)
	callgraph.AddTypedEdges(g, typed)

	runKey := core.SymbolKey(fixtureService, "main", "run")
	greetKey := core.SymbolKey(fixtureService, "greeter", "Greeter.greet")

	callers := callgraph.CallersDetail(g, greetKey)
	found := false
	for _, c := range callers {
		if c.Key == runKey {
			found = true
		}
	}
	if !found {
		t.Fatalf("find_callers(%q) callers = %+v, want it to include %q — O115 regressed: the imported edge never joined the symbol table", greetKey, callers, runKey)
	}
}
