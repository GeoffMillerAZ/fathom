package metrics

import (
	"testing"
	"time"

	heartbeat "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/heartbeat"
)

// TestActivityRecordJoinsCallAndHookLogs proves SR-67 (docs/design/
// surface-reckoning.md §6): ReadActivityRecord joins already-decoded call-
// log rows and hooks-beat-log rows into one ActivityRecord per session,
// with per-tool and per-channel/kind counts and a deterministic order.
func TestActivityRecordJoinsCallAndHookLogs(t *testing.T) {
	calls := []ActivityCallEntry{
		{TS: "2026-09-27T10:00:00Z", Tool: "find_symbol", Session: "s1"},
		{TS: "2026-09-27T10:01:00Z", Tool: "find_symbol", Session: "s1"},
		{TS: "2026-09-27T10:02:00Z", Tool: "search", Session: "s2"},
		{TS: "2026-09-27T09:59:00Z", Tool: "ingest", Session: ""}, // no session (a gate step)
	}
	beats := []heartbeat.Beat{
		{Channel: "edit-hook", Kind: "answered", At: mustParseRFC3339(t, "2026-09-27T10:00:30Z"), Meta: map[string]string{"session": "s1"}},
		{Channel: "edit-hook", Kind: "answered", At: mustParseRFC3339(t, "2026-09-27T10:03:00Z"), Meta: map[string]string{"session": "s2"}},
		{Channel: "edit-hook", Kind: "denied", At: mustParseRFC3339(t, "2026-09-27T10:04:00Z"), Meta: map[string]string{"session": "s2"}},
		{Channel: "session-hook", Kind: "seen", At: mustParseRFC3339(t, "2026-09-27T09:58:00Z")}, // no session meta at all
	}

	got := ReadActivityRecord(calls, beats)

	if len(got) != 3 {
		t.Fatalf("len(got) = %d, want 3 (s1, s2, \"\"); got %+v", len(got), got)
	}

	// s1 and s2 sort before the empty (no-session) group, which is last.
	if got[len(got)-1].Session != "" {
		t.Errorf("the no-session group must sort last, got order %v", sessionsOf(got))
	}

	byID := map[string]ActivityRecord{}
	for _, r := range got {
		byID[r.Session] = r
	}

	s1 := byID["s1"]
	if s1.CallsTotal != 2 || s1.Calls["find_symbol"] != 2 {
		t.Errorf("s1 calls = %+v, want 2 find_symbol", s1.Calls)
	}
	if s1.HookEventsTotal != 1 || s1.HookEvents["edit-hook/answered"] != 1 {
		t.Errorf("s1 hook events = %+v, want 1 edit-hook/answered", s1.HookEvents)
	}
	if s1.First != "2026-09-27T10:00:00Z" {
		t.Errorf("s1.First = %q, want the earliest of its call/beat timestamps", s1.First)
	}

	s2 := byID["s2"]
	if s2.CallsTotal != 1 || s2.Calls["search"] != 1 {
		t.Errorf("s2 calls = %+v, want 1 search", s2.Calls)
	}
	if s2.HookEventsTotal != 2 || s2.HookEvents["edit-hook/answered"] != 1 || s2.HookEvents["edit-hook/denied"] != 1 {
		t.Errorf("s2 hook events = %+v, want 1 answered + 1 denied", s2.HookEvents)
	}

	none := byID[""]
	if none.CallsTotal != 1 || none.HookEventsTotal != 1 {
		t.Errorf("no-session group = %+v, want 1 call + 1 beat, neither merged into a real session", none)
	}
}

// TestActivityRecordEmptyInputsYieldNoRecords: no calls and no beats joins
// to no sessions at all — never a fabricated empty-session row.
func TestActivityRecordEmptyInputsYieldNoRecords(t *testing.T) {
	got := ReadActivityRecord(nil, nil)
	if len(got) != 0 {
		t.Fatalf("ReadActivityRecord(nil, nil) = %+v, want empty", got)
	}
}

func sessionsOf(records []ActivityRecord) []string {
	out := make([]string, len(records))
	for i, r := range records {
		out[i] = r.Session
	}
	return out
}

func mustParseRFC3339(t *testing.T, s string) time.Time {
	t.Helper()
	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return tm
}
