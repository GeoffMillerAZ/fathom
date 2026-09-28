package metrics

import "testing"

// TestJoinVerifyAndSessionLedgers proves SR-68 (docs/design/surface-
// reckoning.md §6): JoinLedgersBySession groups verify-ledger runs and
// session-ledger rows by their shared session id, keeps a session with no
// verify runs, and keeps a verify run with no session id separately named
// rather than dropping or misattributing it.
func TestJoinVerifyAndSessionLedgers(t *testing.T) {
	verifyRuns := []VerifyLedgerRow{
		{Session: "s1", Plan: "surface-reckoning.md", Commit: "abc123", At: "2026-09-27T10:05:00Z", Claimed: 5, Verified: 4, Failing: 1},
		{Session: "s1", Plan: "surface-reckoning.md", Commit: "def456", At: "2026-09-27T10:10:00Z", Claimed: 5, Verified: 5, Failing: 0},
		{Session: "", Plan: "surface-reckoning.md", Commit: "ghi789", At: "2026-09-27T11:00:00Z", Claimed: 5, Verified: 5, Failing: 0}, // the pre-push gate: no live session
	}
	sessions := []SessionLedgerRow{
		{Session: "s1", Repo: "fathom", At: "2026-09-27T10:11:00Z"},
		{Session: "s2", Repo: "fathom", At: "2026-09-27T12:00:00Z"}, // ran, but never called verify
	}

	joined, unjoined := JoinLedgersBySession(verifyRuns, sessions)

	if len(unjoined) != 1 || unjoined[0].Commit != "ghi789" {
		t.Fatalf("unjoined = %+v, want exactly the session-less gate run (commit ghi789)", unjoined)
	}
	if len(joined) != 2 {
		t.Fatalf("joined = %+v, want 2 sessions (s1, s2)", joined)
	}
	if joined[0].Session != "s1" || joined[1].Session != "s2" {
		t.Fatalf("joined sessions = [%q, %q], want sorted [s1, s2]", joined[0].Session, joined[1].Session)
	}

	s1 := joined[0]
	if s1.Ledger == nil || s1.Ledger.Repo != "fathom" {
		t.Errorf("s1.Ledger = %+v, want the fathom session-ledger row", s1.Ledger)
	}
	if len(s1.VerifyRuns) != 2 || s1.VerifyRuns[0].Commit != "abc123" || s1.VerifyRuns[1].Commit != "def456" {
		t.Errorf("s1.VerifyRuns = %+v, want both runs in input order", s1.VerifyRuns)
	}

	s2 := joined[1]
	if s2.Ledger == nil {
		t.Errorf("s2.Ledger = nil, want the session-ledger row for s2")
	}
	if len(s2.VerifyRuns) != 0 {
		t.Errorf("s2.VerifyRuns = %+v, want none — s2 never called verify, and that is a real fact, not merged with s1's runs", s2.VerifyRuns)
	}
}

// TestJoinVerifyAndSessionLedgers_EmptyInputs: nothing in yields nothing
// joined and nothing unjoined — never a fabricated row.
func TestJoinVerifyAndSessionLedgers_EmptyInputs(t *testing.T) {
	joined, unjoined := JoinLedgersBySession(nil, nil)
	if len(joined) != 0 || len(unjoined) != 0 {
		t.Fatalf("JoinLedgersBySession(nil, nil) = (%+v, %+v), want (empty, empty)", joined, unjoined)
	}
}
