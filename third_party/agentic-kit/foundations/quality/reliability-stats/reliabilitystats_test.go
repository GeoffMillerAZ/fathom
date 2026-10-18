package reliabilitystats_test

import (
	"errors"
	"math"
	"math/big"
	"math/rand"
	"sort"
	"testing"

	rs "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/reliability-stats"
)

func near(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

// ---- pass@k (Chen et al. 2021) ----

// TestPassAtK_ChenEtAl2021_KnownAnswers pins hand-computable values of the
// unbiased estimator 1 − C(n−c,k)/C(n,k) from the Codex paper's definition.
func TestPassAtK_ChenEtAl2021_KnownAnswers(t *testing.T) {
	cases := []struct {
		n, c, k int
		want    float64
	}{
		{1, 1, 1, 1},
		{1, 0, 1, 0},
		{10, 5, 1, 0.5},         // pass@1 is the plain pass rate
		{10, 0, 5, 0},           // nothing passed
		{5, 3, 3, 1},            // only 2 failures: a 3-sample must hit a pass
		{10, 3, 2, 24.0 / 45.0}, // 1 − C(7,2)/C(10,2) = 1 − 21/45
		{200, 20, 100, 0},       // 1 − C(180,100)/C(200,100): overwritten below from the exact oracle
		{100, 100, 100, 1},
	}
	// Recompute the last non-trivial row exactly rather than trust a hand value.
	cases[6].want = exactPassAtK(200, 20, 100)
	for _, c := range cases {
		got := rs.PassAtK(c.n, c.c, c.k)
		if !near(got, c.want, 1e-12) {
			t.Errorf("PassAtK(%d,%d,%d) = %.15g, want %.15g", c.n, c.c, c.k, got, c.want)
		}
	}
}

// exactPassAtK is the combinatorial definition evaluated in exact rational
// arithmetic — the oracle the product form must agree with.
func exactPassAtK(n, c, k int) float64 {
	if n-c < k {
		return 1
	}
	num := new(big.Int).Binomial(int64(n-c), int64(k))
	den := new(big.Int).Binomial(int64(n), int64(k))
	r := new(big.Rat).SetFrac(num, den)
	f, _ := new(big.Rat).Sub(big.NewRat(1, 1), r).Float64()
	return f
}

// TestPassAtK_MatchesExactBinomialDefinition sweeps every (n, c, k) with
// n <= 30 against the big.Rat oracle.
func TestPassAtK_MatchesExactBinomialDefinition(t *testing.T) {
	for n := 1; n <= 30; n++ {
		for c := 0; c <= n; c++ {
			for k := 1; k <= n; k++ {
				got, want := rs.PassAtK(n, c, k), exactPassAtK(n, c, k)
				if !near(got, want, 1e-12) {
					t.Fatalf("PassAtK(%d,%d,%d) = %.15g, want %.15g", n, c, k, got, want)
				}
			}
		}
	}
}

func TestPassAtK_UndefinedInputsAreNaN(t *testing.T) {
	for _, c := range [][3]int{{0, 0, 1}, {5, 2, 0}, {5, 2, 6}, {5, -1, 1}, {5, 6, 1}} {
		if got := rs.PassAtK(c[0], c[1], c[2]); !math.IsNaN(got) {
			t.Errorf("PassAtK(%v) = %v, want NaN", c, got)
		}
	}
}

// ---- pass^k (τ-bench) ----

// TestPassPowK_TauBench_KnownAnswer pins the plan's stated answer,
// PassPowK(0.9, 8) = 0.4305 to four places, plus the edges.
func TestPassPowK_TauBench_KnownAnswer(t *testing.T) {
	if got := rs.PassPowK(0.9, 8); math.Round(got*1e4)/1e4 != 0.4305 {
		t.Errorf("PassPowK(0.9, 8) = %.6f, want 0.4305 to 4 dp", got)
	}
	cases := []struct {
		p    float64
		k    int
		want float64
	}{
		{1, 100, 1}, {0, 3, 0}, {0.5, 1, 0.5}, {0.5, 2, 0.25}, {0.7, 0, 1}, {0, 0, 1},
	}
	for _, c := range cases {
		if got := rs.PassPowK(c.p, c.k); !near(got, c.want, 1e-12) {
			t.Errorf("PassPowK(%v, %d) = %v, want %v", c.p, c.k, got, c.want)
		}
	}
	for _, c := range []struct {
		p float64
		k int
	}{{-0.1, 2}, {1.1, 2}, {0.5, -1}, {math.NaN(), 1}} {
		if got := rs.PassPowK(c.p, c.k); !math.IsNaN(got) {
			t.Errorf("PassPowK(%v, %d) = %v, want NaN", c.p, c.k, got)
		}
	}
}

// ---- paired, task-clustered bootstrap ----

var bootstrapSample = []rs.Observation{
	{Task: "t1", Diff: 0.10},
	{Task: "t1", Diff: 0.20},
	{Task: "t1", Diff: 0.15},
	{Task: "t2", Diff: -0.05},
	{Task: "t2", Diff: 0.00},
	{Task: "t2", Diff: 0.05},
	{Task: "t3", Diff: 0.30},
	{Task: "t3", Diff: 0.25},
	{Task: "t3", Diff: 0.35},
	{Task: "t4", Diff: 0.02},
	{Task: "t4", Diff: -0.02},
	{Task: "t4", Diff: 0.04},
	{Task: "t5", Diff: 0.12},
	{Task: "t5", Diff: 0.08},
	{Task: "t5", Diff: 0.10},
	{Task: "t6", Diff: -0.20},
	{Task: "t6", Diff: -0.10},
	{Task: "t6", Diff: -0.15},
}

// TestPairedBootstrapCI_GoldenSeed pins the exact bytes for two seeds:
// the interval is a function of (sample, seed, resamples, level) and
// nothing else. If this test moves, math/rand's sequence moved or the
// quantile rule changed — either is a contract change. The bytes are the
// same on amd64 and arm64: quantile rounds each product explicitly.
func TestPairedBootstrapCI_GoldenSeed(t *testing.T) {
	golden := []struct {
		seed          int64
		low, high, pt float64
	}{
		{42, -0.03354166666666648, 0.18555555555555556, 0.068888888888888875}, // the unfused value: arm64 once fused it 1 ulp lower
		{7, -0.041708333333333292, 0.19166666666666665, 0.068888888888888875},
	}
	for _, g := range golden {
		for run := 0; run < 2; run++ { // twice: same seed, same bytes
			iv, err := rs.PairedBootstrapCI(bootstrapSample, g.seed, 1000, 0.95)
			if err != nil {
				t.Fatalf("seed %d: %v", g.seed, err)
			}
			if iv.Low != g.low || iv.High != g.high || iv.Point != g.pt || iv.Tasks != 6 {
				t.Errorf("seed %d run %d: got %+v, want low=%.17g high=%.17g point=%.17g tasks=6",
					g.seed, run, iv, g.low, g.high, g.pt)
			}
		}
	}
}

// TestPairedBootstrapCI_ResamplesTasksNotTrials: one task with 100 trials
// at +10 and three tasks at −1. Trial-level resampling would give a tight
// interval near +9.7; task-level weighs the big task as one of four, so
// the point is 1.75 and the interval reaches −1 (all four draws land on
// a −1 task) — the pseudo-replication the clustering exists to refuse.
func TestPairedBootstrapCI_ResamplesTasksNotTrials(t *testing.T) {
	var obs []rs.Observation
	for i := 0; i < 100; i++ {
		obs = append(obs, rs.Observation{Task: "big", Diff: 10})
	}
	obs = append(obs, rs.Observation{Task: "a", Diff: -1}, rs.Observation{Task: "b", Diff: -1}, rs.Observation{Task: "c", Diff: -1})
	iv, err := rs.PairedBootstrapCI(obs, 1, 2000, 0.95)
	if err != nil {
		t.Fatal(err)
	}
	if iv.Tasks != 4 || iv.Point != 1.75 {
		t.Errorf("point/tasks: %+v", iv)
	}
	if iv.Low != -1 {
		t.Errorf("Low = %v, want -1 (a replicate of four −1 tasks must be reachable)", iv.Low)
	}
	if iv.High > 10 || iv.High < 1.75 {
		t.Errorf("High = %v out of the task-mean range", iv.High)
	}
}

func TestPairedBootstrapCI_InputOrderIndependent(t *testing.T) {
	want, err := rs.PairedBootstrapCI(bootstrapSample, 99, 500, 0.9)
	if err != nil {
		t.Fatal(err)
	}
	shuffled := append([]rs.Observation(nil), bootstrapSample...)
	rand.New(rand.NewSource(3)).Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
	got, err := rs.PairedBootstrapCI(shuffled, 99, 500, 0.9)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("shuffled input changed the interval: %+v vs %+v", got, want)
	}
}

