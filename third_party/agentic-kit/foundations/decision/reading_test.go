package decision_test

import (
	"encoding/json"
	"errors"
	"math"
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"

	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

// rows builds n samples at one confidence, the first `correct` of them
// correct, all human-labelled.
func rows(n, correct int, confidence float64) []decision.LabeledSample {
	out := make([]decision.LabeledSample, n)
	for i := range out {
		out[i] = decision.LabeledSample{
			Confidence: confidence,
			Correct:    i < correct,
			Provenance: decision.ProvenanceHuman,
		}
	}
	return out
}

// bandReading returns the entry of r.ByBand for band.
func bandReading(t *testing.T, r decision.Reading, band decision.Band) decision.BandReading {
	t.Helper()
	for _, b := range r.ByBand {
		if b.Band == band {
			return b
		}
	}
	t.Fatalf("reading has no %s band: %+v", band, r.ByBand)
	return decision.BandReading{}
}

// round4 rounds v to four decimal places, the precision the guide's oracle
// numbers are quoted at.
func round4(v float64) float64 { return math.Round(v*1e4) / 1e4 }

// wantPtr fails t unless got is present and within tol of want.
func wantPtr(t *testing.T, label string, got *float64, want, tol float64) {
	t.Helper()
	if got == nil {
		t.Errorf("%s = absent, want %v", label, want)
		return
	}
	if math.Abs(*got-want) > tol {
		t.Errorf("%s = %v, want %v (tol %v)", label, *got, want, tol)
	}
}

// wantAbsent fails t unless got is the absent (nil) state.
func wantAbsent(t *testing.T, label string, got *float64) {
	t.Helper()
	if got != nil {
		t.Errorf("%s = %v, want absent", label, *got)
	}
}

// mixedSamples is the hand-worked reading of the package: twelve labelled
// answers, three per confidence region chosen to land on both band
// boundaries.
//
//	unsure  (<0.5):     0.20 wrong, 0.40 wrong, 0.45 right          n=3 correct=1
//	confirm (0.5..0.9): 0.50 right, 0.60 wrong, 0.70 right,
//	                    0.80 right, 0.85 wrong                       n=5 correct=3
//	proceed (>=0.9):    0.90 right, 0.95 right, 0.99 right,
//	                    1.00 right                                   n=4 correct=4
//
// Brier: (0.04+0.16+0.3025+0.25+0.36+0.09+0.04+0.7225+0.01+0.0025+0.0001+0)/12
// = 1.9776/12 = 0.1648.
//
// AUROC: the eight right answers beat the four wrong ones in
// 2+2+3+3+4+4+4+4 = 26 of 32 pairs, no ties: 26/32 = 0.8125.
//
// Provenance: human 7 (rows 1,2,4,6,8,9,10), frontier 3 (3,7,12),
// computed 2 (5,11).
func mixedSamples() []decision.LabeledSample {
	h, f, c := decision.ProvenanceHuman, decision.ProvenanceFrontier, decision.ProvenanceComputed
	return []decision.LabeledSample{
		{Confidence: 0.20, Correct: false, Provenance: h},
		{Confidence: 0.40, Correct: false, Provenance: h},
		{Confidence: 0.45, Correct: true, Provenance: f},
		{Confidence: 0.50, Correct: true, Provenance: h},
		{Confidence: 0.60, Correct: false, Provenance: c},
		{Confidence: 0.70, Correct: true, Provenance: h},
		{Confidence: 0.80, Correct: true, Provenance: f},
		{Confidence: 0.85, Correct: false, Provenance: h},
		{Confidence: 0.90, Correct: true, Provenance: h},
		{Confidence: 0.95, Correct: true, Provenance: h},
		{Confidence: 0.99, Correct: true, Provenance: c},
		{Confidence: 1.00, Correct: true, Provenance: f},
	}
}

var testKey = decision.ReadingKey{
	Site:            "triage_failure",
	TemplateVersion: "3",
	Provider:        "typesafe",
	ResolvedModel:   "model-1.13.0",
}

func TestReadingReportsWilsonBrierAucAndRecalibrationDue(t *testing.T) {
	r, err := decision.NewReading(testKey, mixedSamples())
	if err != nil {
		t.Fatalf("NewReading: %v", err)
	}

	if r.Key != testKey {
		t.Errorf("Key = %+v, want %+v", r.Key, testKey)
	}
	if r.N != 12 || r.Correct != 8 {
		t.Errorf("N, Correct = %d, %d, want 12, 8", r.N, r.Correct)
	}
	wantPtr(t, "Accuracy", r.Accuracy, 8.0/12.0, 1e-12)
	wantPtr(t, "AccuracyLo", r.AccuracyLo, 0.390622088873, 1e-9)
	wantPtr(t, "AccuracyHi", r.AccuracyHi, 0.861879908909, 1e-9)

	// The split by band, in the fixed order unsure, confirm, proceed.
	wantBands := []struct {
		band   decision.Band
		n, ok  int
		lo, hi float64
	}{
		{decision.BandUnsure, 3, 1, 0.061491944720, 0.792340399198},
		{decision.BandConfirm, 5, 3, 0.230724281276, 0.882379225767},
		{decision.BandProceed, 4, 4, 0.510109163545, 1.0},
	}
	if len(r.ByBand) != len(wantBands) {
		t.Fatalf("len(ByBand) = %d, want %d", len(r.ByBand), len(wantBands))
	}
	for i, w := range wantBands {
		b := r.ByBand[i]
		if b.Band != w.band || b.N != w.n || b.Correct != w.ok {
			t.Errorf("ByBand[%d] = {%s n=%d correct=%d}, want {%s n=%d correct=%d}",
				i, b.Band, b.N, b.Correct, w.band, w.n, w.ok)
		}
		wantPtr(t, "ByBand["+string(w.band)+"].WilsonLo", b.WilsonLo, w.lo, 1e-9)
		wantPtr(t, "ByBand["+string(w.band)+"].WilsonHi", b.WilsonHi, w.hi, 1e-9)
	}

	if r.Monotonic == nil || !*r.Monotonic {
		t.Errorf("Monotonic = %v, want true (1/3 <= 3/5 <= 4/4)", r.Monotonic)
	}
	wantPtr(t, "Brier", r.Brier, 0.1648, 1e-12)
	wantPtr(t, "AUROC", r.AUROC, 0.8125, 1e-12)
	if want := (decision.ProvenanceMix{Human: 7, Frontier: 3, Computed: 2}); r.Labels != want {
		t.Errorf("Labels = %+v, want %+v", r.Labels, want)
	}

	// The Wilson lower bound the guide's bars are stated in: a hint shown as
	// calibrated needs 90 of 100, anything served by default needs the
	// Wilson95 lower bound at or above 0.90. Every vector is the guide's
	// (docs/fast-judgment.md section 3), the lower bound quoted to four
	// decimal places.
	vectors := []struct {
		name    string
		n, ok   int
		wantLo  float64 // rounded to four places
		clearsB bool    // lower bound at or above 0.90
	}{
		{"38 of 41 is 0.806 not 0.9", 41, 38, 0.8057, false},
		{"50 of 50", 50, 50, 0.9287, true},
		{"96 of 100", 100, 96, 0.9016, true},
		{"189 of 200", 200, 189, 0.9042, true},
		{"281 of 300", 300, 281, 0.9032, true},
		{"464 of 500", 500, 464, 0.9019, true},
		{"919 of 1000", 1000, 919, 0.9004, true},
		{"49 of 50 is one short", 50, 49, 0.8950, false},
		{"95 of 100 is one short", 100, 95, 0.8882, false},
		{"188 of 200 is one short", 200, 188, 0.8981, false},
		{"280 of 300 is one short", 300, 280, 0.8993, false},
		{"463 of 500 is one short", 500, 463, 0.8997, false},
		{"918 of 1000 is one short", 1000, 918, 0.8994, false},
		{"30 of 30 cannot reach the bar", 30, 30, 0.8865, false},
	}
	for _, v := range vectors {
		t.Run(v.name, func(t *testing.T) {
			// Confidence 0.95 puts every sample in BandProceed, so the
			// overall and the proceed-band intervals must agree.
			got, err := decision.NewReading(testKey, rows(v.n, v.ok, 0.95))
			if err != nil {
				t.Fatalf("NewReading: %v", err)
			}
			if got.AccuracyLo == nil {
				t.Fatal("AccuracyLo absent")
			}
			if lo := round4(*got.AccuracyLo); lo != v.wantLo {
				t.Errorf("AccuracyLo = %v (%v), want %v", lo, *got.AccuracyLo, v.wantLo)
			}
			if clears := *got.AccuracyLo >= 0.90; clears != v.clearsB {
				t.Errorf("AccuracyLo = %v, clears the 0.90 bar = %v, want %v", *got.AccuracyLo, clears, v.clearsB)
			}
			proceed := bandReading(t, got, decision.BandProceed)
			if proceed.WilsonLo == nil || *proceed.WilsonLo != *got.AccuracyLo {
				t.Errorf("proceed band lower bound %v differs from overall %v", proceed.WilsonLo, *got.AccuracyLo)
			}
		})
	}

	// Recalibration is due when the provider, the resolved model or the
	// template version moves, and only then.
	due := []struct {
		name string
		cur  decision.ReadingKey
		want bool
	}{
		{"identical key", testKey, false},
		{"resolved model moved", withKey(testKey, func(k *decision.ReadingKey) { k.ResolvedModel = "model-1.14.0" }), true},
		{"provider changed", withKey(testKey, func(k *decision.ReadingKey) { k.Provider = "llm" }), true},
		{"template version changed", withKey(testKey, func(k *decision.ReadingKey) { k.TemplateVersion = "4" }), true},
		{"all three changed", decision.ReadingKey{Site: testKey.Site, TemplateVersion: "9", Provider: "x", ResolvedModel: "y"}, true},
	}
	for _, d := range due {
		t.Run("recalibration due: "+d.name, func(t *testing.T) {
			if got := r.RecalibrationDue(d.cur); got != d.want {
				t.Errorf("RecalibrationDue(%+v) = %v, want %v", d.cur, got, d.want)
			}
		})
	}
}

func withKey(k decision.ReadingKey, mut func(*decision.ReadingKey)) decision.ReadingKey {
	mut(&k)
	return k
}

// A reading of one site cannot vouch for another: a Site mismatch is a
// caller error, answered fail-closed (due) rather than by a panic or an
// ignored field.
func TestReadingRecalibrationDueTreatsSiteMismatchAsDue(t *testing.T) {
	r, err := decision.NewReading(testKey, rows(10, 9, 0.95))
	if err != nil {
		t.Fatal(err)
	}
	other := withKey(testKey, func(k *decision.ReadingKey) { k.Site = "route_request" })
	if !r.RecalibrationDue(other) {
		t.Error("RecalibrationDue(other site) = false, want true: a reading for one site says nothing about another")
	}
}

// The guide's table of correct answers needed for a Wilson 95% lower bound
// of 0.90, reproduced by scanning k for each n through NewReading.
func TestReadingWilsonReproducesTheCorrectNeededTable(t *testing.T) {
	const unreachable = -1
	table := []struct{ n, need int }{
		{30, unreachable}, {50, 50}, {100, 96}, {200, 189}, {300, 281}, {500, 464}, {1000, 919},
	}
	for _, tc := range table {
		need := unreachable
		for k := 0; k <= tc.n; k++ {
			r, err := decision.NewReading(testKey, rows(tc.n, k, 0.95))
			if err != nil {
				t.Fatal(err)
			}
			if *r.AccuracyLo >= 0.90 {
				need = k
				break
			}
		}
		if need != tc.need {
			t.Errorf("n=%d: correct needed for Wilson95 lower bound 0.90 = %d, want %d", tc.n, need, tc.need)
		}
	}
}

func TestReadingWilsonEdges(t *testing.T) {
	// 0 of n has lower bound exactly 0; n of n has upper bound exactly 1;
	// neither is nudged outside [0,1] by rounding.
	zero, err := decision.NewReading(testKey, rows(3, 0, 0.7))
	if err != nil {
		t.Fatal(err)
	}
	wantPtr(t, "0/3 AccuracyLo", zero.AccuracyLo, 0, 0)
	wantPtr(t, "0/3 AccuracyHi", zero.AccuracyHi, 0.561497031755, 1e-9)
	wantPtr(t, "0/3 Accuracy", zero.Accuracy, 0, 0)

	all, err := decision.NewReading(testKey, rows(3, 3, 0.7))
	if err != nil {
		t.Fatal(err)
	}
	wantPtr(t, "3/3 AccuracyLo", all.AccuracyLo, 0.438502968245, 1e-9)
	wantPtr(t, "3/3 AccuracyHi", all.AccuracyHi, 1, 0)
	wantPtr(t, "3/3 Accuracy", all.Accuracy, 1, 0)

	// The edges are exact at every n, not only where rounding happens to
	// land on them: the bare formula gives 0.9999999999999998 for 15 of 15.
	for n := 1; n <= 300; n++ {
		none, err := decision.NewReading(testKey, rows(n, 0, 0.7))
		if err != nil {
			t.Fatal(err)
		}
		every, err := decision.NewReading(testKey, rows(n, n, 0.7))
		if err != nil {
			t.Fatal(err)
		}
		if *none.AccuracyLo != 0 || *none.ByBand[1].WilsonLo != 0 {
			t.Fatalf("0 of %d: lower bound = %v (band %v), want exactly 0", n, *none.AccuracyLo, *none.ByBand[1].WilsonLo)
		}
		if *every.AccuracyHi != 1 || *every.ByBand[1].WilsonHi != 1 {
			t.Fatalf("%d of %d: upper bound = %v (band %v), want exactly 1", n, n, *every.AccuracyHi, *every.ByBand[1].WilsonHi)
		}
	}
}

func TestReadingBandBoundaries(t *testing.T) {
	// Boundaries follow BandOf exactly: 0.5 is confirm, 0.9 is proceed.
	confs := []float64{0, 0.4999, 0.5, 0.8999, 0.9, 1}
	samples := make([]decision.LabeledSample, len(confs))
	for i, c := range confs {
		samples[i] = decision.LabeledSample{Confidence: c, Correct: true, Provenance: decision.ProvenanceComputed}
	}
	r, err := decision.NewReading(testKey, samples)
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		band decision.Band
		n    int
	}{{decision.BandUnsure, 2}, {decision.BandConfirm, 2}, {decision.BandProceed, 2}}
	for i, w := range want {
		if r.ByBand[i].Band != w.band || r.ByBand[i].N != w.n {
			t.Errorf("ByBand[%d] = {%s n=%d}, want {%s n=%d}", i, r.ByBand[i].Band, r.ByBand[i].N, w.band, w.n)
		}
	}
	for _, s := range samples {
		got := decision.BandOf(s.Confidence)
		var idx int
		switch got {
		case decision.BandUnsure:
			idx = 0
		case decision.BandConfirm:
			idx = 1
		default:
			idx = 2
		}
		if r.ByBand[idx].N == 0 {
			t.Errorf("confidence %v is %s per BandOf but its band is empty in the reading", s.Confidence, got)
		}
	}
}

