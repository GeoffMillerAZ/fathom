package main

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// TestDoctorStrangerCheckTimed is SR-53's anchor test (docs/plans/
// surface-reckoning.md, design §5 row SR-53, F-51): doctorStrangerCheck
// times a sequence of named steps against G16's 5-minute bar, using an
// INJECTED clock (never a bare time.Now — the determinism contract,
// internal/AGENTS.md) so the pass/fail boundary is provably exact without
// a single real timed operation.
func TestDoctorStrangerCheckTimed(t *testing.T) {
	fakeClock := func(times ...time.Time) func() time.Time {
		i := 0
		return func() time.Time {
			tm := times[i]
			if i < len(times)-1 {
				i++
			}
			return tm
		}
	}
	base := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)

	t.Run("under the bar passes", func(t *testing.T) {
		now := fakeClock(base, base.Add(2*time.Minute))
		steps := []strangerStep{{Name: "ingest+verify", Run: func() error { return nil }}}
		got := doctorStrangerCheck(steps, strangerCheckBar, now)
		if got.Status != doctorPass {
			t.Fatalf("Status = %q, want pass; reason: %s", got.Status, got.Reason)
		}
		if !strings.Contains(got.Reason, "2m0s") {
			t.Errorf("Reason = %q, want it to name the elapsed 2m0s", got.Reason)
		}
	})

	t.Run("over the bar fails, naming the elapsed time and the bar", func(t *testing.T) {
		now := fakeClock(base, base.Add(6*time.Minute))
		steps := []strangerStep{{Name: "ingest+verify", Run: func() error { return nil }}}
		got := doctorStrangerCheck(steps, strangerCheckBar, now)
		if got.Status != doctorFail {
			t.Fatalf("Status = %q, want fail", got.Status)
		}
		if !strings.Contains(got.Reason, "6m0s") || !strings.Contains(got.Reason, strangerCheckBar.String()) {
			t.Errorf("Reason = %q, want it to name both the elapsed 6m0s and the %s bar", got.Reason, strangerCheckBar)
		}
	})

	t.Run("a failing step fails at that step, naming it and the elapsed time so far — never claiming the bar was met", func(t *testing.T) {
		now := fakeClock(base, base.Add(30*time.Second))
		steps := []strangerStep{
			{Name: "build", Run: func() error { return nil }},
			{Name: "ingest", Run: func() error { return errors.New("ingest exit 1") }},
			{Name: "verify", Run: func() error { t.Fatal("verify must not run after ingest failed"); return nil }},
		}
		got := doctorStrangerCheck(steps, strangerCheckBar, now)
		if got.Status != doctorFail {
			t.Fatalf("Status = %q, want fail", got.Status)
		}
		if !strings.Contains(got.Reason, "ingest") || !strings.Contains(got.Reason, "ingest exit 1") {
			t.Errorf("Reason = %q, want it to name the failing step and its error", got.Reason)
		}
	})

	t.Run("exactly at the bar passes (a strict > comparison, never >=)", func(t *testing.T) {
		now := fakeClock(base, base.Add(strangerCheckBar))
		steps := []strangerStep{{Name: "s", Run: func() error { return nil }}}
		got := doctorStrangerCheck(steps, strangerCheckBar, now)
		if got.Status != doctorPass {
			t.Fatalf("Status = %q, want pass at exactly the bar", got.Status)
		}
	})
}