func TestPairedBootstrapCI_SingleTaskCollapsesToPoint(t *testing.T) {
	iv, err := rs.PairedBootstrapCI([]rs.Observation{{Task: "only", Diff: 0.3}, {Task: "only", Diff: 0.5}}, 5, 100, 0.95)
	if err != nil {
		t.Fatal(err)
	}
	if iv.Low != 0.4 || iv.High != 0.4 || iv.Point != 0.4 || iv.Tasks != 1 {
		t.Errorf("single task must collapse to its mean: %+v", iv)
	}
}

func TestPairedBootstrapCI_Errors(t *testing.T) {
	ok := []rs.Observation{{Task: "a", Diff: 1}}
	cases := []struct {
		name      string
		obs       []rs.Observation
		resamples int
		level     float64
		want      error
	}{
		{"empty", nil, 100, 0.95, rs.ErrEmptySample},
		{"zero resamples", ok, 0, 0.95, rs.ErrInvalidResamples},
		{"level 0", ok, 100, 0, rs.ErrInvalidLevel},
		{"level 1", ok, 100, 1, rs.ErrInvalidLevel},
		{"level NaN", ok, 100, math.NaN(), rs.ErrInvalidLevel},
		{"NaN diff", []rs.Observation{{Task: "a", Diff: math.NaN()}}, 100, 0.95, rs.ErrInvalidSample},
	}
	for _, c := range cases {
		_, err := rs.PairedBootstrapCI(c.obs, 1, c.resamples, c.level)
		if !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", c.name, err, c.want)
		}
	}
}