func TestReadingEmptyBandIsPresentWithoutAnInterval(t *testing.T) {
	// Only the confirm band has data: unsure and proceed are present with
	// N=0 and no interval; the reading is still whole.
	r, err := decision.NewReading(testKey, rows(10, 7, 0.7))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.ByBand) != 3 {
		t.Fatalf("len(ByBand) = %d, want 3", len(r.ByBand))
	}
	for _, i := range []int{0, 2} {
		b := r.ByBand[i]
		if b.N != 0 || b.Correct != 0 {
			t.Errorf("ByBand[%d] = {n=%d correct=%d}, want empty", i, b.N, b.Correct)
		}
		wantAbsent(t, "empty band WilsonLo", b.WilsonLo)
		wantAbsent(t, "empty band WilsonHi", b.WilsonHi)
	}
	if b := bandReading(t, r, decision.BandConfirm); b.N != 10 || b.Correct != 7 || b.WilsonLo == nil || b.WilsonHi == nil {
		t.Errorf("ByBand[1] = %+v, want the confirm band holding all 10", b)
	}
	wantPtr(t, "confirm WilsonLo", r.ByBand[1].WilsonLo, 0.396778, 1e-6)
	wantPtr(t, "confirm WilsonHi", r.ByBand[1].WilsonHi, 0.892209, 1e-6)
	if r.Monotonic != nil {
		t.Errorf("Monotonic = %v with one band of data, want absent", *r.Monotonic)
	}
}

