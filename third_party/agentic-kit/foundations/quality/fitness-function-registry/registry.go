package fitnessfunctionregistry

import (
	"fmt"
	"sort"
	"time"

	evalgate "github.com/geoffmilleraz/agentic-kit/foundations/quality/eval-gate"
)

// Registry is a deterministic, name-keyed catalogue of FitnessFunctions.
//
// The zero value is NOT ready for use; construct one with NewRegistry. All
// iteration over a Registry (List, ExecuteAll) is sorted by function name so
// that output never depends on map ordering. Registry is not safe for
// concurrent mutation; guard it with your own lock if you register from
// multiple goroutines.
type Registry struct {
	funcs map[string]FitnessFunction
}

// NewRegistry returns an empty, ready-to-use Registry.
func NewRegistry() *Registry {
	return &Registry{funcs: make(map[string]FitnessFunction)}
}

// Register adds ff to the registry. It returns an error when ff.Name is empty,
// when ff.Tier is set but unrecognised, or when a function with the same Name
// is already registered. Registration is rejected as a whole on any error; the
// registry is left unchanged.
//
// A FitnessFunction with no Checks is permitted (it will score 0 on Execute); a
// missing Name is not.
func (r *Registry) Register(ff FitnessFunction) error {
	if r.funcs == nil {
		return fmt.Errorf("fitnessfunctionregistry: Registry not initialised; use NewRegistry")
	}
	if ff.Name == "" {
		return fmt.Errorf("fitnessfunctionregistry: FitnessFunction.Name must not be empty")
	}
	if ff.Tier != "" && !ff.Tier.Valid() {
		return fmt.Errorf("fitnessfunctionregistry: FitnessFunction %q has unrecognised Tier %q (want prototype|integration|apex)", ff.Name, ff.Tier)
	}
	if ff.Threshold < 0 || ff.Threshold > 1 {
		return fmt.Errorf("fitnessfunctionregistry: FitnessFunction %q Threshold %g is outside [0, 1]", ff.Name, ff.Threshold)
	}
	if _, exists := r.funcs[ff.Name]; exists {
		return fmt.Errorf("fitnessfunctionregistry: FitnessFunction %q is already registered", ff.Name)
	}
	r.funcs[ff.Name] = ff
	return nil
}

// Get returns the registered FitnessFunction with the given name and ok=true,
// or the zero value and ok=false when no such function exists.
func (r *Registry) Get(name string) (ff FitnessFunction, ok bool) {
	if r.funcs == nil {
		return FitnessFunction{}, false
	}
	ff, ok = r.funcs[name]
	return ff, ok
}

// List returns every registered FitnessFunction sorted by Name. The result is a
// fresh slice the caller may retain or mutate; the registry's own copies are
// unaffected. Returns a non-nil empty slice when the registry is empty.
func (r *Registry) List() []FitnessFunction {
	out := make([]FitnessFunction, 0, len(r.funcs))
	for _, ff := range r.funcs {
		out = append(out, ff)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

// Len reports how many functions are registered.
func (r *Registry) Len() int {
	return len(r.funcs)
}

// Execute scores a single FitnessFunction deterministically and returns an
// ExecutionResult.
//
// Scoring is delegated to quality/eval-gate: the function's Checks are run
// through an evalgate.Gate configured with the function's effective threshold
// (ff.Threshold, or 1.0 when zero — matching evalgate's own zero-Threshold
// semantics). Score, Passed, and Failures come straight from eval-gate, so the
// weighted-check algorithm is never reimplemented here.
//
// Determinism: Execute is a pure function of (ff, runID, at). It never reads the
// wall clock, never generates IDs, and never iterates a map. The same inputs
// always yield byte-identical output (Failures preserves the input order of
// Checks, exactly as eval-gate returns it). The returned Duration is always 0;
// use ExecuteTimed when a measured duration is wanted.
//
// A function with no Checks yields Score 0; since the effective threshold is
// always > 0 (a zero ff.Threshold is promoted to 1.0), Passed is false. This
// makes an unimplemented invariant fail closed rather than silently pass.
func Execute(ff FitnessFunction, runID string, at time.Time) ExecutionResult {
	gate := evalgate.Gate{Threshold: ff.Threshold}
	res := gate.Evaluate(ff.Checks)

	return ExecutionResult{
		FunctionName: ff.Name,
		Score:        res.Score,
		Passed:       res.Passed,
		Failures:     res.Failures,
		RunID:        runID,
		At:           at,
		Duration:     0,
	}
}

// ExecuteTimed is Execute with wall-clock timing: it measures how long the
// (pure) scoring takes and records it in ExecutionResult.Duration. Every other
// field is identical to Execute(ff, runID, at).
//
// Because it reads the monotonic clock, ExecuteTimed is NOT deterministic in
// the Duration field — Duration is wall-clock and will vary run to run. All
// other fields remain a pure function of (ff, runID, at). Use Execute when you
// need fully reproducible output (e.g. golden tests); use ExecuteTimed when you
// also want a cost signal for a scorecard.
func ExecuteTimed(ff FitnessFunction, runID string, at time.Time) ExecutionResult {
	start := time.Now()
	out := Execute(ff, runID, at)
	out.Duration = time.Since(start)
	return out
}

// ExecuteAll runs every registered function through Execute, in Name order, and
// returns the results in that same deterministic order. runID and at are passed
// through to each Execute call unchanged, so a single ExecuteAll produces one
// coherent, reproducible batch of results sharing a run identity.
//
// ExecuteAll returns a sorted, run-stamped slice of ExecutionResults that
// callers can gate, aggregate, or persist as evidence.
func (r *Registry) ExecuteAll(runID string, at time.Time) []ExecutionResult {
	funcs := r.List() // already sorted by Name
	out := make([]ExecutionResult, 0, len(funcs))
	for _, ff := range funcs {
		out = append(out, Execute(ff, runID, at))
	}
	return out
}
