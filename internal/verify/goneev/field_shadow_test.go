package goneev_test

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/verify"
	"github.com/geoffmilleraz/fathom/internal/verify/goneev"
)

// A `gone: callers Default` row under package p names the function, never
// the same-named struct field Config.Default beside it
// (core.WithoutShadowingFields): its typed callers are read, not one
// unlocated referent for an ambiguous name.
func TestTypedCallersGoFieldNeverShadowsFunc(t *testing.T) {
	fn := core.SymbolNode{Service: "svc", Package: "p", Name: "Default", Kind: core.KindGoFunc, FilePath: "p/p.go", Line: 5}
	field := core.SymbolNode{Service: "svc", Package: "Config", Name: "Config.Default", Kind: core.KindGoField, FilePath: "p/p.go", Line: 3}
	use := core.SymbolNode{Service: "svc", Package: "p", Name: "Use", Kind: core.KindGoFunc, FilePath: "p/use.go", Line: 3}
	nodes := []core.SymbolNode{field, fn, use}
	g := callgraph.Build(nodes)
	callgraph.AddTypedEdges(g, []core.Edge{{FromService: "svc", FromFile: use.FilePath, FromSymbol: use.Key(), ToSymbol: fn.Key(), Kind: core.EdgeDirect}})
	cfg := atHeadConfig(fakeCorpus{files: corpus})
	cfg.FindSymbol = func(string) []core.SymbolNode { return []core.SymbolNode{field, fn} }
	cfg.Graph = g
	cfg.ByKey = map[string]core.SymbolNode{fn.Key(): fn, field.Key(): field, use.Key(): use}

	nonTest, test := goneev.New(context.Background(), cfg).TypedCallers("Default", "p")
	want := []verify.GoneReferent{{File: "p/use.go", Line: 3}}
	if !reflect.DeepEqual(nonTest, want) || len(test) != 0 {
		t.Fatalf("TypedCallers(Default, p) = %v, %v; want the function's caller %v", nonTest, test, want)
	}
}

// TestGoneCallersFieldIsWithheld: field references are not indexed, so a
// `gone: callers Config.Default` row naming a Go struct field never proves
// an absence from its empty referent set: the verdict is Unknown and names
// the reason, never a Pass on zero.
func TestGoneCallersFieldIsWithheld(t *testing.T) {
	field := core.SymbolNode{Service: "svc", Package: "Config", Name: "Config.Default", Kind: core.KindGoField, FilePath: "p/p.go", Line: 3}
	cfg := atHeadConfig(fakeCorpus{files: corpus})
	cfg.FindSymbol = func(string) []core.SymbolNode { return []core.SymbolNode{field} }
	cfg.Graph = callgraph.Build([]core.SymbolNode{field})
	cfg.ByKey = map[string]core.SymbolNode{field.Key(): field}

	c, label := verify.EvaluateGone(verify.Task{ID: "G-1", Gone: "callers Config.Default", Expect: verify.Expect{Package: "p"}}, goneev.New(context.Background(), cfg))
	if c.Status != verify.Unknown || label != "gone-unknown" || !strings.Contains(c.Detail, "field references are not indexed") {
		t.Fatalf("gone: callers on a Go field = %+v (%s), want Unknown naming that field references are not indexed", c, label)
	}
}