func TestReadingMonotonicIsTriState(t *testing.T) {
	mk := func(parts ...[3]float64) []decision.LabeledSample {
		// each part: confidence, n, correct
		var out []decision.LabeledSample
		for _, p := range parts {
			out = append(out, rows(int(p[1]), int(p[2]), p[0])...)
		}
		return out
	}
	cases := []struct {
		name    string
		samples []decision.LabeledSample
		want    string // "absent", "true", "false"
	}{
		{"no samples", nil, "absent"},
		{"one band of data", mk([3]float64{0.7, 10, 5}), "absent"},
		{"two bands rising", mk([3]float64{0.2, 10, 3}, [3]float64{0.95, 10, 9}), "true"},
		{"two bands falling", mk([3]float64{0.2, 10, 9}, [3]float64{0.95, 10, 3}), "false"},
		{"equal accuracy is non-decreasing", mk([3]float64{0.2, 10, 5}, [3]float64{0.7, 4, 2}, [3]float64{0.95, 6, 3}), "true"},
		{"three bands with a dip in the middle", mk([3]float64{0.2, 10, 5}, [3]float64{0.7, 10, 3}, [3]float64{0.95, 10, 9}), "false"},
		{"empty middle band is skipped", mk([3]float64{0.2, 10, 2}, [3]float64{0.95, 10, 8}), "true"},
		// 45.7% right at confidence >= 0.9 beneath a better confirm band: the
		// consumer reading that motivated decision 0017.
		{"proceed worse than confirm", mk([3]float64{0.7, 100, 60}, [3]float64{0.95, 100, 46}), "false"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, err := decision.NewReading(testKey, c.samples)
			if err != nil {
				t.Fatal(err)
			}
			got := "absent"
			if r.Monotonic != nil {
				got = map[bool]string{true: "true", false: "false"}[*r.Monotonic]
			}
			if got != c.want {
				t.Errorf("Monotonic = %s, want %s", got, c.want)
			}
		})
	}
}

