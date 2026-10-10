package main

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// TestScopeFileEdgesToActiveKeepsModuleCallers (LB-F434, O805): the
// active-lines rule drops an edge whose caller definition starts on a line the
// build's preprocessor discards. A file's module symbol (K18) starts on line 1
// by construction, a comment or a guard the preprocessor never keeps, and its
// edges are the file's #includes and file-level references, whose own lines
// an edge does not carry: the rule leaves them in, as it does a caller in a
// file the oracle gives no ranges for. An ordinary caller on a discarded line
// is still dropped.
func TestScopeFileEdgesToActiveKeepsModuleCallers(t *testing.T) {
	mod := core.SymbolNode{Service: "s", Package: "lib", Name: core.ModuleSymbolName, Kind: "CppModule", FilePath: "a.hpp", Line: 1, EndLine: 40}
	dead := core.SymbolNode{Service: "s", Package: "lib", Name: "win_only", Kind: "CppFunc", FilePath: "a.hpp", Line: 5, EndLine: 7}
	live := core.SymbolNode{Service: "s", Package: "lib", Name: "f", Kind: "CppFunc", FilePath: "a.hpp", Line: 20, EndLine: 22}
	to := core.SymbolNode{Service: "s", Package: "lib", Name: "g", Kind: "CppFunc", FilePath: "b.hpp", Line: 3, EndLine: 3}
	nodes := []core.SymbolNode{mod, dead, live, to}
	edges := []core.Edge{
		{FromFile: "a.hpp", FromSymbol: mod.Key(), ToSymbol: to.Key(), Kind: core.EdgeFileResolved},
		{FromFile: "a.hpp", FromSymbol: dead.Key(), ToSymbol: to.Key(), Kind: core.EdgeFileResolved},
		{FromFile: "a.hpp", FromSymbol: live.Key(), ToSymbol: to.Key(), Kind: core.EdgeFileResolved},
	}
	active := map[string][][2]int{"a.hpp": {{3, 3}, {10, 40}}}
	kept, n := scopeFileEdgesToActive(edges, nodes, active)
	if n != 1 || len(kept) != 2 || kept[0].FromSymbol != mod.Key() || kept[1].FromSymbol != live.Key() {
		t.Errorf("kept %v (%d left out), want the module's and f's edges, win_only's left out", kept, n)
	}
}
