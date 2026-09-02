package reliabilitystats

import (
	"math"
	"math/rand"
	"sort"
)

// PairedBootstrapCI returns a percentile bootstrap confidence interval
// for the mean paired difference in obs, resampling TASKS with
// replacement: every trial of a task travels with its task, so k
// repeated trials of one task count as one unit of evidence, not k.
//
// The statistic is the mean over tasks of each task's mean Diff (every
// task weighs the same regardless of how many trials it has). Point is
// that statistic on the observed sample; Low and High are the
// (1−level)/2 and 1−(1−level)/2 quantiles of resamples replicates
// (linear interpolation between order statistics); Tasks is the number
// of distinct clusters. Observations are ordered by (Task, Diff) before
// anything is summed or sampled, so the result depends on the sample and
// the seed, never on input order.
//
// Deterministic: replicates come from math/rand.New(rand.NewSource(seed))
// and are byte-stable across runs for a given seed, sample, and
// resamples count.
//
// Errors: ErrEmptySample; ErrInvalidSample (a NaN or Inf Diff);
// ErrInvalidResamples (resamples < 1); ErrInvalidLevel (level outside
// the open interval (0, 1)).
func PairedBootstrapCI(obs []Observation, seed int64, resamples int, level float64) (Interval, error) {
	if len(obs) == 0 {
		return Interval{}, ErrEmptySample
	}
	if resamples < 1 {
		return Interval{}, ErrInvalidResamples
	}
	if !(level > 0 && level < 1) { // also rejects NaN
		return Interval{}, ErrInvalidLevel
	}
	for _, o := range obs {
		if math.IsNaN(o.Diff) || math.IsInf(o.Diff, 0) {
			return Interval{}, ErrInvalidSample
		}
	}

	ordered := make([]Observation, len(obs))
	copy(ordered, obs)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Task != ordered[j].Task {
			return ordered[i].Task < ordered[j].Task
		}
		return ordered[i].Diff < ordered[j].Diff
	})

	// Per-task means, in task-id order.
	var means []float64
	for i := 0; i < len(ordered); {
		j := i
		var sum float64
		for j < len(ordered) && ordered[j].Task == ordered[i].Task {
			sum += ordered[j].Diff
			j++
		}
		means = append(means, sum/float64(j-i))
		i = j
	}
	t := len(means)
	point := mean(means)

	rng := rand.New(rand.NewSource(seed))
	reps := make([]float64, resamples)
	draw := make([]float64, t)
	for b := range reps {
		for i := range draw {
			draw[i] = means[rng.Intn(t)]
		}
		reps[b] = mean(draw)
	}
	sort.Float64s(reps)

	alpha := 1 - level
	return Interval{
		Low:   quantile(reps, alpha/2),
		High:  quantile(reps, 1-alpha/2),
		Point: point,
		Tasks: t,
	}, nil
}

// mean returns the arithmetic mean of xs, summed in slice order (callers
// pass deterministically ordered slices).
func mean(xs []float64) float64 {
	var sum float64
	for _, x := range xs {
		sum += x
	}
	return sum / float64(len(xs))
}

// quantile returns the q-th quantile of sorted (ascending, non-empty) by
// linear interpolation between order statistics (R type 7, the NumPy
// default): position q·(n−1), interpolated between its floor and ceiling
// neighbours.
func quantile(sorted []float64, q float64) float64 {
	n := len(sorted)
	if n == 1 {
		return sorted[0]
	}
	pos := q * float64(n-1)
	lo := int(math.Floor(pos))
	hi := lo + 1
	if hi >= n {
		return sorted[n-1]
	}
	frac := pos - float64(lo)
	return sorted[lo] + frac*(sorted[hi]-sorted[lo])
}