func TestReadingBrierIsMeanSquaredConfidenceError(t *testing.T) {
	// (0.9-1)^2 + (0.8-0)^2 + (0.5-1)^2 + (0-0)^2 = 0.01+0.64+0.25+0 = 0.9; /4.
	r, err := decision.NewReading(testKey, []decision.LabeledSample{
		{Confidence: 0.9, Correct: true, Provenance: decision.ProvenanceHuman},
		{Confidence: 0.8, Correct: false, Provenance: decision.ProvenanceHuman},
		{Confidence: 0.5, Correct: true, Provenance: decision.ProvenanceHuman},
		{Confidence: 0.0, Correct: false, Provenance: decision.ProvenanceHuman},
	})
	if err != nil {
		t.Fatal(err)
	}
	wantPtr(t, "Brier", r.Brier, 0.225, 1e-12)

	// Perfectly confident and right: 0. Perfectly confident and wrong: 1.
	best, _ := decision.NewReading(testKey, rows(5, 5, 1))
	wantPtr(t, "Brier of confident and right", best.Brier, 0, 0)
	worst, _ := decision.NewReading(testKey, rows(5, 0, 1))
	wantPtr(t, "Brier of confident and wrong", worst.Brier, 1, 0)
}

func TestReadingAUROC(t *testing.T) {
	mk := func(correct, incorrect []float64) []decision.LabeledSample {
		var out []decision.LabeledSample
		for _, c := range correct {
			out = append(out, decision.LabeledSample{Confidence: c, Correct: true, Provenance: decision.ProvenanceHuman})
		}
		for _, c := range incorrect {
			out = append(out, decision.LabeledSample{Confidence: c, Correct: false, Provenance: decision.ProvenanceHuman})
		}
		return out
	}
	cases := []struct {
		name      string
		correct   []float64
		incorrect []float64
		want      float64
	}{
		{"perfect separation", []float64{0.9, 0.8}, []float64{0.3, 0.2}, 1},
		{"perfectly inverted", []float64{0.3, 0.2}, []float64{0.9, 0.8}, 0},
		{"all tied counts half", []float64{0.6, 0.6}, []float64{0.6, 0.6, 0.6}, 0.5},
		// Worked by hand: correct {0.9, 0.6}, incorrect {0.7, 0.3}. Pairs
		// (0.9,0.7)=1 (0.9,0.3)=1 (0.6,0.7)=0 (0.6,0.3)=1 -> 3/4.
		{"mixed, no ties", []float64{0.9, 0.6}, []float64{0.7, 0.3}, 0.75},
		// Correct {0.9, 0.6}, incorrect {0.6, 0.3}: (0.9,0.6)=1 (0.9,0.3)=1
		// (0.6,0.6)=1/2 (0.6,0.3)=1 -> 3.5/4.
		{"mixed with one tie", []float64{0.9, 0.6}, []float64{0.6, 0.3}, 0.875},
		{"single pair right", []float64{0.9}, []float64{0.1}, 1},
		{"single pair tied", []float64{0.5}, []float64{0.5}, 0.5},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, err := decision.NewReading(testKey, mk(c.correct, c.incorrect))
			if err != nil {
				t.Fatal(err)
			}
			wantPtr(t, "AUROC", r.AUROC, c.want, 1e-12)
		})
	}

	t.Run("absent when a class is empty", func(t *testing.T) {
		for name, s := range map[string][]decision.LabeledSample{
			"no samples":     nil,
			"all correct":    mk([]float64{0.9, 0.5}, nil),
			"all incorrect":  mk(nil, []float64{0.9, 0.5}),
			"one correct":    mk([]float64{0.9}, nil),
			"one incorrect":  mk(nil, []float64{0.1}),
			"repeated right": rows(50, 50, 0.95),
		} {
			r, err := decision.NewReading(testKey, s)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			wantAbsent(t, name+" AUROC (absent, never 0.5)", r.AUROC)
		}
	})

	t.Run("matches a pairwise count on a tie-heavy random set", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(7, 11))
		for trial := 0; trial < 20; trial++ {
			n := 5 + rng.IntN(120)
			s := make([]decision.LabeledSample, n)
			for i := range s {
				// A 11-point grid forces plenty of exact ties.
				s[i] = decision.LabeledSample{
					Confidence: float64(rng.IntN(11)) / 10,
					Correct:    rng.IntN(3) != 0,
					Provenance: decision.ProvenanceComputed,
				}
			}
			var pos, neg []float64
			for _, x := range s {
				if x.Correct {
					pos = append(pos, x.Confidence)
				} else {
					neg = append(neg, x.Confidence)
				}
			}
			r, err := decision.NewReading(testKey, s)
			if err != nil {
				t.Fatal(err)
			}
			if len(pos) == 0 || len(neg) == 0 {
				wantAbsent(t, "AUROC with an empty class", r.AUROC)
				continue
			}
			var score float64
			for _, p := range pos {
				for _, q := range neg {
					switch {
					case p > q:
						score++
					case p == q:
						score += 0.5
					}
				}
			}
			wantPtr(t, "AUROC", r.AUROC, score/float64(len(pos)*len(neg)), 1e-12)
		}
	})
}

