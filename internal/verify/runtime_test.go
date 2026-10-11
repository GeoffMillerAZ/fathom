package verify_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// runtimeEvidence is fakeEvidence plus verify.RuntimeEvidence: the newest
// attested run per test name, and the clock a run's age is read against.
type runtimeEvidence struct {
	fakeEvidence
	runs  map[string]verify.AttestedRun
	err   error
	now   time.Time
	asked [][]string
}

func (r *runtimeEvidence) NewestAttestedRuns(tests []string) (map[string]verify.AttestedRun, error) {
	r.asked = append(r.asked, append([]string(nil), tests...))
	if r.err != nil {
		return nil, r.err
	}
	out := map[string]verify.AttestedRun{}
	for _, t := range tests {
		if run, ok := r.runs[t]; ok {
			out[t] = run
		}
	}
	return out, nil
}

func (r *runtimeEvidence) Now() time.Time { return r.now }

// TestRuntimePassReadsAttestedRunInformational proves TC-29 (I8 and
// ADR-023 as amended by ADR-067): runtime_pass reads the newest attested
// run of a row's certifying tests — Pass or Fail with the run's commit and
// age, Unknown only when no run exists — and never changes the row's
// status. verify executes nothing: the evidence hands it what test_runs
// holds.
func TestRuntimePassReadsAttestedRunInformational(t *testing.T) {
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	sym := node("Charge", "pay/charge.go", 10, core.TierVerified)
	base := fakeEvidence{
		symbols: map[string][]core.SymbolNode{"Charge": {sym}},
		tests:   map[string][]string{"Charge": {"svc::pay.TestChargeOther", "svc::pay.TestCharge"}},
		reach:   map[string]bool{"TestCharge->Charge": true},
	}
	row := verify.Task{ID: "R-1", Claimed: true, Expect: verify.Expect{Symbol: "Charge", ReachableFrom: "TestCharge"}}
	run := func(status string, at time.Time) verify.AttestedRun {
		return verify.AttestedRun{Test: "svc::pay.TestCharge", Status: status, Attempts: 1,
			Commit: "0123456789abcdef0123456789abcdef01234567", At: at, Run: "sha256:feedface", Suite: "unit"}
	}

	cases := []struct {
		name       string
		runs       map[string]verify.AttestedRun
		want       verify.CriterionStatus
		wantDetail []string
	}{
		{"a passing run reads Pass with its commit and age", map[string]verify.AttestedRun{"TestCharge": run("pass", now.Add(-3*time.Hour))},
			verify.Pass, []string{"TestCharge", "passed", "0123456789ab", "3h old"}},
		{"a failing run reads Fail and the row stays verified", map[string]verify.AttestedRun{"TestCharge": run("fail", now.Add(-50*time.Hour))},
			verify.Fail, []string{"TestCharge", "failed", "0123456789ab", "2d2h old"}},
		{"a flake passed after retries", map[string]verify.AttestedRun{"TestCharge": {Test: "TestCharge", Status: "flake", Attempts: 3, At: now.Add(-90 * time.Second)}},
			verify.Pass, []string{"flake", "3 attempts", "unbound to a commit", "1m old"}},
		{"no run is Unknown, naming the tests it looked for", nil,
			verify.Unknown, []string{"no attested run", "TestCharge"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ev := &runtimeEvidence{fakeEvidence: base, runs: tc.runs, now: now}
			v := verify.VerifyPlan(verify.Plan{Tasks: []verify.Task{row}}, ev).Tasks[0]
			if v.Status != verify.Pass || v.Label != "verified" {
				t.Fatalf("runtime_pass changed the row: status=%s label=%s, want pass/verified (informational, I8)", v.Status, v.Label)
			}
			c, ok := findCrit(v, "runtime_pass")
			if !ok || c.Status != tc.want {
				t.Fatalf("runtime_pass = %+v, want %s", c, tc.want)
			}
			for _, w := range tc.wantDetail {
				if !strings.Contains(c.Detail, w) {
					t.Errorf("runtime_pass detail %q lacks %q", c.Detail, w)
				}
			}
			if g := verify.Gate(verify.Report{Tasks: []verify.TaskVerdict{v}}, true); !g.Pass {
				t.Errorf("a failing attested run gated the row under --strict: %v", g.Failures)
			}
			// The row names its certifying test (reachable_from): only that
			// test is asked for, never the other reaching tests.
			if len(ev.asked) != 1 || len(ev.asked[0]) != 1 || ev.asked[0][0] != "TestCharge" {
				t.Errorf("asked for %v, want [[TestCharge]]", ev.asked)
			}
		})
	}

	// certified_by names the certifying tests ahead of reachable_from; a row
	// naming neither reads the tests that statically reach its symbol; the
	// newest run among them answers.
	t.Run("certified_by and the reaching tests name the certifying tests", func(t *testing.T) {
		ev := &runtimeEvidence{fakeEvidence: base, now: now, runs: map[string]verify.AttestedRun{
			"svc::pay.TestCharge":      run("fail", now.Add(-time.Hour)),
			"svc::pay.TestChargeOther": {Test: "svc::pay.TestChargeOther", Status: "pass", At: now.Add(-2 * time.Hour)},
		}}
		v := verify.VerifyPlan(verify.Plan{Tasks: []verify.Task{{ID: "R-2", Expect: verify.Expect{Symbol: "Charge"}}}}, ev).Tasks[0]
		c, _ := findCrit(v, "runtime_pass")
		if c.Status != verify.Fail || !strings.Contains(c.Detail, "TestCharge failed") {
			t.Errorf("reaching tests: runtime_pass = %+v, want the newest run (TestCharge, failed)", c)
		}
		if v.Status != verify.Pass {
			t.Errorf("row status %s, want pass", v.Status)
		}
	})

	// A failed read is named, never read as "no run" (I38).
	t.Run("a failed read is named", func(t *testing.T) {
		ev := &runtimeEvidence{fakeEvidence: base, now: now, err: errors.New("database is locked")}
		v := verify.VerifyPlan(verify.Plan{Tasks: []verify.Task{row}}, ev).Tasks[0]
		c, _ := findCrit(v, "runtime_pass")
		if c.Status != verify.Unknown || !strings.Contains(c.Detail, "database is locked") || v.Status != verify.Pass {
			t.Errorf("runtime_pass = %+v (row %s), want Unknown naming the failed read", c, v.Status)
		}
	})

	// An evidence source with no attested-run read says so; it never
	// fabricates a verdict.
	t.Run("an evidence source without attested runs", func(t *testing.T) {
		v := verify.VerifyPlan(verify.Plan{Tasks: []verify.Task{row}}, base).Tasks[0]
		c, _ := findCrit(v, "runtime_pass")
		if c.Status != verify.Unknown || !strings.Contains(c.Detail, "no attested-run evidence") {
			t.Errorf("runtime_pass = %+v, want Unknown naming the missing evidence", c)
		}
	})

	// A failing run never becomes the gate's or SARIF's reason for a row
	// that fails on its own contract.
	t.Run("a failing run is never a gate reason", func(t *testing.T) {
		ev := &runtimeEvidence{fakeEvidence: fakeEvidence{}, now: now, runs: map[string]verify.AttestedRun{"TestGhost": run("fail", now)}}
		v := verify.VerifyPlan(verify.Plan{Tasks: []verify.Task{{ID: "R-3", Claimed: true, Expect: verify.Expect{Symbol: "Ghost", ReachableFrom: "TestGhost"}}}}, ev).Tasks[0]
		if c, _ := findCrit(v, "runtime_pass"); c.Status != verify.Fail {
			t.Fatalf("runtime_pass = %+v, want Fail", c)
		}
		g := verify.Gate(verify.Report{Tasks: []verify.TaskVerdict{v}}, true)
		if g.Pass || len(g.Failures) != 1 || strings.Contains(g.Failures[0], "attested run") {
			t.Errorf("gate failures = %v, want the symbol_present reason only", g.Failures)
		}
		for _, r := range verify.GateSARIF([]string{"p.md"}, []verify.Report{{Tasks: []verify.TaskVerdict{v}}}, true) {
			if r.RuleID == "runtime_pass" {
				t.Errorf("SARIF carries runtime_pass as a refusal: %+v", r)
			}
		}
	})
}