// ---- Wilcoxon signed-rank ----

// TestWilcoxon_Wilcoxon1945_ExactTableN10: with ranks 1..10 and the single
// negative difference at rank 8, T = 8 and the exact two-sided p is
// 2·25/1024 = 0.048828125 — the published critical value for n = 10 at
// α = 0.05 is T <= 8, and T = 9 (2·33/1024 = 0.064453125) must not reach it.
func TestWilcoxon_Wilcoxon1945_ExactTableN10(t *testing.T) {
	mk := func(negAt int) ([]float64, []float64) {
		x := make([]float64, 10)
		y := make([]float64, 10)
		for i := range x {
			x[i] = float64(i + 1)
			if i+1 == negAt {
				x[i] = -x[i]
			}
		}
		return x, y
	}
	x, y := mk(8)
	r, err := rs.Wilcoxon(x, y)
	if err != nil {
		t.Fatal(err)
	}
	if r.W != 8 || r.WPlus != 47 || r.WMinus != 8 || r.N != 10 || r.Zeros != 0 || !r.Exact {
		t.Errorf("stats: %+v", r)
	}
	if r.P != 0.048828125 {
		t.Errorf("P(T=8, n=10) = %.12g, want 0.048828125", r.P)
	}
	x, y = mk(9)
	r, _ = rs.Wilcoxon(x, y)
	if r.P != 0.064453125 {
		t.Errorf("P(T=9, n=10) = %.12g, want 0.064453125", r.P)
	}
}