func TestReadingCountsProvenance(t *testing.T) {
	s := []decision.LabeledSample{
		{Confidence: 0.9, Correct: true, Provenance: decision.ProvenanceComputed},
		{Confidence: 0.9, Correct: true, Provenance: decision.ProvenanceComputed},
		{Confidence: 0.9, Correct: false, Provenance: decision.ProvenanceFrontier},
		{Confidence: 0.9, Correct: true, Provenance: decision.ProvenanceHuman},
		{Confidence: 0.9, Correct: true, Provenance: decision.ProvenanceComputed},
	}
	r, err := decision.NewReading(testKey, s)
	if err != nil {
		t.Fatal(err)
	}
	if want := (decision.ProvenanceMix{Human: 1, Frontier: 1, Computed: 3}); r.Labels != want {
		t.Errorf("Labels = %+v, want %+v", r.Labels, want)
	}
}

func TestReadingOfNoSamplesIsWholeButEmpty(t *testing.T) {
	for _, s := range [][]decision.LabeledSample{nil, {}} {
		r, err := decision.NewReading(testKey, s)
		if err != nil {
			t.Fatalf("NewReading(empty): %v", err)
		}
		if r.N != 0 || r.Correct != 0 || r.Key != testKey {
			t.Errorf("empty reading = %+v", r)
		}
		wantAbsent(t, "Accuracy", r.Accuracy)
		wantAbsent(t, "AccuracyLo", r.AccuracyLo)
		wantAbsent(t, "AccuracyHi", r.AccuracyHi)
		wantAbsent(t, "Brier", r.Brier)
		wantAbsent(t, "AUROC", r.AUROC)
		if r.Monotonic != nil {
			t.Errorf("Monotonic = %v, want absent", *r.Monotonic)
		}
		if len(r.ByBand) != 3 {
			t.Fatalf("len(ByBand) = %d, want 3", len(r.ByBand))
		}
		for i, want := range []decision.Band{decision.BandUnsure, decision.BandConfirm, decision.BandProceed} {
			b := r.ByBand[i]
			if b.Band != want || b.N != 0 || b.WilsonLo != nil || b.WilsonHi != nil {
				t.Errorf("ByBand[%d] = %+v, want empty %s", i, b, want)
			}
		}
		if (r.Labels != decision.ProvenanceMix{}) {
			t.Errorf("Labels = %+v, want zero", r.Labels)
		}
	}
}

