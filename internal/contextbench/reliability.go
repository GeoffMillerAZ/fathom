package contextbench

import (
	"errors"
	"fmt"
	"math"
	"sort"

	reliabilitystats "github.com/geoffmilleraz/agentic-kit/foundations/quality/reliability-stats"
)

// The reliability figures (PB-10; docs/vision/26-proof-credibility.md §2.2,
// ADR-031 §3). A paired verdict says "more successes at fewer tokens"; it
// does not say how much evidence stands behind that sentence. These
// figures do — and they are REPORTED, never gating. Criterion.Evaluate
// reads none of them (TestReliabilityNeverGates pins it): the
// pre-registered clauses stay the gate, because a criterion whose verdict
// depends on a resampling procedure is a criterion that can be re-rolled.
//
// Every primitive comes from agentic-kit's reliability-stats, whose
// non-guarantees apply verbatim: a percentile bootstrap, no
// multiple-comparison correction, coarse exact p at small n. The row
// carries the task count, the trial count and the seed beside every
// figure so a reader weighs it by the evidence, not the decimals.

// DefaultBootstrapSeed is the seed used when a criterion registers none.
// It is the date PB-10 registered the figures, so the number is a
// documented constant rather than a magic one; the public corpus's
// machine_criterion.reliability.bootstrap_seed pins the same value.
const DefaultBootstrapSeed int64 = 20260902

// DefaultBootstrapResamples is the replicate count when a criterion
// registers none.
const DefaultBootstrapResamples = 2000

// reliabilityLevel is the interval's confidence level. It is not
// configurable: the plan, the vision and the public criterion all say
// "95% CI", and a level a corpus could lower is a bar a corpus could
// lower.
const reliabilityLevel = 0.95

// ReliabilityConfig is the block a corpus may register under
// machine_criterion.reliability. Only the seed and the replicate count are
// read; the other keys the public corpus writes there (the primary
// statistic, the CI level, the task floor) are prose for the reader, and
// Evaluate reads none of this block.
type ReliabilityConfig struct {
	// BootstrapSeed seeds the task-clustered bootstrap; 0 means
	// DefaultBootstrapSeed.
	BootstrapSeed int64 `json:"bootstrap_seed,omitempty"`
	// Resamples is the bootstrap replicate count; 0 means
	// DefaultBootstrapResamples.
	Resamples int `json:"resamples,omitempty"`
}

// Reliability is the set of reported figures for one paired run. Every
// field is additive and omitempty on the ledger row; an absent figure
// reads as "not recorded", never as zero.
type Reliability struct {
	// CISuccessDiff is the seeded, task-clustered paired percentile
	// bootstrap 95% CI on the per-task success-rate difference, treatment
	// minus control. One observation per paired task (its rate difference),
	// resampled by task, so k trials of one task are one unit of evidence.
	CISuccessDiff *reliabilitystats.Interval `json:"ci_success_diff,omitempty"`
	// BootstrapSeed and Resamples are what produced CISuccessDiff, recorded
	// so the interval reproduces from the row alone.
	BootstrapSeed int64 `json:"bootstrap_seed,omitempty"`
	Resamples     int   `json:"resamples,omitempty"`

	// WilcoxonP is the two-sided signed-rank p-value on the per-task MEAN
	// TOKENS, treatment minus control. Tokens, not success differences,
	// because at n=3 most tasks tie on success (both arms 3/3 or 0/3) and
	// the test would drop nearly every pair — while tokens differ on almost
	// every task and are the quantity the criterion's second clause caps.
	// Absent when no pair differs (the test is undefined, not p=1).
	WilcoxonP *float64 `json:"wilcoxon_p,omitempty"`
	// WilcoxonN is the pair count after zero differences were dropped;
	// WilcoxonZeros the count dropped; WilcoxonExact whether P came from
	// the enumerated null (n <= 25) or the normal approximation.
	WilcoxonN     int  `json:"wilcoxon_n,omitempty"`
	WilcoxonZeros int  `json:"wilcoxon_zeros,omitempty"`
	WilcoxonExact bool `json:"wilcoxon_exact,omitempty"`

	// A12 is Vargha–Delaney's A over the per-task success rates: the
	// probability a treatment task's rate exceeds a control task's, ties
	// half; 0.5 is no effect.
	A12 *float64 `json:"a12,omitempty"`

	// PassAt1 and PassPow3 are per arm, each the mean over paired tasks of
	// the task's unbiased pass@1 (its success rate) and pass^3 (rate³ — the
	// probability three independent attempts all pass). Keyed by arm name;
	// the map marshals with sorted keys.
	PassAt1  map[string]float64 `json:"pass_at_1,omitempty"`
	PassPow3 map[string]float64 `json:"pass_pow_3,omitempty"`
}

