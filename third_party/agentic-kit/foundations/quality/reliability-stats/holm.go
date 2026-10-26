package reliabilitystats

import (
	"errors"
	"math"
	"sort"
)

// ErrInvalidPValue is returned when a p-value is not finite or outside [0, 1].
var ErrInvalidPValue = errors.New("reliabilitystats: p-value must be in [0, 1]")

// HolmAdjust returns Holm's step-down family-wise error adjustment in
// the original input order. In sorted order, adjusted p[i] is
// min(1, max over j<=i of (m-j)*p[j]), with zero-based indices.
// Equal p-values receive equal adjustments; input is never mutated.
// An empty family returns nil, nil. Invalid probabilities return
// ErrInvalidPValue. Tests do not call this implicitly: callers define
// the complete comparison family before inspecting its results.
// CUE mirror for the input and output: #PValues in holm.cue.
func HolmAdjust(p []float64) ([]float64, error) {
	order := make([]int, len(p))
	for i, v := range p {
		if math.IsNaN(v) || v < 0 || v > 1 {
			return nil, ErrInvalidPValue
		}
		order[i] = i
	}
	if len(p) == 0 {
		return nil, nil
	}
	sort.SliceStable(order, func(i, j int) bool { return p[order[i]] < p[order[j]] })
	adjusted := make([]float64, len(p))
	var running float64
	for rank, i := range order {
		running = math.Max(running, math.Min(1, float64(len(p)-rank)*p[i]))
		adjusted[i] = running
	}
	return adjusted, nil
}
