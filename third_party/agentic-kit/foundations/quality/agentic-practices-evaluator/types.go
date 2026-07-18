// Package agenticpracticesevaluator scores an agent run against a configurable
// best-practice checklist. Checklist items (Practice) are data — callers
// supply them (from CUE or programmatically). The evaluator is deterministic:
// for a given (AgentRun, []Practice) pair the Scorecard is always the same.
//
// Concurrency: values are immutable and safe for concurrent use.
package agenticpracticesevaluator

// Practice is a single best-practice checklist item. The Check function is
// the deterministic predicate that decides whether the run satisfies this
// practice. Name and Description come from the caller's data source.
type Practice struct {
	// Name is a unique, human-readable identifier for the practice
	// (e.g. "uses-evidence-citations").
	Name string

	// Description is a human-readable explanation of what the practice checks.
	Description string

	// Check is the deterministic predicate evaluated against an AgentRun.
	// It returns true when the run satisfies this practice.
	Check func(run AgentRun) bool
}

// AgentRun holds the observable attributes of a completed agent execution.
// Fields are intentionally generic so the evaluator is org-agnostic.
type AgentRun struct {
	// Steps is the ordered sequence of step names the agent performed.
	Steps []string

	// Citations is the list of evidence citations produced by the run.
	Citations []string

	// Metadata holds arbitrary key/value pairs attached to the run.
	// Checklist predicates may inspect these values.
	Metadata map[string]string
}

// Miss records a practice that was not satisfied by the run.
type Miss struct {
	// PracticeName is the Name of the unsatisfied Practice.
	PracticeName string

	// Description is copied from the Practice for caller convenience.
	Description string
}

// Scorecard is the outcome of an Evaluate call.
type Scorecard struct {
	// Score is a normalised pass-rate in [0.0, 1.0].
	// 1.0 means every practice passed; 0.0 means none passed.
	// If no practices are configured, Score is 1.0.
	Score float64

	// Misses contains one entry for each practice the run failed.
	Misses []Miss
}
