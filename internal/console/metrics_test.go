package console_test

import (
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
)

func intp(n int) *int { return &n }

// An observations file that exists and cannot be read (chmod 000, the
// external-volume EPERM wedge) is an ERROR naming the repo and the failure
// — never "not reporting", and never folded into a smaller lower bound that
// reads as fewer defects (ADR-045's truth states, invariant I38).
func TestOpenObservations_UnreadableFileIsError(t *testing.T) {
	f := console.FleetFacts{Services: []console.ServiceFact{
		{Name: "alpha", OpenObservations: intp(3)},
		{Name: "beta", ObservationsErr: "open /r/beta/.fathom/observations.md: permission denied"},
		{Name: "gamma"},
	}}
	v := console.OpenObservations(f)
	if v.State != console.StateError {
		t.Fatalf("state = %s (%+v), want error", v.State, v)
	}
	for _, want := range []string{"beta", "permission denied", "1 of 3"} {
		if !strings.Contains(v.Reason, want) {
			t.Errorf("reason %q must name %q", v.Reason, want)
		}
	}
	if err := v.Validate(); err != nil {
		t.Error(err)
	}
	// Without the read failure the same fleet is the named lower bound.
	f.Services[1].ObservationsErr = ""
	if v := console.OpenObservations(f); v.State != console.StatePartial || v.V != 3 {
		t.Errorf("no read failure = %+v, want partial 3", v)
	}
}

// Per-repo sessions share the one dedupe rule: distinct session ids per
// lowered repo label, never ledger rows.
func TestDistinctSessionsByRepo(t *testing.T) {
	data := []byte(`{"session_id":"s1","repo":"alpha"}
{"session_id":"s1","repo":"alpha"}
{"session_id":"s2","repo":"Alpha"}
{"session_id":"s3","repo":"beta"}
{"repo":"beta","at":"2026-09-25T10:00:00Z"}
not json
{"session_id":"s4"}
`)
	got := console.DistinctSessionsByRepo(data)
	if got["alpha"] != 2 || got["beta"] != 2 || len(got) != 2 {
		t.Errorf("by repo = %v, want alpha 2, beta 2", got)
	}
	if sessions, rows := console.DistinctSessions(data); sessions != 5 || rows != 6 {
		t.Errorf("DistinctSessions = %d sessions from %d rows, want 5 from 6", sessions, rows)
	}
}

// TestAgentSurfaceTools_AbsentNeverZero: a surface that did not answer
// tools/list is not measured — never a 0-tool surface — and a measured
// count names its schema bytes.
func TestAgentSurfaceTools_AbsentNeverZero(t *testing.T) {
	if v := console.AgentSurfaceTools(0, 0); v.State != console.StateAbsent || v.Reason == "" {
		t.Errorf("unanswered surface = %+v, want absent with a reason", v)
	}
	if v := console.AgentSurfaceTools(25, 18117); v.State != console.StatePresent || v.V != 25 || !strings.Contains(v.Denominator, "18,117 bytes") {
		t.Errorf("measured surface = %+v", v)
	}
}

// TestCallTotal_NamesConsoleReadsExcluded: the console's own reads are
// left out of the total and named in its denominator.
func TestCallTotal_NamesConsoleReadsExcluded(t *testing.T) {
	log := console.ResolvedCallLog{Path: "/l", Rule: console.RuleDaemon, Note: "n"}
	v := console.CallTotal(console.CallLogFacts{Log: log, Rows: 5, Unattributed: 2, ConsoleReads: 11})
	if v.V != 5 || !strings.Contains(v.Denominator, "11 console page reads excluded (not usage)") {
		t.Errorf("call total = %+v", v)
	}
	if v := console.CallTotal(console.CallLogFacts{Log: log, Rows: 5}); strings.Contains(v.Denominator, "console") {
		t.Errorf("no console reads, no mention: %+v", v)
	}
}

// The realization ladder's sessions are the window's (design §3: "no
// sessions in the window" grades idle): a session counts when any of its
// rows falls inside [from, to); a row with no timestamp cannot be placed
// and is not counted.
func TestDistinctSessionsIn_CountsTheWindowOnly(t *testing.T) {
	data := []byte(`{"session_id":"s1","repo":"alpha","at":"2026-09-01T10:00:00Z"}
{"session_id":"s2","repo":"alpha","at":"2026-09-20T10:00:00Z"}
{"session_id":"s2","repo":"alpha","at":"2026-09-21T10:00:00Z"}
{"session_id":"s3","repo":"Beta","at":"2026-08-01T10:00:00Z"}
{"session_id":"s3","repo":"beta","at":"2026-09-24T10:00:00Z"}
{"session_id":"s4","repo":"beta"}
{"session_id":"s5","repo":"gamma","at":"2026-09-26T00:00:00Z"}
`)
	from := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
	got := console.DistinctSessionsIn(data, strings.ToLower, from, to)
	if got["alpha"] != 1 || got["beta"] != 1 || got["gamma"] != 0 || len(got) != 2 {
		t.Errorf("sessions in the window = %v, want alpha 1, beta 1", got)
	}
}

// The gating rung counts verify RUNS in the window — one invocation writes
// a row per plan, all stamped with the same instant — per service.
func TestVerifyRunsIn_CountsRunsNotPlanRows(t *testing.T) {
	at := func(d, h int) time.Time { return time.Date(2026, 9, d, h, 0, 0, 0, time.UTC) }
	runs := []console.LedgerRun{
		{At: at(20, 9), Service: "conjure", Plan: "docs/plans/a.md"},
		{At: at(20, 9), Service: "conjure", Plan: "docs/plans/b.md"},
		{At: at(20, 9), Service: "conjure", Plan: "docs/plans/c.md"},
		{At: at(22, 9), Service: "Conjure", Plan: "docs/plans/a.md"},
		{At: at(1, 9), Service: "conjure", Plan: "docs/plans/a.md"},  // before the window
		{At: at(26, 1), Service: "conjure", Plan: "docs/plans/a.md"}, // after it
		{At: at(21, 9), Service: "agentic-kit", Plan: "docs/plans/x.md"},
		{At: time.Time{}, Service: "conjure", Plan: "docs/plans/a.md"}, // no timestamp
	}
	got := console.VerifyRunsIn(runs, strings.ToLower, at(12, 0), at(26, 0))
	if got["conjure"] != 2 || got["agentic-kit"] != 1 || len(got) != 2 {
		t.Errorf("verify runs in the window = %v, want conjure 2, agentic-kit 1", got)
	}
}
