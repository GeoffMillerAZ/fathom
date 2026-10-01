package decision

import "context"

// None is the zero-configuration Decider used when no fast-decision
// provider is wired up. It never calls out; Decide always fails with
// ErrNoProvider so callers can detect "not configured" via errors.Is
// instead of inspecting a nil-vs-real-provider flag at every call site.
//
// Concurrency: None is stateless and safe for concurrent use.
type None struct{}

// Name returns "none".
func (None) Name() string { return "none" }

// Decide always returns a Response naming Provider "none" and
// ErrNoProvider.
func (None) Decide(_ context.Context, _ Request) (Response, error) {
	return Response{Provider: "none"}, ErrNoProvider
}

// Capabilities declares that None answers no kind, bills nothing, makes no
// round trip and reports no probabilities: every Request fails with
// ErrNoProvider before any work.
func (None) Capabilities() Capabilities {
	return Capabilities{
		Declared:            true,
		Latency:             LatencyNone,
		Cost:                CostNone,
		ConfidenceSemantics: ConfidenceNone,
	}
}