// TestWilcoxon_ExactTableN25: the published two-sided α = 0.05 critical
// value for n = 25 is T <= 89; T = 90 must exceed 0.05.
func TestWilcoxon_ExactTableN25(t *testing.T) {
	// ranks 1..25; negatives at ranks summing to w.
	mk := func(negRanks ...int) ([]float64, []float64) {
		x := make([]float64, 25)
		y := make([]float64, 25)
		neg := map[int]bool{}
		for _, r := range negRanks {
			neg[r] = true
		}
		for i := range x {
			x[i] = float64(i + 1)
			if neg[i+1] {
				x[i] = -x[i]
			}
		}
		return x, y
	}
	r89, _ := rs.Wilcoxon(mk(25, 24, 23, 17)) // 25+24+23+17 = 89
	r90, _ := rs.Wilcoxon(mk(25, 24, 23, 18)) // 90
	if r89.W != 89 || r90.W != 90 || !r89.Exact {
		t.Fatalf("stats: %+v / %+v", r89, r90)
	}
	if r89.P > 0.05 || r90.P <= 0.05 {
		t.Errorf("P(89) = %.6f (want <= 0.05), P(90) = %.6f (want > 0.05)", r89.P, r90.P)
	}
}

// TestWilcoxon_AllOneSign_SmallN: the coarse floor stated in the package
// doc — n = 5 all positive gives exactly 2/32; n = 6 gives 2/64.
func TestWilcoxon_AllOneSign_SmallN(t *testing.T) {
	for n, want := range map[int]float64{5: 0.0625, 6: 0.03125} {
		x := make([]float64, n)
		for i := range x {
			x[i] = float64(i + 1)
		}
		r, err := rs.Wilcoxon(x, make([]float64, n))
		if err != nil {
			t.Fatal(err)
		}
		if r.P != want || r.W != 0 {
			t.Errorf("n=%d: %+v, want P=%v W=0", n, r, want)
		}
	}
}

// TestWilcoxon_TiesAverageRanksZerosDropped uses the textbook ten-pair
// example (one zero difference, two tied |d| = 5): after dropping the
// zero n = 9, the tied pair takes ranks 1.5/1.5, W+ = 27, W− = 18.
func TestWilcoxon_TiesAverageRanksZerosDropped(t *testing.T) {
	x := []float64{125, 115, 130, 140, 140, 115, 140, 125, 140, 135}
	y := []float64{110, 122, 125, 120, 140, 124, 123, 137, 135, 145}
	r, err := rs.Wilcoxon(x, y)
	if err != nil {
		t.Fatal(err)
	}
	if r.N != 9 || r.Zeros != 1 || r.WPlus != 27 || r.WMinus != 18 || r.W != 18 || !r.Exact {
		t.Errorf("stats: %+v", r)
	}
	if want := bruteForceWilcoxonP(x, y); r.P != want {
		t.Errorf("P = %.12g, want brute-force %.12g", r.P, want)
	}
}

// bruteForceWilcoxonP is an independent oracle: enumerate every sign
// assignment of the observed tie-adjusted ranks and count rank sums at or
// below min(W+, W−).
func bruteForceWilcoxonP(x, y []float64) float64 {
	type d struct {
		abs float64
		pos bool
	}
	var ds []d
	for i := range x {
		v := x[i] - y[i]
		if v > 0 {
			ds = append(ds, d{v, true})
		} else if v < 0 {
			ds = append(ds, d{-v, false})
		}
	}
	sort.SliceStable(ds, func(i, j int) bool { return ds[i].abs < ds[j].abs })
	ranks := make([]float64, len(ds))
	for i := 0; i < len(ds); {
		j := i
		for j < len(ds) && ds[j].abs == ds[i].abs {
			j++
		}
		for k := i; k < j; k++ {
			ranks[k] = float64(i+1+j) / 2
		}
		i = j
	}
	var wp, wm float64
	for i, v := range ds {
		if v.pos {
			wp += ranks[i]
		} else {
			wm += ranks[i]
		}
	}
	w := math.Min(wp, wm)
	n := len(ranks)
	count := 0
	for mask := 0; mask < 1<<n; mask++ {
		var s float64
		for i := 0; i < n; i++ {
			if mask&(1<<i) != 0 {
				s += ranks[i]
			}
		}
		if s <= w {
			count++
		}
	}
	p := 2 * float64(count) / float64(int(1)<<n)
	if p > 1 {
		p = 1
	}
	return p
}

