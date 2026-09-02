package main

import (
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/harness"
)

// preSeamFathomHookSpecs is the fathomHookSpecs literal exactly as
// cmd/fathom/init.go carried it before the harness seam (HEAD 3f0c3b1).
// HP-2's proof: the registry-derived table is this table. Do not edit this
// literal to make the test pass — it is what every wired repository holds.
var preSeamFathomHookSpecs = []hookSpec{
	{event: "SessionStart", matcher: "startup|resume|clear|compact", marker: "fathom session-hook", sub: "session-hook", timeout: 10},
	{event: "SessionEnd", matcher: "", marker: "fathom session-end-hook", sub: "session-end-hook", timeout: 10},
	{event: "PreCompact", matcher: "manual|auto", marker: "fathom pre-compact-hook", sub: "pre-compact-hook", timeout: 10},
	{event: "PostToolUse", matcher: "Edit|Write|MultiEdit", marker: "fathom edit-hook", sub: "edit-hook", timeout: 5},
	{event: "PreToolUse", matcher: "Edit|Write|MultiEdit", marker: "fathom edit-hook --pre", sub: "edit-hook --pre", timeout: 5},
	{event: "PreToolUse", matcher: "Bash", marker: "fathom edit-hook --pre", sub: "edit-hook --pre", timeout: 5},
	{event: "PreToolUse", matcher: "Grep|Glob|Read", marker: "fathom edit-hook --pre", sub: "edit-hook --pre", timeout: 5},
	{event: "Stop", matcher: "", marker: "fathom stop-hook", sub: "stop-hook", timeout: 10},
}

// TestHookSpecsForClaude pins HP-2: init, plugin render, reconcile and
// doctor all read fathomHookSpecs, and fathomHookSpecs is now
// hookSpecsFor("claude-code") — byte-identical to the pre-seam literal.
func TestHookSpecsForClaude(t *testing.T) {
	got := hookSpecsFor(harness.ClaudeCode)
	if !reflect.DeepEqual(got, preSeamFathomHookSpecs) {
		t.Fatalf("hookSpecsFor(claude-code) drifted from the pre-seam table:\n got %+v\nwant %+v", got, preSeamFathomHookSpecs)
	}
	if !reflect.DeepEqual(fathomHookSpecs, preSeamFathomHookSpecs) {
		t.Fatalf("fathomHookSpecs drifted from the pre-seam table:\n got %+v\nwant %+v", fathomHookSpecs, preSeamFathomHookSpecs)
	}
	if got := hookSpecsFor("nope"); got != nil {
		t.Errorf("unknown id must yield no hook table, got %+v", got)
	}
	if got := hookSpecsFor(harness.Agents); got != nil {
		t.Errorf("the generic pull tier has no hook table, got %+v", got)
	}
	// Codex documents its bindings as data; the table exists but init
	// never plans it (initTargetsFor: Hooks=false until a codec lands).
	if got := hookSpecsFor(harness.Codex); len(got) == 0 {
		t.Error("codex documents hook bindings; hookSpecsFor must expose them as data")
	}
}
