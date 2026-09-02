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

// claimHookSpecAdditions are the two rows vision 26 §4 appended AFTER the
// pre-seam table (PC-23 InstructionsLoaded, PC-24 ConfigChange) — the one
// deliberate change to what init, plugin render, reconcile and doctor
// carry since the seam. Appended, never interleaved: the pre-seam prefix
// stays byte-identical, so a repo wired before this change gains exactly
// two entries on its next init/reconcile and loses nothing.
var claimHookSpecAdditions = []hookSpec{
	{event: "InstructionsLoaded", matcher: "", marker: "fathom session-hook --event instructions-loaded", sub: "session-hook --event instructions-loaded", timeout: 5},
	{event: "ConfigChange", matcher: "", marker: "fathom config audit --hook", sub: "config audit --hook", timeout: 10},
}

// TestHookSpecsForClaude pins HP-2: init, plugin render, reconcile and
// doctor all read fathomHookSpecs, and fathomHookSpecs is now
// hookSpecsFor("claude-code") — the pre-seam literal as a byte-identical
// prefix, followed by the vision 26 §4 additions.
func TestHookSpecsForClaude(t *testing.T) {
	want := append(append([]hookSpec{}, preSeamFathomHookSpecs...), claimHookSpecAdditions...)
	got := hookSpecsFor(harness.ClaudeCode)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("hookSpecsFor(claude-code) drifted from the pre-seam table + the vision 26 §4 additions:\n got %+v\nwant %+v", got, want)
	}
	if !reflect.DeepEqual(fathomHookSpecs, want) {
		t.Fatalf("fathomHookSpecs drifted from the pre-seam table + the vision 26 §4 additions:\n got %+v\nwant %+v", fathomHookSpecs, want)
	}
	if !reflect.DeepEqual(got[:len(preSeamFathomHookSpecs)], preSeamFathomHookSpecs) {
		t.Fatalf("the pre-seam prefix is not byte-identical:\n got %+v\nwant %+v", got[:len(preSeamFathomHookSpecs)], preSeamFathomHookSpecs)
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
