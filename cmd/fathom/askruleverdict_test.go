package main

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestEditHookAsksVerdictOnIgnoredRule is SR-48's anchor test (docs/plans/
// surface-reckoning.md, design §5 row SR-48, F-31): when the SAME served
// governing rule (Rule text + Src citation) with a claim id tops a file's
// obligations two turns running, the edit hook's second turn asks for a
// one-key verdict. A different rule, a resolved file (no rule now), or no
// prior memo all earn silence instead (a claimless rule is never asked
// about: TestRuleVerdictAskNeedsAClaimAndAllegesNoViolation).
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
	const claimID = "cl-ignored-1"

	now := time.Now()

	// Turn 1: no prior memo exists yet — the rule is served for the first
	// time. Nothing to ask; the memo is written for next turn.
	text, ok := withRuleVerdictAskAt(sessionID, service, file, rule, src, claimID, "obligations text", true, now, nil)
	if strings.Contains(text, ruleVerdictAskMarker) {
		t.Fatalf("turn 1 (first serving, no prior memo) asked a verdict; text = %q", text)
	}
	if !ok {
		t.Fatalf("turn 1: ok = false, want true (existing text passes through unchanged)")
	}

	// Turn 2: the SAME rule still tops the file. This turn must ask.
	text2, ok2 := withRuleVerdictAskAt(sessionID, service, file, rule, src, claimID, "obligations text", true, now.Add(time.Minute), nil)
	if !ok2 {
		t.Fatalf("turn 2: ok = false, want true")
	}
	if !strings.Contains(text2, ruleVerdictAskMarker) {
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
	text3, ok3 := withRuleVerdictAskAt(sessionID, service, file, "a different rule entirely", "internal/CLAUDE.md#L20-L22", "cl-ignored-2", "obligations text", true, now.Add(2*time.Minute), nil)
	if !ok3 || strings.Contains(text3, ruleVerdictAskMarker) {
		t.Fatalf("turn 3 (a different rule now tops the file) should not ask; text = %q, ok = %v", text3, ok3)
	}

	// Turn 4: the file is now clean (rule == "", nothing governs it any
	// more) — no ask, whatever the memo says.
	text4, ok4 := withRuleVerdictAskAt(sessionID, service, file, "", "", "", "", false, now.Add(3*time.Minute), nil)
	if strings.Contains(text4, ruleVerdictAskMarker) {
		t.Fatalf("turn 4 (file now clean) should not ask; text = %q", text4)
	}
	if ok4 {
		t.Fatalf("turn 4: ok = true, want false (no rule served, no pre-existing text, nothing to say)")
	}

	// No session id: askRuleVerdict's memo is never consulted or written
	// (the same leakage bound guardstamp.go's stamps use) — always silent
	// pass-through.
	if text, ok := withRuleVerdictAskAt("", service, file, rule, src, claimID, "obligations text", true, now, nil); text != "obligations text" || !ok {
		t.Errorf("with no session id, withRuleVerdictAskAt = (%q, %v), want the input text unchanged", text, ok)
	}
}