func TestReadingRefusesInvalidSamples(t *testing.T) {
	ok := decision.LabeledSample{Confidence: 0.7, Correct: true, Provenance: decision.ProvenanceHuman}
	cases := []struct {
		name    string
		bad     decision.LabeledSample
		wantMsg string
	}{
		{"confidence above 1", decision.LabeledSample{Confidence: 1.0000001, Provenance: decision.ProvenanceHuman}, "confidence"},
		{"confidence below 0", decision.LabeledSample{Confidence: -0.0000001, Provenance: decision.ProvenanceHuman}, "confidence"},
		{"confidence NaN", decision.LabeledSample{Confidence: math.NaN(), Provenance: decision.ProvenanceHuman}, "NaN"},
		{"confidence +Inf", decision.LabeledSample{Confidence: math.Inf(1), Provenance: decision.ProvenanceHuman}, "confidence"},
		{"confidence -Inf", decision.LabeledSample{Confidence: math.Inf(-1), Provenance: decision.ProvenanceHuman}, "confidence"},
		{"unknown provenance", decision.LabeledSample{Confidence: 0.5, Provenance: decision.Provenance("oracle")}, "provenance"},
		{"empty provenance", decision.LabeledSample{Confidence: 0.5}, "provenance"},
		{"provenance is case sensitive", decision.LabeledSample{Confidence: 0.5, Provenance: decision.Provenance("Human")}, "provenance"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// The bad sample sits at index 3 behind three good ones, so an
			// error that names the wrong index (or none) is caught.
			samples := []decision.LabeledSample{ok, ok, ok, c.bad, ok}
			r, err := decision.NewReading(testKey, samples)
			if err == nil {
				t.Fatalf("NewReading accepted %+v, reading %+v", c.bad, r)
			}
			if !errors.Is(err, decision.ErrInvalidSample) {
				t.Errorf("error %q is not ErrInvalidSample", err)
			}
			if !strings.Contains(err.Error(), "sample 3") {
				t.Errorf("error %q does not name sample 3", err)
			}
			if !strings.Contains(err.Error(), c.wantMsg) {
				t.Errorf("error %q does not mention %q", err, c.wantMsg)
			}
			if !reflect.DeepEqual(r, decision.Reading{}) {
				t.Errorf("a refused input returned a partial reading: %+v", r)
			}
		})
	}

	t.Run("the first invalid sample is the one named", func(t *testing.T) {
		samples := []decision.LabeledSample{ok, {Confidence: 2, Provenance: decision.ProvenanceHuman}, {Confidence: -1, Provenance: "x"}}
		_, err := decision.NewReading(testKey, samples)
		if err == nil || !strings.Contains(err.Error(), "sample 1") {
			t.Errorf("error = %v, want one naming sample 1", err)
		}
	})

	t.Run("the boundaries 0 and 1 are valid", func(t *testing.T) {
		_, err := decision.NewReading(testKey, []decision.LabeledSample{
			{Confidence: 0, Provenance: decision.ProvenanceHuman},
			{Confidence: 1, Correct: true, Provenance: decision.ProvenanceFrontier},
		})
		if err != nil {
			t.Errorf("NewReading(0 and 1) = %v, want nil", err)
		}
	})
}

