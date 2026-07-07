package apisurface_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/apisurface"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// helpers

func node(name string, kind core.SourceKind, sig string) core.SymbolNode {
	return core.SymbolNode{
		Name:      name,
		Kind:      kind,
		Signature: sig,
		Package:   "pkg",
	}
}

// --- Surface tests ---

func TestSurface_EmptyInput(t *testing.T) {
	got := apisurface.Surface(nil)
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

func TestSurface_ExcludesUnexportedNames(t *testing.T) {
	nodes := []core.SymbolNode{
		node("unexported", core.KindGoFunc, "func unexported()"),
		node("alsoPrivate", core.KindGoType, "type alsoPrivate struct{}"),
		node("_", core.KindGoVar, "var _ = 0"),
	}
	got := apisurface.Surface(nodes)
	if len(got) != 0 {
		t.Fatalf("expected 0 exported, got %v", got)
	}
}

func TestSurface_IncludesExportedKinds(t *testing.T) {
	nodes := []core.SymbolNode{
		node("PublicFunc", core.KindGoFunc, "func PublicFunc() error"),
		node("PublicMethod", core.KindGoMethod, "func (T) PublicMethod() string"),
		node("PublicType", core.KindGoType, "type PublicType struct{}"),
		node("PublicInterface", core.KindGoInterface, "type PublicInterface interface{}"),
		node("PublicConst", core.KindGoConst, "const PublicConst = 1"),
		node("PublicVar", core.KindGoVar, "var PublicVar int"),
	}
	got := apisurface.Surface(nodes)
	if len(got) != 6 {
		t.Fatalf("expected 6 exported nodes, got %d: %v", len(got), got)
	}
}

func TestSurface_ExcludesNonGoKinds(t *testing.T) {
	// Non-Go kinds should not appear in the API surface even if the name is uppercase.
	nodes := []core.SymbolNode{
		node("TerraformResource", core.KindTerraformResource, ""),
		node("K8sWorkload", core.KindK8sWorkload, ""),
		node("DocChunk", core.KindDocChunk, ""),
		node("ModuleDep", core.KindModuleDep, ""),
		node("CUEDefinition", core.KindCUEDefinition, ""),
	}
	got := apisurface.Surface(nodes)
	if len(got) != 0 {
		t.Fatalf("expected 0, got %v", got)
	}
}

func TestSurface_MixedExportedAndPrivate(t *testing.T) {
	nodes := []core.SymbolNode{
		node("Exported", core.KindGoFunc, "func Exported()"),
		node("private", core.KindGoFunc, "func private()"),
		node("AnotherExport", core.KindGoType, "type AnotherExport struct{}"),
	}
	got := apisurface.Surface(nodes)
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d: %v", len(got), got)
	}
}

func TestSurface_Sorted(t *testing.T) {
	// Names in reverse alphabetical order — Surface must return them sorted.
	nodes := []core.SymbolNode{
		node("Zebra", core.KindGoFunc, "func Zebra()"),
		node("Apple", core.KindGoType, "type Apple struct{}"),
		node("Mango", core.KindGoConst, "const Mango = 1"),
	}
	got := apisurface.Surface(nodes)
	if len(got) != 3 {
		t.Fatalf("expected 3, got %d", len(got))
	}
	if got[0].Name != "Apple" || got[1].Name != "Mango" || got[2].Name != "Zebra" {
		t.Fatalf("wrong order: %v", []string{got[0].Name, got[1].Name, got[2].Name})
	}
}

func TestSurface_SortedByPackageThenName(t *testing.T) {
	// When nodes come from different packages, sort by Package then Name.
	nodes := []core.SymbolNode{
		{Name: "Foo", Kind: core.KindGoFunc, Package: "b"},
		{Name: "Bar", Kind: core.KindGoFunc, Package: "a"},
		{Name: "Baz", Kind: core.KindGoFunc, Package: "a"},
	}
	got := apisurface.Surface(nodes)
	if len(got) != 3 {
		t.Fatalf("expected 3, got %d", len(got))
	}
	// a/Bar, a/Baz, b/Foo
	if got[0].Package != "a" || got[0].Name != "Bar" {
		t.Errorf("pos 0: want a/Bar, got %s/%s", got[0].Package, got[0].Name)
	}
	if got[1].Package != "a" || got[1].Name != "Baz" {
		t.Errorf("pos 1: want a/Baz, got %s/%s", got[1].Package, got[1].Name)
	}
	if got[2].Package != "b" || got[2].Name != "Foo" {
		t.Errorf("pos 2: want b/Foo, got %s/%s", got[2].Package, got[2].Name)
	}
}

func TestSurface_DoesNotMutateInput(t *testing.T) {
	nodes := []core.SymbolNode{
		node("Exported", core.KindGoFunc, "func Exported()"),
		node("private", core.KindGoFunc, "func private()"),
	}
	// Record key scalar fields before the call.
	type snapshot struct {
		name, sig string
		kind      core.SourceKind
	}
	before := make([]snapshot, len(nodes))
	for i, n := range nodes {
		before[i] = snapshot{n.Name, n.Signature, n.Kind}
	}
	_ = apisurface.Surface(nodes)
	for i, n := range nodes {
		if n.Name != before[i].name || n.Signature != before[i].sig || n.Kind != before[i].kind {
			t.Errorf("input was mutated at index %d", i)
		}
	}
}

// --- Diff tests ---

