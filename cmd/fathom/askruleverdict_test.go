package main

import (
	"strings"
	"testing"
	"time"
)

// TestEditHookAsksVerdictOnIgnoredRule is SR-48's anchor test (docs/plans/
// surface-reckoning.md, design §5 row SR-48, F-31): when the SAME served
// governing rule (Rule text + Src citation) tops a file's obligations two
// turns running, the edit hook's second turn asks for a one-key verdict —
// the edit that followed serving it the first time did not address it. A
// different rule, a resolved file (no rule now), or no prior memo all earn
// silence instead.
func TestEditHookAsksVerdictOnIgnoredRule(t *testing.T) {
	dir := t.TempDir()
	orig := ruleVerdictMemoDir
	ruleVerdictMemoDir = dir
	defer func() { ruleVerdictMemoDir = orig }()

	const sessionID = "sess-1"
	const service = "widgetco"
	const file = "internal/widget/widget.go"
	const rule = "never write to hardcoded workspace paths"
	const src = "internal/CLAUDE.md#L10-L12"

	now := time.Now()

	// Turn 1: no prior memo exists yet — the rule is served for the first
	// time. Nothing to ask; the memo is written for next turn.
	text, ok := withRuleVerdictAskAt(sessionID, service, file, rule, src, "obligations text", true, now)
	if strings.Contains(text, "served rule ignored") {
		t.Fatalf("turn 1 (first serving, no prior memo) asked a verdict; text = %q", text)
	}
	if !ok {
		t.Fatalf("turn 1: ok = false, want true (existing text passes through unchanged)")
	}

	// Turn 2: the SAME rule still tops the file — the edit ignored it.
	// This turn must ask.
	text2, ok2 := withRuleVerdictAskAt(sessionID, service, file, rule, src, "obligations text", true, now.Add(time.Minute))
	if !ok2 {
		t.Fatalf("turn 2: ok = false, want true")
	}
	if !strings.Contains(text2, "served rule ignored") {
		t.Fatalf("turn 2 (same rule served twice running) did not ask a verdict; text = %q", text2)
	}
	if !strings.Contains(text2, rule) || !strings.Contains(text2, src) {
		t.Errorf("verdict ask = %q, want it to cite the rule %q and its src %q", text2, rule, src)
	}
	if !strings.Contains(text2, "obligations text") {
		t.Errorf("verdict ask dropped the existing payload text: %q", text2)
	}

	// Turn 3: a DIFFERENT rule now tops the file (the first was addressed;
	// this is the first time the second one is seen) — no ask.
	text3, ok3 := withRuleVerdictAskAt(sessionID, service, file, "a different rule entirely", "internal/CLAUDE.md#L20-L22", "obligations text", true, now.Add(2*time.Minute))
	if !ok3 || strings.Contains(text3, "served rule ignored") {
		t.Fatalf("turn 3 (a different rule now tops the file) should not ask; text = %q, ok = %v", text3, ok3)
	}

	// Turn 4: the file is now clean (rule == "", nothing governs it any
	// more) — no ask, whatever the memo says.
	text4, ok4 := withRuleVerdictAskAt(sessionID, service, file, "", "", "", false, now.Add(3*time.Minute))
	if strings.Contains(text4, "served rule ignored") {
		t.Fatalf("turn 4 (file now clean) should not ask; text = %q", text4)
	}
	if ok4 {
		t.Fatalf("turn 4: ok = true, want false (no rule served, no pre-existing text, nothing to say)")
	}

	// No session id: askRuleVerdict's memo is never consulted or written
	// (the same leakage bound guardstamp.go's stamps use) — always silent
	// pass-through.
	if text, ok := withRuleVerdictAskAt("", service, file, rule, src, "obligations text", true, now); text != "obligations text" || !ok {
		t.Errorf("with no session id, withRuleVerdictAskAt = (%q, %v), want the input text unchanged", text, ok)
	}
}

// TestAskRuleVerdict is askRuleVerdict's own unit test — the pure decision
// function, isolated from memo file I/O.
func TestAskRuleVerdict(t *testing.T) {
	rule, src := "no bare goroutines", "internal/CLAUDE.md#L10-L12"

	if got := askRuleVerdict(nil, rule, src); got != "" {
		t.Errorf("no prior memo: askRuleVerdict = %q, want \"\"", got)
	}
	prior := &ruleVerdictMemo{Rule: rule, Src: src}
	if got := askRuleVerdict(prior, "", ""); got != "" {
		t.Errorf("current rule resolved (\"\"): askRuleVerdict = %q, want \"\"", got)
	}
	if got := askRuleVerdict(prior, "a different rule", src); got != "" {
		t.Errorf("different rule text: askRuleVerdict = %q, want \"\"", got)
	}
	if got := askRuleVerdict(prior, rule, "a different src"); got != "" {
		t.Errorf("different src: askRuleVerdict = %q, want \"\"", got)
	}
	got := askRuleVerdict(prior, rule, src)
	if got == "" {
		t.Fatal("same rule+src as prior: askRuleVerdict = \"\", want a verdict ask")
	}
	if !strings.Contains(got, "corroborate") || !strings.Contains(got, "refute") {
		t.Errorf("askRuleVerdict = %q, want both a corroborate and a refute path named", got)
	}
	if !strings.Contains(got, "fathom claim") {
		t.Errorf("askRuleVerdict = %q, want the fathom claim command named", got)
	}
}