func TestReadingIsDeterministicAndDoesNotTouchItsInput(t *testing.T) {
	samples := mixedSamples()
	before := append([]decision.LabeledSample(nil), samples...)

	first, err := decision.NewReading(testKey, samples)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(samples, before) {
		t.Error("NewReading modified its samples argument")
	}
	second, _ := decision.NewReading(testKey, samples)
	if !reflect.DeepEqual(first, second) {
		t.Error("two readings of the same samples differ")
	}

	// Sample order must not change any number, down to the last bit: a
	// reading is a statistic of the labelled set.
	rng := rand.New(rand.NewPCG(3, 5))
	for trial := 0; trial < 25; trial++ {
		shuffled := append([]decision.LabeledSample(nil), samples...)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		got, err := decision.NewReading(testKey, shuffled)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(first, got) {
			t.Fatalf("a shuffled sample order changed the reading:\n first %+v\n got   %+v", first, got)
		}
	}

	// The bands come out in the fixed order whatever the data.
	for _, s := range [][]decision.LabeledSample{nil, rows(4, 2, 0.95), rows(4, 2, 0.1), mixedSamples()} {
		r, _ := decision.NewReading(testKey, s)
		got := []decision.Band{r.ByBand[0].Band, r.ByBand[1].Band, r.ByBand[2].Band}
		want := []decision.Band{decision.BandUnsure, decision.BandConfirm, decision.BandProceed}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("band order = %v, want %v", got, want)
		}
	}
}

