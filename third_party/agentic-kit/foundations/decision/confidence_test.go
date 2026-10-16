// Package decision_test exercises the fast-decision port via external
// black-box tests. All tests are table-driven and race-safe.
package decision_test

import (
	"math"
	"testing"

	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

func floatsClose(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func TestConfidence(t *testing.T) {
	tests := []struct {
		name  string
		probs map[string]float64
		want  float64
	}{
		{"nil map (k=0) is certain", nil, 1},
		{"single entry (k=1) is certain regardless of value", map[string]float64{"a": 0.01}, 1},
		{"k=2 basic", map[string]float64{"a": 0.7, "b": 0.3}, 0.4}, // (2*0.7-1)/1
		{"k=2 max=1 -> 1", map[string]float64{"a": 1.0, "b": 0.0}, 1},
		{"k=2 max=0.5 (uniform) -> 0", map[string]float64{"a": 0.5, "b": 0.5}, 0},
		{"k=3 identity (3*max-1)/2, max=0.6", map[string]float64{"a": 0.6, "b": 0.3, "c": 0.1}, 0.4},
		{"k=3 identity, max=1.0", map[string]float64{"a": 1.0, "b": 0.0, "c": 0.0}, 1},
		{"k=3 identity, max=1/3 (uniform) -> 0", map[string]float64{"a": 1.0 / 3, "b": 1.0 / 3, "c": 1.0 / 3}, 0},
		{"k=5 low max clamps to 0", map[string]float64{"a": 0.1, "b": 0.1, "c": 0.1, "d": 0.1, "e": 0.1}, 0},
		{"out-of-range max clamps to 1 (formula alone would give 2)", map[string]float64{"a": 1.5, "b": 0.0}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decision.Confidence(tt.probs)
			if !floatsClose(got, tt.want) {
				t.Errorf("Confidence(%v) = %v, want %v", tt.probs, got, tt.want)
			}
			if got < 0 || got > 1 {
				t.Errorf("Confidence(%v) = %v, out of [0,1]", tt.probs, got)
			}
		})
	}
}

func TestBandOf(t *testing.T) {
	tests := []struct {
		name       string
		confidence float64
		want       decision.Band
	}{
		{"just below unsure threshold", decision.UnsureBelow - 0.0001, decision.BandUnsure},
		{"exactly at unsure threshold is confirm, not unsure", decision.UnsureBelow, decision.BandConfirm},
		{"mid-band", 0.75, decision.BandConfirm},
		{"just below proceed threshold", decision.ProceedAtOrAbove - 0.0001, decision.BandConfirm},
		{"exactly at proceed threshold is proceed", decision.ProceedAtOrAbove, decision.BandProceed},
		{"above proceed threshold", 1.0, decision.BandProceed},
		{"zero", 0.0, decision.BandUnsure},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := decision.BandOf(tt.confidence); got != tt.want {
				t.Errorf("BandOf(%v) = %v, want %v", tt.confidence, got, tt.want)
			}
		})
	}
}
