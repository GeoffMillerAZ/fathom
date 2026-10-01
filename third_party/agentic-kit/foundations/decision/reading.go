package decision

import (
	"errors"
	"fmt"
	"math"
	"sort"
)

// ErrInvalidSample is wrapped by the error NewReading returns when a
// LabeledSample is refused: a Confidence outside [0, 1] (NaN and the
// infinities included) or a Provenance that is not one of the three
// declared classes. The message names the index of the first offending
// sample; test with errors.Is.
var ErrInvalidSample = errors.New("decision: invalid labeled sample")

// Provenance records where a label came from: who or what decided that an
// answer was right or wrong. A reading reports the mix because a frontier
// model's labels must never be the only truth when the arm being compared
// is itself a frontier model.
type Provenance string

// The three label provenance classes. NewReading refuses any other value,
// the empty string included, so every label states where it came from.
const (
	// ProvenanceHuman is a label a person adjudicated.
	ProvenanceHuman Provenance = "human"
	// ProvenanceFrontier is a label a frontier model produced.
	ProvenanceFrontier Provenance = "frontier"
	// ProvenanceComputed is a label derived by a program from ground truth,
	// with no judgment involved.
	ProvenanceComputed Provenance = "computed"
)

// ReadingKey identifies what a Reading is a reading OF: calibration is per
// (site, template version, provider, resolved model), and nothing about a
// reading survives a change to any of them.
type ReadingKey struct {
	// Site names the call site whose answers were graded.
	Site string `json:"site"`
	// TemplateVersion is the version of the question template the answers
	// were produced from.
	TemplateVersion string `json:"template_version"`
	// Provider names the Decider that answered (Response.Provider).
	Provider string `json:"provider"`
	// ResolvedModel is the model ID the provider actually resolved, never a
	// moving alias (Response.Model).
	ResolvedModel string `json:"resolved_model"`
}

// LabeledSample is one graded answer: the Confidence the answer carried,
// whether it turned out Correct, and the Provenance of that verdict.
type LabeledSample struct {
	// Confidence is the answer's Confidence, in [0, 1].
	Confidence float64 `json:"confidence"`
	// Correct is whether the answer was right.
	Correct bool `json:"correct"`
	// Provenance is where the Correct verdict came from.
	Provenance Provenance `json:"provenance"`
}

// ProvenanceMix counts labels per Provenance class.
type ProvenanceMix struct {
	// Human is the number of ProvenanceHuman labels.
	Human int `json:"human"`
	// Frontier is the number of ProvenanceFrontier labels.
	Frontier int `json:"frontier"`
	// Computed is the number of ProvenanceComputed labels.
	Computed int `json:"computed"`
}

// BandReading is the reading of one confidence Band: how many samples fell
// in it, how many of those were right, and the Wilson 95% interval of that
// proportion.
//
// A Band with no samples is still reported, with N == 0 and both bounds nil:
// there is no interval to state, and the absence is explicit rather than a
// zero that could be mistaken for one. The per-band accuracy is
// Correct / N.
type BandReading struct {
	// Band is the confidence band, classified by BandOf.
	Band Band `json:"band"`
	// N is the number of samples in the band.
	N int `json:"n"`
	// Correct is the number of those samples that were right.
	Correct int `json:"correct"`
	// WilsonLo is the lower bound of the Wilson 95% interval of
	// Correct / N; nil when N == 0.
	WilsonLo *float64 `json:"wilson_lo"`
	// WilsonHi is the upper bound of the Wilson 95% interval of
	// Correct / N; nil when N == 0.
	WilsonHi *float64 `json:"wilson_hi"`
}

// Reading is the calibration reading of one ReadingKey: the statistics the
// method in docs/fast-judgment.md asks for before a confidence is trusted,
// computed from a set of LabeledSamples. It is a plain value; its JSON form
// follows the calibration record template's reading block, with null for a
// statistic that cannot be stated yet.
//
// Every pointer field is nil, never zero, when its statistic does not exist
// (no samples, or a class or band missing): a Brier score, an AUROC of 0.5
// or an accuracy of 0 would each be a claim the data cannot support.
type Reading struct {
	// Key is the (site, template version, provider, resolved model) the
	// reading is of.
	Key ReadingKey `json:"key"`
	// N is the number of samples.
	N int `json:"n"`
	// Correct is the number of samples that were right.
	Correct int `json:"correct"`
	// Accuracy is Correct / N; nil when N == 0.
	Accuracy *float64 `json:"accuracy"`
	// AccuracyLo is the lower bound of the Wilson 95% interval of Accuracy;
	// nil when N == 0.
	AccuracyLo *float64 `json:"accuracy_wilson_lo"`
	// AccuracyHi is the upper bound of the Wilson 95% interval of Accuracy;
	// nil when N == 0.
	AccuracyHi *float64 `json:"accuracy_wilson_hi"`
	// ByBand is the per-band split, always three entries in the fixed order
	// BandUnsure, BandConfirm, BandProceed.
	ByBand []BandReading `json:"by_band"`
	// Monotonic is whether accuracy is non-decreasing across the bands that
	// have data, taken in ByBand order and compared exactly. It is nil (a
	// third state, neither true nor false) when fewer than two bands have
	// data, because one point has no direction. When it is false no
	// confidence threshold can repair the site.
	Monotonic *bool `json:"monotonic"`
	// Brier is the Brier score: the mean of (confidence - correct)^2, with
	// correct as 1 or 0. Lower is better; nil when N == 0.
	Brier *float64 `json:"brier"`
	// AUROC is the area under the ROC curve of Confidence separating right
	// answers from wrong ones: the probability that a randomly chosen right
	// answer carried a higher confidence than a randomly chosen wrong one,
	// ties counting one half (the Mann-Whitney statistic over n1*n0 pairs).
	// 1 is perfect separation, 0.5 no better than chance. It is nil when
	// either class is empty; 0.5 would be an invented value.
	AUROC *float64 `json:"auroc"`
	// Labels is the provenance mix of the samples.
	Labels ProvenanceMix `json:"labels"`
}

