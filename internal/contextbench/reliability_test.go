package contextbench_test

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/contextbench"
)

// ── PB-10: reliability figures, reported never gating ─────────────────

// fiveTasks is a paired corpus with known answers: three trials per cell,
// treatment better on T1 and T2, equal on T3 and T4, worse on T5.
//
//	task  treatment  control   diff   tokens t/c
//	T1    3/3        1/3      +0.667   100/200
//	T2    2/3        0/3      +0.667   150/300
//	T3    3/3        3/3       0       200/200
//	T4    0/3        0/3       0       250/100
//	T5    1/3        3/3      -0.667   300/150
func fiveTasks() []contextbench.ArmResult {
	cell := func(task, arm string, passes int, tokens int64) []contextbench.ArmResult {
		var out []contextbench.ArmResult
		for i := 1; i <= 3; i++ {
			out = append(out, res(task, arm, i, i <= passes, tokens))
		}
		return out
	}
	var all []contextbench.ArmResult
	all = append(all, cell("T1", "fathom", 3, 100)...)
	all = append(all, cell("T1", "grep", 1, 200)...)
	all = append(all, cell("T2", "fathom", 2, 150)...)
	all = append(all, cell("T2", "grep", 0, 300)...)
	all = append(all, cell("T3", "fathom", 3, 200)...)
	all = append(all, cell("T3", "grep", 3, 200)...)
	all = append(all, cell("T4", "fathom", 0, 250)...)
	all = append(all, cell("T4", "grep", 0, 100)...)
	all = append(all, cell("T5", "fathom", 1, 300)...)
	all = append(all, cell("T5", "grep", 3, 150)...)
	return all
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestComputeReliability_KnownAnswers(t *testing.T) {
	p := contextbench.Pair(fiveTasks(), "fathom", "grep")
	r, err := contextbench.ComputeReliability(p, contextbench.ReliabilityConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if r.BootstrapSeed != contextbench.DefaultBootstrapSeed || r.Resamples != contextbench.DefaultBootstrapResamples {
		t.Errorf("defaults not recorded on the result: seed=%d resamples=%d", r.BootstrapSeed, r.Resamples)
	}

	// CI: the point estimate is the mean over tasks of the per-task
	// success-rate difference: (2/3 + 2/3 + 0 + 0 − 2/3)/5 = 2/15.
	if r.CISuccessDiff == nil {
		t.Fatal("ci_success_diff must be filled for a paired run")
	}
	if !near(r.CISuccessDiff.Point, 2.0/15) || r.CISuccessDiff.Tasks != 5 {
		t.Errorf("ci point/tasks = %v/%d, want 0.1333/5", r.CISuccessDiff.Point, r.CISuccessDiff.Tasks)
	}
	if r.CISuccessDiff.Low > r.CISuccessDiff.Point || r.CISuccessDiff.High < r.CISuccessDiff.Point {
		t.Errorf("interval [%v, %v] must bracket the point %v", r.CISuccessDiff.Low, r.CISuccessDiff.High, r.CISuccessDiff.Point)
	}
	if r.CISuccessDiff.Low < -2.0/3 || r.CISuccessDiff.High > 2.0/3 {
		t.Errorf("a resample of task means cannot leave [−2/3, 2/3]: [%v, %v]", r.CISuccessDiff.Low, r.CISuccessDiff.High)
	}

	// Wilcoxon on per-task mean tokens (treatment − control): differences
	// −100, −150, 0, +150, +150 → zero dropped, n = 4, |d| ranks
	// {100→1, 150→3, 150→3, 150→3}; W+ = 6, W− = 4, W = 4; exact.
	if r.WilcoxonP == nil || r.WilcoxonN != 4 || r.WilcoxonZeros != 1 {
		t.Fatalf("wilcoxon n/zeros = %d/%d (p=%v), want 4/1", r.WilcoxonN, r.WilcoxonZeros, r.WilcoxonP)
	}
	if !r.WilcoxonExact {
		t.Error("n = 4 must be the exact branch")
	}
	// 2^4 = 16 sign assignments over ranks {1,3,3,3}; Pr[T <= 4] counts
	// subsets summing to <= 4: {}, {1}, {3}×3, {1,3}×3 = 8 → p = min(1, 2·8/16) = 1.
	if !near(*r.WilcoxonP, 1) {
		t.Errorf("wilcoxon_p = %v, want 1 (four pairs cannot reach significance)", *r.WilcoxonP)
	}

	// A12 over per-task success rates: a = {1, 2/3, 1, 0, 1/3},
	// b = {1/3, 0, 1, 0, 1}. Pairwise wins + half-ties over 25 pairs:
	// a=1 vs b: >, >, =, >, = → 3 + 2·½ = 4 (T1 and T3: 8)
	// a=2/3:    >, >, <, >, < → 3
	// a=0:      <, =, <, =, < → 1
	// a=1/3:    =, >, <, >, < → 2.5
	// total = 8 + 3 + 1 + 2.5 = 14.5 → 0.58.
	if r.A12 == nil || !near(*r.A12, 14.5/25) {
		t.Errorf("a12 = %v, want 0.58", r.A12)
	}

	// pass@1 = mean over tasks of c/n (the unbiased estimator at k = 1 is
	// the success rate); pass^3 = mean over tasks of rate³.
	wantPass1 := map[string]float64{"fathom": (1 + 2.0/3 + 1 + 0 + 1.0/3) / 5, "grep": (1.0/3 + 0 + 1 + 0 + 1) / 5}
	wantPow3 := map[string]float64{
		"fathom": (1 + math.Pow(2.0/3, 3) + 1 + 0 + math.Pow(1.0/3, 3)) / 5,
		"grep":   (math.Pow(1.0/3, 3) + 0 + 1 + 0 + 1) / 5,
	}
	for arm, want := range wantPass1 {
		if !near(r.PassAt1[arm], want) {
			t.Errorf("pass_at_1[%s] = %v, want %v", arm, r.PassAt1[arm], want)
		}
	}
	for arm, want := range wantPow3 {
		if !near(r.PassPow3[arm], want) {
			t.Errorf("pass_pow_3[%s] = %v, want %v", arm, r.PassPow3[arm], want)
		}
	}
}

func TestComputeReliability_IsDeterministicUnderItsSeed(t *testing.T) {
	p := contextbench.Pair(fiveTasks(), "fathom", "grep")
	a, err := contextbench.ComputeReliability(p, contextbench.ReliabilityConfig{BootstrapSeed: 7})
	if err != nil {
		t.Fatal(err)
	}
	b, err := contextbench.ComputeReliability(p, contextbench.ReliabilityConfig{BootstrapSeed: 7})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("same seed, same sample, different result:\n%+v\n%+v", a, b)
	}
	if a.BootstrapSeed != 7 {
		t.Errorf("the corpus's seed must be recorded on the result, got %d", a.BootstrapSeed)
	}
	// Input order must not matter: Pair sorts, and the bootstrap orders
	// observations before sampling.
	rev := fiveTasks()
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	c, err := contextbench.ComputeReliability(contextbench.Pair(rev, "fathom", "grep"), contextbench.ReliabilityConfig{BootstrapSeed: 7})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, c) {
		t.Fatal("reversed input order changed the reliability figures")
	}
}

