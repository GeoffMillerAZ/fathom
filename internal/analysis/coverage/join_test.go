package coverage

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func fn(name, file string, line, endLine int) core.SymbolNode {
	return core.SymbolNode{
		Service:  "fathom",
		Package:  "widget",
		Name:     name,
		Kind:     core.KindGoFunc,
		FilePath: file,
		Line:     line,
		EndLine:  endLine,
	}
}

func TestJoinSymbols_CoveredAndUncovered(t *testing.T) {
	symbols := []core.SymbolNode{
		fn("Covered", "internal/widget/widget.go", 10, 14),
		fn("Uncovered", "internal/widget/widget.go", 20, 24),
	}
	profile := Profile{
		Mode: ModeSet,
		Blocks: []Block{
			{ImportPath: "github.com/geoffmilleraz/fathom/internal/widget/widget.go", StartLine: 10, StartCol: 20, EndLine: 12, EndCol: 2, NumStmt: 2, Count: 1},
			{ImportPath: "github.com/geoffmilleraz/fathom/internal/widget/widget.go", StartLine: 20, StartCol: 20, EndLine: 22, EndCol: 2, NumStmt: 3, Count: 0},
		},
	}

	out := JoinSymbols(profile, symbols)
	if len(out) != 2 {
		t.Fatalf("len(out) = %d, want 2", len(out))
	}
	// Sorted by (File, Line): same file, so by Line — Covered (10) then
	// Uncovered (20).
	if out[0].Symbol != symbols[0].Key() || out[0].Statements != 2 || out[0].Covered != 2 || out[0].Percent != 100 {
		t.Errorf("Covered = %+v", out[0])
	}
	if out[1].Symbol != symbols[1].Key() || out[1].Statements != 3 || out[1].Covered != 0 || out[1].Percent != 0 {
		t.Errorf("Uncovered = %+v", out[1])
	}
}

func TestJoinSymbols_UnmatchedBlockOmitted(t *testing.T) {
	symbols := []core.SymbolNode{fn("F", "internal/widget/widget.go", 10, 14)}
	profile := Profile{Blocks: []Block{
		{ImportPath: "github.com/geoffmilleraz/fathom/internal/other/other.go", StartLine: 1, EndLine: 2, NumStmt: 1, Count: 1},
	}}
	out := JoinSymbols(profile, symbols)
	if len(out) != 0 {
		t.Fatalf("expected no joined symbols for a file with no matching indexed symbol, got %+v", out)
	}
}

func TestJoinSymbols_SymbolWithNoBlockOmitted(t *testing.T) {
	symbols := []core.SymbolNode{fn("F", "internal/widget/widget.go", 10, 14)}
	// The one block falls outside F's [10,14] span.
	profile := Profile{Blocks: []Block{
		{ImportPath: "github.com/geoffmilleraz/fathom/internal/widget/widget.go", StartLine: 100, EndLine: 102, NumStmt: 1, Count: 1},
	}}
	out := JoinSymbols(profile, symbols)
	if len(out) != 0 {
		t.Fatalf("expected F to be omitted (no profiled block inside its span), got %+v", out)
	}
}

func TestJoinSymbols_NonFuncKindIgnored(t *testing.T) {
	symbols := []core.SymbolNode{{
		Service: "fathom", Package: "widget", Name: "T", Kind: core.KindGoType,
		FilePath: "internal/widget/widget.go", Line: 10, EndLine: 14,
	}}
	profile := Profile{Blocks: []Block{
		{ImportPath: "github.com/geoffmilleraz/fathom/internal/widget/widget.go", StartLine: 10, EndLine: 12, NumStmt: 1, Count: 1},
	}}
	out := JoinSymbols(profile, symbols)
	if len(out) != 0 {
		t.Fatalf("a GoType symbol should never be joined (no statement body), got %+v", out)
	}
}

func TestJoinSymbols_SynthesizesEndLineWhenUnknown(t *testing.T) {
	// The Go extractor never sets EndLine (EndLine==0 here, as it is for
	// every real Go function in this index today) — JoinSymbols must still
	// join by synthesizing an end line from the next symbol's own Line.
	first := fn("First", "internal/widget/widget.go", 3, 0)    // EndLine unknown
	second := fn("Second", "internal/widget/widget.go", 10, 0) // EndLine unknown, last in file
	profile := Profile{Blocks: []Block{
		// Falls between First's Line (3) and Second's Line (10): must join
		// to First via the synthesized end line (Second.Line - 1 = 9).
		{ImportPath: "github.com/geoffmilleraz/fathom/internal/widget/widget.go", StartLine: 4, EndLine: 5, NumStmt: 1, Count: 1},
		// Falls after Second's Line, with no next symbol to bound it — must
		// join to Second via the math.MaxInt32 "rest of file" fallback.
		{ImportPath: "github.com/geoffmilleraz/fathom/internal/widget/widget.go", StartLine: 12, EndLine: 14, NumStmt: 1, Count: 0},
	}}
	out := JoinSymbols(profile, []core.SymbolNode{first, second})
	if len(out) != 2 {
		t.Fatalf("out = %+v, want 2 joined symbols", out)
	}
	if out[0].Symbol != first.Key() || out[0].Covered != 1 {
		t.Errorf("First = %+v", out[0])
	}
	if out[1].Symbol != second.Key() || out[1].Covered != 0 {
		t.Errorf("Second = %+v", out[1])
	}
}

func TestJoinSymbols_DeterministicOrder(t *testing.T) {
	symbols := []core.SymbolNode{
		fn("B", "internal/widget/b.go", 5, 10),
		fn("A", "internal/widget/a.go", 5, 10),
	}
	profile := Profile{Blocks: []Block{
		{ImportPath: "github.com/geoffmilleraz/fathom/internal/widget/b.go", StartLine: 5, EndLine: 6, NumStmt: 1, Count: 1},
		{ImportPath: "github.com/geoffmilleraz/fathom/internal/widget/a.go", StartLine: 5, EndLine: 6, NumStmt: 1, Count: 1},
	}}
	out := JoinSymbols(profile, symbols)
	if len(out) != 2 || out[0].File != "internal/widget/a.go" || out[1].File != "internal/widget/b.go" {
		t.Fatalf("expected (file, line) order a.go then b.go, got %+v", out)
	}
}