// A reading serializes to the shape of the calibration record template
// (docs/fast-judgment.md section 3): by_band entries carrying
// band/n/correct/wilson_lo/wilson_hi, and null for a statistic that does
// not exist yet.
func TestReadingMarshalsToTheCalibrationRecordShape(t *testing.T) {
	r, err := decision.NewReading(testKey, mixedSamples())
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"n", "by_band", "monotonic", "brier", "auroc"} {
		if _, ok := got[k]; !ok {
			t.Errorf("marshaled reading has no %q key: %s", k, raw)
		}
	}
	bands, _ := got["by_band"].([]any)
	if len(bands) != 3 {
		t.Fatalf("by_band = %v, want 3 entries", got["by_band"])
	}
	first, _ := bands[0].(map[string]any)
	for _, k := range []string{"band", "n", "correct", "wilson_lo", "wilson_hi"} {
		if _, ok := first[k]; !ok {
			t.Errorf("by_band[0] has no %q key: %v", k, first)
		}
	}

	empty, _ := decision.NewReading(testKey, nil)
	raw, _ = json.Marshal(empty)
	var e map[string]any
	if err := json.Unmarshal(raw, &e); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"monotonic", "brier", "auroc"} {
		if v, present := e[k]; !present || v != nil {
			t.Errorf("empty reading %q = %v (present %v), want an explicit null", k, v, present)
		}
	}
	emptyBands, _ := e["by_band"].([]any)
	for i, b := range emptyBands {
		m, _ := b.(map[string]any)
		if m["wilson_lo"] != nil || m["wilson_hi"] != nil {
			t.Errorf("empty by_band[%d] = %v, want null Wilson bounds", i, m)
		}
	}
}
