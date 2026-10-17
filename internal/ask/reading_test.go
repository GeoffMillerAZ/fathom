package ask

import (
	"math"
	"testing"
)

func near(a, b float64) bool { return math.Abs(a-b) < 5e-4 }

func TestWilsonMatchesPublishedIntervals(t *testing.T) {
	cases := []struct {
		k, n   int
		lo, hi float64
	}{
		{8, 10, 0.4902, 0.9433},
		{0, 10, 0, 0.2775},
		{10, 10, 0.7225, 1},
		{0, 0, 0, 0},
	}
	for _, c := range cases {
		lo, hi := Wilson(c.k, c.n)
		if !near(lo, c.lo) || !near(hi, c.hi) {
			t.Errorf("Wilson(%d, %d) = (%.4f, %.4f), want (%.4f, %.4f)", c.k, c.n, lo, hi, c.lo, c.hi)
		}
	}
}

func TestAUROCCountsPairsAndTies(t *testing.T) {
	items := []Item{{P: 0.9, Truth: true}, {P: 0.8, Truth: true}, {P: 0.7}, {P: 0.3, Truth: true}, {P: 0.2}}
	if auc, ok := AUROC(items); !ok || !near(auc, 5.0/6) {
		t.Errorf("AUROC = %v, %v; want 5/6", auc, ok)
	}
	if auc, ok := AUROC([]Item{{P: 0.5, Truth: true}, {P: 0.5}}); !ok || auc != 0.5 {
		t.Errorf("a tie: AUROC = %v, %v; want 0.5", auc, ok)
	}
	if _, ok := AUROC([]Item{{P: 0.5, Truth: true}}); ok {
		t.Error("one class only: AUROC defined, want undefined")
	}
}

func TestBandEdges(t *testing.T) {
	for p, want := range map[float64]int{0: 0, 0.0999: 0, 0.1: 1, 0.4999: 1, 0.5: 2, 0.8999: 2, 0.9: 3, 1: 3} {
		if got := bandOf(p); got != want {
			t.Errorf("bandOf(%v) = %d (%s), want %d (%s)", p, got, Bands[got].Name, want, Bands[want].Name)
		}
	}
}

// TestReadQualifiesOnlyWhenAMajorityBandBeatsTheBaseline pins EXP-M1's bar:
// a band with at least half the items, accuracy at least 0.9, and the model
// beating the baseline on that band's items.
func TestReadQualifiesOnlyWhenAMajorityBandBeatsTheBaseline(t *testing.T) {
	var items []Item
	for range 6 {
		items = append(items, Item{P: 0.95, Truth: true, Baseline: false})
	}
	for range 4 {
		items = append(items, Item{P: 0.05, Truth: false, Baseline: false})
	}
	r := Read(items)
	if !r.Qualifies || r.QualifyingBand != "0.9 and above" {
		t.Fatalf("want QUALIFIES in 0.9 and above, got %+v", r)
	}
	top := r.Bands[3]
	if top.N != 6 || !near(top.Share, 0.6) || top.Accuracy != 1 || top.BaselineAccuracy != 0 || top.Lift != 1 {
		t.Errorf("top band = %+v", top)
	}
	if r.Accuracy != 1 || !near(r.BaselineAccuracy, 0.4) || !near(r.Lift, 0.6) || r.N != 10 {
		t.Errorf("overall = %+v", r)
	}
	lo, hi := Wilson(6, 6)
	if top.WilsonLow != lo || top.WilsonHigh != hi {
		t.Errorf("top band Wilson = (%v, %v), want (%v, %v)", top.WilsonLow, top.WilsonHigh, lo, hi)
	}

	// The baseline as good as the model on the majority band: NONE.
	for i := range 6 {
		items[i].Baseline = true
	}
	if r := Read(items); r.Qualifies {
		t.Errorf("baseline ties the model: want NONE, got %+v", r)
	}
	// A 0.9-accurate band holding under half the items: NONE.
	mixed := []Item{{P: 0.95, Truth: true}, {P: 0.95, Truth: true}, {P: 0.3, Truth: true}, {P: 0.3, Truth: true}, {P: 0.6}}
	if r := Read(mixed); r.Qualifies {
		t.Errorf("minority band: want NONE, got %+v", r)
	}
	if r := Read(nil); r.Qualifies || r.N != 0 || len(r.Bands) != 4 {
		t.Errorf("no items: %+v", r)
	}
}

func TestPercentileNearestRank(t *testing.T) {
	xs := []float64{50, 10, 40, 20, 30}
	if p := Percentile(xs, 0.5); p != 30 {
		t.Errorf("p50 = %v, want 30", p)
	}
	if p := Percentile(xs, 0.95); p != 50 {
		t.Errorf("p95 = %v, want 50", p)
	}
	if p := Percentile(nil, 0.5); p != 0 {
		t.Errorf("empty p50 = %v", p)
	}
}
