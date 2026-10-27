package harness

// egressweb_test.go: the egress guard's web binding (ADR-083): one
// PreToolUse(WebFetch|WebSearch) row on Claude Code, appended after
// ADR-080's rows, on by default (the guard warns by default), context or a
// deny; Codex and every other harness name it absent. DELIBERATE GOLDEN
// CHANGE (2026-10-26): the committed plugin tree and cmd/fathom's
// TestEgressWebFetchHostCheck are the twins.

import (
	"strings"
	"testing"
)

var egressWebClaudeHookAddition = []struct {
	event, matcher, marker, sub string
	timeout                     int
}{
	{"PreToolUse", "WebFetch|WebSearch", "fathom edit-hook --pre", "edit-hook --pre", 5},
}

func TestClaudeHookTableEgressWebAddition(t *testing.T) {
	d, _ := Lookup(ClaudeCode)
	base := len(d.Hooks) - len(egressWebClaudeHookAddition)
	for i, want := range egressWebClaudeHookAddition {
		got := d.Hooks[base+i]
		if got.Harness != want.event || got.Matcher != want.matcher || got.Marker != want.marker || got.Verb != want.sub || got.TimeoutSec != want.timeout {
			t.Errorf("addition %d = {%s %q %q %q %d}, want %+v", i, got.Harness, got.Matcher, got.Marker, got.Verb, got.TimeoutSec, want)
		}
		if got.Event != EventPreTool || got.Capability != CapGuardPreWeb || got.Vehicle != VehicleHookInject || got.ObserveOnly || got.OptIn != "" {
			t.Errorf("row = %+v, want pre-tool, guard-pre-web, hook-inject, not observe-only, not opt-in", got)
		}
		if got.Input["url"] != "tool_input.url" || got.Input["allowed_domains"] != "tool_input.allowed_domains" {
			t.Errorf("input = %v, want the WebFetch url and the WebSearch allowed_domains", got.Input)
		}
	}
	for _, tool := range []string{"WebFetch", "WebSearch"} {
		if c := ClassifyTool(ClaudeCode, tool); c != ToolWeb {
			t.Errorf("%s classifies as %q, want %q", tool, c, ToolWeb)
		}
	}
	if d.Vehicles[CapGuardPreWeb] != VehicleHookInject {
		t.Errorf("claude-code guard-pre-web = %s", d.Vehicles[CapGuardPreWeb])
	}
	for _, other := range All() {
		if other.ID == ClaudeCode {
			continue
		}
		if v := other.Vehicles[CapGuardPreWeb]; v != VehicleAbsent {
			t.Errorf("%s guard-pre-web = %q, want absent", other.ID, v)
		}
	}
	codex, _ := Lookup(Codex)
	if note := codex.Notes[CapGuardPreWeb]; !strings.Contains(note, "web search") || !strings.Contains(note, "MCP") {
		t.Errorf("codex guard-pre-web note %q must say web search and MCP calls are not intercepted", note)
	}
}

func TestClaudeCodecReadsWebToolInput(t *testing.T) {
	c, _ := CodecFor(ClaudeCode)
	in, err := c.DecodeInput(EventPreTool, []byte(`{"session_id":"s","tool_name":"WebFetch","tool_input":{"url":"https://docs.example.com/x","prompt":"summarize"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if in.Tool != ToolWeb || in.URL != "https://docs.example.com/x" || in.WebDomains != "" {
		t.Errorf("WebFetch: tool %q url %q domains %v", in.Tool, in.URL, in.WebDomains)
	}
	in, err = c.DecodeInput(EventPreTool, []byte(`{"session_id":"s","tool_name":"WebSearch","tool_input":{"query":"q","allowed_domains":["docs.example.com","api.example.org"]}}`))
	if err != nil {
		t.Fatal(err)
	}
	if in.Tool != ToolWeb || in.URL != "" || in.WebDomains != "docs.example.com\napi.example.org" {
		t.Errorf("WebSearch: tool %q url %q domains %v", in.Tool, in.URL, in.WebDomains)
	}
}
