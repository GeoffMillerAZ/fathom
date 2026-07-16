package callgraph_test

import (
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// FIX 1 (resolver half): the extractor canonicalizes an aliased selector call
// to "<import-path-last-segment>.Name" (e.g. "evidence-signature.PreconditionSatisfied").
// For hyphenated directories the declared package name collapses the hyphen
// ("evidencesignature"), so byPkgName can never match — but the import path's
// last segment IS the defining directory's base name by Go import mechanics
// (an import path resolves to exactly that directory). The resolver therefore
// also indexes "dir-base.Name" → key, unique-only.

// TestAliasCanonicalCallee_DirBaseResolves: the real failing claim — the
// evidence-signature tests import the package as `sig` and call
// sig.PreconditionSatisfied; canonicalized to
// "evidence-signature.PreconditionSatisfied" it must resolve to the node whose
// defining file lives in .../evidence-signature/.
func TestAliasCanonicalCallee_DirBaseResolves(t *testing.T) {
	nodes := []core.SymbolNode{
		{
			Service: "svc", Package: "evidencesignature", Name: "PreconditionSatisfied",
			Kind: core.KindGoFunc, FilePath: "foundations/evidence/evidence-signature/types.go",
		},
		{
			Service: "svc", Package: "evidencesignature_test", Name: "TestPreconditionSatisfied_Existence",
			Kind: core.KindGoFunc, FilePath: "foundations/evidence/evidence-signature/evidencesignature_test.go",
			Calls: []string{"evidence-signature.PreconditionSatisfied"},
		},
	}
	g := callgraph.Build(nodes)

	want := []string{"svc::evidencesignature_test.TestPreconditionSatisfied_Existence"}
	if got := callgraph.TestImpact(g, "svc::evidencesignature.PreconditionSatisfied"); !reflect.DeepEqual(got, want) {
		t.Errorf("TestImpact(PreconditionSatisfied) = %v, want %v", got, want)
	}
	if !callgraph.Reachable(g, "TestPreconditionSatisfied_Existence", "PreconditionSatisfied") {
		t.Error("Reachable(test, PreconditionSatisfied) = false, want true")
	}
}

// TestAliasCanonicalCallee_AmbiguousDirBaseResolvesToNothing: two directories
// with the same base name each defining the same symbol name make the
// dir-base form genuinely ambiguous — it must resolve to NOTHING, never
// first-match-wins.
func TestAliasCanonicalCallee_AmbiguousDirBaseResolvesToNothing(t *testing.T) {
	nodes := []core.SymbolNode{
		{
			Service: "a", Package: "util", Name: "Do",
			Kind: core.KindGoFunc, FilePath: "foundations/a/util/util.go",
		},
		{
			Service: "b", Package: "util", Name: "Do",
			Kind: core.KindGoFunc, FilePath: "kits/b/util/util.go",
		},
		{
			Service: "a", Package: "util_test", Name: "TestDo",
			Kind: core.KindGoFunc, FilePath: "foundations/a/util/util_test.go",
			Calls: []string{"util.Do"},
		},
	}
	g := callgraph.Build(nodes)

	if g.EdgeCount() != 0 {
		t.Errorf("EdgeCount() = %d, want 0 (ambiguous dir-base callee must not resolve)", g.EdgeCount())
	}
}
