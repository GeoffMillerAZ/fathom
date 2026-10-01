package main

// ruleoutcome_track_test.go — SR-61's anchor test (docs/plans/
// surface-reckoning.md, design §6 row SR-61, F-27/U4): the edit hook
// records served/still_served/top_changed per claim id across turns, mirroring
// TestEditHookAsksVerdictOnIgnoredRule's own direct-call shape
// (askruleverdict_test.go) — trackRuleOutcomeAt is the exact SR-48-style
// turn-over-turn comparison, generalized from rule text to a claim id.

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

func TestEditHookRecordsRuleOutcomes(t *testing.T) {
	memoDir := t.TempDir()
	origMemoDir := ruleOutcomeMemoDir
	ruleOutcomeMemoDir = memoDir
	t.Cleanup(func() { ruleOutcomeMemoDir = origMemoDir })

	logPath := filepath.Join(t.TempDir(), "rule-outcomes.jsonl")
	t.Setenv("FATHOM_RULE_OUTCOME_LOG", logPath)

	const sessionID = "sess-1"
	const service = "widgetco"
	const file = "internal/widget/widget.go"
	const claimA = "claim-a"
	const claimB = "claim-b"

	now := time.Now()

	// Turn 1: claimA tops the file for the first time — served only, no
	// prior memo to compare against.
	trackRuleOutcomeAt(sessionID, service, file, claimA, now)
	rows := readRuleOutcomeRows(t, logPath)
	if len(rows) != 1 || rows[0].RuleID != claimA || rows[0].Event != ruleOutcomeServed {
		t.Fatalf("turn 1: rows = %+v, want exactly one served row for %s", rows, claimA)
	}

	// Turn 2: claimA STILL tops the file — the edit that followed serving
	// it did not address it. served AND still_served.
	trackRuleOutcomeAt(sessionID, service, file, claimA, now.Add(time.Minute))
	rows = readRuleOutcomeRows(t, logPath)
	if len(rows) != 3 {
		t.Fatalf("turn 2: want 3 total rows (1 from turn 1, served+still_served from turn 2), got %d: %+v", len(rows), rows)
	}
	if rows[1].RuleID != claimA || rows[1].Event != ruleOutcomeServed {
		t.Errorf("turn 2 row 2 = %+v, want rule_id=%s event=served", rows[1], claimA)
	}
	if rows[2].RuleID != claimA || rows[2].Event != ruleOutcomeStillServed {
		t.Errorf("turn 2 row 3 = %+v, want rule_id=%s event=still_served", rows[2], claimA)
	}

	// Turn 3: claimB now tops the file instead — claimA was addressed
	// (top_changed), claimB is served for the first time (no still_served: it
	// never topped the file before).
	trackRuleOutcomeAt(sessionID, service, file, claimB, now.Add(2*time.Minute))
	rows = readRuleOutcomeRows(t, logPath)
	if len(rows) != 5 {
		t.Fatalf("turn 3: want 5 total rows, got %d: %+v", len(rows), rows)
	}
	gotEvents := map[string]bool{}
	for _, r := range rows[3:] {
		gotEvents[r.RuleID+":"+r.Event] = true
	}
	if !gotEvents[claimB+":"+ruleOutcomeServed] {
		t.Errorf("turn 3: expected %s served, rows = %+v", claimB, rows[3:])
	}
	if !gotEvents[claimA+":"+ruleOutcomeTopChanged] {
		t.Errorf("turn 3: expected %s top_changed, rows = %+v", claimA, rows[3:])
	}
	if gotEvents[claimB+":"+ruleOutcomeStillServed] {
		t.Errorf("turn 3: %s must not be recorded still_served on its first serving, rows = %+v", claimB, rows[3:])
	}

	// Turn 4: the file is now clean (claimID == "") — claimB was
	// top_changed (no rule tops it now); nothing served.
	trackRuleOutcomeAt(sessionID, service, file, "", now.Add(3*time.Minute))
	rows = readRuleOutcomeRows(t, logPath)
	if len(rows) != 6 {
		t.Fatalf("turn 4: want 6 total rows, got %d: %+v", len(rows), rows)
	}
	if rows[5].RuleID != claimB || rows[5].Event != ruleOutcomeTopChanged {
		t.Errorf("turn 4 row = %+v, want rule_id=%s event=top_changed", rows[5], claimB)
	}

	// No session id: the memo is never consulted or written (the same
	// leakage bound guardstamp.go's stamps use) — served still records
	// (recordRuleOutcome itself needs no session), but never still_served or
	// top_changed, since there is no memo to compare against.
	trackRuleOutcomeAt("", service, file, claimA, now.Add(4*time.Minute))
	rows = readRuleOutcomeRows(t, logPath)
	if len(rows) != 7 || rows[6].Event != ruleOutcomeServed || rows[6].Session != "" {
		t.Errorf("no-session turn: rows = %+v, want exactly one more served row with no session", rows)
	}
}