func TestComputeReliability_NoPairsIsReportedNotZero(t *testing.T) {
	// Every task ties on tokens: Wilcoxon has no non-zero pair and is
	// undefined — the field stays absent, never 0 or 1 (a 0 would claim
	// significance; a 1 would claim a test was run).
	in := []contextbench.ArmResult{
		res("T1", "fathom", 1, true, 100), res("T1", "grep", 1, false, 100),
		res("T2", "fathom", 1, true, 100), res("T2", "grep", 1, true, 100),
	}
	r, err := contextbench.ComputeReliability(contextbench.Pair(in, "fathom", "grep"), contextbench.ReliabilityConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if r.WilcoxonP != nil {
		t.Errorf("wilcoxon_p must be absent when no pair differs, got %v", *r.WilcoxonP)
	}
	if r.WilcoxonZeros != 2 {
		t.Errorf("the dropped zero differences must still be counted: %d", r.WilcoxonZeros)
	}
	if r.CISuccessDiff == nil || !near(r.CISuccessDiff.Point, 0.5) {
		t.Errorf("the CI is still computable: %+v", r.CISuccessDiff)
	}
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "wilcoxon_p") {
		t.Errorf("an undefined test must not serialise as a number: %s", raw)
	}
}

func TestComputeReliability_UnpairedRunIsAnError(t *testing.T) {
	_, err := contextbench.ComputeReliability(contextbench.PairedSummary{}, contextbench.ReliabilityConfig{})
	if err == nil {
		t.Fatal("a summary with no paired task has nothing to resample; want an error, got nil")
	}
}

