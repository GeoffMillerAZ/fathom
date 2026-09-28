package main

import (
	"strings"
	"testing"
)

// TestFmtThousands pins BR-1's readability floor: every count in the brief
// goes through the observatory's existing thousands formatter, so a
// five-figure number arrives as a number a person can read rather than a
// digit string they have to parse.
func TestFmtThousands(t *testing.T) {
	for _, tc := range []struct {
		in   int
		want string
	}{
		{0, "0"},
		{7, "7"},
		{999, "999"},
		{1000, "1,000"},
		{11118, "11,118"},
		{17727, "17,727"},
		{60782, "60,782"},
		{1234567, "1,234,567"},
		{-4321, "-4,321"},
	} {
		if got := fmtThousands(tc.in); got != tc.want {
			t.Errorf("fmtThousands(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestBriefTuningIsGroundedInState pins BR-1's work order: every entry is
// derived from live model state and carries a command. A model with
// everything switched on yields no entries at all — the section never
// invents advice to look busy.
func TestBriefTuningIsGroundedInState(t *testing.T) {
	t.Run("gaps produce ranked actions with commands", func(t *testing.T) {
		m := observatoryModel{}
		m.Services = []obsServiceRow{
			{fleetServiceRow: fleetServiceRow{Service: "quiet", CallsTotal: 3}, Grade: "C"},
			{fleetServiceRow: fleetServiceRow{Service: "busy", CallsTotal: 900}, Grade: "C"},
			{fleetServiceRow: fleetServiceRow{Service: "stale", CallsTotal: 10}, Grade: "A", FreshnessRemedy: "cd /x && fathom ingest"},
		}
		m.Impact.LedgerSessions = 100
		m.Impact.TokensOffSessions = 0
		zero := 0
		m.Impact.VerifyCatchesTotal = &zero
		m.Impact.VerifyCatchesNote = "verify ran in 2 of 100 ledger sessions"

		acts := briefTuning(m, foldBriefFleet(m))
		if len(acts) < 4 {
			t.Fatalf("want an action per live gap, got %d: %+v", len(acts), acts)
		}
		for i, a := range acts {
			// Every entry states the gap and the payoff; it then carries
			// EITHER a command to run or a note saying why there is none.
			// A silent entry — no command, no explanation — is the failure.
			if a.What == "" || a.Unlock == "" || (a.Cmd == "" && a.Note == "") {
				t.Errorf("action %d is incomplete: %+v", i, a)
			}
		}
		// The grade gap outranks everything: it is the entry the short
		// version quotes as "the one thing most worth changing".
		if !strings.Contains(acts[0].What, "graded below A") {
			t.Errorf("first action = %q, want the grade gap ranked first", acts[0].What)
		}
		// Busiest-first ordering inside the grade gap, so the reader sees
		// the repository where it matters most.
		if strings.Index(acts[0].What, "busy") > strings.Index(acts[0].What, "quiet") {
			t.Errorf("below-A services must be listed busiest first: %q", acts[0].What)
		}
		if !strings.Contains(acts[0].Cmd, "fleet reconcile") {
			t.Errorf("the grade gap must carry its remedy command: %q", acts[0].Cmd)
		}
	})

	t.Run("a fully configured fleet yields no invented advice", func(t *testing.T) {
		m := observatoryModel{}
		m.Services = []obsServiceRow{{fleetServiceRow: fleetServiceRow{Service: "ok"}, Grade: "A"}}
		m.Impact.LedgerSessions = 10
		m.Impact.TokensOffSessions = 5
		m.Impact.ExpSessions = 4
		turns := 3
		m.Impact.FirstEditMedian = &turns
		if acts := briefTuning(m, foldBriefFleet(m)); len(acts) != 0 {
			t.Errorf("want no actions for a fully configured fleet, got %+v", acts)
		}
	})
}

// TestVerifyRanSessions pins the numerator parse: an unparseable note
// yields -1 so the caller stays silent instead of guessing.
func TestVerifyRanSessions(t *testing.T) {
	if got := verifyRanSessions("verify ran in 2 of 188 ledger sessions"); got != 2 {
		t.Errorf("got %d, want 2", got)
	}
	if got := verifyRanSessions("verify never ran in any ledger session"); got != -1 {
		t.Errorf("got %d, want -1 for a note without a numerator", got)
	}
	if got := verifyRanSessions("verify ran in many sessions"); got != -1 {
		t.Errorf("got %d, want -1 for a non-numeric numerator", got)
	}
}

// TestMdInlineEscapesBeforeEmitting pins the brief page's inline renderer:
// bold and code become tags, an unpaired delimiter stays literal, and no
// author-supplied character can ever open a tag.
func TestMdInlineEscapesBeforeEmitting(t *testing.T) {
	got := mdInline("**bold** and *emphasis* and `code`")
	for _, tag := range []string{"<strong", "<em>", "<code"} {
		if !strings.Contains(got, tag) {
			t.Errorf("missing %s in %q", tag, got)
		}
	}
	if strings.Contains(got, "*") {
		t.Errorf("no delimiter may survive into the output: %q", got)
	}
	got = mdInline("a **dangling pair")
	if strings.Contains(got, "<strong") {
		t.Errorf("an unpaired delimiter must stay literal: %q", got)
	}
	got = mdInline("<script>alert(1)</script> **x**")
	if strings.Contains(got, "<script") {
		t.Fatalf("escaping must happen before tag emission: %q", got)
	}
	if !strings.Contains(got, "&lt;script&gt;") {
		t.Errorf("the raw text must survive escaped: %q", got)
	}
}
