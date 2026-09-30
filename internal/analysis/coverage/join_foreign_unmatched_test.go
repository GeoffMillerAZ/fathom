package coverage

import (
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// TestJoinForeignBlocksNamesUnmatchedFiles: JoinForeignBlocks omits a block
// whose file resolves to no indexed file, and names that file — sorted,
// distinct — instead of dropping it without a trace.
func TestJoinForeignBlocksNamesUnmatchedFiles(t *testing.T) {
	symbols := []core.SymbolNode{pyFn("fn", "src/widget.py", 10, 12)}
	blocks := []ForeignBlock{
		{File: "/ci/src/widget.py", StartLine: 10, EndLine: 10, Statements: 1, Covered: 1},
		{File: "vendor/zzz.py", StartLine: 1, EndLine: 1, Statements: 1, Covered: 1},
		{File: "vendor/aaa.py", StartLine: 1, EndLine: 1, Statements: 1},
		{File: "vendor/zzz.py", StartLine: 2, EndLine: 2, Statements: 1},
	}
	got := JoinForeignBlocks(blocks, symbols)
	if want := []string{"vendor/aaa.py", "vendor/zzz.py"}; !reflect.DeepEqual(got.Unmatched, want) {
		t.Errorf("Unmatched = %v, want %v", got.Unmatched, want)
	}
	if len(got.BySymbol) != 1 || got.BySymbol[0].Covered != 1 {
		t.Errorf("BySymbol = %+v, want the one matched symbol joined", got.BySymbol)
	}
	if none := JoinForeignBlocks(blocks[:1], symbols); none.Unmatched != nil {
		t.Errorf("Unmatched = %v, want nil when every block resolved", none.Unmatched)
	}
}
