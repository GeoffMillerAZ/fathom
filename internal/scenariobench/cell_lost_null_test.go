package scenariobench

import "testing"

// A different name proves that the capability, rather than "null", owns
// the exemption. The embedded harness still returns identity only.
type turnlessFixtureHarness struct{ NullHarness }

func (turnlessFixtureHarness) Name() string { return "turnless-fixture" }

type turnsRequiredNullHarness struct{ NullHarness }

func (turnsRequiredNullHarness) ExpectsNoTurns() bool { return false }

func TestCellLost_NullHarnessOnlyWaivesZeroTurns(t *testing.T) {
	sessions := []SessionResult{{}}
	for _, h := range []Harness{NewNullHarness(), turnlessFixtureHarness{}} {
		if reason := CellLost(sessions, h); reason != "" {
			t.Errorf("%s deliberately ran no turns, got LOST: %s", h.Name(), reason)
		}
	}
	if CellLost(sessions) == "" || CellLost(sessions, &FakeHarness{}) == "" || CellLost(sessions, turnsRequiredNullHarness{}) == "" {
		t.Error("zero turns must remain LOST without an affirmative capability")
	}
	for _, tc := range []struct {
		name     string
		sessions []SessionResult
	}{
		{"absent", nil},
		{"errors", []SessionResult{{Error: "session failed"}}},
		{"instrument", []SessionResult{{InstrumentError: "missing server"}, {}}},
		{"answer key", []SessionResult{{AnswerKeyLeak: true}, {}}},
		{"sandbox", []SessionResult{{SandboxEscape: true}, {}}},
		{"baseline", []SessionResult{{BaselineRanFathom: true}, {}}},
	} {
		want := CellLost(tc.sessions)
		if got := CellLost(tc.sessions, NewNullHarness()); want == "" || got != want {
			t.Errorf("%s: null gate = %q, want unchanged defect %q", tc.name, got, want)
		}
	}
}
