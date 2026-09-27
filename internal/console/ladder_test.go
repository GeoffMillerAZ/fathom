package console

import (
	"strings"
	"testing"
)

func wiredFacts() LadderFacts {
	return LadderFacts{
		Service: "svc", StanzaNote: "present", HooksNote: "3 fathom hook reference(s) in project settings",
		Freshness: "fresh", FreshRemedy: "fathom post-commit-hook --dir /r --service svc --wait --timeout 2m",
		WindowDays: 14, HasPlans: true,
	}
}

// TestRealizationLadder_RemedyOnFirstUnmetRung pins OC-25 (RG-5): the
// ladder is wired → fresh → queried → preventing → gating, every rung
// carries its evidence, and only the FIRST unmet rung carries its remedy;
// a rung no instrument judged is unknown (never unmet), a rung that does
// not apply is not applicable, and neither counts against the repository;
// an idle repository is told its first unmet rung, never graded F.
func TestRealizationLadder_RemedyOnFirstUnmetRung(t *testing.T) {
	f := wiredFacts()
	f.Calls, f.Sessions, f.ObligationServes, f.VerifyRuns = 120, 4, 0, 0
	l := RealizationLadder(f)
	names := []string{}
	for _, r := range l.Rungs {
		names = append(names, r.Name)
		if r.Evidence == "" {
			t.Errorf("rung %s has no evidence", r.Name)
		}
	}
	if strings.Join(names, ",") != "wired,fresh,queried,preventing,gating" {
		t.Fatalf("ladder order = %v", names)
	}
	if l.FirstUnmet != "preventing" || l.Rungs[3].Remedy == "" || l.Remedy != l.Rungs[3].Remedy {
		t.Fatalf("first unmet = %q, remedy %q", l.FirstUnmet, l.Remedy)
	}
	for i, r := range l.Rungs {
		if i != 3 && r.Remedy != "" {
			t.Errorf("rung %s carries a remedy; only the first unmet rung may", r.Name)
		}
	}
	if l.Rungs[4].State != RungUnmet || l.Rungs[4].Fix() == "" {
		t.Errorf("gating is unmet with its own fix kept for the grade: %+v", l.Rungs[4])
	}
	if l.Grade != "C — not preventing" || l.Realized != 3 || l.Available != 5 {
		t.Errorf("grade = %q (%d/%d)", l.Grade, l.Realized, l.Available)
	}
	if !strings.Contains(l.Rungs[2].Evidence, "120 call(s) in the last 14 days") {
		t.Errorf("queried evidence = %q", l.Rungs[2].Evidence)
	}

	// The call log was not read: queried and preventing are unknown, the
	// first unmet rung moves to gating, and the denominator shrinks.
	f.CallLogNote = "no call log at /x"
	l = RealizationLadder(f)
	if l.Rungs[2].State != RungUnknown || l.Rungs[3].State != RungUnknown || !strings.Contains(l.Rungs[2].Evidence, "not measured") {
		t.Errorf("an unread call log must make queried/preventing unknown: %+v", l.Rungs[2:4])
	}
	if l.FirstUnmet != "gating" || l.Available != 3 {
		t.Errorf("first unmet = %q, available %d", l.FirstUnmet, l.Available)
	}

	// No plans: gating does not apply and carries no remedy.
	f = wiredFacts()
	f.Calls, f.Sessions, f.ObligationServes, f.HasPlans = 5, 2, 1, false
	l = RealizationLadder(f)
	if l.Rungs[4].State != RungNotApplicable || l.Rungs[4].Fix() != "" || l.Grade != "A" || l.FirstUnmet != "" {
		t.Errorf("no plans = %+v grade %q", l.Rungs[4], l.Grade)
	}
	// An unreadable docs/plans is not "no plans": gating is unknown.
	f.PlansNote = "docs/plans was not read (operation not permitted)"
	if l := RealizationLadder(f); l.Rungs[4].State != RungUnknown || !strings.Contains(l.Rungs[4].Evidence, "not permitted") {
		t.Errorf("unreadable plans = %+v", l.Rungs[4])
	}

	// Unknown freshness is not a defect.
	f = wiredFacts()
	f.Freshness, f.Calls, f.Sessions, f.ObligationServes, f.VerifyRuns = "unknown: service not in statusz", 1, 1, 1, 1
	if l := RealizationLadder(f); l.Rungs[1].State != RungUnknown || l.Grade != "A" {
		t.Errorf("unknown freshness = %+v grade %q", l.Rungs[1], l.Grade)
	}

	// Idle: no sessions — graded idle, the first unmet rung still named.
	f = wiredFacts()
	l = RealizationLadder(f)
	if l.Grade != "idle" || !l.Idle || l.FirstUnmet != "queried" || l.Rungs[2].Remedy == "" {
		t.Errorf("idle = %+v", l)
	}

	// Not wired: the bottom rung carries the remedy, nothing above it does.
	f = wiredFacts()
	f.StanzaNote, f.Calls, f.Sessions = "absent", 0, 3
	l = RealizationLadder(f)
	if l.FirstUnmet != "wired" || l.Rungs[0].Remedy != "fathom fleet reconcile --apply" || l.Rungs[2].Remedy != "" {
		t.Errorf("not wired = %+v", l.Rungs)
	}
}

