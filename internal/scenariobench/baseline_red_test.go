package scenariobench

import (
	"context"
	"testing"
)

// fakeDetectorWorktree returns a Worktree whose Path/CellDir are both
// plain temp directories — enough for runDetector (via RunBaselineDetectors)
// to shell out a detector command and write its log, with no git/subject
// machinery needed.
func fakeDetectorWorktree(t *testing.T) Worktree {
	t.Helper()
	return Worktree{Path: t.TempDir(), CellDir: t.TempDir()}
}

// TestRunBaselineDetectors_RecordsRedAndGreen: a fake detector that always
// exits 1 (red against expect_exit 0) is recorded Red; one that always
// exits 0 is not.
func TestRunBaselineDetectors_RecordsRedAndGreen(t *testing.T) {
	wt := fakeDetectorWorktree(t)
	task := Task{
		Oracle: Oracle{Detectors: []Detector{
			{Name: "always-red", Command: "exit 1", ExpectExit: 0},
			{Name: "always-green", Command: "exit 0", ExpectExit: 0},
		}},
	}
	baseline, err := RunBaselineDetectors(context.Background(), wt, task)
	if err != nil {
		t.Fatal(err)
	}
	if len(baseline) != 2 {
		t.Fatalf("baseline = %+v, want 2 entries", baseline)
	}
	if !baseline["always-red"].Red {
		t.Error("always-red must be recorded Red")
	}
	if baseline["always-green"].Red {
		t.Error("always-green must not be recorded Red")
	}

	// No detectors: nil, not an error.
	empty, err := RunBaselineDetectors(context.Background(), wt, Task{})
	if err != nil || empty != nil {
		t.Errorf("RunBaselineDetectors with no detectors = (%v, %v), want (nil, nil)", empty, err)
	}
}

// TestVoidBaselineRedDetectors_ExcludesBaselineRedFromPassFail is O57
// finding 3's anchor: a detector already red at baseline (a fake
// "instrument-broken" detector — the deliverable's own "fake detector")
// is voided post-session and Passed flips to true once it is the only
// thing keeping the cell failing; a detector that was GREEN at baseline
// but turned red during the session (a real, agent-caused violation)
// stays violated and Passed stays false.
func TestVoidBaselineRedDetectors_ExcludesBaselineRedFromPassFail(t *testing.T) {
	task := Task{
		Oracle: Oracle{Detectors: []Detector{
			{Name: "instrument-broken", ExpectExit: 0},
			{Name: "real-rule", ExpectExit: 0},
		}},
	}
	baseline := map[string]BaselineDetectorResult{
		"instrument-broken": {Name: "instrument-broken", Exit: 1, Red: true},
		"real-rule":         {Name: "real-rule", Exit: 0, Red: false},
	}

	// Post-session: the instrument-broken one is STILL red (as it always
	// was, unrelated to the agent), and the real rule is also red now —
	// a genuine violation this session caused.
	result := &OracleResult{
		Passed:       false,
		FailToPassOK: true,
		PassToPassOK: true,
		Detectors: []DetectorResult{
			{Name: "instrument-broken", Exit: 1, Violated: true},
			{Name: "real-rule", Exit: 1, Violated: true},
		},
	}
	VoidBaselineRedDetectors(result, baseline, task, nil)

	if !result.Detectors[0].VoidedBaselineRed || result.Detectors[0].Violated {
		t.Errorf("instrument-broken detector = %+v, want voided and not violated", result.Detectors[0])
	}
	if result.Detectors[0].Note == "" {
		t.Error("expected a note explaining the void")
	}
	if result.Detectors[1].VoidedBaselineRed || !result.Detectors[1].Violated {
		t.Errorf("real-rule detector = %+v, want NOT voided and still violated (a real violation)", result.Detectors[1])
	}
	if result.Passed {
		t.Error("Passed must stay false: real-rule is a genuine, non-baseline violation")
	}

	// Second scenario: ONLY the instrument-broken detector is red
	// post-session (the agent never touched anything real-rule governs) —
	// voiding it must flip Passed to true.
	result2 := &OracleResult{
		Passed:       false,
		FailToPassOK: true,
		PassToPassOK: true,
		Detectors: []DetectorResult{
			{Name: "instrument-broken", Exit: 1, Violated: true},
			{Name: "real-rule", Exit: 0, Violated: false},
		},
	}
	VoidBaselineRedDetectors(result2, baseline, task, nil)
	if !result2.Passed {
		t.Errorf("Passed = false, want true once the only violation is voided as baseline_red (%+v)", result2)
	}
}

// TestVoidBaselineRedDetectors_NoOpWhenNothingVoided: a result with no
// baseline-red detector among its violations is left untouched.
func TestVoidBaselineRedDetectors_NoOpWhenNothingVoided(t *testing.T) {
	task := Task{Oracle: Oracle{Detectors: []Detector{{Name: "real-rule", ExpectExit: 0}}}}
	baseline := map[string]BaselineDetectorResult{"real-rule": {Name: "real-rule", Exit: 0, Red: false}}
	result := &OracleResult{
		Passed:       false,
		FailToPassOK: true,
		PassToPassOK: true,
		Detectors:    []DetectorResult{{Name: "real-rule", Exit: 1, Violated: true}},
	}
	VoidBaselineRedDetectors(result, baseline, task, nil)
	if result.Detectors[0].VoidedBaselineRed || result.Passed {
		t.Errorf("expected no change: %+v", result)
	}
}

// TestVoidBaselineRedDetectors_RefusalAwareNeverRecomputesPassed: a
// refusal-aware task's Passed (decided by ScoreRefusal, not the detector
// set) must survive untouched even when a detector is baseline-red.
func TestVoidBaselineRedDetectors_RefusalAwareNeverRecomputesPassed(t *testing.T) {
	task := Task{
		Oracle: Oracle{
			Detectors: []Detector{{Name: "change-present", ExpectExit: 0}},
			Refusal:   &Refusal{MustCite: []string{"X"}},
		},
	}
	baseline := map[string]BaselineDetectorResult{"change-present": {Name: "change-present", Exit: 1, Red: true}}
	result := &OracleResult{
		Passed:  true, // ScoreRefusal already decided this
		Outcome: RefusalOutcomeRefused,
		Detectors: []DetectorResult{
			{Name: "change-present", Exit: 1, Violated: true},
		},
	}
	VoidBaselineRedDetectors(result, baseline, task, nil)
	if !result.Passed || result.Outcome != RefusalOutcomeRefused {
		t.Errorf("refusal-aware result must survive untouched, got %+v", result)
	}
}