// wilsonZ95 is the standard normal quantile for a two-sided 95% interval.
const wilsonZ95 = 1.959963984540054

// bandOrder is the fixed order of Reading.ByBand.
var bandOrder = [3]Band{BandUnsure, BandConfirm, BandProceed}

// bandIndex returns the position of b in bandOrder.
func bandIndex(b Band) int {
	switch b {
	case BandUnsure:
		return 0
	case BandConfirm:
		return 1
	default:
		return 2
	}
}

// NewReading computes the Reading of samples for key. It is pure compute:
// no I/O, no clock, no table, no cache, and it neither modifies nor retains
// samples, so it is safe to call concurrently.
//
// The result does not depend on the order of samples, down to the last bit
// (counts are exact integers; the Brier terms are summed in sorted order),
// and it never iterates a map, so equal inputs give equal, deterministic
// output.
//
// No samples is not an error: it is a whole Reading with N == 0, three empty
// bands and every statistic nil.
//
// A sample with a Confidence outside [0, 1] (NaN and the infinities
// included) or a Provenance that is not ProvenanceHuman, ProvenanceFrontier
// or ProvenanceComputed is refused: NewReading returns the zero Reading and
// an error wrapping ErrInvalidSample that names the index of the first such
// sample. Nothing is clamped, defaulted or skipped: a calibration set with
// a bad row is a bad calibration set.
func NewReading(key ReadingKey, samples []LabeledSample) (Reading, error) {
	for i, s := range samples {
		if err := validateSample(i, s); err != nil {
			return Reading{}, err
		}
	}

	r := Reading{Key: key, N: len(samples), ByBand: make([]BandReading, len(bandOrder))}
	for i, b := range bandOrder {
		r.ByBand[i].Band = b
	}
	terms := make([]float64, 0, len(samples))
	var right, wrong []float64
	for _, s := range samples {
		band := &r.ByBand[bandIndex(BandOf(s.Confidence))]
		band.N++
		y := 0.0
		if s.Correct {
			y = 1
			r.Correct++
			band.Correct++
			right = append(right, s.Confidence)
		} else {
			wrong = append(wrong, s.Confidence)
		}
		terms = append(terms, (s.Confidence-y)*(s.Confidence-y))
		switch s.Provenance {
		case ProvenanceHuman:
			r.Labels.Human++
		case ProvenanceFrontier:
			r.Labels.Frontier++
		case ProvenanceComputed:
			r.Labels.Computed++
		}
	}

	if r.N > 0 {
		r.Accuracy = ptr(float64(r.Correct) / float64(r.N))
		lo, hi := wilson95(r.Correct, r.N)
		r.AccuracyLo, r.AccuracyHi = ptr(lo), ptr(hi)
		r.Brier = ptr(meanSorted(terms))
	}
	for i := range r.ByBand {
		if b := &r.ByBand[i]; b.N > 0 {
			lo, hi := wilson95(b.Correct, b.N)
			b.WilsonLo, b.WilsonHi = ptr(lo), ptr(hi)
		}
	}
	r.Monotonic = monotonic(r.ByBand)
	if a, ok := auroc(right, wrong); ok {
		r.AUROC = ptr(a)
	}
	return r, nil
}

// RecalibrationDue reports whether current names a different provider,
// resolved model or template version than the reading was made for, in which
// case nothing the reading says holds any more and the site must be
// re-graded.
//
// A Site mismatch is a caller error: a reading of one site says nothing
// about another, so RecalibrationDue answers true (fail closed, the safe
// answer to "may I rely on this reading?") rather than panicking or ignoring
// the field. Key readings by site and the case never arises.
func (r Reading) RecalibrationDue(current ReadingKey) bool {
	k := r.Key
	return current.Site != k.Site ||
		current.Provider != k.Provider ||
		current.ResolvedModel != k.ResolvedModel ||
		current.TemplateVersion != k.TemplateVersion
}

