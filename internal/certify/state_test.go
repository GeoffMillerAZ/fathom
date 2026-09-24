package certify

// state_test.go — TC-21's own test: TestCertificateState_FlakyFromLastThreeRuns.
import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestCertificateState_FlakyFromLastThreeRuns(t *testing.T) {
	t.Run("a flake within the last three runs is flaky, even though the last run passed", func(t *testing.T) {
		runs := []store.TestCaseRun{
			{RunDigest: "d3", Status: "pass"},
			{RunDigest: "d2", Status: "pass"},
			{RunDigest: "d1", Status: "flake"},
		}
		state, last, flaky := CertificateState(true, runs)
		if state != StateFlaky {
			t.Errorf("state = %s, want %s", state, StateFlaky)
		}
		if !flaky {
			t.Error("flaky = false, want true")
		}
		if last.RunDigest != "d3" {
			t.Errorf("last = %+v, want the most recent run (d3)", last)
		}
	})

	t.Run("a flake OUTSIDE the last-three window never taints a clean run", func(t *testing.T) {
		runs := []store.TestCaseRun{
			{RunDigest: "d5", Status: "pass"},
			{RunDigest: "d4", Status: "pass"},
			{RunDigest: "d3", Status: "pass"},
			{RunDigest: "d2", Status: "pass"},
			{RunDigest: "d1", Status: "flake"},
		}
		state, _, flaky := CertificateState(true, runs)
		if state != StateLive || flaky {
			t.Errorf("state = %s flaky=%v, want %s/false (flake sits outside the 3-run window)", state, flaky, StateLive)
		}
	})

	t.Run("unreached short-circuits before flakiness is even considered", func(t *testing.T) {
		runs := []store.TestCaseRun{{RunDigest: "d1", Status: "flake"}}
		state, _, flaky := CertificateState(false, runs)
		if state != StateUnreachable || flaky {
			t.Errorf("state = %s flaky=%v, want %s/false", state, flaky, StateUnreachable)
		}
	})

	t.Run("no attested run at all is unproven, not flaky", func(t *testing.T) {
		state, last, flaky := CertificateState(true, nil)
		if state != StateUnproven || flaky {
			t.Errorf("state = %s flaky=%v, want %s/false", state, flaky, StateUnproven)
		}
		if last != (store.TestCaseRun{}) {
			t.Errorf("last = %+v, want the zero value", last)
		}
	})

	t.Run("a broken last run is broken, even with a flake elsewhere in the window", func(t *testing.T) {
		runs := []store.TestCaseRun{
			{RunDigest: "d2", Status: "fail"},
			{RunDigest: "d1", Status: "flake"},
		}
		state, _, flaky := CertificateState(true, runs)
		if state != StateBroken {
			t.Errorf("state = %s, want %s (a failing last run is never hidden behind an old flake)", state, StateBroken)
		}
		if !flaky {
			t.Error("flaky = false, want true (the flake fact is still reported, even though it did not determine state)")
		}
	})
}