// The law: the figures are reported, never gating. A criterion's verdict
// is byte-identical whether or not the reliability figures were computed,
// whatever they say, and whether or not the criterion carries a
// reliability block.
func TestReliabilityNeverGates(t *testing.T) {
	// T5 is the arm's only loss; the CI on five tasks straddles zero — a
	// gate on the CI would refuse this run. H2-1's clauses pass it.
	p := contextbench.Pair(fiveTasks(), "fathom", "grep")
	r, err := contextbench.ComputeReliability(p, contextbench.ReliabilityConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if !(r.CISuccessDiff.Low < 0 && r.CISuccessDiff.High > 0) {
		t.Fatalf("precondition: the interval must straddle zero for this test to bite: %+v", r.CISuccessDiff)
	}
	bare := contextbench.Criterion{ID: "X", RequireStrictlyMoreSuccesses: true, MaxTokenRatio: 2.0, MinTrials: 3}
	withBlock := bare
	withBlock.Reliability = &contextbench.ReliabilityConfig{BootstrapSeed: 99}
	evBare := bare.Evaluate(p)
	evBlock := withBlock.Evaluate(p)
	if evBare.Verdict != contextbench.VerdictPass {
		t.Fatalf("precondition: H2-1-shaped clauses pass this run: %+v", evBare)
	}
	if !reflect.DeepEqual(evBare, evBlock) {
		t.Fatalf("a reliability block changed the verdict:\n%+v\n%+v", evBare, evBlock)
	}
	// And a row carrying the figures evaluates no differently: the
	// evaluation is computed from the summary alone.
	row := contextbench.NewLedgerRow("r", "t", "c", "corpus", "m", p, evBare)
	row.Reliability = &r
	if row.Evaluation != evBare {
		t.Error("attaching reliability figures to a row must not touch its evaluation")
	}
}

func TestLedgerRow_ReliabilityIsAdditiveAndOmittedWhenAbsent(t *testing.T) {
	// A row without figures serialises exactly as before PB-10.
	row := contextbench.NewLedgerRow("r", "t", "c", "corpus", "m", contextbench.PairedSummary{Tasks: 1}, contextbench.Evaluation{})
	raw, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"ci_success_diff", "wilcoxon_p", "a12", "pass_at_1", "pass_pow_3", "bootstrap_seed"} {
		if strings.Contains(string(raw), k) {
			t.Errorf("absent figures must be omitted, found %q in %s", k, raw)
		}
	}
	// A row with them round-trips through the ledger.
	p := contextbench.Pair(fiveTasks(), "fathom", "grep")
	r, err := contextbench.ComputeReliability(p, contextbench.ReliabilityConfig{})
	if err != nil {
		t.Fatal(err)
	}
	row.Reliability = &r
	raw, err = json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	var back contextbench.LedgerRow
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back.Reliability, &r) {
		t.Fatalf("round trip lost figures:\n%+v\n%+v", back.Reliability, r)
	}
	for _, k := range []string{`"ci_success_diff":{`, `"wilcoxon_p":`, `"a12":`, `"pass_at_1":{`, `"pass_pow_3":{`, `"bootstrap_seed":20260902`} {
		if !strings.Contains(string(raw), k) {
			t.Errorf("want %s in the row: %s", k, raw)
		}
	}
}

func TestRenderLedgerMarkdown_ShowsTheCIOrNotRecorded(t *testing.T) {
	p := contextbench.Pair(fiveTasks(), "fathom", "grep")
	r, err := contextbench.ComputeReliability(p, contextbench.ReliabilityConfig{})
	if err != nil {
		t.Fatal(err)
	}
	with := row("r-with", contextbench.VerdictPass)
	with.Reliability = &r
	md := contextbench.RenderLedgerMarkdown([]contextbench.LedgerRow{row("r-bare", contextbench.VerdictFail), with})
	lines := strings.Split(strings.TrimSpace(md), "\n")
	if !strings.Contains(lines[0], "| Δsuccess 95% CI |") {
		t.Fatalf("header lacks the CI column: %s", lines[0])
	}
	if !strings.Contains(lines[2], "| n/r |") {
		t.Errorf("a row without figures shows n/r, never a number: %s", lines[2])
	}
	if !strings.Contains(lines[3], "+0.133 [") || !strings.Contains(lines[3], "] p=1.000 A12=0.58") {
		t.Errorf("the row with figures shows point, interval, p and A12: %s", lines[3])
	}
}

func TestLoadCriterion_ReadsTheReliabilityBlock(t *testing.T) {
	// The public corpus (PB-14) registers its seed under
	// machine_criterion.reliability.bootstrap_seed; the golden derivation
	// fixture is the committed example.
	c, err := contextbench.LoadCriterionByID("publicloc/testdata/corpus.golden.json", "H2-2")
	if err != nil {
		t.Fatal(err)
	}
	if c.Reliability == nil || c.Reliability.BootstrapSeed != 20260902 {
		t.Fatalf("H2-2's registered bootstrap seed must load: %+v", c.Reliability)
	}
}
