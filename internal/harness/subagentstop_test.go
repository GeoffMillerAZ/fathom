package harness

// subagentstop_test.go — RO-38 (docs/plans/readouts.md): SubagentStop joins
// the port with its Claude Code binding and its additionalContext entry,
// opt-in behind [hooks] claim_check, and every other harness names the
// claim check absent. RO-V1 (docs/bench/readings/readouts/stop-facts.md)
// read the keys on Claude Code 2.1.302.

import (
	"encoding/json"
	"strings"
	"testing"
)

// claimCheckClaudeHookAddition is RO-38's one row, appended after TR-2's
// pair. DELIBERATE GOLDEN CHANGE (2026-10-18): cmd/fathom's
// TestHookSpecsForClaudeWiresSubagentStopOnlyUnderClaimCheck is the twin.
var claimCheckClaudeHookAddition = []struct {
	event, matcher, marker, sub string
	timeout                     int
}{
	{"SubagentStop", "", "fathom stop-hook --event subagent-stop", "stop-hook --event subagent-stop", 2},
}

func TestClaudeHookTableClaimCheckAddition(t *testing.T) {
	d, _ := Lookup(ClaudeCode)
	base := len(preSeamClaudeHookTable) + len(postSeamClaudeHookAdditions) + len(waveThreeClaudeHookAdditions) + len(promptHookClaudeHookAddition) + len(testRunClaudeHookAddition)
	for i, want := range claimCheckClaudeHookAddition {
		got := d.Hooks[base+i]
		if got.Harness != want.event || got.Matcher != want.matcher || got.Marker != want.marker || got.Verb != want.sub || got.TimeoutSec != want.timeout {
			t.Errorf("addition %d = {%s %q %q %q %d}, want {%s %q %q %q %d}", i,
				got.Harness, got.Matcher, got.Marker, got.Verb, got.TimeoutSec,
				want.event, want.matcher, want.marker, want.sub, want.timeout)
		}
		if got.Event != EventSubagentStop || got.Capability != CapClaimCheck || got.Vehicle != VehicleHookInject {
			t.Errorf("row = %+v, want the subagent-stop event, claim-check, hook-inject", got)
		}
		if got.OptIn != "hooks.claim_check" {
			t.Errorf("opt-in = %q, want hooks.claim_check (written only while the check is on)", got.OptIn)
		}
	}
	for _, b := range d.Hooks {
		if b.OptIn != "" && b.Event != EventSubagentStop {
			t.Errorf("%s/%s carries opt-in %q: only RO-38's row is opt-in", b.Harness, b.Matcher, b.OptIn)
		}
	}
	if d.Vehicles[CapClaimCheck] != VehicleHookInject {
		t.Errorf("claude-code claim-check = %s", d.Vehicles[CapClaimCheck])
	}
	for _, other := range All() {
		if other.ID == ClaudeCode {
			continue
		}
		if v := other.Vehicles[CapClaimCheck]; v != VehicleAbsent {
			t.Errorf("%s claim-check = %q, want absent (unbound, named by doctor)", other.ID, v)
		}
		if _, ok := other.Binding(EventSubagentStop); ok {
			t.Errorf("%s binds subagent-stop; only Claude Code does", other.ID)
		}
	}
}

func TestClaudeCodecSubagentStop(t *testing.T) {
	c, _ := CodecFor(ClaudeCode)
	raw := `{"session_id":"s1","transcript_path":"/t/s1.jsonl","cwd":"/r","agent_id":"a1","agent_type":"general-purpose",` +
		`"hook_event_name":"SubagentStop","stop_hook_active":true,"agent_transcript_path":"/t/s1/subagents/agent-a1.jsonl",` +
		`"last_assistant_message":"all tests pass"}`
	in, err := c.DecodeInput(EventSubagentStop, []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if in.LastMessage != "all tests pass" || in.AgentID != "a1" || in.AgentType != "general-purpose" || !in.ReEntrant || in.SessionID != "s1" {
		t.Errorf("decoded %+v", in)
	}
	stop, err := c.DecodeInput(EventStop, []byte(`{"session_id":"s1","last_assistant_message":"done","agent_id":"x"}`))
	if err != nil || stop.LastMessage != "done" || stop.AgentID != "" {
		t.Errorf("stop decoded %+v (%v): want the message and no agent id", stop, err)
	}
	post, err := c.DecodeInput(EventPostTool, []byte(`{"session_id":"s1","tool_name":"Bash","tool_input":{"command":"go test ./..."},"agent_id":"a1"}`))
	if err != nil || post.AgentID != "a1" || post.LastMessage != "" {
		t.Errorf("post-tool decoded %+v (%v): want the subagent's id", post, err)
	}
	out, err := c.EncodeOutput(EventSubagentStop, HookOutput{Context: "claim check: line"})
	if err != nil {
		t.Fatal(err)
	}
	var env struct {
		HSO struct {
			Event   string `json:"hookEventName"`
			Context string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if json.Unmarshal(out, &env) != nil || env.HSO.Event != "SubagentStop" || env.HSO.Context != "claim check: line" {
		t.Errorf("encoded %s, want SubagentStop additionalContext", out)
	}
	if strings.Contains(string(out), "systemMessage") {
		t.Errorf("SubagentStop documents additionalContext; got %s", out)
	}
}
