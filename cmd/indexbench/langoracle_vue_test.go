package main

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/langbench"
)

// TestComponentOwnCallers (wave lf7, design §1.8 W4 for Vue and Svelte): a
// component's L2 pairs are its module symbol's alone; an injected script's
// function and a style's selector make their own language's pairs, left out
// and counted.
func TestComponentOwnCallers(t *testing.T) {
	m := langbench.Manifest{ID: "vue", Kinds: map[string]langbench.KindSpec{
		"component": {SourceKind: "VueComponent", Canonical: "module"},
		"function":  {SourceKind: "JSFunc", Canonical: "function", Callable: true},
	}}
	nodes := []core.SymbolNode{
		{Name: core.ModuleSymbolName, Kind: "VueComponent", FilePath: "a.vue", Calls: []string{"ElButton", "onMounted"}},
		{Name: "inc", Kind: "JSFunc", FilePath: "a.vue", Calls: []string{"emit"}},
		{Name: ".x", Kind: "CSSSelector", FilePath: "a.vue", Calls: []string{"--c"}},
	}
	counts := map[string]int{}
	files := map[string]bool{"a.vue": true}
	if direct := componentOwnCallers(m, nodes, files, map[string]int{}); len(direct) != 1 {
		t.Fatalf("componentOwnCallers = %+v, want the module symbol alone", direct)
	}
	got := htmlOwnCallers(m, nodes, files, counts)
	if len(got) != 1 || got[0].Kind != "VueComponent" {
		t.Fatalf("own callers = %+v, want the module symbol alone", got)
	}
	if counts["vue_injected_call_pairs"] != 2 {
		t.Fatalf("counts = %v, want 2 injected pairs", counts)
	}
	if fileResolveLang["vue"] != "vue" || fileResolveLang["svelte"] != "svelte" {
		t.Fatalf("fileResolveLang lacks vue or svelte: %v", fileResolveLang)
	}
	html := langbench.Manifest{ID: "html"}
	if len(htmlOwnCallers(html, nodes, files, map[string]int{})) != 0 {
		t.Fatal("html keeps only its own kinds, none of these")
	}
}