// TestWilcoxon_BruteForceAgreement compares the subset-sum recurrence
// against full enumeration on fixed samples with ties and mixed signs.
func TestWilcoxon_BruteForceAgreement(t *testing.T) {
	samples := [][]float64{
		{1.5, -2, 2, 3.5, -3.5, 4, 0.5},
		{0.1, 0.1, 0.1, -0.2, 0.3, -0.3, 0.4, 0.5, -0.05, 0.6, 0.7},
		{-1, -2, -3, 4, 5, 6, 7, 8, 9, 10, -11, 12},
		{2, 2, 2, 2, -2, -2, 3, -3},
	}
	for i, diffs := range samples {
		r, err := rs.Wilcoxon(diffs, make([]float64, len(diffs)))
		if err != nil {
			t.Fatalf("sample %d: %v", i, err)
		}
		if want := bruteForceWilcoxonP(diffs, make([]float64, len(diffs))); r.P != want {
			t.Errorf("sample %d: P = %.12g, want %.12g", i, r.P, want)
		}
		if r.WPlus+r.WMinus != float64(r.N*(r.N+1))/2 {
			t.Errorf("sample %d: rank sums do not partition n(n+1)/2: %+v", i, r)
		}
	}
}

// TestWilcoxon_NormalApproximationAboveExactLimit: n = 26 switches to the
// approximation. All-positive gives a tiny p; a perfectly balanced sample
// (W+ = W−) gives p = 1, and the approximation is continuous with the
// exact branch at the boundary.
func TestWilcoxon_NormalApproximationAboveExactLimit(t *testing.T) {
	x := make([]float64, 26)
	for i := range x {
		x[i] = float64(i + 1)
	}
	r, err := rs.Wilcoxon(x, make([]float64, 26))
	if err != nil {
		t.Fatal(err)
	}
	if r.Exact || r.W != 0 || r.P > 1e-4 {
		t.Errorf("all-positive n=26: %+v", r)
	}

	bal := make([]float64, 26)
	for i := range bal {
		bal[i] = float64(i + 1)
		if i%2 == 1 {
			bal[i] = -bal[i]
		}
	}
	// Ranks 1..26; odd ranks positive (sum 169), even negative (sum 182).
	r, _ = rs.Wilcoxon(bal, make([]float64, 26))
	if r.Exact || r.WPlus != 169 || r.WMinus != 182 || r.P < 0.5 {
		t.Errorf("near-balanced n=26: %+v", r)
	}
	sym := []float64{1, -1, 2, -2, 3, -3, 4, -4, 5, -5, 6, -6, 7, -7, 8, -8, 9, -9, 10, -10, 11, -11, 12, -12, 13, -13}
	r, _ = rs.Wilcoxon(sym, make([]float64, 26))
	if r.P != 1 || r.WPlus != r.WMinus {
		t.Errorf("symmetric n=26 must give p = 1: %+v", r)
	}
}

