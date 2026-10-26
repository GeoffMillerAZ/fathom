package extractor

import (
	"slices"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
)

// TestSplitTSMatches_CallNoneDropsTheCallAtItsNode: a node a match captures as
// @call.none is call-shaped but no call (an HLSL attribute [numthreads(8, 8, 1)]
// or a register(b0) clause, which the grammar reads as call_expression), so a
// @reference.call match over that same node is dropped, whichever match comes
// first. A call elsewhere with the same callee name stays.
func TestSplitTSMatches_CallNoneDropsTheCallAtItsNode(t *testing.T) {
	src := []byte("[Domain(1)] void f() { Domain(2); }")
	kinds := map[string]tsDefKind{"function": {Kind: core.SourceKind("HlslFunc"), Callable: true}}
	attr := tsengine.Node{StartByte: 1, EndByte: 10}
	attrName := tsengine.Node{StartByte: 1, EndByte: 7}
	body := tsengine.Node{StartByte: 23, EndByte: 32}
	bodyName := tsengine.Node{StartByte: 23, EndByte: 29}
	callAt := func(site, name tsengine.Node) tsengine.Match {
		return tsengine.Match{Captures: []tsengine.Capture{{Name: "reference.call", Node: site}, {Name: "name", Node: name}}}
	}
	none := tsengine.Match{Captures: []tsengine.Capture{{Name: "call.none", Node: attr}}}
	for _, order := range [][]tsengine.Match{
		{callAt(attr, attrName), none, callAt(body, bodyName)},
		{none, callAt(attr, attrName), callAt(body, bodyName)},
	} {
		_, calls, _ := splitTSMatches(order, kinds, src, nil)
		if len(calls) != 1 || calls[0].start != body.StartByte || calls[0].text != "Domain" {
			t.Fatalf("calls = %+v, want only the body's Domain at %d", calls, body.StartByte)
		}
	}
}

// TestSplitTSMatches_LocalScopeDropsDefinitions: a node a match captures as
// @local.scope (a function's body) holds locals, so a definition inside it is
// dropped, whatever pattern named it: a query may name a declaration under a
// preprocessor conditional at any depth without anchoring every shape at the
// file's top level. A definition outside every such range stays.
func TestSplitTSMatches_LocalScopeDropsDefinitions(t *testing.T) {
	src := []byte("int g; void f() { int l; }")
	kinds := map[string]tsDefKind{"variable": {Kind: core.SourceKind("HlslVariable")}, "function": {Kind: core.SourceKind("HlslFunc"), Callable: true}}
	def := func(kind string, start, end, nameStart, nameEnd uint32) tsengine.Match {
		return tsengine.Match{Captures: []tsengine.Capture{{Name: "definition." + kind, Node: tsengine.Node{StartByte: start, EndByte: end}}, {Name: "name", Node: tsengine.Node{StartByte: nameStart, EndByte: nameEnd}}}}
	}
	body := tsengine.Node{StartByte: 16, EndByte: 26}
	local := tsengine.Match{Captures: []tsengine.Capture{{Name: "local.scope", Node: body}}}
	defs, _, _ := splitTSMatches([]tsengine.Match{def("variable", 18, 24, 22, 23), def("variable", 0, 6, 4, 5), def("function", 7, 26, 12, 13), local}, kinds, src, nil)
	var names []string
	for _, d := range defs {
		names = append(names, d.name)
	}
	if want := []string{"g", "f"}; !slices.Equal(names, want) {
		t.Fatalf("defs = %v, want %v (l is a local of f's body)", names, want)
	}
}