// ComputeReliability derives the reported figures from a paired summary's
// per-task rows. It reads PerTask only — the same rows the verdict was
// computed over, after leak exclusion — so the figures and the verdict
// describe the same tasks. A summary with no paired task is an error:
// there is nothing to resample, and an empty interval would be a claim.
func ComputeReliability(p PairedSummary, cfg ReliabilityConfig) (Reliability, error) {
	if len(p.PerTask) == 0 {
		return Reliability{}, errors.New("reliability: no paired task — nothing to resample")
	}
	seed := cfg.BootstrapSeed
	if seed == 0 {
		seed = DefaultBootstrapSeed
	}
	resamples := cfg.Resamples
	if resamples == 0 {
		resamples = DefaultBootstrapResamples
	}

	rows := append([]PairedTask(nil), p.PerTask...)
	sort.Slice(rows, func(i, j int) bool { return rows[i].TaskID < rows[j].TaskID })

	obs := make([]reliabilitystats.Observation, 0, len(rows))
	tRates := make([]float64, 0, len(rows))
	cRates := make([]float64, 0, len(rows))
	tTokens := make([]float64, 0, len(rows))
	cTokens := make([]float64, 0, len(rows))
	var tPass1, cPass1, tPow3, cPow3 float64
	for _, r := range rows {
		if r.TreatmentTrials == 0 || r.ControlTrials == 0 {
			return Reliability{}, fmt.Errorf("reliability: task %s has a cell with no trials", r.TaskID)
		}
		tr := float64(r.TreatmentSuccesses) / float64(r.TreatmentTrials)
		cr := float64(r.ControlSuccesses) / float64(r.ControlTrials)
		obs = append(obs, reliabilitystats.Observation{Task: r.TaskID, Diff: tr - cr})
		tRates = append(tRates, tr)
		cRates = append(cRates, cr)
		tTokens = append(tTokens, r.TreatmentMeanTokens)
		cTokens = append(cTokens, r.ControlMeanTokens)
		tPass1 += reliabilitystats.PassAtK(r.TreatmentTrials, r.TreatmentSuccesses, 1)
		cPass1 += reliabilitystats.PassAtK(r.ControlTrials, r.ControlSuccesses, 1)
		tPow3 += reliabilitystats.PassPowK(tr, 3)
		cPow3 += reliabilitystats.PassPowK(cr, 3)
	}
	n := float64(len(rows))

	out := Reliability{BootstrapSeed: seed, Resamples: resamples}

	ci, err := reliabilitystats.PairedBootstrapCI(obs, seed, resamples, reliabilityLevel)
	if err != nil {
		return Reliability{}, fmt.Errorf("reliability: bootstrap: %w", err)
	}
	out.CISuccessDiff = &ci

	w, err := reliabilitystats.Wilcoxon(tTokens, cTokens)
	switch {
	case errors.Is(err, reliabilitystats.ErrNoPairs):
		// Undefined: every task tied on tokens. Report the zeros, no p.
		out.WilcoxonZeros = len(rows)
	case err != nil:
		return Reliability{}, fmt.Errorf("reliability: wilcoxon: %w", err)
	default:
		pv := w.P
		out.WilcoxonP = &pv
		out.WilcoxonN = w.N
		out.WilcoxonZeros = w.Zeros
		out.WilcoxonExact = w.Exact
	}

	if a := reliabilitystats.A12(tRates, cRates); !math.IsNaN(a) {
		out.A12 = &a
	}

	out.PassAt1 = map[string]float64{p.Treatment: tPass1 / n, p.Control: cPass1 / n}
	out.PassPow3 = map[string]float64{p.Treatment: tPow3 / n, p.Control: cPow3 / n}
	return out, nil
}

// renderReliability is the ledger table's one-cell summary of the
// figures: point and interval of the success difference, then p and A12.
// A row without figures renders "n/r" — not recorded — never a number.
func renderReliability(r *Reliability) string {
	if r == nil || r.CISuccessDiff == nil {
		return "n/r"
	}
	s := fmt.Sprintf("%+.3f [%+.3f, %+.3f]", r.CISuccessDiff.Point, r.CISuccessDiff.Low, r.CISuccessDiff.High)
	if r.WilcoxonP != nil {
		s += fmt.Sprintf(" p=%.3f", *r.WilcoxonP)
	}
	if r.A12 != nil {
		s += fmt.Sprintf(" A12=%.2f", *r.A12)
	}
	return s
}
