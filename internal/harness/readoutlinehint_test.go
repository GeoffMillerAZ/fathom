package harness

// readoutlinehint_test.go: H8's Read outline hint (docs/design/readouts.md
// section 20): one PreToolUse(Read) row of its own, appended after RO-38's,
// opt-in behind [hooks] read_outline_hint, context only; every other
// harness names the hint absent. DELIBERATE GOLDEN CHANGE (2026-10-18):
// cmd/fathom's TestReadOutlineHintOnlyForWholeReadsOfLongUneditedSourceFiles
// is the twin.

import "testing"

var readOutlineHintClaudeHookAddition = []struct {
	event, matcher, marker, sub string
	timeout                     int
}{
	{"PreToolUse", "Read", "fathom readout-hook --event pre-read", "readout-hook --event pre-read", 2},
}

func TestClaudeHookTableReadOutlineHintAddition(t *testing.T) {
	d, _ := Lookup(ClaudeCode)
	base := len(d.Hooks) - len(readOutlineHintClaudeHookAddition)
	for i, want := range readOutlineHintClaudeHookAddition {
		got := d.Hooks[base+i]
		if got.Harness != want.event || got.Matcher != want.matcher || got.Marker != want.marker || got.Verb != want.sub || got.TimeoutSec != want.timeout {
			t.Errorf("addition %d = {%s %q %q %q %d}, want %+v", i, got.Harness, got.Matcher, got.Marker, got.Verb, got.TimeoutSec, want)
		}
		if got.Event != EventPreTool || got.Capability != CapReadOutlineHint || got.Vehicle != VehicleHookInject || got.ObserveOnly {
			t.Errorf("row = %+v, want pre-tool, read-outline-hint, hook-inject, not observe-only", got)
		}
		if got.OptIn != "hooks.read_outline_hint" {
			t.Errorf("opt-in = %q, want hooks.read_outline_hint", got.OptIn)
		}
		if got.Output["context"] != "hookSpecificOutput.additionalContext" || len(got.Output) != 2 {
			t.Errorf("output = %v, want additionalContext and the event name only", got.Output)
		}
	}
	if d.Vehicles[CapReadOutlineHint] != VehicleHookInject {
		t.Errorf("claude-code read-outline-hint = %s", d.Vehicles[CapReadOutlineHint])
	}
	for _, other := range All() {
		if other.ID == ClaudeCode {
			continue
		}
		if v := other.Vehicles[CapReadOutlineHint]; v != VehicleAbsent {
			t.Errorf("%s read-outline-hint = %q, want absent", other.ID, v)
		}
	}
}

func TestClaudeCodecReadsAReadRange(t *testing.T) {
	c, _ := CodecFor(ClaudeCode)
	for _, tc := range []struct {
		input string
		want  bool
	}{
		{`{"file_path":"/r/a.go"}`, false},
		{`{"file_path":"/r/a.go","offset":null,"limit":null}`, false},
		{`{"file_path":"/r/a.go","offset":10}`, true},
		{`{"file_path":"/r/a.go","limit":40}`, true},
	} {
		in, err := c.DecodeInput(EventPreTool, []byte(`{"session_id":"s","tool_name":"Read","tool_use_id":"toolu_1","tool_input":`+tc.input+`}`))
		if err != nil {
			t.Fatal(err)
		}
		if in.ReadRange != tc.want || in.ToolUseID != "toolu_1" {
			t.Errorf("%s: ReadRange %v, ToolUseID %q; want %v, toolu_1", tc.input, in.ReadRange, in.ToolUseID, tc.want)
		}
	}
}