func TestWilcoxon_Errors(t *testing.T) {
	if _, err := rs.Wilcoxon([]float64{1, 2}, []float64{1}); !errors.Is(err, rs.ErrLengthMismatch) {
		t.Errorf("mismatch: %v", err)
	}
	if _, err := rs.Wilcoxon([]float64{1, 2, 3}, []float64{1, 2, 3}); !errors.Is(err, rs.ErrNoPairs) {
		t.Errorf("all zeros: %v", err)
	}
	if _, err := rs.Wilcoxon(nil, nil); !errors.Is(err, rs.ErrNoPairs) {
		t.Errorf("empty: %v", err)
	}
	if _, err := rs.Wilcoxon([]float64{math.NaN()}, []float64{0}); !errors.Is(err, rs.ErrInvalidSample) {
		t.Errorf("NaN: %v", err)
	}
}

// ---- A12 (Vargha & Delaney; Arcuri & Briand 2011) ----

// TestA12_ArcuriBriand2011_KnownAnswers pins the definition's landmarks:
// total dominance 1 and 0, identical samples 0.5, and the half-credit
// tie rule.
func TestA12_ArcuriBriand2011_KnownAnswers(t *testing.T) {
	cases := []struct {
		a, b []float64
		want float64
	}{
		{[]float64{4, 5, 6}, []float64{1, 2, 3}, 1},
		{[]float64{1, 2, 3}, []float64{4, 5, 6}, 0},
		{[]float64{1, 2, 3}, []float64{1, 2, 3}, 0.5},
		{[]float64{1, 2, 3}, []float64{2, 2, 2}, 0.5},            // 0 + 1.5 + 3 over 9
		{[]float64{2, 3, 4, 5}, []float64{1, 2, 3}, 10.0 / 12.0}, // ties at 2 and 3 count half
		{[]float64{7}, []float64{7}, 0.5},
	}
	for _, c := range cases {
		if got := rs.A12(c.a, c.b); !near(got, c.want, 1e-12) {
			t.Errorf("A12(%v, %v) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

// rankSumA12 is Arcuri & Briand's published formula
// A = (R1/m − (m+1)/2)/n with R1 the rank sum of the first sample under
// average ranks over the pooled data — the oracle the pairwise form must
// match.
func rankSumA12(a, b []float64) float64 {
	type v struct {
		x     float64
		first bool
	}
	pooled := make([]v, 0, len(a)+len(b))
	for _, x := range a {
		pooled = append(pooled, v{x, true})
	}
	for _, x := range b {
		pooled = append(pooled, v{x, false})
	}
	sort.SliceStable(pooled, func(i, j int) bool { return pooled[i].x < pooled[j].x })
	var r1 float64
	for i := 0; i < len(pooled); {
		j := i
		for j < len(pooled) && pooled[j].x == pooled[i].x {
			j++
		}
		avg := float64(i+1+j) / 2
		for k := i; k < j; k++ {
			if pooled[k].first {
				r1 += avg
			}
		}
		i = j
	}
	m, n := float64(len(a)), float64(len(b))
	return (r1/m - (m+1)/2) / n
}

func TestA12_MatchesRankSumForm(t *testing.T) {
	samples := []struct{ a, b []float64 }{
		{[]float64{0.9, 0.8, 0.95, 0.7}, []float64{0.6, 0.85, 0.5}},
		{[]float64{1, 1, 1, 2}, []float64{1, 2, 2}},
		{[]float64{3, 1, 4, 1, 5, 9, 2, 6}, []float64{5, 3, 5, 8, 9, 7}},
		{[]float64{0}, []float64{0, 0, 1}},
	}
	for _, s := range samples {
		got, want := rs.A12(s.a, s.b), rankSumA12(s.a, s.b)
		if !near(got, want, 1e-12) {
			t.Errorf("A12(%v, %v) = %v, rank-sum form %v", s.a, s.b, got, want)
		}
		if sum := rs.A12(s.a, s.b) + rs.A12(s.b, s.a); !near(sum, 1, 1e-12) {
			t.Errorf("A12 is not complementary: %v", sum)
		}
	}
}

func TestA12_EmptyIsNaN(t *testing.T) {
	if !math.IsNaN(rs.A12(nil, []float64{1})) || !math.IsNaN(rs.A12([]float64{1}, nil)) {
		t.Error("empty sample must be NaN, not a claim")
	}
}
