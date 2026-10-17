package ask

import (
	"math"
	"sort"
)

// Item is one judged span: the model's probability that the statement is
// true, the computed truth, and the lexical baseline's answer. The model's
// answer is P >= 0.5.
type Item struct {
	P        float64
	Truth    bool
	Baseline bool
}

// Band is one pre-registered probability band, [Lo, Hi).
type Band struct {
	Name   string
	Lo, Hi float64
}

// Bands are E1's four probability bands; the last includes 1.
var Bands = []Band{
	{Name: "below 0.1", Lo: 0, Hi: 0.1},
	{Name: "0.1 to 0.5", Lo: 0.1, Hi: 0.5},
	{Name: "0.5 to 0.9", Lo: 0.5, Hi: 0.9},
	{Name: "0.9 and above", Lo: 0.9, Hi: math.Inf(1)},
}

func bandOf(p float64) int {
	for i := len(Bands) - 1; i > 0; i-- {
		if p >= Bands[i].Lo {
			return i
		}
	}
	return 0
}

// BandReading is one band's reading: its items' share of all, the model's
// accuracy with Wilson 95% bounds, and the lexical baseline's accuracy on
// the same items with the model's lift over it.
type BandReading struct {
	Band             string  `json:"band"`
	N                int     `json:"n"`
	Share            float64 `json:"share"`
	Accuracy         float64 `json:"accuracy"`
	WilsonLow        float64 `json:"wilson_low"`
	WilsonHigh       float64 `json:"wilson_high"`
	BaselineAccuracy float64 `json:"baseline_accuracy"`
	Lift             float64 `json:"lift"`
}

// Reading is one template's E1 reading (EXP-M1's columns but latency and
// cost, which the harness adds).
type Reading struct {
	N                int           `json:"n"`
	Accuracy         float64       `json:"accuracy"`
	AUROC            float64       `json:"auroc"`
	AUROCDefined     bool          `json:"auroc_defined"`
	BaselineAccuracy float64       `json:"baseline_accuracy"`
	Lift             float64       `json:"lift"`
	Bands            []BandReading `json:"bands"`
	// Qualifies is EXP-M1's bar: one band holding at least 50% of the
	// items reads accuracy of at least 0.9 and beats the baseline's
	// accuracy on those items. QualifyingBand names the first such band.
	Qualifies      bool   `json:"qualifies"`
	QualifyingBand string `json:"qualifying_band,omitempty"`
}

// WilsonZ is the normal quantile for a two-sided 95% interval.
const WilsonZ = 1.959963984540054

// Wilson is the Wilson score interval for k successes in n trials at 95%;
// (0, 0) when n is 0.
func Wilson(k, n int) (lo, hi float64) {
	if n == 0 {
		return 0, 0
	}
	p, nf, z2 := float64(k)/float64(n), float64(n), WilsonZ*WilsonZ
	den := 1 + z2/nf
	mid := p + z2/(2*nf)
	half := WilsonZ * math.Sqrt(p*(1-p)/nf+z2/(4*nf*nf))
	return math.Max(0, (mid-half)/den), math.Min(1, (mid+half)/den)
}

// AUROC is the area under the ROC curve of P against Truth: the share of
// (true, false) pairs the true item outscores, a tie counting half.
// defined is false when either class is empty.
func AUROC(items []Item) (auc float64, defined bool) {
	var pos, neg []float64
	for _, it := range items {
		if it.Truth {
			pos = append(pos, it.P)
		} else {
			neg = append(neg, it.P)
		}
	}
	if len(pos) == 0 || len(neg) == 0 {
		return 0, false
	}
	sort.Float64s(neg)
	var wins float64
	for _, p := range pos {
		below := sort.SearchFloat64s(neg, p)
		ties := sort.SearchFloat64s(neg, math.Nextafter(p, math.Inf(1))) - below
		wins += float64(below) + 0.5*float64(ties)
	}
	return wins / float64(len(pos)*len(neg)), true
}

func accuracy(correct, n int) float64 {
	if n == 0 {
		return 0
	}
	return float64(correct) / float64(n)
}

// Read computes a template's reading over its judged items.
func Read(items []Item) Reading {
	r := Reading{N: len(items), Bands: make([]BandReading, len(Bands))}
	type tally struct{ n, model, base int }
	per := make([]tally, len(Bands))
	var model, base int
	for _, it := range items {
		b := bandOf(it.P)
		per[b].n++
		if (it.P >= 0.5) == it.Truth {
			per[b].model++
			model++
		}
		if it.Baseline == it.Truth {
			per[b].base++
			base++
		}
	}
	r.Accuracy, r.BaselineAccuracy = accuracy(model, len(items)), accuracy(base, len(items))
	r.Lift = r.Accuracy - r.BaselineAccuracy
	r.AUROC, r.AUROCDefined = AUROC(items)
	for i, b := range Bands {
		t := per[i]
		br := BandReading{Band: b.Name, N: t.n, Accuracy: accuracy(t.model, t.n), BaselineAccuracy: accuracy(t.base, t.n)}
		if len(items) > 0 {
			br.Share = float64(t.n) / float64(len(items))
		}
		br.WilsonLow, br.WilsonHigh = Wilson(t.model, t.n)
		br.Lift = br.Accuracy - br.BaselineAccuracy
		r.Bands[i] = br
		if !r.Qualifies && t.n > 0 && br.Share >= 0.5 && br.Accuracy >= 0.9 && br.Accuracy > br.BaselineAccuracy {
			r.Qualifies, r.QualifyingBand = true, b.Name
		}
	}
	return r
}

// Percentile is the nearest-rank q-th percentile (0 < q <= 1) of xs; 0 for
// none.
func Percentile(xs []float64, q float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	i := int(math.Ceil(q*float64(len(s)))) - 1
	return s[max(0, min(i, len(s)-1))]
}
