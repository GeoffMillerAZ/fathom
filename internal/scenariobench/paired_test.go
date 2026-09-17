package scenariobench

import (
	"strings"
	"testing"

	reliabilitystats "github.com/geoffmilleraz/agentic-kit/foundations/quality/reliability-stats"
)

// TestComputePairedRunStats_MeanMatchesHandComputation pins the point
// statistics (never seed-dependent) against a hand-computed mean over
// three tasks — the same shape wave N15 item 1's own reference bootstrap
// checked lca-run-1 against: mean per-task success-rate diff, and mean
// per-task token ratio.
func TestComputePairedRunStats_MeanMatchesHandComputation(t *testing.T) {
	rows := []ScenarioLedgerRow{
		// t1: diff 0, ratio 0.5
		{TaskID: "t1", Arm: "fathom", Trials: 3, Successes: 3, Readings: Readings{TotalTokens: 3000}},
		{TaskID: "t1", Arm: "control", Trials: 3, Successes: 3, Readings: Readings{TotalTokens: 6000}},
		// t2: diff +1/3, ratio 1.5
		{TaskID: "t2", Arm: "fathom", Trials: 3, Successes: 2, Readings: Readings{TotalTokens: 4500}},
		{TaskID: "t2", Arm: "control", Trials: 3, Successes: 1, Readings: Readings{TotalTokens: 3000}},
		// t3: diff -1/3, ratio 3.0
		{TaskID: "t3", Arm: "fathom", Trials: 3, Successes: 1, Readings: Readings{TotalTokens: 9000}},
		{TaskID: "t3", Arm: "control", Trials: 3, Successes: 2, Readings: Readings{TotalTokens: 3000}},
	}
	stats, err := ComputePairedRunStats(rows, "fathom", "control")
	if err != nil {
		t.Fatalf("ComputePairedRunStats: %v", err)
	}
	if stats.Tasks != 3 {
		t.Fatalf("Tasks = %d, want 3", stats.Tasks)
	}
	// mean diff = (0 + 1/3 - 1/3) / 3 = 0
	if got, want := stats.SuccessDiff.Point, 0.0; got < want-1e-9 || got > want+1e-9 {
		t.Errorf("SuccessDiff.Point = %v, want %v", got, want)
	}
	if stats.SuccessDiff.Low > stats.SuccessDiff.Point || stats.SuccessDiff.High < stats.SuccessDiff.Point {
		t.Errorf("SuccessDiff CI [%v, %v] must bracket the point %v", stats.SuccessDiff.Low, stats.SuccessDiff.High, stats.SuccessDiff.Point)
	}
	// mean ratio = (0.5 + 1.5 + 3.0) / 3 = 1.6666...
	if got, want := stats.TokenRatio.Point, 5.0/3.0; got < want-1e-9 || got > want+1e-9 {
		t.Errorf("TokenRatio.Point = %v, want %v", got, want)
	}
	if stats.TokenRatioTasks != 3 {
		t.Errorf("TokenRatioTasks = %d, want 3", stats.TokenRatioTasks)
	}
	if stats.TokenRatio.Low > stats.TokenRatio.Point || stats.TokenRatio.High < stats.TokenRatio.Point {
		t.Errorf("TokenRatio CI [%v, %v] must bracket the point %v", stats.TokenRatio.Low, stats.TokenRatio.High, stats.TokenRatio.Point)
	}
}

// TestComputePairedRunStats_UnpairedTaskExcludedNotErrored proves a task
// present in only one arm is simply skipped — never dropped into an
// error, and never silently paired against nothing.
func TestComputePairedRunStats_UnpairedTaskExcludedNotErrored(t *testing.T) {
	rows := []ScenarioLedgerRow{
		{TaskID: "t1", Arm: "fathom", Trials: 3, Successes: 3, Readings: Readings{TotalTokens: 100}},
		{TaskID: "t1", Arm: "control", Trials: 3, Successes: 3, Readings: Readings{TotalTokens: 100}},
		{TaskID: "t2", Arm: "fathom", Trials: 3, Successes: 1, Readings: Readings{TotalTokens: 100}}, // no control row
	}
	stats, err := ComputePairedRunStats(rows, "fathom", "control")
	if err != nil {
		t.Fatalf("ComputePairedRunStats: %v", err)
	}
	if stats.Tasks != 1 {
		t.Errorf("Tasks = %d, want 1 (t2 has no control row and must not be paired)", stats.Tasks)
	}
}

// TestComputePairedRunStats_VoidedRowsExcluded proves an InstrumentDefect
// row never enters the pairing — mirroring renderRunVerdict's own
// exclusion of voided rows from every arithmetic.
func TestComputePairedRunStats_VoidedRowsExcluded(t *testing.T) {
	rows := []ScenarioLedgerRow{
		{TaskID: "t1", Arm: "fathom", Trials: 3, Successes: 3, Readings: Readings{TotalTokens: 100}},
		{TaskID: "t1", Arm: "control", Trials: 3, Successes: 3, Readings: Readings{TotalTokens: 100}},
		{TaskID: "t2", Arm: "fathom", Trials: 3, Successes: 3, InstrumentDefect: "O41"},
		{TaskID: "t2", Arm: "control", Trials: 3, Successes: 0},
	}
	stats, err := ComputePairedRunStats(rows, "fathom", "control")
	if err != nil {
		t.Fatalf("ComputePairedRunStats: %v", err)
	}
	if stats.Tasks != 1 {
		t.Errorf("Tasks = %d, want 1 (t2's voided fathom row must exclude the pair)", stats.Tasks)
	}
}

// TestComputePairedRunStats_NoPairedTaskIsError proves the "nothing to
// resample" case refuses rather than returning a fabricated zero-task
// stats value.
func TestComputePairedRunStats_NoPairedTaskIsError(t *testing.T) {
	rows := []ScenarioLedgerRow{
		{TaskID: "t1", Arm: "fathom", Trials: 3, Successes: 3},
	}
	if _, err := ComputePairedRunStats(rows, "fathom", "control"); err == nil {
		t.Error("expected an error when no task pairs across the two arms")
	}
}

// TestRenderPairedRunStats_Format pins the rendered line's shape: task
// count, seed, resamples, the signed success-diff CI, and the token
// ratio CI when measured.
func TestRenderPairedRunStats_Format(t *testing.T) {
	stats := PairedRunStats{
		Tasks:       39,
		SuccessDiff: reliabilitystats.Interval{Point: 0.009, Low: -0.077, High: 0.094, Tasks: 39},
	}
	out := RenderPairedRunStats(stats)
	if !strings.HasPrefix(out, "Paired (bootstrap, n=39, seed 20260902, 20000 resamples): success-rate diff +0.009 [-0.077, +0.094] (95% CI)") {
		t.Errorf("unexpected render (no token ratio measured):\n%s", out)
	}
	if strings.Contains(out, "token ratio") {
		t.Errorf("must omit the token ratio clause when TokenRatioTasks is 0:\n%s", out)
	}

	withRatio := PairedRunStats{
		Tasks:           39,
		SuccessDiff:     reliabilitystats.Interval{Point: 0.009, Low: -0.077, High: 0.094, Tasks: 39},
		TokenRatio:      reliabilitystats.Interval{Point: 2.00, Low: 1.71, High: 2.34, Tasks: 39},
		TokenRatioTasks: 39,
	}
	out2 := RenderPairedRunStats(withRatio)
	if !strings.Contains(out2, "token ratio 2.00× [1.71×, 2.34×]") {
		t.Errorf("expected the token ratio clause, got:\n%s", out2)
	}
}
