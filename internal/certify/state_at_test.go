package certify

// state_at_test.go — CV-3's own test: TestCertificateStateAt_EveryRule
// (design doc §1.1, O114). Exercises the ladder's rules in the exact
// order design doc §1.1 states them: unreached, no runs, newest run
// failed, newest run unbound, reach changed since the newest passing
// bound run, a flake in the last three, otherwise live.

import (
	"errors"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/store"
)

// alwaysUnchanged is a reachUnchangedSince stub that reports the reach as
// untouched — the common case for every rule this file tests EXCEPT the
// one that specifically exercises a changed reach.
func alwaysUnchanged(string) (bool, error) { return true, nil }

func TestCertificateStateAt_EveryRule(t *testing.T) {
	sha := strings.Repeat("a", 40)

	t.Run("rule 1: unreached short-circuits before runs are even consulted", func(t *testing.T) {
		runs := []store.TestCaseRun{{RunDigest: "d1", Status: "pass", Commit: sha}}
		state, _, flaky, reason, stale, err := CertificateStateAt(false, runs, alwaysUnchanged)
		if err != nil {
			t.Fatal(err)
		}
		if state != StateUnreachable || flaky || reason != "" || stale {
			t.Errorf("got state=%s flaky=%v reason=%q stale=%v, want %s/false/\"\"/false", state, flaky, reason, stale, StateUnreachable)
		}
	})

	t.Run("rule 2: no attested run at all is unproven, \"no attested run\"", func(t *testing.T) {
		state, last, flaky, reason, stale, err := CertificateStateAt(true, nil, alwaysUnchanged)
		if err != nil {
			t.Fatal(err)
		}
		if state != StateUnproven || flaky || stale {
			t.Errorf("got state=%s flaky=%v stale=%v, want %s/false/false", state, flaky, stale, StateUnproven)
		}
		if reason != "no attested run" {
			t.Errorf("reason = %q, want %q", reason, "no attested run")
		}
		if last != (store.TestCaseRun{}) {
			t.Errorf("last = %+v, want the zero value", last)
		}
	})

	t.Run("rule 3: newest run failed is broken, even when unbound", func(t *testing.T) {
		runs := []store.TestCaseRun{{RunDigest: "d1", Status: "fail"}}
		state, last, _, reason, stale, err := CertificateStateAt(true, runs, alwaysUnchanged)
		if err != nil {
			t.Fatal(err)
		}
		if state != StateBroken || reason != "" || stale {
			t.Errorf("got state=%s reason=%q stale=%v, want %s/\"\"/ false", state, reason, stale, StateBroken)
		}
		if last.RunDigest != "d1" {
			t.Errorf("last = %+v, want d1", last)
		}
	})

	t.Run("rule 3 before rule 6: a failing last run is broken even with a flake elsewhere in the window", func(t *testing.T) {
		runs := []store.TestCaseRun{
			{RunDigest: "d2", Status: "fail", Commit: sha},
			{RunDigest: "d1", Status: "flake", Commit: sha},
		}
		state, _, flaky, _, _, err := CertificateStateAt(true, runs, alwaysUnchanged)
		if err != nil {
			t.Fatal(err)
		}
		if state != StateBroken {
			t.Errorf("state = %s, want %s", state, StateBroken)
		}
		if !flaky {
			t.Error("flaky = false, want true (the flake fact is still reported even though it did not decide state)")
		}
	})

	t.Run("rule 4: newest run unbound (no commit) is unproven, \"run not bound to a commit\"", func(t *testing.T) {
		runs := []store.TestCaseRun{{RunDigest: "d1", Status: "pass", Commit: "", Dirty: false}}
		state, _, _, reason, stale, err := CertificateStateAt(true, runs, alwaysUnchanged)
		if err != nil {
			t.Fatal(err)
		}
		if state != StateUnproven || stale {
			t.Errorf("got state=%s stale=%v, want %s/false", state, stale, StateUnproven)
		}
		if reason != "run not bound to a commit" {
			t.Errorf("reason = %q, want %q", reason, "run not bound to a commit")
		}
	})

	t.Run("rule 4: newest run unbound (dirty tree) is unproven, \"run not bound to a commit\"", func(t *testing.T) {
		runs := []store.TestCaseRun{{RunDigest: "d1", Status: "pass", Commit: sha, Dirty: true}}
		state, _, _, reason, _, err := CertificateStateAt(true, runs, alwaysUnchanged)
		if err != nil {
			t.Fatal(err)
		}
		if state != StateUnproven || reason != "run not bound to a commit" {
			t.Errorf("got state=%s reason=%q, want %s/%q", state, reason, StateUnproven, "run not bound to a commit")
		}
	})

	t.Run("rule 5: reach changed since the newest passing bound run is unproven AND stale", func(t *testing.T) {
		runs := []store.TestCaseRun{{RunDigest: "d1", Status: "pass", Commit: sha}}
		changed := func(commit string) (bool, error) {
			if commit != sha {
				t.Errorf("reachUnchangedSince called with commit %q, want %q", commit, sha)
			}
			return false, nil
		}
		state, _, _, reason, stale, err := CertificateStateAt(true, runs, changed)
		if err != nil {
			t.Fatal(err)
		}
		if state != StateUnproven {
			t.Errorf("state = %s, want %s", state, StateUnproven)
		}
		if !stale {
			t.Error("stale = false, want true — the claim's own status must be marked stale")
		}
		want := "reach changed since the attested run at " + sha
		if reason != want {
			t.Errorf("reason = %q, want %q", reason, want)
		}
	})

	t.Run("rule 5 propagates a reachUnchangedSince error rather than guessing", func(t *testing.T) {
		boom := errors.New("boom")
		erroring := func(string) (bool, error) { return false, boom }
		runs := []store.TestCaseRun{{RunDigest: "d1", Status: "pass", Commit: sha}}
		_, _, _, _, _, err := CertificateStateAt(true, runs, erroring)
		if !errors.Is(err, boom) {
			t.Errorf("err = %v, want it to wrap %v", err, boom)
		}
	})

	t.Run("rule 6: a flake within the last three runs is flaky, even though the last run passed", func(t *testing.T) {
		runs := []store.TestCaseRun{
			{RunDigest: "d3", Status: "pass", Commit: sha},
			{RunDigest: "d2", Status: "pass", Commit: sha},
			{RunDigest: "d1", Status: "flake", Commit: sha},
		}
		state, last, flaky, reason, stale, err := CertificateStateAt(true, runs, alwaysUnchanged)
		if err != nil {
			t.Fatal(err)
		}
		if state != StateFlaky || !flaky || reason != "" || stale {
			t.Errorf("got state=%s flaky=%v reason=%q stale=%v, want %s/true/\"\"/false", state, flaky, reason, stale, StateFlaky)
		}
		if last.RunDigest != "d3" {
			t.Errorf("last = %+v, want the most recent run (d3)", last)
		}
	})

	t.Run("a flake OUTSIDE the last-three window never taints a clean run", func(t *testing.T) {
		runs := []store.TestCaseRun{
			{RunDigest: "d5", Status: "pass", Commit: sha},
			{RunDigest: "d4", Status: "pass", Commit: sha},
			{RunDigest: "d3", Status: "pass", Commit: sha},
			{RunDigest: "d2", Status: "pass", Commit: sha},
			{RunDigest: "d1", Status: "flake", Commit: sha},
		}
		state, _, flaky, _, _, err := CertificateStateAt(true, runs, alwaysUnchanged)
		if err != nil {
			t.Fatal(err)
		}
		if state != StateLive || flaky {
			t.Errorf("state=%s flaky=%v, want %s/false (flake sits outside the 3-run window)", state, flaky, StateLive)
		}
	})

	t.Run("rule 7: otherwise live — reached, bound, passing, reach unchanged, no flake", func(t *testing.T) {
		runs := []store.TestCaseRun{{RunDigest: "d1", Status: "pass", Commit: sha}}
		state, last, flaky, reason, stale, err := CertificateStateAt(true, runs, alwaysUnchanged)
		if err != nil {
			t.Fatal(err)
		}
		if state != StateLive || flaky || reason != "" || stale {
			t.Errorf("got state=%s flaky=%v reason=%q stale=%v, want %s/false/\"\"/false", state, flaky, reason, stale, StateLive)
		}
		if last.RunDigest != "d1" {
			t.Errorf("last = %+v, want d1", last)
		}
	})
}
