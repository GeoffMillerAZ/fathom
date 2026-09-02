package callgraph_test

import (
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// FIX 2: a BARE callee name (no dot) recorded from a caller in package P is,
// by Go language semantics, a reference into P itself whenever P declares that
// name — a bare (unqualified) identifier CANNOT denote another package's
// symbol; the only cross-package reference form in Go is a qualified
// identifier (spec: "Qualified identifiers"), which the extractor records with
// a dot. Binding a bare callee to the caller's own package therefore can never
// shadow a genuinely-intended cross-package call: such a call is always
// recorded qualified and never reaches this rule.
//
// The extractor's per-FILE localFuncs already resolves bare calls to same-file
// symbols; a bare call to a same-package symbol declared in ANOTHER file fell
// through to global bare-name uniqueness and was dropped as soon as any other
// package declared the same name.

// TestBareCallee_SamePackageBinds pins the real failing claim — llm-surface
// {symbol: New, package: foundations/llmclient/openai}: the openai tests are
// in-package but live in different files from config.go, so "New" was
// recorded bare and went globally ambiguous (dozens of packages declare New).
func TestBareCallee_SamePackageBinds(t *testing.T) {
	nodes := []core.SymbolNode{
		{
			Service: "svc", Package: "openai", Name: "New",
			Kind: core.KindGoFunc, FilePath: "foundations/llmclient/openai/config.go",
		},
		{
			// A second New elsewhere makes the bare name globally ambiguous —
			// exactly the situation that used to drop the edge.
			Service: "svc", Package: "bedrock", Name: "New",
			Kind: core.KindGoFunc, FilePath: "foundations/llmclient/bedrock/client.go",
		},
		{
			Service: "svc", Package: "openai", Name: "TestNew",
			Kind: core.KindGoFunc, FilePath: "foundations/llmclient/openai/client_test.go",
			Calls: []string{"New"},
		},
	}
	g := callgraph.Build(nodes)

	want := []string{"svc::foundations/llmclient/openai/openai.TestNew"}
	if got := callgraph.TestImpact(g, "svc::openai.New"); !reflect.DeepEqual(got, want) {
		t.Errorf("TestImpact(openai.New) = %v, want %v", got, want)
	}
	// The other package's New gains no edge: this is language semantics, not
	// first-match-wins.
	if got := callgraph.TestImpact(g, "svc::bedrock.New"); len(got) != 0 {
		t.Errorf("TestImpact(bedrock.New) = %v, want [] (bare New binds to the CALLER's package only)", got)
	}
}

// TestBareCallee_UndeclaredInCallerPackageStaysAmbiguous: when the caller's
// package does NOT declare the bare name, the rule does not fire and genuine
// global ambiguity keeps resolving to nothing.
func TestBareCallee_UndeclaredInCallerPackageStaysAmbiguous(t *testing.T) {
	nodes := []core.SymbolNode{
		{Service: "svc", Package: "alpha", Name: "New", Kind: core.KindGoFunc, FilePath: "alpha/a.go"},
		{Service: "svc", Package: "beta", Name: "New", Kind: core.KindGoFunc, FilePath: "beta/b.go"},
		{
			Service: "svc", Package: "gamma", Name: "TestNew",
			Kind: core.KindGoFunc, FilePath: "gamma/g_test.go",
			Calls: []string{"New"},
		},
	}
	g := callgraph.Build(nodes)
	if g.EdgeCount() != 0 {
		t.Errorf("EdgeCount() = %d, want 0 (caller's package declares no New; ambiguity stays honest)", g.EdgeCount())
	}
}

// TestBareCallee_MethodCallerDoesNotBindThroughReceiverName: a method node's
// Package field is its RECEIVER TYPE name, not a Go package, so the
// same-package rule must not fire for method callers — a receiver name that
// collides with a real package name must not fabricate an edge.
func TestBareCallee_MethodCallerDoesNotBindThroughReceiverName(t *testing.T) {
	nodes := []core.SymbolNode{
		{
			// A real package literally named "Client" declaring Helper.
			Service: "svc", Package: "Client", Name: "Helper",
			Kind: core.KindGoFunc, FilePath: "client/helper.go",
		},
		{
			// Another Helper so the bare name is globally ambiguous.
			Service: "svc", Package: "other", Name: "Helper",
			Kind: core.KindGoFunc, FilePath: "other/helper.go",
		},
		{
			// Method caller: Package = receiver type "Client" — a name
			// collision with the package above, not membership in it.
			Service: "svc", Package: "Client", Name: "Client.Do",
			Kind: core.KindGoMethod, FilePath: "api/client.go",
			Calls: []string{"Helper"},
		},
	}
	g := callgraph.Build(nodes)
	if g.EdgeCount() != 0 {
		t.Errorf("EdgeCount() = %d, want 0 (receiver name is not a package; no binding through it)", g.EdgeCount())
	}
}
