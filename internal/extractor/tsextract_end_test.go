package extractor

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
)

// splitTSMatches is splitTSMatchesWith with no call qualification and the
// call attachments dropped: the shape these end-capture tests read.
func splitTSMatches(matches []tsengine.Match, kinds map[string]tsDefKind, src []byte, norm func(string) string) ([]*tsDef, []tsCallSite, int) {
	defs, calls, _, typeRefs := splitTSMatchesWith(matches, kinds, src, norm, false)
	return defs, calls, typeRefs
}

// TestSplitTSMatches_EndCaptureExtendsDefinition: a grammar that puts a
// callable's body in a sibling of its signature (tree-sitter-dart's
// function_signature + function_body, LB-Q-dart) cannot capture one node that
// spans both, so a match may also capture @end: the definition then reaches
// the end of that node — its EndLine, the calls its body makes and the
// definitions nested in it all follow. @end never shortens a definition.
func TestSplitTSMatches_EndCaptureExtendsDefinition(t *testing.T) {
	src := []byte("void f() {\n  g();\n}\n")
	kinds := map[string]tsDefKind{"function": {Kind: core.SourceKind("DartFunction"), Callable: true}}
	sig := tsengine.Node{StartByte: 0, EndByte: 8, StartRow: 0}
	body := tsengine.Node{StartByte: 9, EndByte: uint32(len(src) - 1), StartRow: 0}
	name := tsengine.Node{StartByte: 5, EndByte: 6, StartRow: 0}

	t.Run("extends", func(t *testing.T) {
		defs, _, _ := splitTSMatches([]tsengine.Match{{Captures: []tsengine.Capture{
			{Name: "definition.function", Node: sig},
			{Name: "name", Node: name},
			{Name: "body", Node: body},
			{Name: "end", Node: body},
		}}}, kinds, src, nil)
		if len(defs) != 1 {
			t.Fatalf("defs = %d, want 1", len(defs))
		}
		d := defs[0]
		if d.start != 0 || d.end != body.EndByte {
			t.Errorf("range = [%d,%d), want [0,%d)", d.start, d.end, body.EndByte)
		}
		if !d.hasBody || d.bodyStart != body.StartByte {
			t.Errorf("body = %v/%d, want %d", d.hasBody, d.bodyStart, body.StartByte)
		}
		if got := tsEndLine(src, d); got != 3 {
			t.Errorf("EndLine = %d, want 3", got)
		}
		if got := tsSignature(src, d, 200); got != "void f()" {
			t.Errorf("Signature = %q, want %q", got, "void f()")
		}
	})

	t.Run("never shortens", func(t *testing.T) {
		defs, _, _ := splitTSMatches([]tsengine.Match{{Captures: []tsengine.Capture{
			{Name: "definition.function", Node: tsengine.Node{StartByte: 0, EndByte: body.EndByte}},
			{Name: "name", Node: name},
			{Name: "end", Node: sig},
		}}}, kinds, src, nil)
		if len(defs) != 1 || defs[0].end != body.EndByte {
			t.Fatalf("defs = %+v, want one ending at %d", defs, body.EndByte)
		}
	})
}
