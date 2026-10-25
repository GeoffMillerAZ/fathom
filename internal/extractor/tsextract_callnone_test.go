package extractor

import (
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