// validateSample refuses a sample outside the contract, naming its index.
func validateSample(i int, s LabeledSample) error {
	switch {
	case math.IsNaN(s.Confidence):
		return fmt.Errorf("%w: sample %d: confidence is NaN, want a number in [0, 1]", ErrInvalidSample, i)
	case s.Confidence < 0 || s.Confidence > 1: // also catches the infinities
		return fmt.Errorf("%w: sample %d: confidence %v is outside [0, 1]", ErrInvalidSample, i, s.Confidence)
	}
	switch s.Provenance {
	case ProvenanceHuman, ProvenanceFrontier, ProvenanceComputed:
		return nil
	}
	return fmt.Errorf("%w: sample %d: unknown provenance %q, want %q, %q or %q",
		ErrInvalidSample, i, string(s.Provenance), ProvenanceHuman, ProvenanceFrontier, ProvenanceComputed)
}

// wilson95 returns the Wilson score interval, at 95% confidence, of correct
// successes in n trials (n > 0):
//
//	(2k + z^2 -/+ z*sqrt(z^2 + 4k(n-k)/n)) / (2(n + z^2))
//
// It stays inside [0, 1] by construction: the bound that is exactly 0 (all
// wrong) or exactly 1 (all right) is returned exactly rather than left to
// rounding. Unlike the normal approximation it is meaningful at small n and
// at the edges, which is why the calibration bars are stated in it: 38 of 41
// is a lower bound of 0.806, not 0.9.
func wilson95(correct, n int) (lo, hi float64) {
	k, nn := float64(correct), float64(n)
	z2 := wilsonZ95 * wilsonZ95
	half := wilsonZ95 * math.Sqrt(z2+4*k*(nn-k)/nn)
	den := 2 * (nn + z2)
	lo, hi = clamp01((2*k+z2-half)/den), clamp01((2*k+z2+half)/den)
	if correct == 0 {
		lo = 0
	}
	if correct == n {
		hi = 1
	}
	return lo, hi
}

// monotonic reports whether accuracy is non-decreasing across the bands
// that have data, or nil when fewer than two bands do. Accuracies are
// compared as exact fractions (cross-multiplied integers), so equal
// proportions at different n never disagree through rounding.
func monotonic(bands []BandReading) *bool {
	var have int
	var prevN, prevCorrect int
	ok := true
	for _, b := range bands {
		if b.N == 0 {
			continue
		}
		if have > 0 && prevCorrect*b.N > b.Correct*prevN {
			ok = false
		}
		have++
		prevN, prevCorrect = b.N, b.Correct
	}
	if have < 2 {
		return nil
	}
	return &ok
}

// meanSorted returns the mean of xs, summing in ascending order so the
// result depends only on the multiset of values, never on their order. xs
// is sorted in place and must not be empty.
func meanSorted(xs []float64) float64 {
	sort.Float64s(xs)
	var sum float64
	for _, x := range xs {
		sum += x
	}
	return sum / float64(len(xs))
}

// auroc returns the area under the ROC curve of confidence separating the
// right answers (right) from the wrong ones (wrong): the Mann-Whitney
// statistic, the fraction of (right, wrong) pairs in which the right answer
// has the higher confidence, a tie counting one half. The boolean is false
// when either class is empty, where the statistic does not exist.
//
// Worked by hand: right {0.9, 0.6}, wrong {0.6, 0.3}. The four pairs are
// (0.9, 0.6) win, (0.9, 0.3) win, (0.6, 0.6) tie, (0.6, 0.3) win, so
// (3 + 1/2) / 4 = 0.875. Perfect separation gives 1; all confidences equal
// gives every pair a tie and so exactly 0.5.
//
// The count runs in O(n log n) over sorted copies rather than over every
// pair, on integers doubled to keep the half-points exact. It sorts right
// and wrong in place.
func auroc(right, wrong []float64) (float64, bool) {
	if len(right) == 0 || len(wrong) == 0 {
		return 0, false
	}
	sort.Float64s(right)
	sort.Float64s(wrong)
	var twice int64 // twice the number of wins, ties adding one each
	below := 0      // wrong[:below] are strictly below the current value
	for i := 0; i < len(right); {
		v := right[i]
		j := i
		for j < len(right) && right[j] == v {
			j++
		}
		for below < len(wrong) && wrong[below] < v {
			below++
		}
		equal := below
		for equal < len(wrong) && wrong[equal] == v {
			equal++
		}
		twice += int64(j-i) * (2*int64(below) + int64(equal-below))
		i = j
	}
	return float64(twice) / float64(2*int64(len(right))*int64(len(wrong))), true
}

// ptr returns a pointer to a copy of v.
func ptr(v float64) *float64 { return &v }
