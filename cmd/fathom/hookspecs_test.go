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
	{event: "PreToolUse", matcher: "Grep|Glob|Read", marker: "fathom edit-hook --pre", sub: "edit-hook --pre", timeout: 5, async: true}, // RT-169
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

// waveThreeHookSpecAdditions are HP-26..HP-31 (docs/plans/harness-port.md
// wave 3): the advisory events, appended after the vision 26 §4 pair.
//
// DELIBERATE GOLDEN CHANGE (2026-09-12), stated rather than slipped in:
// this is the second change to what `fathom init` writes since the seam,
// and it is additive in exactly the same way. The pre-seam eight and the
// vision 26 §4 two are untouched byte for byte (the assertions below still
// pin the prefix), so a repository wired before this change gains these
// seven entries on its next `init`/`fleet reconcile` and loses nothing.
// internal/harness/harness_test.go carries the registry-side twin.
var waveThreeHookSpecAdditions = []hookSpec{
	{event: "SubagentStart", matcher: "", marker: "fathom session-hook --event subagent-start", sub: "session-hook --event subagent-start", timeout: 5},
	{event: "PostToolUseFailure", matcher: `mcp__fathom__.*|mcp__plugin_[^_]*_fathom__.*`, marker: "fathom session-hook --event tool-failure", sub: "session-hook --event tool-failure", timeout: 5},
	{event: "FileChanged", matcher: "CLAUDE.md|AGENTS.md|concerns.cue|config.toml", marker: "fathom config audit --hook --event file-changed", sub: "config audit --hook --event file-changed", timeout: 10},
	{event: "CwdChanged", matcher: "", marker: "fathom session-hook --event cwd-changed", sub: "session-hook --event cwd-changed", timeout: 5},
	{event: "DirectoryAdded", matcher: "", marker: "fathom session-hook --event cwd-changed", sub: "session-hook --event cwd-changed", timeout: 5},
	{event: "Setup", matcher: "", marker: "fathom doctor --brief", sub: "doctor --brief", timeout: 15},
}

// promptHookSpecAddition is O57 items 4/5's two rows, appended after
// waveThreeHookSpecAdditions for the same reason that group was appended
// after the vision 26 §4 pair. internal/harness/harness_test.go carries
// the registry-side twin.
var promptHookSpecAddition = []hookSpec{
	{event: "UserPromptSubmit", matcher: "", marker: "fathom prompt-hook", sub: "prompt-hook", timeout: 10},
	{event: "PostToolUse", matcher: "Bash", marker: "fathom edit-hook", sub: "edit-hook", timeout: 5},
}

// testRunHookSpecAddition is TR-2's two rows (docs/plans/test-run-hook.md),
// appended after promptHookSpecAddition so nothing above moves: the
// test-run hook on PostToolUse(Bash) and on PostToolUseFailure(Bash) —
// TR-V1 read failing test runs arriving on both. The markers are chosen so
// neither is a substring of the other.
var testRunHookSpecAddition = []hookSpec{
	{event: "PostToolUse", matcher: "Bash", marker: "fathom session-hook --event test-run", sub: "session-hook --event test-run", timeout: 5},
	{event: "PostToolUseFailure", matcher: "Bash", marker: "fathom session-hook --event failed-test-run", sub: "session-hook --event failed-test-run", timeout: 5},
}

// readoutHookSpecAddition is the readout hook's pair (ADR-071), opted in
// on the user's [readouts] mode.
var readoutHookSpecAddition = []hookSpec{
	{event: "PostToolUse", matcher: "Bash", marker: "fathom readout-hook --event post-tool", sub: "readout-hook --event post-tool", timeout: 5, optIn: harness.OptInReadouts},
	{event: "PostToolUseFailure", matcher: "Bash", marker: "fathom readout-hook --event tool-failure", sub: "readout-hook --event tool-failure", timeout: 5, optIn: harness.OptInReadouts},
}

// TestHookSpecsForClaude pins HP-2: init, plugin render, reconcile and
// doctor all read fathomHookSpecs, and fathomHookSpecs is now
// hookSpecsFor("claude-code") — the pre-seam literal as a byte-identical
// prefix, followed by the vision 26 §4 additions.
func TestHookSpecsForClaude(t *testing.T) {
	want := append(append(append(append(append([]hookSpec{}, preSeamFathomHookSpecs...), claimHookSpecAdditions...), waveThreeHookSpecAdditions...), promptHookSpecAddition...), testRunHookSpecAddition...)
	got := hookSpecsFor(harness.ClaudeCode)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("hookSpecsFor(claude-code) drifted from the pre-seam table + the vision 26 §4 + wave 3 additions:\n got %+v\nwant %+v", got, want)
	}
	if opt := hookSpecsFor(harness.ClaudeCode, harness.OptInReadouts); !reflect.DeepEqual(opt[len(want):], readoutHookSpecAddition) {
		t.Fatalf("the readouts opt-in rows drifted:\n got %+v\nwant %+v", opt[len(want):], readoutHookSpecAddition)
	}
	if !reflect.DeepEqual(fathomHookSpecs, want) {
		t.Fatalf("fathomHookSpecs drifted from the pre-seam table + the vision 26 §4 + wave 3 additions:\n got %+v\nwant %+v", fathomHookSpecs, want)
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
