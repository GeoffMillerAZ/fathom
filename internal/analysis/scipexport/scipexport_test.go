package scipexport

import (
	"context"
	"testing"

	"google.golang.org/protobuf/proto"

	scip "github.com/geoffmilleraz/fathom/third_party/scip"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// fixtureNodes' FilePaths are deliberately root-level (not under a "pkg/"
// subdirectory matching the package name) so core.SymbolNode.Key()
// (QualifiedPackage: path.Dir(filePath) == "." leaves the bare package name
// unqualified) yields the simple "svc::pkg.A" keys this test's fixtureTypedEdges
// assume — a nested "pkg/a.go" would instead directory-qualify to
// "svc::pkg/pkg.A", which is Key()'s correct (if unintuitive) contract, not
// a bug, but would make these fixtures harder to read.
func fixtureNodes() []core.SymbolNode {
	return []core.SymbolNode{
		{Service: "svc", Package: "pkg", Name: "A", Kind: core.KindGoFunc, FilePath: "a.go", Line: 10, EndLine: 12, Signature: "func A()", Calls: []string{"svc::pkg.B"}},
		{Service: "svc", Package: "pkg", Name: "B", Kind: core.KindGoFunc, FilePath: "a.go", Line: 20, Signature: "func B()"},
		{Service: "svc", Package: "pkg", Name: "MyType", Kind: core.KindGoType, FilePath: "types.go", Line: 1},
	}
}

func fixtureTypedEdges() []core.Edge {
	return []core.Edge{
		{FromSymbol: "svc::pkg.A", ToSymbol: "svc::pkg.B", Kind: core.EdgeDirect},
		{FromSymbol: "svc::pkg.A", ToSymbol: "svc::pkg.Nowhere", Kind: core.EdgeDirect}, // unresolved target: skipped
	}
}

// ---------------------------------------------------------------------
// SymbolID / formatSymbolID
// ---------------------------------------------------------------------

func TestSymbolID_Deterministic(t *testing.T) {
	a := SymbolID("svc", "pkg", "A", core.KindGoFunc)
	b := SymbolID("svc", "pkg", "A", core.KindGoFunc)
	if a != b {
		t.Fatalf("SymbolID not deterministic: %q vs %q", a, b)
	}
	if a == SymbolID("svc", "pkg", "B", core.KindGoFunc) {
		t.Fatalf("SymbolID collided for different names")
	}
	if a == SymbolID("other-svc", "pkg", "A", core.KindGoFunc) {
		t.Fatalf("SymbolID collided across services")
	}
}

// TestSymbolID_Format pins the exact rendered grammar (scheme SP manager SP
// name SP version SP descriptors) against a literal string, rather than
// parsing it back — third_party/scip vendors only the generated wire-format
// types, not the hand-written parser (see its README.md), so fathom's own
// producer and consumer of this format agree by construction, not by
// round-tripping through a parser this package doesn't have.
func TestSymbolID_Format(t *testing.T) {
	cases := []struct {
		name string
		kind core.SourceKind
		want string
	}{
		{"function: term suffix", core.KindGoFunc, "fathom service svc . pkg/A."},
		{"variable: term suffix", core.KindGoVar, "fathom service svc . pkg/A."},
		{"const: term suffix", core.KindGoConst, "fathom service svc . pkg/A."},
		{"method: method suffix", core.KindGoMethod, "fathom service svc . pkg/A()."},
		{"type: type suffix", core.KindGoType, "fathom service svc . pkg/A#"},
		{"interface: type suffix", core.KindGoInterface, "fathom service svc . pkg/A#"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := SymbolID("svc", "pkg", "A", tc.kind); got != tc.want {
				t.Errorf("SymbolID(svc, pkg, A, %v) = %q, want %q", tc.kind, got, tc.want)
			}
		})
	}
}

