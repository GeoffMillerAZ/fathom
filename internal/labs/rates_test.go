package labs

import (
	"math"
	"testing"
)

// Values evaluated by hand from p=k/n and the 95% Wilson formula, rounded
// to 12 decimals. They are independent of the implementation's ask.Wilson call.
func TestLabBinaryRatesHandComputed(t *testing.T) {
	got := BinaryRates(ConfusionTable{TP: 8, FP: 2, TN: 9, FN: 1})
	cases := []struct {
		name          string
		got           Rate
		value, lo, hi float64
	}{
		{"TPR", got.TPR, 0.888888888889, 0.565000294423, 0.980109112361},
		{"TNR", got.TNR, 0.818181818182, 0.523019438039, 0.948632310254},
		{"PPV", got.PPV, 0.800000000000, 0.490162471537, 0.943317848546},
		{"NPV", got.NPV, 0.900000000000, 0.595849973205, 0.982123786905},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, pair := range [][2]float64{{tc.got.Value, tc.value}, {tc.got.Lo, tc.lo}, {tc.got.Hi, tc.hi}} {
				if math.IsNaN(pair[0]) || math.Abs(pair[0]-pair[1]) > 1e-11 {
					t.Fatalf("got %.12f, want %.12f", pair[0], pair[1])
				}
			}
		})
	}
}

func TestLabBinaryRatesEmptyMargins(t *testing.T) {
	cases := []struct {
		name  string
		table ConfusionTable
		empty int
	}{
		{"TPR", ConfusionTable{FP: 2, TN: 3}, 0},
		{"TNR", ConfusionTable{TP: 2, FN: 3}, 1},
		{"PPV", ConfusionTable{TN: 2, FN: 3}, 2},
		{"NPV", ConfusionTable{TP: 2, FP: 3}, 3},
		{"all", ConfusionTable{}, -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := BinaryRates(tc.table)
			for i, r := range []Rate{got.TPR, got.TNR, got.PPV, got.NPV} {
				if i == tc.empty || tc.empty == -1 {
					if !math.IsNaN(r.Value) || !math.IsNaN(r.Lo) || !math.IsNaN(r.Hi) {
						t.Fatalf("empty margin %d: %+v", i, r)
					}
				} else if math.IsNaN(r.Value) || math.IsNaN(r.Lo) || math.IsNaN(r.Hi) {
					t.Fatalf("nonempty margin %d: %+v", i, r)
				}
			}
		})
	}
}

func TestLabBinaryRatesExtremes(t *testing.T) {
	for _, tc := range []struct {
		name  string
		table ConfusionTable
		value float64
	}{
		{"perfect", ConfusionTable{TP: 4, TN: 4}, 1},
		{"wrong", ConfusionTable{FP: 4, FN: 4}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := BinaryRates(tc.table)
			for _, r := range []Rate{got.TPR, got.TNR, got.PPV, got.NPV} {
				if r.Value != tc.value || r.Lo < 0 || r.Hi > 1 || r.Lo >= r.Hi {
					t.Fatalf("invalid extreme: %+v", r)
				}
			}
		})
	}
}
