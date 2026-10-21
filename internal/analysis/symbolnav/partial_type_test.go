package symbolnav_test

import (
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/analysis/symbolnav"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// A partial type has multiple source declarations, but only one canonical
// navigation key. Exercise the real dialect, not only hand-made symbol rows.
func TestResolveTieredCSharpPartialTypeFromSource(t *testing.T) {
	e, err := extractor.NewTSExtractor("csharp")
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	var nodes []core.SymbolNode
	for _, file := range []string{"UI.cs", "Preview.cs", "Panel.cs"} {
		part, err := e.Extract("svc", "Editor/"+file, []byte("namespace Workshop { public partial class Panel { } }"))
		if err != nil {
			t.Fatal(err)
		}
		nodes = append(nodes, part...)
	}
	key := "svc::Editor/Workshop.Panel"
	for _, query := range []string{"Panel", "Workshop.Panel", "svc::Workshop.Panel", key} {
		got, candidates, ok := symbolnav.ResolveTiered(nodes, query)
		if !ok || got != key || len(candidates) != 1 {
			t.Errorf("query %q = (%q, %v, %v), want one canonical key %q", query, got, candidates, ok, key)
		} else if candidates[0].FilePath != "Editor/Panel.cs" {
			t.Errorf("query %q chooses %s, want deterministic Editor/Panel.cs", query, candidates[0].FilePath)
		}
	}
	before := append([]core.SymbolNode(nil), nodes...)
	ctx := symbolnav.SymbolContext(nodes, callgraph.Build(nodes), "Panel")
	if ctx.Def.FilePath != "Editor/Panel.cs" {
		t.Errorf("definition = %v", ctx.Def)
	}
	if len(ctx.Definitions) != 3 {
		t.Fatalf("source declarations = %v, want all three locations", ctx.Definitions)
	}
	if !reflect.DeepEqual(nodes, before) {
		t.Fatal("navigation mutated extractor output")
	}
	for i, j := 0, len(nodes)-1; i < j; i, j = i+1, j-1 {
		nodes[i], nodes[j] = nodes[j], nodes[i]
	}
	// Repeated input rows are not an additional source declaration, and input
	// order does not choose the primary snippet or reorder the locations.
	nodes = append(nodes, nodes...)
	got := symbolnav.SymbolContext(nodes, callgraph.Build(nodes), key)
	if !reflect.DeepEqual(ctx, got) {
		t.Fatalf("reversed/duplicate rows change context:\n%v\n%v", ctx, got)
	}
}

func TestResolveTieredPartialTypeKeepsDistinctKeysAmbiguous(t *testing.T) {
	nodes := []core.SymbolNode{
		{Service: "svc", Package: "Zulu", Name: "Panel", FilePath: "Editor/B.cs", Kind: core.KindCSharpClass},
		{Service: "svc", Package: "Alpha", Name: "Panel", FilePath: "Editor/C.cs", Kind: core.KindCSharpClass},
		{Service: "svc", Package: "Zulu", Name: "Panel", FilePath: "Editor/A.cs", Kind: core.KindCSharpClass},
	}
	key, candidates, ok := symbolnav.ResolveTiered(nodes, "Panel")
	if ok || key != "" || len(candidates) != 2 {
		t.Fatalf("resolution = (%q, %v, %v), want two distinct keys", key, candidates, ok)
	}
	if candidates[0].Package != "Alpha" || candidates[1].FilePath != "Editor/A.cs" {
		t.Fatalf("unstable representatives: %v", candidates)
	}
}
