package graphmemo_test

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"math/rand"
	"reflect"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/graphmemo"
)

// These tests deliberately ASSERT the known RT-197 defects under W12-1.
// PASS means a counterexample still exists, not that patching is correct.
// A future full repair must deliberately update these defect oracles to assert
// correctness instead. Do not skip them or weaken comparisons to accommodate it.
type rt197Imports map[string]*types.Package

func (i rt197Imports) Import(path string) (*types.Package, error) {
	if p := i[path]; p != nil {
		return p, nil
	}
	return nil, fmt.Errorf("unregistered fixture import %q", path)
}

func rt197Checked(t *testing.T, path, src string, imp rt197Imports) *types.Package {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path+".go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	cfg := types.Config{Importer: imp}
	p, err := cfg.Check(path, fset, []*ast.File{f}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func rt197Extract(t *testing.T, file, src string) []core.SymbolNode {
	t.Helper()
	n, err := extractor.NewGoExtractor().Extract("svc", file, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func rt197Named(t *testing.T, ns []core.SymbolNode, name string) core.SymbolNode {
	t.Helper()
	for _, n := range ns {
		if n.Name == name {
			return n
		}
	}
	t.Fatalf("missing extracted %q", name)
	return core.SymbolNode{}
}

func rt197Keys(g *callgraph.Graph, caller string) []string {
	out := []string{}
	for _, e := range callgraph.CalleesDetail(g, caller) {
		out = append(out, e.Key)
	}
	return out
}

func rt197Order(ns []core.SymbolNode, mode int) []core.SymbolNode {
	out := append([]core.SymbolNode(nil), ns...)
	switch mode {
	case 1:
		for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
			out[i], out[j] = out[j], out[i]
		}
	case 2:
		sort.SliceStable(out, func(i, j int) bool { return out[i].Key() < out[j].Key() })
	case 3:
		rand.New(rand.NewSource(20261115)).Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	}
	return out
}

// Adding a uniquely keyed method makes an alias ambiguous. A full build drops
// the outside caller's edge, but syntax and BOTH patches retain the stale edge.
func TestKnownDefectRT197UniqueKeyAliasAmbiguity(t *testing.T) {
	const a = "package a\ntype A struct{}\nfunc (A) Run() {}\n"
	const b0 = "package b\ntype B struct{}\n"
	const b1 = "package b\ntype B struct{}\nfunc (B) Run() {}\n"
	const c = "package c\nimport \"example/a\"\nfunc Call(h a.A) { h.Run() }\n"
	pa := rt197Checked(t, "example/a", a, nil)
	rt197Checked(t, "example/b", b0, nil)
	rt197Checked(t, "example/b", b1, nil)
	rt197Checked(t, "example/c", c, rt197Imports{"example/a": pa})
	an, bn0, bn1, cn := rt197Extract(t, "a/handler.go", a), rt197Extract(t, "b/handler.go", b0), rt197Extract(t, "b/handler.go", b1), rt197Extract(t, "c/call.go", c)
	ar, br, cr := rt197Named(t, an, "A.Run"), rt197Named(t, bn1, "B.Run"), rt197Named(t, cn, "Call")
	if ar.Package != "A" || br.Package != "B" || !reflect.DeepEqual(cr.Calls, []string{"h.Run"}) {
		t.Fatalf("actual shapes: ar=%+v br=%+v Call.Calls=%v", ar, br, cr.Calls)
	}
	before := append(append(append([]core.SymbolNode{}, an...), bn0...), cn...)
	after := append(append([]core.SymbolNode{}, before...), br)
	seen := map[string]bool{}
	for _, n := range after {
		if seen[n.Key()] {
			t.Fatalf("duplicate full key %q", n.Key())
		}
		seen[n.Key()] = true
	}
	for mode := 0; mode < 4; mode++ {
		old, fresh := callgraph.Build(rt197Order(before, mode)), callgraph.Build(rt197Order(after, mode))
		patched := old.ReplacePackage("B", []core.SymbolNode{br}, nil)
		got, want := rt197Keys(patched, cr.Key()), rt197Keys(fresh, cr.Key())
		if !reflect.DeepEqual(got, []string{ar.Key()}) || len(want) != 0 {
			t.Fatalf("order%d patch=%v fresh=%v", mode, got, want)
		}
		bothPatch := callgraph.BuildBoth(rt197Order(before, mode), nil).ReplacePackage("B", []core.SymbolNode{br}, nil)
		bothFresh := callgraph.BuildBoth(rt197Order(after, mode), nil)
		if !reflect.DeepEqual(rt197Keys(bothPatch, cr.Key()), got) || len(rt197Keys(bothFresh, cr.Key())) != 0 {
			t.Fatalf("BOTH order%d differs from syntax witness", mode)
		}
		t.Logf("order%d all keys unique; extracted %s Calls=%v; patch=%v fresh=%v; BOTH same mismatch", mode, cr.Key(), cr.Calls, got, want)
	}
}

// Valid Go permits two init declarations in one file. The full build keeps the
// last declaration's calls, while the patch combines both declarations' calls.
func TestKnownDefectRT197SameFileInit(t *testing.T) {
	const old = "package p\nfunc A(){}\nfunc B(){}\n"
	const changed = "package p\nfunc A(){}\nfunc B(){}\nfunc init(){A()}\nfunc init(){B()}\n"
	rt197Checked(t, "example/p", changed, nil)
	before, after := rt197Extract(t, "p/init.go", old), rt197Extract(t, "p/init.go", changed)
	init := rt197Named(t, after, "init")
	files := map[string]map[string]bool{}
	for _, n := range after {
		if files[n.Key()] == nil {
			files[n.Key()] = map[string]bool{}
		}
		files[n.Key()][n.FilePath] = true
	}
	for k, paths := range files {
		if len(paths) != 1 {
			t.Fatalf("cross-file collision %q", k)
		}
	}
	for mode := 0; mode < 2; mode++ {
		repl := rt197Order(after, mode)
		patched := callgraph.Build(before).ReplacePackage("p", repl, nil)
		fresh := callgraph.Build(repl)
		got, want := rt197Keys(patched, init.Key()), rt197Keys(fresh, init.Key())
		expectedFresh := []string{rt197Named(t, after, "B").Key()}
		if mode == 1 {
			expectedFresh = []string{rt197Named(t, after, "A").Key()}
		}
		expectedPatch := []string{rt197Named(t, after, "A").Key(), rt197Named(t, after, "B").Key()}
		if !reflect.DeepEqual(got, expectedPatch) || !reflect.DeepEqual(want, expectedFresh) {
			t.Fatalf("order%d patch=%v fresh=%v", mode, got, want)
		}
		t.Logf("order%d one file per key; actual extracted init patch=%v fresh=%v", mode, got, want)
	}
}

// PatchPackage filters the package out and appends its replacement, moving an
// unchanged symbol behind another package. The authoritative order is supplied
// explicitly here. This test does not execute or prove store ingestion ordering.
func TestKnownDefectRT197SnapshotAppendOrder(t *testing.T) {
	a := core.SymbolNode{Service: "svc", Package: "p", Name: "A", FilePath: "p/stable.go", Kind: core.KindGoFunc}
	b := core.SymbolNode{Service: "svc", Package: "q", Name: "B", FilePath: "q/q.go", Kind: core.KindGoFunc}
	c := core.SymbolNode{Service: "svc", Package: "p", Name: "C", FilePath: "p/changed.go", Kind: core.KindGoFunc}
	changed := c
	changed.Signature = "changed at t4"
	sc := graphmemo.Scope{Store: "rt197-known-defect-snapshot", Service: "svc", Watermark: "fixed"}
	t.Cleanup(func() { graphmemo.DropScope(sc) })
	_, err := graphmemo.SnapshotFor(context.Background(), sc, func(context.Context) ([]core.SymbolNode, error) { return []core.SymbolNode{a, b, c}, nil })
	if err != nil {
		t.Fatal(err)
	}
	graphmemo.PatchPackage(sc, "p", []core.SymbolNode{a, changed}, nil)
	patched, ok := graphmemo.PeekSnapshot(sc)
	if !ok {
		t.Fatal("snapshot was not cached; witness invalid")
	}
	fresh := graphmemo.NewSnapshot([]core.SymbolNode{a, b, changed})
	if !reflect.DeepEqual(patched.Nodes, []core.SymbolNode{b, a, changed}) || !reflect.DeepEqual(fresh.Nodes, []core.SymbolNode{a, b, changed}) || reflect.DeepEqual(patched.Nodes, fresh.Nodes) {
		t.Fatalf("patch=%v fresh=%v", patched.Nodes, fresh.Nodes)
	}
	t.Logf("actual PatchPackage Nodes=%s,%s,%s; supplied authoritative-order fresh Nodes=%s,%s,%s; all keys unique", patched.Nodes[0].Name, patched.Nodes[1].Name, patched.Nodes[2].Name, fresh.Nodes[0].Name, fresh.Nodes[1].Name, fresh.Nodes[2].Name)
}