func TestDiff_EmptyBoth(t *testing.T) {
	c := apisurface.Diff(nil, nil)
	if len(c.Added) != 0 || len(c.Removed) != 0 || len(c.Changed) != 0 {
		t.Fatalf("expected empty Changes, got %+v", c)
	}
}

func TestDiff_AllAdded(t *testing.T) {
	newNodes := []core.SymbolNode{
		node("Foo", core.KindGoFunc, "func Foo()"),
		node("Bar", core.KindGoType, "type Bar struct{}"),
	}
	c := apisurface.Diff(nil, newNodes)
	if len(c.Added) != 2 {
		t.Fatalf("expected 2 added, got %v", c.Added)
	}
	if len(c.Removed) != 0 || len(c.Changed) != 0 {
		t.Fatalf("unexpected removed or changed: %+v", c)
	}
}

func TestDiff_AllRemoved(t *testing.T) {
	oldNodes := []core.SymbolNode{
		node("Foo", core.KindGoFunc, "func Foo()"),
	}
	c := apisurface.Diff(oldNodes, nil)
	if len(c.Removed) != 1 {
		t.Fatalf("expected 1 removed, got %v", c.Removed)
	}
	if len(c.Added) != 0 || len(c.Changed) != 0 {
		t.Fatalf("unexpected added or changed: %+v", c)
	}
}

func TestDiff_SignatureChanged(t *testing.T) {
	oldNodes := []core.SymbolNode{
		node("Foo", core.KindGoFunc, "func Foo() error"),
	}
	newNodes := []core.SymbolNode{
		node("Foo", core.KindGoFunc, "func Foo(ctx context.Context) error"),
	}
	c := apisurface.Diff(oldNodes, newNodes)
	if len(c.Changed) != 1 {
		t.Fatalf("expected 1 changed, got %v", c.Changed)
	}
	if len(c.Added) != 0 || len(c.Removed) != 0 {
		t.Fatalf("unexpected added or removed: %+v", c)
	}
}

func TestDiff_Unchanged(t *testing.T) {
	nodes := []core.SymbolNode{
		node("Foo", core.KindGoFunc, "func Foo() error"),
	}
	c := apisurface.Diff(nodes, nodes)
	if len(c.Added) != 0 || len(c.Removed) != 0 || len(c.Changed) != 0 {
		t.Fatalf("expected all empty, got %+v", c)
	}
}

func TestDiff_Mixed(t *testing.T) {
	oldNodes := []core.SymbolNode{
		node("Kept", core.KindGoFunc, "func Kept()"),
		node("Gone", core.KindGoType, "type Gone struct{}"),
		node("Changed", core.KindGoConst, "const Changed = 1"),
	}
	newNodes := []core.SymbolNode{
		node("Kept", core.KindGoFunc, "func Kept()"),           // unchanged
		node("Changed", core.KindGoConst, "const Changed = 2"), // signature changed
		node("New", core.KindGoVar, "var New int"),             // added
	}
	c := apisurface.Diff(oldNodes, newNodes)
	if len(c.Added) != 1 || c.Added[0] != "pkg.New" {
		t.Errorf("Added: want [pkg.New], got %v", c.Added)
	}
	if len(c.Removed) != 1 || c.Removed[0] != "pkg.Gone" {
		t.Errorf("Removed: want [pkg.Gone], got %v", c.Removed)
	}
	if len(c.Changed) != 1 || c.Changed[0] != "pkg.Changed" {
		t.Errorf("Changed: want [pkg.Changed], got %v", c.Changed)
	}
}

func TestDiff_Deterministic(t *testing.T) {
	// Run Diff twice over the same input and check output slices are identical.
	oldNodes := []core.SymbolNode{
		node("Alpha", core.KindGoFunc, "func Alpha()"),
		node("Beta", core.KindGoType, "type Beta struct{}"),
		node("Gamma", core.KindGoConst, "const Gamma = 1"),
	}
	newNodes := []core.SymbolNode{
		node("Alpha", core.KindGoFunc, "func Alpha() error"), // changed
		node("Delta", core.KindGoVar, "var Delta int"),       // added
	}
	c1 := apisurface.Diff(oldNodes, newNodes)
	c2 := apisurface.Diff(oldNodes, newNodes)
	eq := func(a, b []string) bool {
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
	if !eq(c1.Added, c2.Added) || !eq(c1.Removed, c2.Removed) || !eq(c1.Changed, c2.Changed) {
		t.Fatalf("non-deterministic: %+v vs %+v", c1, c2)
	}
}

func TestDiff_KeyIncludesPackage(t *testing.T) {
	// Two nodes with the same Name but different Package are distinct API entries.
	old := []core.SymbolNode{
		{Name: "Foo", Kind: core.KindGoFunc, Package: "pkga", Signature: "func Foo()"},
	}
	nw := []core.SymbolNode{
		{Name: "Foo", Kind: core.KindGoFunc, Package: "pkgb", Signature: "func Foo()"},
	}
	c := apisurface.Diff(old, nw)
	// pkga.Foo removed, pkgb.Foo added — they are distinct keys
	if len(c.Removed) != 1 || c.Removed[0] != "pkga.Foo" {
		t.Errorf("Removed: want [pkga.Foo], got %v", c.Removed)
	}
	if len(c.Added) != 1 || c.Added[0] != "pkgb.Foo" {
		t.Errorf("Added: want [pkgb.Foo], got %v", c.Added)
	}
}
