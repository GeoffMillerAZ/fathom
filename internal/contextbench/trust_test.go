package contextbench

import (
	"os"
	"strings"
	"testing"
)

// TestTrustedWithoutRecheck_Fixture pins the RC-17 reading against a small,
// hand-built stream transcript (testdata/contextbench/trust-stream.jsonl):
//
//   - call 1, mcp__fathom__find_symbol({"symbol":"Foo"}), result names
//     internal/pkg/foo.go. Two turns later a Read targets that exact path —
//     a re-check, so this call is NOT trusted.
//   - call 2, mcp__fathom__references({"symbol":"Bar"}), result names
//     internal/pkg/bar.go. Nothing later targets it — trusted.
//   - the Read call itself is not a fathom call and is not counted.
func TestTrustedWithoutRecheck_Fixture(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/contextbench/trust-stream.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	got := TrustedWithoutRecheck(raw)
	if got.Calls != 2 {
		t.Fatalf("Calls = %d, want 2", got.Calls)
	}
	if got.Trusted != 1 {
		t.Fatalf("Trusted = %d, want 1", got.Trusted)
	}
	if got.Rate != 0.5 {
		t.Fatalf("Rate = %v, want 0.5", got.Rate)
	}
	find, ok := got.ByTool["find_symbol"]
	if !ok || find.Calls != 1 || find.Trusted != 0 || find.Rate != 0 {
		t.Fatalf("by_tool[find_symbol] = %+v, want {1 0 0}", find)
	}
	refs, ok := got.ByTool["references"]
	if !ok || refs.Calls != 1 || refs.Trusted != 1 || refs.Rate != 1 {
		t.Fatalf("by_tool[references] = %+v, want {1 1 1}", refs)
	}
	if _, ok := got.ByTool["Read"]; ok {
		t.Fatalf("a non-fathom tool call must never be counted: %+v", got.ByTool)
	}
}

// TestTrustedWithoutRecheck_NoFathomCalls: a transcript with no fathom call
// at all carries no reading — Calls stays 0 and ByTool is nil, the same
// "not recorded" convention GoldSourceRate follows for empty input.
func TestTrustedWithoutRecheck_NoFathomCalls(t *testing.T) {
	transcript := []byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"1","name":"Grep","input":{"pattern":"foo"}}],"usage":{}}}
{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"1","content":"no matches"}]}}
`)
	got := TrustedWithoutRecheck(transcript)
	if got.Calls != 0 || got.Trusted != 0 || got.ByTool != nil {
		t.Fatalf("got %+v, want the zero value", got)
	}
}

// TestTrustedWithoutRecheck_OutsideWindow: a re-check more than
// recheckWindowTurns turns later does not count — the call stays trusted.
func TestTrustedWithoutRecheck_OutsideWindow(t *testing.T) {
	var b strings.Builder
	b.WriteString(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"1","name":"mcp__fathom__find_symbol","input":{"symbol":"Foo"}}],"usage":{}}}` + "\n")
	b.WriteString(`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"1","content":"internal/pkg/foo.go"}]}}` + "\n")
	// Three filler turns with no tool calls — pushes the eventual Read
	// outside the window.
	for i := 0; i < recheckWindowTurns; i++ {
		b.WriteString(`{"type":"assistant","message":{"content":[{"type":"text","text":"thinking"}],"usage":{}}}` + "\n")
	}
	b.WriteString(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"2","name":"Read","input":{"file_path":"internal/pkg/foo.go"}}],"usage":{}}}` + "\n")
	b.WriteString(`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"2","content":"package pkg"}]}}` + "\n")

	got := TrustedWithoutRecheck([]byte(b.String()))
	if got.Calls != 1 || got.Trusted != 1 {
		t.Fatalf("got %+v, want a trusted call (the Read fell outside the window)", got)
	}
}

func TestAggregateTrustedWithoutRecheck(t *testing.T) {
	a := TrustedWithoutRecheckStats{Calls: 2, Trusted: 1, ByTool: map[string]TrustedToolStats{
		"find_symbol": {Calls: 1, Trusted: 0},
		"references":  {Calls: 1, Trusted: 1},
	}}
	b := TrustedWithoutRecheckStats{Calls: 1, Trusted: 1, ByTool: map[string]TrustedToolStats{
		"references": {Calls: 1, Trusted: 1},
	}}
	results := []ArmResult{
		{TaskID: "T1", Arm: "fathom", TrustedWithoutRecheck: &a},
		{TaskID: "T2", Arm: "fathom", TrustedWithoutRecheck: &b},
		{TaskID: "T3", Arm: "grep"}, // no reading — must not affect the total
	}
	got := AggregateTrustedWithoutRecheck(results)
	if got == nil {
		t.Fatal("got nil, want an aggregate")
	}
	if got.Calls != 3 || got.Trusted != 2 {
		t.Fatalf("got calls=%d trusted=%d, want 3/2", got.Calls, got.Trusted)
	}
	refs := got.ByTool["references"]
	if refs.Calls != 2 || refs.Trusted != 2 || refs.Rate != 1 {
		t.Fatalf("by_tool[references] = %+v, want {2 2 1}", refs)
	}
	find := got.ByTool["find_symbol"]
	if find.Calls != 1 || find.Trusted != 0 || find.Rate != 0 {
		t.Fatalf("by_tool[find_symbol] = %+v, want {1 0 0}", find)
	}
}

func TestAggregateTrustedWithoutRecheck_Empty(t *testing.T) {
	if got := AggregateTrustedWithoutRecheck(nil); got != nil {
		t.Fatalf("got %+v, want nil", got)
	}
	if got := AggregateTrustedWithoutRecheck([]ArmResult{{TaskID: "T1", Arm: "grep"}}); got != nil {
		t.Fatalf("got %+v, want nil (no cell recorded a reading)", got)
	}
}

func TestRenderTrustedWithoutRecheck(t *testing.T) {
	if got := RenderTrustedWithoutRecheck(nil); got != "" {
		t.Fatalf("nil renders %q, want \"\"", got)
	}
	stats := &TrustedWithoutRecheckStats{Calls: 3, Trusted: 2, Rate: 2.0 / 3.0, ByTool: map[string]TrustedToolStats{
		"references":  {Calls: 2, Trusted: 2, Rate: 1},
		"find_symbol": {Calls: 1, Trusted: 0, Rate: 0},
	}}
	got := RenderTrustedWithoutRecheck(stats)
	if !strings.Contains(got, "trusted_without_recheck: calls=3 trusted=2") {
		t.Fatalf("summary line missing: %q", got)
	}
	// Highest rate first.
	if strings.Index(got, "references") > strings.Index(got, "find_symbol") {
		t.Fatalf("expected references (rate 1) rendered before find_symbol (rate 0): %q", got)
	}
}
