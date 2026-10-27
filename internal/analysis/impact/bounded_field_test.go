package impact_test

import (
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/analysis/impact"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// A field edge is deliberately present even though field references are not
// indexed today. Exclusion must hold at seeds, dependents, and cap lookahead.
func TestBlastRadiusBoundedExcludedKeysAreOutsideWalk(t *testing.T) {
	newNode := func(name, file string, kind core.SourceKind) core.SymbolNode {
		return core.SymbolNode{Service: "svc", Package: "p", Name: name, FilePath: file, Kind: kind}
	}
	target := newNode("Target", "changed.go", core.KindGoFunc)
	field := newNode("T.Field", "changed.go", core.KindGoField)
	caller := newNode("Caller", "caller.go", core.KindGoFunc)
	fieldCaller := newNode("FieldCaller", "field_caller.go", core.KindGoFunc)
	deepField := newNode("T.Deep", "other.go", core.KindGoField)
	nodes := []core.SymbolNode{deepField, fieldCaller, caller, field, target}
	edges := []core.Edge{
		{FromService: "svc", FromSymbol: caller.Key(), ToSymbol: target.Key(), Kind: core.EdgeDirect},
		{FromService: "svc", FromSymbol: fieldCaller.Key(), ToSymbol: field.Key(), Kind: core.EdgeDirect},
		{FromService: "svc", FromSymbol: deepField.Key(), ToSymbol: caller.Key(), Kind: core.EdgeDirect},
	}
	g := callgraph.BuildBoth(nodes, edges)
	exclude := map[string]bool{field.Key(): true, deepField.Key(): true}
	got := impact.BlastRadiusBounded(g, nodes, []string{"changed.go"}, impact.Options{
		MaxDepth: 1, MaxSymbols: 1, ExcludeKeys: exclude,
	})
	if want := []string{target.Key()}; !reflect.DeepEqual(got.Symbols, want) {
		t.Errorf("Symbols = %v, want %v", got.Symbols, want)
	}
	if got.TotalSymbols != 2 || !got.Truncated || got.DepthCapped {
		t.Errorf("bounds = total %d, truncated %t, depth_capped %t; want 2, true, false", got.TotalSymbols, got.Truncated, got.DepthCapped)
	}
	if len(got.Tests) != 0 || !reflect.DeepEqual(got.Files, []string{"changed.go"}) {
		t.Errorf("tests/files include excluded path: %v / %v", got.Tests, got.Files)
	}

	// The option is query-specific. The default bounded walk retains its
	// previous graph behavior for callers such as benchmarks.
	plain := impact.BlastRadiusBounded(g, nodes, []string{"changed.go"}, impact.Options{MaxDepth: 1})
	if plain.TotalSymbols != 4 {
		t.Errorf("default total = %d, want 4 (both seeds and both callers)", plain.TotalSymbols)
	}
}
