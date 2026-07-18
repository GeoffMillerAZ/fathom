// Package proberuntime runs registered probes and collects pass/fail results.
//
// Usage:
//
//	rt := NewRuntime()
//	rt.Register("db-ping", ProbeFunc(func(ctx context.Context) ProbeResult {
//	    // ... check the database ...
//	    return ProbeResult{Name: "db-ping", Status: StatusPass}
//	}))
//	results := rt.RunAll(ctx)
//
// RunAll executes probes in deterministic (alphabetical-by-name) order so test
// output is stable regardless of registration order. Panics inside probes are
// recovered and reported as StatusError results.
//
// Register is not safe to call concurrently with any other method; once
// registration is complete, concurrent RunAll calls are safe (RunAll only
// reads the registry, though the probes themselves must tolerate concurrent
// execution if called that way).
package proberuntime

import (
	"context"
	"errors"
	"fmt"
)

// ProbeStatus represents the outcome of a single probe execution.
type ProbeStatus string

const (
	// StatusPass indicates the probe completed without issues.
	StatusPass ProbeStatus = "pass"

	// StatusFail indicates the probe detected a problem.
	StatusFail ProbeStatus = "fail"

	// StatusError indicates the probe itself encountered an unexpected error.
	StatusError ProbeStatus = "error"
)

// ErrDuplicateName is returned when Register is called with a name that is
// already registered.
var ErrDuplicateName = errors.New("proberuntime: duplicate probe name")

// ErrEmptyName is returned when Register is called with an empty name.
var ErrEmptyName = errors.New("proberuntime: probe name must not be empty")

// ProbeResult is the outcome of a single probe execution.
type ProbeResult struct {
	// Name is the registered name of the probe.
	Name string

	// Status is the execution outcome.
	Status ProbeStatus

	// Detail is an optional human-readable description.
	Detail string
}

// Probe is the interface that all probes must implement.
type Probe interface {
	// Run executes the probe and returns its result.
	// Implementations should respect ctx cancellation.
	// The returned ProbeResult.Name is overwritten by the Runtime with the
	// registered name, so implementations may leave it empty.
	Run(ctx context.Context) ProbeResult
}

// ProbeFunc is a function that implements the Probe interface.
type ProbeFunc func(ctx context.Context) ProbeResult

// Run implements Probe.
func (f ProbeFunc) Run(ctx context.Context) ProbeResult { return f(ctx) }

// Runtime manages a registry of named probes and executes them.
// It is not safe for concurrent use from multiple goroutines.
type Runtime struct {
	order  []string
	probes map[string]Probe
}

// NewRuntime returns an empty Runtime.
func NewRuntime() *Runtime {
	return &Runtime{probes: make(map[string]Probe)}
}

// Register adds a probe under the given name.
// Returns ErrEmptyName if name is empty.
// Returns ErrDuplicateName if a probe with that name is already registered.
func (rt *Runtime) Register(name string, p Probe) error {
	if name == "" {
		return ErrEmptyName
	}
	if _, exists := rt.probes[name]; exists {
		return ErrDuplicateName
	}
	rt.probes[name] = p
	rt.order = append(rt.order, name)
	return nil
}

// RunAll executes all registered probes in deterministic (alphabetical) order
// and returns their results. Panics inside probes are recovered and reported as
// StatusError results with a detail describing the panic.
func (rt *Runtime) RunAll(ctx context.Context) []ProbeResult {
	// Sort names for deterministic execution order.
	names := make([]string, len(rt.order))
	copy(names, rt.order)
	sortStrings(names)

	results := make([]ProbeResult, 0, len(names))
	for _, name := range names {
		p := rt.probes[name]
		result := runSafe(ctx, name, p)
		result.Name = name
		results = append(results, result)
	}
	return results
}

// runSafe executes p.Run(ctx) and recovers from any panic.
func runSafe(ctx context.Context, name string, p Probe) (result ProbeResult) {
	defer func() {
		if r := recover(); r != nil {
			result = ProbeResult{
				Name:   name,
				Status: StatusError,
				Detail: fmt.Sprintf("probe panicked: %v", r),
			}
		}
	}()
	return p.Run(ctx)
}

// sortStrings sorts a slice of strings in place using insertion sort.
func sortStrings(ss []string) {
	for i := 1; i < len(ss); i++ {
		for j := i; j > 0 && ss[j] < ss[j-1]; j-- {
			ss[j], ss[j-1] = ss[j-1], ss[j]
		}
	}
}