// TestAskRuleVerdict is askRuleVerdict's own unit test — the pure decision
// function, isolated from memo file I/O.
func TestAskRuleVerdict(t *testing.T) {
	rule, src, claimID := "no bare goroutines", "internal/CLAUDE.md#L10-L12", "cl-goroutines"

	if got := askRuleVerdict(nil, rule, src, claimID); got != "" {
		t.Errorf("no prior memo: askRuleVerdict = %q, want \"\"", got)
	}
	prior := &ruleVerdictMemo{Rule: rule, Src: src, ClaimID: claimID}
	if got := askRuleVerdict(prior, "", "", ""); got != "" {
		t.Errorf("current rule resolved (\"\"): askRuleVerdict = %q, want \"\"", got)
	}
	if got := askRuleVerdict(prior, "a different rule", src, claimID); got != "" {
		t.Errorf("different rule text: askRuleVerdict = %q, want \"\"", got)
	}
	if got := askRuleVerdict(prior, rule, "a different src", claimID); got != "" {
		t.Errorf("different src: askRuleVerdict = %q, want \"\"", got)
	}
	if got := askRuleVerdict(prior, rule, src, ""); got != "" {
		t.Errorf("no claim id: askRuleVerdict = %q, want \"\"", got)
	}
	got := askRuleVerdict(prior, rule, src, claimID)
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

// TestEditHookRecordsVerdictAnswer is SR-48w's anchor test (F-31): the ask
// names the served rule's claim id; the UserPromptSubmit hook records the
// user's exact one-key reply against the session's pending ask; the
// following edit-hook turn writes it as a refute through claim.go's
// verdict core — the claim event with the note, the status flip and the
// closed validity window, the GL-10 rule-outcome row attributed to the
// session — and clears the ask so it is written once. Prose is never a
// reply, and a key with no pending ask records nothing.
func TestEditHookRecordsVerdictAnswer(t *testing.T) {
	hermeticObsEnv(t)
	isolateHookRun(t)
	orig := ruleVerdictMemoDir
	ruleVerdictMemoDir = t.TempDir()
	t.Cleanup(func() { ruleVerdictMemoDir = orig })
	outcomeLog := filepath.Join(t.TempDir(), "rule-outcomes.jsonl")
	t.Setenv("FATHOM_RULE_OUTCOME_LOG", outcomeLog)

	const (
		sessionID = "sess-verdict"
		service   = "widgetco"
		file      = "internal/widget/widget.go"
		rule      = "never write to hardcoded workspace paths"
		src       = "internal/CLAUDE.md#L10-L12"
		claimID   = "cl-verdict-1"
	)
	storePath := filepath.Join(t.TempDir(), "s.sqlite")
	seedClaim(t, storePath, service, claimID)
	record := func(ctx context.Context, in claimVerdictInput) error {
		return recordVerdictInStore(ctx, storePath, in)
	}
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)

	// Turns 1 and 2: the same rule tops the file twice; the ask names the claim.
	if text, _ := withRuleVerdictAskAt(sessionID, service, file, rule, src, claimID, "obligations text", true, now, record); strings.Contains(text, ruleVerdictAskMarker) {
		t.Fatalf("turn 1 asked: %q", text)
	}
	ask, _ := withRuleVerdictAskAt(sessionID, service, file, rule, src, claimID, "obligations text", true, now.Add(time.Minute), record)
	if !strings.Contains(ask, ruleVerdictAskMarker) || !strings.Contains(ask, "claim "+claimID) {
		t.Fatalf("turn 2 ask does not name the claim id %s: %q", claimID, ask)
	}
	if !strings.Contains(ask, "c — corroborate") || !strings.Contains(ask, "r — refute") {
		t.Errorf("the ask does not name its two keys: %q", ask)
	}

	// Prose is never a reply, and a key with no pending ask records nothing.
	for _, prose := range []string{"run the tests", "cr", "c'mon, fix it", "C++ templates", "refute it"} {
		if _, ok := recordVerdictReply(sessionID, prose, now.Add(2*time.Minute)); ok {
			t.Errorf("prose %q was recorded as a one-key reply", prose)
		}
	}
	if _, ok := recordVerdictReply("another-session", "c", now.Add(2*time.Minute)); ok {
		t.Error("a key with no pending ask for its session was recorded")
	}

	// The user's next prompt is the one-key reply, through the real prompt hook.
	origNow := verdictReplyNow
	verdictReplyNow = func() time.Time { return now.Add(2 * time.Minute) }
	t.Cleanup(func() { verdictReplyNow = origNow })
	hookStdin(t, fmt.Sprintf(`{"session_id":%q,"cwd":%q,"hook_event_name":"UserPromptSubmit","prompt":%q}`,
		sessionID, t.TempDir(), "r: superseded by ADR-060"))
	var out, errb bytes.Buffer
	if code := run([]string{"prompt-hook", "--url=http://127.0.0.1:1"}, &out, &errb); code != 0 {
		t.Fatalf("prompt-hook exit %d: %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "one-key verdict on claim "+claimID) {
		t.Fatalf("prompt hook did not acknowledge the reply: %q", out.String())
	}
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	if evs, _ := st.ClaimEvents(context.Background(), claimID); len(evs) != 0 {
		t.Fatalf("the prompt hook wrote the verdict itself (%d events); the next edit turn writes it", len(evs))
	}
	st.Close()

	// Turn 3: the next edit-hook turn (another file) writes it as a refute.
	text, ok := withRuleVerdictAskAt(sessionID, service, "internal/widget/other.go", "", "", "", "", false, now.Add(3*time.Minute), record)
	if !ok || !strings.Contains(text, "recorded the user's one-key verdict: claim "+claimID+" refuted") {
		t.Fatalf("turn 3 did not report the recorded verdict: %q (ok=%v)", text, ok)
	}
	st, err = store.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	evs, err := st.ClaimEvents(ctx, claimID)
	if err != nil || len(evs) != 1 {
		t.Fatalf("claim events = %+v (%v), want one refute", evs, err)
	}
	if evs[0].Verdict != "refuted" || evs[0].Note != "superseded by ADR-060" || !strings.Contains(evs[0].AssertedBy, sessionID) {
		t.Errorf("event = %+v, want verdict refuted, the reply's note, asserted by the session", evs[0])
	}
	claims, _ := st.ClaimsForService(ctx, service, "")
	if len(claims) != 1 || claims[0].Status != core.ClaimRefuted || claims[0].ValidTo.IsZero() {
		t.Errorf("claim after the refute = %+v, want status refuted with its validity window closed", claims)
	}
	log := readRuleOutcomeLog(outcomeLog, now.Add(time.Hour))
	var refuted int
	for _, e := range log.Events {
		if e.RuleID == claimID && e.Event == "refuted" && e.Session == sessionID {
			refuted++
		}
	}
	if refuted != 1 {
		t.Errorf("rule-outcome log refuted rows for %s from %s = %d, want 1: %+v", claimID, sessionID, refuted, log.Events)
	}

	// Written once: the next turn records nothing more.
	if text, _ := withRuleVerdictAskAt(sessionID, service, file, "", "", "", "", false, now.Add(4*time.Minute), record); strings.Contains(text, "recorded") {
		t.Errorf("the answered ask was written twice: %q", text)
	}
	if evs, _ := st.ClaimEvents(ctx, claimID); len(evs) != 1 {
		t.Errorf("claim events after a later turn = %d, want 1", len(evs))
	}
}

// TestParseVerdictReply pins the exact-key grammar (GL-X4).
func TestParseVerdictReply(t *testing.T) {
	for _, c := range []struct {
		in, verdict, note string
		ok                bool
	}{
		{"c", "corroborated", "", true},
		{"  R  ", "refuted", "", true},
		{"c: the violation is real", "corroborated", "the violation is real", true},
		{"r: does not apply to generated code", "refuted", "does not apply to generated code", true},
		{"r : superseded by ADR-060", "refuted", "superseded by ADR-060", true},
		// A key and prose with no colon is chat shorthand, never a verdict:
		// a refute flips the claim's status and closes its validity window.
		{"r u sure the tests pass?", "", "", false},
		{"c the diff above", "", "", false},
		{"r does not apply to generated code", "", "", false},
		{"cr", "", "", false},
		{"c'mon", "", "", false},
		{"run it", "", "", false},
		{"C++", "", "", false},
		{"", "", "", false},
		{"yes", "", "", false},
	} {
		v, n, ok := parseVerdictReply(c.in)
		if v != c.verdict || n != c.note || ok != c.ok {
			t.Errorf("parseVerdictReply(%q) = (%q, %q, %v), want (%q, %q, %v)", c.in, v, n, ok, c.verdict, c.note, c.ok)
		}
	}
}