// TestRealizationLadder_GatingReadsTheVerifyLedger pins the gating rung to
// its signal (docs/design/realization-grade.md §2: "verify runs in the
// window"): the runs the verify ledger records (SD-1 — the recorded-run
// signal that closed O30, where a pre-push gate's runs never reached the
// session ledger), named as such. A verify ledger that could not be read
// makes gating unknown, never unmet — and a session ledger that could not
// be read never grades a repository idle.
func TestRealizationLadder_GatingReadsTheVerifyLedger(t *testing.T) {
	f := wiredFacts()
	f.Calls, f.Sessions, f.ObligationServes, f.VerifyRuns = 10, 2, 3, 4
	l := RealizationLadder(f)
	g := l.Rungs[4]
	if g.State != RungMet || !strings.Contains(g.Evidence, "4 verify run(s) in the last 14 days") ||
		!strings.Contains(g.Evidence, "verify ledger") || strings.Contains(g.Evidence, "session") {
		t.Errorf("gating = %+v", g)
	}
	if l.Grade != "A" || l.FirstUnmet != "" {
		t.Errorf("a gating repository = %q, first unmet %q", l.Grade, l.FirstUnmet)
	}

	f.VerifyRuns = 0
	l = RealizationLadder(f)
	if l.Rungs[4].State != RungUnmet || l.FirstUnmet != "gating" || !strings.Contains(l.Remedy, "pre-push") {
		t.Errorf("no runs in the window = %+v, remedy %q", l.Rungs[4], l.Remedy)
	}

	f.VerifyLedgerNote = "no verify ledger at /h/.fathom/verify-ledger.jsonl"
	l = RealizationLadder(f)
	if l.Rungs[4].State != RungUnknown || !strings.Contains(l.Rungs[4].Evidence, "not measured — no verify ledger") ||
		l.FirstUnmet != "" || l.Grade != "A" || l.Available != 4 {
		t.Errorf("an unread verify ledger = %+v grade %q (%d/%d)", l.Rungs[4], l.Grade, l.Realized, l.Available)
	}

	// Sessions not measured: the rungs are graded, never "idle".
	f = wiredFacts()
	f.Calls, f.ObligationServes, f.VerifyRuns = 1, 1, 1
	f.SessionsNote = "the session ledger could not be read (permission denied)"
	l = RealizationLadder(f)
	if l.Idle || l.Grade != "A" || !strings.Contains(l.SessionsNote, "permission denied") {
		t.Errorf("unread sessions = idle %v grade %q note %q", l.Idle, l.Grade, l.SessionsNote)
	}
}
