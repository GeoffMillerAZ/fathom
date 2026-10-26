package reliabilitystats_test

import (
	"encoding/json"
	"math"
	"testing"

	rs "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/reliability-stats"
)

func TestEquivalenceTOST_ThreeVerdictBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name    string
		x       []float64
		margin  float64
		verdict string
	}{
		{"zero equivalent", []float64{0, 0}, 1, "equivalent"},
		{"negative inside margin", []float64{-.25, -.25}, 1, "equivalent"},
		{"positive margin boundary", []float64{1, 1}, 1, "inconclusive"},
		{"negative margin boundary", []float64{-1, -1}, 1, "harm"},
		{"harm", []float64{-5, -4, -3}, 1, "harm"},
		{"unexpected positive effect", []float64{3, 4, 5}, 1, "inconclusive"},
		{"wide interval", []float64{-3, 3}, 1, "inconclusive"},
		// For df=1 at alpha=.25 the TOST upper endpoint is -1, but the
		// fixed one-sided 95% upper bound is positive, so harm is unproven.
		{"harm uses fixed 95 percent", []float64{-3, -1}, 1, "inconclusive"},
		{"zero harm boundary", []float64{0, 0}, math.SmallestNonzeroFloat64, "equivalent"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			alpha := .05
			if tc.name == "harm uses fixed 95 percent" {
				alpha = .25
			}
			r, err := rs.EquivalenceTOST(tc.x, make([]float64, len(tc.x)), tc.margin, alpha)
			if err != nil {
				t.Fatal(err)
			}
			// Verify the public wire contract as well as the three-way result.
			encoded, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			var got struct {
				Verdict string   `json:"verdict"`
				Upper95 *float64 `json:"upper_95"`
			}
			if err := json.Unmarshal(encoded, &got); err != nil {
				t.Fatal(err)
			}
			if got.Verdict != tc.verdict || got.Upper95 == nil {
				t.Fatalf("got %s, want verdict %q and 95%% upper bound", encoded, tc.verdict)
			}
			if got.Verdict == "harm" && !(*got.Upper95 < 0) {
				t.Errorf("harm without negative upper bound: %s", encoded)
			}
			if r.Equivalent != (got.Verdict == "equivalent") {
				t.Errorf("inconsistent compatibility bool: %s", encoded)
			}
		})
	}
}

func TestEquivalenceTOST_Upper95Calibration(t *testing.T) {
	// One-sided 95% upper bound for df=1: mean + cot(.05*pi)*SE.
	// It remains fixed when the caller changes the TOST alpha.
	for _, alpha := range []float64{.05, .25} {
		r, err := rs.EquivalenceTOST([]float64{-3, -1}, []float64{0, 0}, 1, alpha)
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		var got struct {
			Upper95 *float64 `json:"upper_95"`
		}
		if err := json.Unmarshal(encoded, &got); err != nil {
			t.Fatal(err)
		}
		want := -2 + 1/math.Tan(.05*math.Pi)
		if got.Upper95 == nil || !near(*got.Upper95, want, 1e-10) {
			t.Errorf("alpha=%v got %s, want upper_95=%v", alpha, encoded, want)
		}
	}
}
