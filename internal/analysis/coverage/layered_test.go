package coverage

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func TestJoinLayeredSymbols_TwoLayers(t *testing.T) {
	// Two functions in one file: A covered only by unit's run, B covered
	// only by integration's run — the real-world shape (`go test
	// -coverprofile=unit.out ./...` then `go test -tags integration
	// -coverprofile=integration.out ./...`).
	symA := fn("A", "pkg/widget.go", 3, 6)
	symB := fn("B", "pkg/widget.go", 10, 13)

	unit := LabeledProfile{Layer: "unit", Profile: Profile{Mode: ModeSet, Blocks: []Block{
		{ImportPath: "example.com/mod/pkg/widget.go", StartLine: 3, StartCol: 1, EndLine: 5, EndCol: 2, NumStmt: 2, Count: 1},
		{ImportPath: "example.com/mod/pkg/widget.go", StartLine: 10, StartCol: 1, EndLine: 12, EndCol: 2, NumStmt: 3, Count: 0},
	}}}
	integration := LabeledProfile{Layer: "integration", Profile: Profile{Mode: ModeSet, Blocks: []Block{
		{ImportPath: "example.com/mod/pkg/widget.go", StartLine: 3, StartCol: 1, EndLine: 5, EndCol: 2, NumStmt: 2, Count: 0},
		{ImportPath: "example.com/mod/pkg/widget.go", StartLine: 10, StartCol: 1, EndLine: 12, EndCol: 2, NumStmt: 3, Count: 1},
	}}}

	out := JoinLayeredSymbols([]LabeledProfile{unit, integration}, []core.SymbolNode{symA, symB})
	if len(out) != 2 {
		t.Fatalf("out = %+v, want 2 symbols", out)
	}

	byKey := map[string]SymbolLayerCoverage{}
	for _, sc := range out {
		byKey[sc.Symbol] = sc
	}

	a := byKey[symA.Key()]
	if a.ByLayer["unit"].Covered != 2 || a.ByLayer["integration"].Covered != 0 {
		t.Errorf("A ByLayer = %+v", a.ByLayer)
	}
	if got := a.CoveringLayers(); len(got) != 1 || got[0] != "unit" {
		t.Errorf("A CoveringLayers() = %v, want [unit]", got)
	}
	if got := a.UncoveredAt(); len(got) != 1 || got[0] != "integration" {
		t.Errorf("A UncoveredAt() = %v, want [integration]", got)
	}
	// Any is the union: A is covered by unit, so Any.Covered > 0 even
	// though integration alone shows 0.
	if a.Any.Covered != 2 || a.Any.Statements != 2 {
		t.Errorf("A Any = %+v, want covered by the union", a.Any)
	}

	b := byKey[symB.Key()]
	if b.ByLayer["unit"].Covered != 0 || b.ByLayer["integration"].Covered != 3 {
		t.Errorf("B ByLayer = %+v", b.ByLayer)
	}
	if got := b.CoveringLayers(); len(got) != 1 || got[0] != "integration" {
		t.Errorf("B CoveringLayers() = %v, want [integration]", got)
	}
	if b.Any.Covered != 3 {
		t.Errorf("B Any = %+v, want covered by the union", b.Any)
	}
}

func TestJoinLayeredSymbols_NeitherLayerCovers(t *testing.T) {
	sym := fn("Dead", "pkg/widget.go", 3, 6)
	unit := LabeledProfile{Layer: "unit", Profile: Profile{Blocks: []Block{
		{ImportPath: "example.com/mod/pkg/widget.go", StartLine: 3, EndLine: 5, NumStmt: 1, Count: 0},
	}}}
	e2e := LabeledProfile{Layer: "e2e", Profile: Profile{Blocks: []Block{
		{ImportPath: "example.com/mod/pkg/widget.go", StartLine: 3, EndLine: 5, NumStmt: 1, Count: 0},
	}}}
	out := JoinLayeredSymbols([]LabeledProfile{unit, e2e}, []core.SymbolNode{sym})
	if len(out) != 1 {
		t.Fatalf("out = %+v", out)
	}
	sc := out[0]
	if len(sc.CoveringLayers()) != 0 {
		t.Errorf("CoveringLayers() = %v, want none", sc.CoveringLayers())
	}
	if got := sc.UncoveredAt(); len(got) != 2 {
		t.Errorf("UncoveredAt() = %v, want both layers", got)
	}
	if sc.Any.Covered != 0 {
		t.Errorf("Any.Covered = %d, want 0", sc.Any.Covered)
	}
}

func TestJoinLayeredSymbols_LayerWithNoDataIsAbsentNotZero(t *testing.T) {
	// integration's profile has NO block at all for this file (e.g. a
	// build tag excluded it) — it must be absent from ByLayer, never a
	// fabricated 0%.
	sym := fn("OnlyUnit", "pkg/widget.go", 3, 6)
	unit := LabeledProfile{Layer: "unit", Profile: Profile{Blocks: []Block{
		{ImportPath: "example.com/mod/pkg/widget.go", StartLine: 3, EndLine: 5, NumStmt: 1, Count: 1},
	}}}
	integration := LabeledProfile{Layer: "integration", Profile: Profile{Blocks: nil}}
	out := JoinLayeredSymbols([]LabeledProfile{unit, integration}, []core.SymbolNode{sym})
	if len(out) != 1 {
		t.Fatalf("out = %+v", out)
	}
	if _, ok := out[0].ByLayer["integration"]; ok {
		t.Errorf("integration should be ABSENT (no data), not present: %+v", out[0].ByLayer)
	}
	if _, ok := out[0].ByLayer["unit"]; !ok {
		t.Errorf("unit should be present: %+v", out[0].ByLayer)
	}
}

func TestUnionBlocks_DedupesIdenticalSpanAcrossLayers(t *testing.T) {
	// The SAME span appears in both profiles (same compiled source): the
	// union must count it ONCE, not twice.
	p1 := LabeledProfile{Layer: "unit", Profile: Profile{Blocks: []Block{
		{ImportPath: "f.go", StartLine: 1, StartCol: 1, EndLine: 2, EndCol: 2, NumStmt: 5, Count: 1},
	}}}
	p2 := LabeledProfile{Layer: "integration", Profile: Profile{Blocks: []Block{
		{ImportPath: "f.go", StartLine: 1, StartCol: 1, EndLine: 2, EndCol: 2, NumStmt: 5, Count: 0},
	}}}
	merged := unionBlocks([]LabeledProfile{p1, p2})
	if len(merged) != 1 {
		t.Fatalf("merged = %+v, want exactly one deduped span", merged)
	}
	if merged[0].Count != 1 {
		t.Errorf("merged[0].Count = %d, want 1 (covered by unit)", merged[0].Count)
	}
	if merged[0].NumStmt != 5 {
		t.Errorf("merged[0].NumStmt = %d, want 5 (not doubled)", merged[0].NumStmt)
	}
}
