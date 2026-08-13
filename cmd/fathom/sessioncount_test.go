package main

// sessioncount_test.go — a session is a SESSION, not a turn.
//
// Claude Code's Stop event fires at every turn boundary, not at session
// close, so a long session appends a ledger row each time the assistant
// finishes speaking. Measured on the live ledger 2026-08-09: 200 rows
// across 8 distinct sessions — one resume_as_code session wrote 74 of
// them, several with identical turn counts.
//
// Counting rows inflated every denominator ~25x ("of 188 sessions" was of
// 8), inflated the discovery total 17x by summing the same session's
// cumulative figure once per turn, and would have let the pre-registered
// "≥20 sessions per arm" criterion be satisfied by roughly ONE real
// session.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLedgerCountsSessionsNotStops(t *testing.T) {
	dir := hermeticObsEnv(t)
	// One session, three stops — the real shape. Cumulative figures grow
	// with each stop, so the LAST row is the complete one.
	rows := []string{
		`{"schema_version":2,"session_id":"s1","arm":"on","at":"2026-08-08T10:00:00Z","tokens_note":"","turn_count":10,"discovery_actions_total":100,"tokens_in":10,"tokens_out":10}`,
		`{"schema_version":2,"session_id":"s1","arm":"on","at":"2026-08-08T11:00:00Z","tokens_note":"","turn_count":40,"discovery_actions_total":400,"tokens_in":40,"tokens_out":40}`,
		`{"schema_version":2,"session_id":"s1","arm":"on","at":"2026-08-08T12:00:00Z","tokens_note":"","turn_count":90,"discovery_actions_total":900,"tokens_in":90,"tokens_out":90}`,
		// A genuinely separate session.
		`{"schema_version":2,"session_id":"s2","arm":"on","at":"2026-08-08T13:00:00Z","tokens_note":"","turn_count":12,"discovery_actions_total":50,"tokens_in":60,"tokens_out":60}`,
	}
	if err := os.WriteFile(filepath.Join(dir, "ledger.jsonl"), []byte(strings.Join(rows, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	imp := buildImpactModel(time.Date(2026, 8, 8, 14, 0, 0, 0, time.UTC))

	if imp.LedgerSessions != 2 {
		t.Fatalf("LedgerSessions = %d, want 2 — four stops, two sessions", imp.LedgerSessions)
	}
	if imp.LedgerStops != 4 {
		t.Errorf("LedgerStops = %d, want 4 — the raw count must stay visible so the correction is auditable", imp.LedgerStops)
	}
	// The decisive one: summing every row counted s1's cumulative total
	// three times (100+400+900+50 = 1450) instead of once (900+50 = 950).
	if imp.DiscoveryTotal == nil {
		t.Fatal("discovery total absent")
	}
	if *imp.DiscoveryTotal != 950 {
		t.Errorf("DiscoveryTotal = %d, want 950 — a cumulative per-session figure must be counted ONCE, "+
			"not once per turn (summing rows gives 1450)", *imp.DiscoveryTotal)
	}
	// The last row wins: it has seen the most of the transcript.
	if imp.TokensOnSessions != 2 {
		t.Errorf("TokensOnSessions = %d, want 2", imp.TokensOnSessions)
	}
	// Per-turn since the 2026-08-12 amendment: s1 final = (90+90)/90 = 2,
	// s2 = (60+60)/12 = 10; upper-middle of {2,10} = 10. The load-bearing
	// half is unchanged: s1 contributes its FINAL row, not an earlier
	// partial.
	if imp.TokensOnMedian == nil || *imp.TokensOnMedian != 10 {
		t.Errorf("on median = %v, want 10/turn", imp.TokensOnMedian)
	}
}

// TestRowsWithoutSessionIDAreNotMerged pins the edge: anonymous rows are
// distinct, never collapsed into one another.
func TestRowsWithoutSessionIDAreNotMerged(t *testing.T) {
	dir := hermeticObsEnv(t)
	rows := []string{
		`{"schema_version":2,"arm":"on","at":"2026-08-08T10:00:00Z","repo":"a","tokens_note":""}`,
		`{"schema_version":2,"arm":"on","at":"2026-08-08T11:00:00Z","repo":"b","tokens_note":""}`,
	}
	if err := os.WriteFile(filepath.Join(dir, "ledger.jsonl"), []byte(strings.Join(rows, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	imp := buildImpactModel(time.Date(2026, 8, 8, 14, 0, 0, 0, time.UTC))
	if imp.LedgerSessions != 2 {
		t.Errorf("LedgerSessions = %d, want 2 — rows with no id cannot be deduped, so each counts on its own terms", imp.LedgerSessions)
	}
}

// TestRateRefusesWhatItCannotJoin pins the rating verb's own honesty rule:
// a rating with no session to attach to is a number in a file pretending
// to be evidence.
func TestRateRefusesWhatItCannotJoin(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("FATHOM_SESSION_RATINGS", filepath.Join(dir, "ratings.jsonl"))
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	t.Setenv("CLAUDE_SESSION_ID", "")
	hermeticObsEnv(t)
	t.Setenv("FATHOM_SESSION_RATINGS", filepath.Join(dir, "ratings.jsonl"))

	var out, errb strings.Builder
	if code := cmdRate([]string{"4"}, &out, &errb); code == 0 {
		t.Fatal("a rating with no session id must be refused, not stored")
	}
	if !strings.Contains(errb.String(), "not data") {
		t.Errorf("the refusal must say why: %s", errb.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "ratings.jsonl")); err == nil {
		t.Error("a refused rating must not have written a file")
	}
}

func TestRateRecordsAndRerating(t *testing.T) {
	dir := t.TempDir()
	hermeticObsEnv(t)
	p := filepath.Join(dir, "ratings.jsonl")
	t.Setenv("FATHOM_SESSION_RATINGS", p)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "sess-abc")

	var out, errb strings.Builder
	if code := cmdRate([]string{"4", "--note", "got there, slowly"}, &out, &errb); code != 0 {
		t.Fatalf("exit: %s", errb.String())
	}
	if !strings.Contains(out.String(), "4/5") {
		t.Errorf("the verb must confirm what it recorded: %s", out.String())
	}

	t.Run("out-of-range is refused", func(t *testing.T) {
		var o, e strings.Builder
		if code := cmdRate([]string{"7"}, &o, &e); code == 0 {
			t.Error("7 is not on a 1-5 scale")
		}
		if code := cmdRate([]string{"0"}, &o, &e); code == 0 {
			t.Error("0 is not on a 1-5 scale")
		}
	})

	t.Run("re-rating corrects rather than duplicating", func(t *testing.T) {
		var o, e strings.Builder
		if code := cmdRate([]string{"2", "--note", "on reflection, no"}, &o, &e); code != 0 {
			t.Fatalf("exit: %s", e.String())
		}
		rs := loadRatings(p)
		if len(rs) != 1 {
			t.Fatalf("want one rating for one session, got %d", len(rs))
		}
		if rs["sess-abc"].Rating != 2 {
			t.Errorf("rating = %d, want 2 — the latest wins", rs["sess-abc"].Rating)
		}
		// ...and the correction is still visible in history.
		raw, _ := os.ReadFile(p)
		if n := strings.Count(strings.TrimSpace(string(raw)), "\n") + 1; n != 2 {
			t.Errorf("the log must stay append-only so the correction is auditable; %d line(s)", n)
		}
	})
}
