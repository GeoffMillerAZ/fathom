package contextbench_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/contextbench"
)

// res is a terse ArmResult builder for the table tests below.
func res(task, arm string, trial int, success bool, tokens int64) contextbench.ArmResult {
	return contextbench.ArmResult{TaskID: task, Arm: arm, Trial: trial, Success: success, TotalTokens: tokens}
}

func TestStatsByTaskArm_AggregatesTrialsAndFlagsFlaky(t *testing.T) {
	in := []contextbench.ArmResult{
		res("T2", "grep", 1, true, 300),
		res("T1", "fathom", 1, true, 100),
		res("T1", "fathom", 2, false, 200),
		res("T1", "fathom", 3, true, 300),
	}
	got := contextbench.StatsByTaskArm(in)
	if len(got) != 2 {
		t.Fatalf("want 2 cells, got %d: %+v", len(got), got)
	}
	// Sorted by (task, arm) — determinism contract.
	if got[0].TaskID != "T1" || got[0].Arm != "fathom" || got[1].TaskID != "T2" {
		t.Fatalf("cells not sorted by (task, arm): %+v", got)
	}
	c := got[0]
	if c.Trials != 3 || c.Successes != 2 {
		t.Fatalf("trials/successes = %d/%d, want 3/2", c.Trials, c.Successes)
	}
	if !c.Flaky {
		t.Error("a cell that passed 2 of 3 trials must be Flaky")
	}
	if c.MeanTokens != 200 || c.MedianTokens != 200 {
		t.Errorf("mean/median tokens = %v/%v, want 200/200", c.MeanTokens, c.MedianTokens)
	}
	// Population stddev of {100,200,300} is sqrt(20000/3) ≈ 81.65.
	if c.StdDevTokens < 81.6 || c.StdDevTokens > 81.7 {
		t.Errorf("stddev = %v, want ≈81.65", c.StdDevTokens)
	}
	if got[1].Flaky {
		t.Error("a single-trial all-pass cell must not be Flaky")
	}
}

func TestStatsByTaskArm_MedianOfEvenTrialsAveragesMiddlePair(t *testing.T) {
	in := []contextbench.ArmResult{
		res("T1", "a", 1, true, 10),
		res("T1", "a", 2, true, 20),
		res("T1", "a", 3, true, 30),
		res("T1", "a", 4, true, 100),
	}
	got := contextbench.StatsByTaskArm(in)
	if got[0].MedianTokens != 25 {
		t.Fatalf("median = %v, want 25 (mean of the two middle values)", got[0].MedianTokens)
	}
	if got[0].MeanTokens != 40 {
		t.Fatalf("mean = %v, want 40", got[0].MeanTokens)
	}
}

func TestPair_MedianRatioResistsASingleOutlier(t *testing.T) {
	// Four tasks where the treatment is cheaper, one where it is
	// catastrophically worse: exactly the H3-1 shape (T13 at 5.20× erased a
	// win on 8 of 13 tasks). The aggregate ratio flips; the paired median
	// does not, which is why the ledger records both.
	var in []contextbench.ArmResult
	for _, tc := range []struct {
		id                 string
		treatment, control int64
	}{
		{"T1", 50, 100},
		{"T2", 50, 100},
		{"T3", 50, 100},
		{"T4", 50, 100},
		{"T5", 5000, 100},
	} {
		in = append(in, res(tc.id, "fathom", 1, true, tc.treatment))
		in = append(in, res(tc.id, "grep", 1, true, tc.control))
	}
	p := contextbench.Pair(in, "fathom", "grep")
	if p.MedianTokenRatio != 0.5 {
		t.Errorf("median token ratio = %v, want 0.5", p.MedianTokenRatio)
	}
	if p.AggregateTokenRatio <= 1 {
		t.Errorf("aggregate token ratio = %v, want >1 (the outlier dominates the total)", p.AggregateTokenRatio)
	}
	if p.TasksTreatmentCheaper != 4 || p.TasksControlCheaper != 1 {
		t.Errorf("cheaper counts = %d/%d, want 4/1", p.TasksTreatmentCheaper, p.TasksControlCheaper)
	}
}

func TestPair_TaskMissingFromOneArmIsAbsentNotDropped(t *testing.T) {
	in := []contextbench.ArmResult{
		res("T1", "fathom", 1, true, 50),
		res("T1", "grep", 1, true, 100),
		res("T2", "grep", 1, true, 100), // fathom arm never ran this task
	}
	p := contextbench.Pair(in, "fathom", "grep")
	if len(p.Absent) != 1 || p.Absent[0] != "T2" {
		t.Fatalf("Absent = %v, want [T2] — a task missing from an arm must be reported, never dropped", p.Absent)
	}
	if p.Tasks != 1 {
		t.Errorf("Tasks = %d, want 1 (only the fully paired task is compared)", p.Tasks)
	}
}

