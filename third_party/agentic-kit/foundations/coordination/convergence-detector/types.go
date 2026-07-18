// Package convergencedetector decides when a stream of parallel-worker results
// has converged on a stable answer, is still exploring, or should be abandoned.
// It is a pure decision function: it does NOT spawn workers or manage execution.
//
// Plan-only status (D7): this package ships as a planner primitive. The worker
// executor (kits/worker-pool with agent-harness as the worker unit) is not yet
// built; it will be added when a real fan-out workload subscribes.
//
// The Detector is fully deterministic: given the same sequence of Feed calls
// it always produces the same Verdict. It uses no internal clocks or random
// sources.
//
// Concurrency contract: Detector is not safe for concurrent use; callers that
// share a Detector across goroutines must guard access with their own
// synchronization.
package convergencedetector

import "errors"

// Verdict is the convergence decision produced by the Detector.
type Verdict string

const (
	// VerdictConverged means enough signals in the current window agree on one value.
	VerdictConverged Verdict = "converged"

	// VerdictExploring means the window is not yet stable but the signal budget is not exhausted.
	VerdictExploring Verdict = "exploring"

	// VerdictAbandoned means the signal budget is exhausted without convergence.
	VerdictAbandoned Verdict = "abandoned"
)

// ErrInvalidPolicy is returned when a Policy fails validation.
var ErrInvalidPolicy = errors.New("convergencedetector: invalid policy")

// Policy configures the convergence detector.
type Policy struct {
	// Window is the number of most-recent signals considered for convergence.
	// Must be >= 1.
	Window int

	// Threshold is the minimum fraction of signals in the window that must
	// share the same value to declare convergence. Must be in (0, 1].
	Threshold float64

	// AbandonAfter is the maximum total signal count before the detector
	// declares abandoned if convergence has not been reached. Must be >= 1.
	AbandonAfter int
}

// validate returns an error if the policy is not valid.
func (p Policy) validate() error {
	if p.Window < 1 {
		return ErrInvalidPolicy
	}
	if p.Threshold <= 0 || p.Threshold > 1 {
		return ErrInvalidPolicy
	}
	if p.AbandonAfter < 1 {
		return ErrInvalidPolicy
	}
	return nil
}

// Signal is a single result value emitted by a worker.
type Signal struct {
	// Value is the string-keyed result from the worker.
	Value string
}

// DetectorState is the observable state of a Detector after processing signals.
type DetectorState struct {
	// SignalCount is the total number of signals received so far.
	SignalCount int

	// Verdict is the most recent convergence decision.
	Verdict Verdict
}

// Detector receives a stream of result signals and decides at each step
// whether the swarm has converged, is still exploring, or should be abandoned.
//
// It is intentionally simple and stateless between Feed calls — all state
// is captured in the ring buffer of recent signals.
type Detector struct {
	policy  Policy
	signals []string // ring buffer of the last Window signals
	total   int      // total signals received
}

// NewDetector creates a Detector governed by the given Policy.
// Returns ErrInvalidPolicy if the policy fails validation.
func NewDetector(p Policy) (*Detector, error) {
	if err := p.validate(); err != nil {
		return nil, err
	}
	return &Detector{
		policy:  p,
		signals: make([]string, 0, p.Window),
	}, nil
}

// Feed records one result signal and returns the current Verdict.
// The logic is:
//  1. Append the signal value to the window, evicting the oldest if full.
//  2. If total signals >= AbandonAfter AND not converged → abandoned.
//  3. If the window has >= Window signals AND the most-frequent value appears
//     in at least Threshold fraction → converged.
//  4. Otherwise → exploring.
//
// Feed is deterministic: identical call sequences always produce identical results.
func (d *Detector) Feed(s Signal) Verdict {
	d.total++

	// Maintain a sliding window of the last Policy.Window values.
	if len(d.signals) < d.policy.Window {
		d.signals = append(d.signals, s.Value)
	} else {
		// Shift left to evict the oldest entry, then append.
		copy(d.signals, d.signals[1:])
		d.signals[len(d.signals)-1] = s.Value
	}

	// Check convergence only when the window is full.
	if len(d.signals) >= d.policy.Window {
		dominant, count := mostFrequent(d.signals)
		_ = dominant
		frac := float64(count) / float64(len(d.signals))
		if frac >= d.policy.Threshold {
			return VerdictConverged
		}
	}

	// Check abandon budget.
	if d.total >= d.policy.AbandonAfter {
		return VerdictAbandoned
	}

	return VerdictExploring
}

// State returns the current observable state of the detector.
func (d *Detector) State() DetectorState {
	return DetectorState{
		SignalCount: d.total,
		Verdict:     d.currentVerdict(),
	}
}

// currentVerdict recomputes the verdict without advancing state.
func (d *Detector) currentVerdict() Verdict {
	if len(d.signals) >= d.policy.Window {
		_, count := mostFrequent(d.signals)
		frac := float64(count) / float64(len(d.signals))
		if frac >= d.policy.Threshold {
			return VerdictConverged
		}
	}
	if d.total >= d.policy.AbandonAfter {
		return VerdictAbandoned
	}
	return VerdictExploring
}

// mostFrequent returns the most common string in ss and its count.
// Ties are broken lexicographically (smallest string wins) for determinism.
func mostFrequent(ss []string) (string, int) {
	counts := make(map[string]int, len(ss))
	for _, s := range ss {
		counts[s]++
	}
	best := ""
	bestCount := 0
	for _, s := range ss {
		c := counts[s]
		if c > bestCount || (c == bestCount && s < best) {
			best = s
			bestCount = c
		}
	}
	return best, bestCount
}
