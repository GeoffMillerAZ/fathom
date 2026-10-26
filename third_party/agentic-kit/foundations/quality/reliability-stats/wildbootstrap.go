package reliabilitystats

import (
	"errors"
	"math"
	"math/rand"
	"sort"
)

// ErrInsufficientClusters is returned when a wild bootstrap has fewer
// than two distinct clusters: cluster residual variance is undefined.
var ErrInsufficientClusters = errors.New("reliabilitystats: at least two clusters required")

// WildClusterBootstrapCI returns an unstudentized percentile wild-cluster
// interval for the equally weighted mean of task means, independent of the
// cluster key. Each task first averages its trials. Cluster identifies the
// independent multiplier group (empty defaults to Task for task clustering).
// For T tasks with means m[t], grand mean m and G clusters, a cluster's
// residual is sum over its tasks of (m[t]-m)/T. Each replicate is m plus
// the sum of one Webb weight per cluster times its residual times
// sqrt(G/(G-1)). Tasks reports T and Clusters reports G. Repeating a task's
// trials cannot change its weight. Tasks must nest within clusters.
// Webb's six weights {-sqrt(3/2), -1, -sqrt(1/2), sqrt(1/2), 1, sqrt(3/2)}
// each have probability 1/6. Bounds use the existing type-7 quantile rule.
//
// The RNG is local and seeded; sorting by (Task, Diff) makes the result
// independent of input order. Input is never mutated. Independent clusters
// and a meaningful number of them remain caller assumptions; this is not
// a studentized interval or a guarantee of small-cluster coverage.
//
// Errors: ErrEmptySample; ErrInsufficientClusters; ErrInvalidResamples;
// ErrInvalidLevel; ErrInconsistentCluster (a task belongs to multiple
// clusters); ErrInvalidSample (empty task, nonfinite difference or arithmetic overflow).
// CUE mirror: #WildClusterInterval in wildbootstrap.cue.
func WildClusterBootstrapCI(obs []Observation, seed int64, resamples int, level float64) (Interval, error) {
	if len(obs) == 0 {
		return Interval{}, ErrEmptySample
	}
	if resamples < 1 {
		return Interval{}, ErrInvalidResamples
	}
	if !(level > 0 && level < 1) {
		return Interval{}, ErrInvalidLevel
	}
	for _, o := range obs {
		if o.Task == "" || math.IsNaN(o.Diff) || math.IsInf(o.Diff, 0) {
			return Interval{}, ErrInvalidSample
		}
	}
	ordered := append([]Observation(nil), obs...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Task != ordered[j].Task {
			return ordered[i].Task < ordered[j].Task
		}
		return ordered[i].Diff < ordered[j].Diff
	})
	var means []float64
	var taskClusters []string
	for i := 0; i < len(ordered); {
		j := i + 1
		for j < len(ordered) && ordered[j].Task == ordered[i].Task {
			j++
		}
		cluster := ordered[i].Cluster
		if cluster == "" {
			cluster = ordered[i].Task
		}
		var m float64
		for k := i; k < j; k++ {
			key := ordered[k].Cluster
			if key == "" {
				key = ordered[k].Task
			}
			if key != cluster {
				return Interval{}, ErrInconsistentCluster
			}
			m += ordered[k].Diff / float64(j-i)
		}
		means = append(means, m)
		taskClusters = append(taskClusters, cluster)
		i = j
	}
	tasks := len(means)
	var point float64
	for _, m := range means {
		point += m / float64(tasks)
	}
	byCluster := make(map[string]float64)
	for i, m := range means {
		byCluster[taskClusters[i]] += (m - point) / float64(tasks)
	}
	keys := make([]string, 0, len(byCluster))
	for key := range byCluster {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	g := len(keys)
	if g < 2 {
		return Interval{}, ErrInsufficientClusters
	}
	correction := math.Sqrt(float64(g) / float64(g-1))
	residuals := make([]float64, g)
	for i, key := range keys {
		residuals[i] = byCluster[key] * correction
	}
	weights := [...]float64{-math.Sqrt(1.5), -1, -math.Sqrt(.5), math.Sqrt(.5), 1, math.Sqrt(1.5)}
	rng := rand.New(rand.NewSource(seed))
	reps := make([]float64, resamples)
	for b := range reps {
		var delta float64
		for _, residual := range residuals {
			delta += float64(weights[rng.Intn(len(weights))] * residual)
		}
		reps[b] = point + delta
		if math.IsNaN(reps[b]) || math.IsInf(reps[b], 0) {
			return Interval{}, ErrInvalidSample
		}
	}
	sort.Float64s(reps)
	alpha := 1 - level
	return Interval{Low: quantile(reps, alpha/2), High: quantile(reps, 1-alpha/2), Point: point, Tasks: tasks, Clusters: g}, nil
}
