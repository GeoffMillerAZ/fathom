package coverage

import (
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func pyFn(name, file string, line, endLine int) core.SymbolNode {
	return core.SymbolNode{
		Service:  "fathom",
		Package:  "widget",
		Name:     name,
		Kind:     core.KindPyFunc,
		FilePath: file,
		Line:     line,
		EndLine:  endLine,
	}
}

func TestJoinForeignBlocksByRange(t *testing.T) {
	// src/widget.py has two Python functions, both with real (L1)
	// end-line ranges — the whole file resolves at range granularity.
	symbols := []core.SymbolNode{
		pyFn("covered_fn", "src/widget.py", 10, 12),
		pyFn("uncovered_fn", "src/widget.py", 20, 22),
	}
	blocks := []ForeignBlock{
		{File: "src/widget.py", StartLine: 10, EndLine: 10, Statements: 1, Covered: 1},
		{File: "src/widget.py", StartLine: 11, EndLine: 11, Statements: 1, Covered: 1},
		{File: "src/widget.py", StartLine: 20, EndLine: 20, Statements: 1, Covered: 0},
	}

	got := joinForeignBlocks(blocks, symbols)
	if len(got.ByFile) != 0 {
		t.Fatalf("ByFile = %+v, want none (file is range-eligible)", got.ByFile)
	}
	if len(got.BySymbol) != 2 {
		t.Fatalf("len(BySymbol) = %d, want 2: %+v", len(got.BySymbol), got.BySymbol)
	}
	if got.BySymbol[0].Symbol != symbols[0].Key() || got.BySymbol[0].Statements != 2 || got.BySymbol[0].Covered != 2 {
		t.Errorf("BySymbol[0] = %+v", got.BySymbol[0])
	}
	if got.BySymbol[1].Symbol != symbols[1].Key() || got.BySymbol[1].Statements != 1 || got.BySymbol[1].Covered != 0 {
		t.Errorf("BySymbol[1] = %+v", got.BySymbol[1])
	}
}

func TestJoinForeignBlocksByFile_NoRangedSymbols(t *testing.T) {
	// No indexed symbols at all for this file (an extractor that never
	// produced function-level symbols, or L1 ranges not computed) — every
	// block rolls up to one ForeignFileCoverage entry.
	blocks := []ForeignBlock{
		{File: "src/legacy.rb", StartLine: 1, EndLine: 1, Statements: 1, Covered: 1},
		{File: "src/legacy.rb", StartLine: 2, EndLine: 2, Statements: 1, Covered: 0},
	}
	// A symbol must still name the file for it to resolve — a class
	// symbol with EndLine 0 (no range) is exactly the "no ranges" case.
	symbols := []core.SymbolNode{
		{Service: "fathom", Package: "legacy", Name: "Legacy", Kind: core.KindRubyClass, FilePath: "src/legacy.rb", Line: 1, EndLine: 0},
	}

	got := joinForeignBlocks(blocks, symbols)
	if len(got.BySymbol) != 0 {
		t.Fatalf("BySymbol = %+v, want none (file has no ranged function symbols)", got.BySymbol)
	}
	if len(got.ByFile) != 1 {
		t.Fatalf("len(ByFile) = %d, want 1: %+v", len(got.ByFile), got.ByFile)
	}
	want := ForeignFileCoverage{File: "src/legacy.rb", Statements: 2, Covered: 1, Percent: 50}
	if !reflect.DeepEqual(got.ByFile[0], want) {
		t.Errorf("ByFile[0] = %+v, want %+v", got.ByFile[0], want)
	}
}

func TestJoinForeignBlocksByFile_OneUnrangedSymbolDemotesWholeFile(t *testing.T) {
	// Two Ruby methods in the same file; one has a real range, the other
	// does not (an extractor that only sometimes resolves EndLine) — the
	// whole file must roll up, never mix synthesized-vs-real boundaries.
	symbols := []core.SymbolNode{
		{Service: "fathom", Package: "widget", Name: "ranged", Kind: core.KindRubyMethod, FilePath: "src/widget.rb", Line: 1, EndLine: 3},
		{Service: "fathom", Package: "widget", Name: "unranged", Kind: core.KindRubyMethod, FilePath: "src/widget.rb", Line: 10, EndLine: 0},
	}
	blocks := []ForeignBlock{
		{File: "src/widget.rb", StartLine: 1, EndLine: 1, Statements: 1, Covered: 1},
	}
	got := joinForeignBlocks(blocks, symbols)
	if len(got.BySymbol) != 0 {
		t.Fatalf("BySymbol = %+v, want none (file demoted to file-level)", got.BySymbol)
	}
	if len(got.ByFile) != 1 || got.ByFile[0].Statements != 1 || got.ByFile[0].Covered != 1 {
		t.Fatalf("ByFile = %+v, want one rolled-up entry", got.ByFile)
	}
}

func TestJoinForeignBlocksByRange_TestAttributionUnioned(t *testing.T) {
	symbols := []core.SymbolNode{pyFn("f", "src/a.py", 1, 3)}
	blocks := []ForeignBlock{
		{File: "src/a.py", StartLine: 1, EndLine: 1, Statements: 1, Covered: 1, Tests: []string{"unit"}},
		{File: "src/a.py", StartLine: 2, EndLine: 2, Statements: 1, Covered: 1, Tests: []string{"integration"}},
	}
	got := joinForeignBlocks(blocks, symbols)
	if len(got.BySymbol) != 1 {
		t.Fatalf("len(BySymbol) = %d, want 1", len(got.BySymbol))
	}
	want := []string{"integration", "unit"}
	if !reflect.DeepEqual(got.BySymbol[0].Tests, want) {
		t.Errorf("Tests = %+v, want %+v", got.BySymbol[0].Tests, want)
	}
}

func TestJoinForeignBlocksByRange_UnmatchedFileOmitted(t *testing.T) {
	symbols := []core.SymbolNode{pyFn("f", "src/a.py", 1, 3)}
	blocks := []ForeignBlock{{File: "src/other.py", StartLine: 1, EndLine: 1, Statements: 1, Covered: 1}}
	got := joinForeignBlocks(blocks, symbols)
	if len(got.BySymbol) != 0 || len(got.ByFile) != 0 {
		t.Fatalf("got = %+v, want an empty result for a block matching no known file", got)
	}
}