func TestSymbolID_EscapesSpacesAndBackticks(t *testing.T) {
	got := SymbolID("my svc", "pkg", "A", core.KindGoFunc)
	want := "fathom service `my svc` . pkg/A."
	if got != want {
		t.Errorf("SymbolID with a spaced service name = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------
// BuildFromNodes: documents, occurrences, ranges
// ---------------------------------------------------------------------

func TestBuildFromNodes_DocumentsAndOccurrences(t *testing.T) {
	idx := BuildFromNodes("svc", fixtureNodes(), nil, nil)

	if idx.Metadata == nil || idx.Metadata.ToolInfo == nil || idx.Metadata.ToolInfo.Name != "fathom" {
		t.Fatalf("Metadata.ToolInfo.Name = %+v, want fathom", idx.Metadata)
	}
	if len(idx.Documents) != 2 {
		t.Fatalf("got %d documents, want 2 (a.go, types.go): %+v", len(idx.Documents), idx.Documents)
	}
	// Sorted by RelativePath.
	if idx.Documents[0].RelativePath != "a.go" || idx.Documents[1].RelativePath != "types.go" {
		t.Fatalf("documents not sorted by path: %q, %q", idx.Documents[0].RelativePath, idx.Documents[1].RelativePath)
	}

	docA := idx.Documents[0]
	if len(docA.Occurrences) != 2 {
		t.Fatalf("a.go: got %d occurrences, want 2 (A, B)", len(docA.Occurrences))
	}
	if len(docA.Symbols) != 2 {
		t.Fatalf("a.go: got %d symbols, want 2 (A, B)", len(docA.Symbols))
	}

	var aOcc *scip.Occurrence
	wantA := SymbolID("svc", "pkg", "A", core.KindGoFunc)
	for _, o := range docA.Occurrences {
		if o.Symbol == wantA {
			aOcc = o
		}
	}
	if aOcc == nil {
		t.Fatalf("no occurrence found for A's symbol %q", wantA)
	}
	// Range is a SYNTHETIC one-column name-marker at the start line only
	// (10, 0-based 9) — never the whole body span.
	if got, want := aOcc.Range, []int32{9, 0, 9, 1}; !int32SliceEqual(got, want) {
		t.Errorf("A occurrence Range = %v, want %v (a synthetic name-marker, not the body span)", got, want)
	}
	// EnclosingRange carries the real body span, Line..EndLine (10..12,
	// 0-based 9..11).
	if got, want := aOcc.EnclosingRange, []int32{9, 0, 11, 1}; !int32SliceEqual(got, want) {
		t.Errorf("A occurrence EnclosingRange = %v, want %v", got, want)
	}
	if aOcc.SymbolRoles != int32(scip.SymbolRole_Definition) {
		t.Errorf("A occurrence SymbolRoles = %d, want Definition", aOcc.SymbolRoles)
	}
}

func TestBuildFromNodes_RangeFallsBackToLineWhenNoEndLine(t *testing.T) {
	// B has no EndLine set (fixtureNodes leaves it zero) — EnclosingRange
	// must fall back to Line..Line, per T5-5's "0 == unknown" convention.
	idx := BuildFromNodes("svc", fixtureNodes(), nil, nil)
	wantB := SymbolID("svc", "pkg", "B", core.KindGoFunc)
	var bOcc *scip.Occurrence
	for _, d := range idx.Documents {
		for _, o := range d.Occurrences {
			if o.Symbol == wantB {
				bOcc = o
			}
		}
	}
	if bOcc == nil {
		t.Fatal("no occurrence found for B")
	}
	if got, want := bOcc.EnclosingRange, []int32{19, 0, 19, 1}; !int32SliceEqual(got, want) {
		t.Errorf("B EnclosingRange = %v, want %v (EndLine unset, falls back to Line)", got, want)
	}
}

// ---------------------------------------------------------------------
// BuildFromNodes: typed edges -> Relationships + resolution-tier Documentation
// ---------------------------------------------------------------------

func TestBuildFromNodes_TypedEdgesBecomeRelationshipsWithTier(t *testing.T) {
	idx := BuildFromNodes("svc", fixtureNodes(), fixtureTypedEdges(), nil)

	aInfo := findSymbolInfo(idx, SymbolID("svc", "pkg", "A", core.KindGoFunc))
	if aInfo == nil {
		t.Fatalf("no SymbolInformation found for A")
	}

	// Exactly one relationship: A -> B. A -> Nowhere is skipped (unresolved
	// target, not among this service's own indexed nodes).
	if len(aInfo.Relationships) != 1 {
		t.Fatalf("A.Relationships = %+v, want exactly 1 (A->B; A->Nowhere must be skipped)", aInfo.Relationships)
	}
	wantB := SymbolID("svc", "pkg", "B", core.KindGoFunc)
	rel := aInfo.Relationships[0]
	if rel.Symbol != wantB || !rel.IsReference {
		t.Errorf("relationship = %+v, want {Symbol:%q IsReference:true}", rel, wantB)
	}

	// The resolution tier travels as a Documentation line keyed by the
	// target symbol string (package doc comment's documented contract).
	found := false
	wantLine := "fathom:resolution " + wantB + " = " + string(core.EdgeConfidenceOf(core.Edge{Kind: core.EdgeDirect}))
	for _, d := range aInfo.Documentation {
		if d == wantLine {
			found = true
		}
	}
	if !found {
		t.Errorf("A.Documentation = %+v, want a line %q", aInfo.Documentation, wantLine)
	}
	// The signature line stays first, ahead of any resolution line.
	if len(aInfo.Documentation) == 0 || aInfo.Documentation[0] != "func A()" {
		t.Errorf("A.Documentation[0] = %+v, want the signature line first", aInfo.Documentation)
	}
}

func TestBuildFromNodes_HeuristicEdgeTier(t *testing.T) {
	// A syntax-only edge (Kind == "") must be labeled heuristic, never
	// resolved — core.EdgeConfidenceOf's own contract.
	edges := []core.Edge{{FromSymbol: "svc::pkg.A", ToSymbol: "svc::pkg.B", Kind: ""}}
	idx := BuildFromNodes("svc", fixtureNodes(), edges, nil)

	aInfo := findSymbolInfo(idx, SymbolID("svc", "pkg", "A", core.KindGoFunc))
	if aInfo == nil {
		t.Fatal("no SymbolInformation for A")
	}
	wantB := SymbolID("svc", "pkg", "B", core.KindGoFunc)
	wantLine := "fathom:resolution " + wantB + " = heuristic"
	if len(aInfo.Documentation) == 0 || aInfo.Documentation[len(aInfo.Documentation)-1] != wantLine {
		t.Errorf("A.Documentation = %+v, want to end with %q", aInfo.Documentation, wantLine)
	}
}

// TestBuildFromNodes_RelationshipsSortedRegardlessOfInputOrder is the NIT
// fix's real proof (wave-i review): the SAME edges, fed in two different
// orders, must produce byte-identical output — this package sorts
// Relationships/Documentation itself rather than trusting the caller
// (a store) to already return them in a stable order.
func TestBuildFromNodes_RelationshipsSortedRegardlessOfInputOrder(t *testing.T) {
	nodes := []core.SymbolNode{
		{Service: "svc", Package: "pkg", Name: "A", Kind: core.KindGoFunc, FilePath: "a.go", Line: 1},
		{Service: "svc", Package: "pkg", Name: "B", Kind: core.KindGoFunc, FilePath: "a.go", Line: 2},
		{Service: "svc", Package: "pkg", Name: "C", Kind: core.KindGoFunc, FilePath: "a.go", Line: 3},
	}
	forward := []core.Edge{
		{FromSymbol: "svc::pkg.A", ToSymbol: "svc::pkg.B", Kind: core.EdgeDirect},
		{FromSymbol: "svc::pkg.A", ToSymbol: "svc::pkg.C", Kind: core.EdgeDirect},
	}
	reversed := []core.Edge{forward[1], forward[0]}

	idxForward := BuildFromNodes("svc", nodes, forward, nil)
	idxReversed := BuildFromNodes("svc", nodes, reversed, nil)

	if !proto.Equal(idxForward, idxReversed) {
		t.Fatalf("BuildFromNodes depends on typed-edge input order:\nforward=%v\nreversed=%v", idxForward, idxReversed)
	}

	aInfo := findSymbolInfo(idxForward, SymbolID("svc", "pkg", "A", core.KindGoFunc))
	if aInfo == nil || len(aInfo.Relationships) != 2 {
		t.Fatalf("A.Relationships = %+v, want 2", aInfo)
	}
	if aInfo.Relationships[0].Symbol > aInfo.Relationships[1].Symbol {
		t.Errorf("Relationships not sorted: %+v", aInfo.Relationships)
	}
}

func TestBuildFromNodes_LanguageFromSources(t *testing.T) {
	srcs := []core.SourceFile{{Path: "a.go", Lang: "go"}}
	idx := BuildFromNodes("svc", fixtureNodes(), nil, srcs)
	for _, d := range idx.Documents {
		if d.RelativePath == "a.go" && d.Language != "go" {
			t.Errorf("a.go Language = %q, want go", d.Language)
		}
		if d.RelativePath == "types.go" && d.Language != "" {
			t.Errorf("types.go Language = %q, want empty (no Sources entry)", d.Language)
		}
	}
}

func TestBuildFromNodes_Deterministic(t *testing.T) {
	nodes := fixtureNodes()
	edges := fixtureTypedEdges()
	first := BuildFromNodes("svc", nodes, edges, nil)
	for i := 0; i < 10; i++ {
		again := BuildFromNodes("svc", nodes, edges, nil)
		if !proto.Equal(first, again) {
			t.Fatalf("BuildFromNodes is not deterministic across repeated calls (iteration %d)", i)
		}
	}
}

func TestSymbolSet(t *testing.T) {
	idx := BuildFromNodes("svc", fixtureNodes(), nil, nil)
	set := SymbolSet(idx)
	if len(set) != 3 {
		t.Fatalf("SymbolSet = %v, want 3 entries (A, B, MyType)", set)
	}
	if !set[SymbolID("svc", "pkg", "A", core.KindGoFunc)] {
		t.Errorf("SymbolSet missing A")
	}
	if !set[SymbolID("svc", "pkg", "B", core.KindGoFunc)] {
		t.Errorf("SymbolSet missing B")
	}
	if !set[SymbolID("svc", "pkg", "MyType", core.KindGoType)] {
		t.Errorf("SymbolSet missing MyType")
	}
}

// ---------------------------------------------------------------------
// Build (backend-driven)
// ---------------------------------------------------------------------

type fakeBackend struct {
	nodes   []core.SymbolNode
	typed   []core.Edge
	sources []core.SourceFile
	err     error
}

func (f fakeBackend) ListAllSymbols(_ context.Context, _ string) ([]core.SymbolNode, error) {
	return f.nodes, f.err
}

func (f fakeBackend) TypedEdges(_ context.Context, _ string) ([]core.Edge, error) {
	return f.typed, nil
}

func (f fakeBackend) Sources(_ context.Context, _ string) ([]core.SourceFile, error) {
	return f.sources, nil
}

func TestBuild_UsesBackend(t *testing.T) {
	backend := fakeBackend{nodes: fixtureNodes(), typed: fixtureTypedEdges()}
	idx, err := Build(context.Background(), backend, "svc")
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(idx.Documents) != 2 {
		t.Fatalf("Build: got %d documents, want 2", len(idx.Documents))
	}
}

func TestBuild_PropagatesListError(t *testing.T) {
	backend := fakeBackend{err: errBoom}
	if _, err := Build(context.Background(), backend, "svc"); err == nil {
		t.Fatal("Build: want error when ListAllSymbols fails")
	}
}

var errBoom = &boomError{}

type boomError struct{}

func (*boomError) Error() string { return "boom" }

// ---------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------

func int32SliceEqual(a, b []int32) bool {
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

func findSymbolInfo(idx *scip.Index, symbol string) *scip.SymbolInformation {
	for _, d := range idx.Documents {
		for _, s := range d.Symbols {
			if s.Symbol == symbol {
				return s
			}
		}
	}
	return nil
}