func TestCriterion_IncompleteCoverageIsIndeterminateNotPass(t *testing.T) {
	// ADR-027: an instrument may not degrade toward "healthy". A run that
	// silently lost an arm's rows must never be able to report a win.
	in := []contextbench.ArmResult{
		res("T1", "fathom", 1, true, 50),
		res("T1", "grep", 1, false, 100),
		res("T2", "grep", 1, true, 100), // absent from the treatment arm
	}
	p := contextbench.Pair(in, "fathom", "grep")
	c := contextbench.Criterion{
		ID:                           "H2-1",
		Description:                  "strictly more successes at ≤1× tokens",
		RequireStrictlyMoreSuccesses: true,
		MaxTokenRatio:                1.0,
	}
	got := c.Evaluate(p)
	if got.Verdict != contextbench.VerdictIndeterminate {
		t.Fatalf("verdict = %s, want indeterminate — coverage is incomplete", got.Verdict)
	}
	if got.Reason == "" {
		t.Error("an indeterminate verdict must carry a reason")
	}
}

func TestCriterion_PassAndFailAreBothReachable(t *testing.T) {
	win := []contextbench.ArmResult{
		res("T1", "fathom", 1, true, 50), res("T1", "grep", 1, false, 100),
		res("T2", "fathom", 1, true, 50), res("T2", "grep", 1, true, 100),
	}
	c := contextbench.Criterion{ID: "H2-1", RequireStrictlyMoreSuccesses: true, MaxTokenRatio: 1.0}
	if got := c.Evaluate(contextbench.Pair(win, "fathom", "grep")); got.Verdict != contextbench.VerdictPass {
		t.Fatalf("verdict = %s (%s), want pass", got.Verdict, got.Reason)
	}

	// Equal success, cheaper tokens: criterion 1 fails. This is the exact
	// 2026-07-24 home-bench shape, and it must read FAIL, not pass.
	equal := []contextbench.ArmResult{
		res("T1", "fathom", 1, true, 50), res("T1", "grep", 1, true, 100),
		res("T2", "fathom", 1, true, 50), res("T2", "grep", 1, true, 100),
	}
	got := c.Evaluate(contextbench.Pair(equal, "fathom", "grep"))
	if got.Verdict != contextbench.VerdictFail {
		t.Fatalf("verdict = %s, want fail — equal success does not meet 'strictly more'", got.Verdict)
	}
}

func TestCriterion_TokenRatioUsesTheAggregateNotTheFriendlierMedian(t *testing.T) {
	// The pre-registered H2-1 criterion is written over totals. The median
	// is reported for interpretation but must NOT be what the gate reads,
	// or a criterion could be silently loosened after the data arrived.
	in := []contextbench.ArmResult{
		res("T1", "fathom", 1, true, 50), res("T1", "grep", 1, false, 100),
		res("T2", "fathom", 1, true, 50), res("T2", "grep", 1, false, 100),
		res("T3", "fathom", 1, true, 5000), res("T3", "grep", 1, false, 100),
	}
	p := contextbench.Pair(in, "fathom", "grep")
	if p.MedianTokenRatio >= 1 {
		t.Fatalf("precondition: median ratio should be <1, got %v", p.MedianTokenRatio)
	}
	c := contextbench.Criterion{ID: "H2-1", RequireStrictlyMoreSuccesses: true, MaxTokenRatio: 1.0}
	got := c.Evaluate(p)
	if got.Verdict != contextbench.VerdictFail {
		t.Fatalf("verdict = %s, want fail — the aggregate ratio is >1 even though the median is not", got.Verdict)
	}
}

// TestMachineCriterionMatchesTheRegisteredProse pins the home corpus's
// machine-evaluable criterion to the values its prose registration states.
// Loosening any of them — dropping "strictly", raising the token cap,
// lowering the replication floor — must be a deliberate edit here, visible
// in review, never a quiet change that lets a weaker run read as a win.
func TestMachineCriterionMatchesTheRegisteredProse(t *testing.T) {
	c, err := contextbench.LoadCriterion("../../testdata/contextbench/tasks-home.json")
	if err != nil {
		t.Fatalf("LoadCriterion: %v", err)
	}
	if c.ID != "H2-1" {
		t.Errorf("id = %q, want H2-1 (the id the plan registered)", c.ID)
	}
	if !c.RequireStrictlyMoreSuccesses {
		t.Error("the registration says STRICTLY GREATER success; equal must not pass")
	}
	if c.MaxTokenRatio != 1.0 {
		t.Errorf("max token ratio = %v, want 1.0 (no token premium for winning)", c.MaxTokenRatio)
	}
	if c.MinTrials != 3 {
		t.Errorf("min trials = %d, want 3 (the replication floor registered 2026-09-01)", c.MinTrials)
	}
}

func TestLoadCriterion_CorpusWithoutOneIsAnError(t *testing.T) {
	// The original grep-native corpus carries no machine criterion; scoring
	// it must fail loudly rather than fabricate a bar after the fact.
	if _, err := contextbench.LoadCriterion("../../testdata/contextbench/tasks.json"); err == nil {
		t.Fatal("want an error for a corpus with no machine_criterion, got nil")
	}
}