// TestTrackRuleOutcomeAtMemoRoundTrip is the memo's own unit test —
// isolated from the signal log, mirrors readGuardStamp/readRuleVerdictMemo
// coverage for their own memo types.
func TestTrackRuleOutcomeAtMemoRoundTrip(t *testing.T) {
	dir := t.TempDir()
	orig := ruleOutcomeMemoDir
	ruleOutcomeMemoDir = dir
	t.Cleanup(func() { ruleOutcomeMemoDir = orig })

	const sessionID = "sess-2"
	const service = "widgetco"
	const file = "a.go"
	now := time.Now()

	if _, found := readRuleOutcomeMemo(sessionID, service, file, now); found {
		t.Fatalf("no memo written yet: found = true, want false")
	}
	writeRuleOutcomeMemo(sessionID, service, file, "claim-x", now)
	m, found := readRuleOutcomeMemo(sessionID, service, file, now)
	if !found || m.ClaimID != "claim-x" {
		t.Fatalf("readRuleOutcomeMemo = (%+v, %v), want claim-x, true", m, found)
	}
	// Expired past ruleOutcomeMemoTTL: read as absent.
	if _, found := readRuleOutcomeMemo(sessionID, service, file, now.Add(ruleOutcomeMemoTTL+time.Minute)); found {
		t.Errorf("expired memo still read as found")
	}
	// No session id: never read.
	if _, found := readRuleOutcomeMemo("", service, file, now); found {
		t.Errorf("session-less read found a memo, want always false")
	}
}

// TestEditHookFetchDeclaresTheClaimIDOrigin pins the other half of SR-61's
// payload cut-back: the daemon serves a rule's claim id only to origin
// "edit-hook" (internal/mcp claimIDsForCaller), so the edit hook's own
// obligations request must declare exactly that origin — a rename on
// either side would silently stop the served/still_served/top_changed signal.
func TestEditHookFetchDeclaresTheClaimIDOrigin(t *testing.T) {
	var gotOrigin string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotOrigin = r.Header.Get(originHeaderName)
		fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{\"service\":\"svc\",\"file\":\"a.go\",\"rules\":[{\"rule\":\"r\",\"src\":\"d.md#L1-L1\",\"claim_id\":\"abc\",\"tier\":\"doc-claimed\"}]}"}]}}`)
	}))
	defer srv.Close()

	orig := rpcOrigin
	t.Cleanup(func() { rpcOrigin = orig })
	rpcOrigin = ""
	// cmdEditHook stamps rpcOrigin before it reads a single byte of stdin;
	// a dead URL keeps this off the network.
	var out, errb bytes.Buffer
	cmdEditHook([]string{"--url", "http://127.0.0.1:1"}, &out, &errb)
	if rpcOrigin != mcp.OriginEditHook {
		t.Fatalf("cmdEditHook declares origin %q, want %q — the daemon serves claim ids to that origin only", rpcOrigin, mcp.OriginEditHook)
	}

	o, _, _, ok, err := fetchObligations(srv.URL, "svc", "a.go", 0, 2*time.Second, semantic.ObligationLayerAll)
	if !ok || err != nil {
		t.Fatalf("fetchObligations: ok=%v err=%v", ok, err)
	}
	if gotOrigin != mcp.OriginEditHook {
		t.Errorf("request origin header = %q, want %q", gotOrigin, mcp.OriginEditHook)
	}
	if len(o.Rules) != 1 || o.Rules[0].ClaimID != "abc" {
		t.Errorf("hook decoded rules = %+v, want the claim id kept", o.Rules)
	}
}

// TestRuleOutcomeRepeatEditEmitsStillServedNeverViolated is JF-1's bar
// (docs/plans/judgment-foundations.md, docs/design/judgment-foundations.md
// section 5): a repeat edit to the same file under a prose rule emits
// still_served and never violated, and a changed top claim emits
// top_changed. The tracker evaluates no check, so a repeat edit says only
// that the same rule still tops the file's served set — it is not evidence
// that the edit ignored the rule. A checkable rule denies and records
// blocked (PR-10), which is the one place a violation is measured.
func TestRuleOutcomeRepeatEditEmitsStillServedNeverViolated(t *testing.T) {
	origMemoDir := ruleOutcomeMemoDir
	ruleOutcomeMemoDir = t.TempDir()
	t.Cleanup(func() { ruleOutcomeMemoDir = origMemoDir })

	logPath := filepath.Join(t.TempDir(), "rule-outcomes.jsonl")
	t.Setenv("FATHOM_RULE_OUTCOME_LOG", logPath)

	const (
		session = "sess-jf1"
		service = "widgetco"
		file    = "internal/widget/widget.go"
		prose   = "claim-prose" // a prose rule: no checkable condition behind the claim id
		other   = "claim-other"
	)
	now := time.Now()

	// Turn 1: the prose rule tops the file for the first time.
	trackRuleOutcomeAt(session, service, file, prose, now)
	// Turn 2: a repeat edit to the same file; the same rule tops it again.
	trackRuleOutcomeAt(session, service, file, prose, now.Add(time.Minute))
	// Turn 3: a different claim tops the file: the prose rule's top changed.
	trackRuleOutcomeAt(session, service, file, other, now.Add(2*time.Minute))

	var got []string
	for _, r := range readRuleOutcomeRows(t, logPath) {
		got = append(got, r.RuleID+":"+r.Event)
		if r.Event == "violated" || r.Event == "complied" {
			t.Errorf("row %+v: the tracker must never emit the retired name %q (nothing writes violated, ADR-057; blocked is the checkable rule's event)", r, r.Event)
		}
	}
	want := []string{
		prose + ":served",                          // turn 1
		prose + ":served", prose + ":still_served", // turn 2: the repeat edit
		other + ":served", prose + ":top_changed", // turn 3: the top claim changed
	}
	if len(got) != len(want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("rows = %v, want %v", got, want)
		}
	}
	if ruleOutcomeStillServed != "still_served" || ruleOutcomeTopChanged != "top_changed" {
		t.Errorf("constants = %q, %q, want still_served, top_changed", ruleOutcomeStillServed, ruleOutcomeTopChanged)
	}
}
