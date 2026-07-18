// Package expresslane provides the express-lane fast-path gate for agentic-kit.
//
// Assess inspects a Task and deterministically decides whether it can be
// handled on the fast lane (skipping the full pipeline) or must go through the
// full pipeline. The decision is based on two sequential checks:
//
//  1. Size check — tasks that exceed configured byte or token thresholds are
//     too large for the fast lane and are routed to the full pipeline.
//
//  2. Spec-challenge gate — tasks that arrive with an incomplete or ambiguous
//     specification (missing required fields or zero estimated complexity) are
//     also routed to the full pipeline.
//
// Both checks are deterministic: given the same Task and Config, Assess always
// returns the same Decision.
//
// All functions in this package are pure and safe for concurrent use.
package expresslane

// Lane identifies which processing path a task should take.
type Lane string

const (
	// LaneFast means the task may be handled by the express (fast) lane.
	LaneFast Lane = "fast"
	// LaneFull means the task must go through the full pipeline.
	LaneFull Lane = "full"
)

// Reason identifies why a particular Lane was chosen.
type Reason string

const (
	// ReasonSizeExceeded means the task's payload exceeded the size threshold.
	ReasonSizeExceeded Reason = "size_exceeded"
	// ReasonSpecIncomplete means the task specification failed the spec-challenge gate.
	ReasonSpecIncomplete Reason = "spec_incomplete"
	// ReasonEligible means the task passed all gates and qualifies for the fast lane.
	ReasonEligible Reason = "eligible"
)

// Task is the unit of work assessed by the express-lane gate.
type Task struct {
	// ID is a stable identifier for the task.
	ID string
	// Payload is the raw content of the task (e.g. serialized request body).
	Payload []byte
	// Spec is the structured specification of what the task requires.
	Spec TaskSpec
}

// TaskSpec carries the declarative specification of a task.
type TaskSpec struct {
	// Description is a human-readable summary of the task.
	Description string
	// EstimatedComplexity is the caller's estimate of complexity on a 1–5 scale.
	// A value of 0 means unspecified and fails the spec-challenge gate.
	EstimatedComplexity int
	// RequiredCapabilities lists the capabilities the task needs.
	RequiredCapabilities []string
}

// Decision is the result of Assess. It is immutable once returned.
type Decision struct {
	// TaskID echoes the assessed task's ID for traceability.
	TaskID string
	// Lane is the routing decision.
	Lane Lane
	// Reason explains why the Lane was chosen.
	Reason Reason
}

// Config controls the thresholds used by the express-lane gate.
// A zero-value Config uses sensible defaults (see DefaultConfig).
type Config struct {
	// MaxPayloadBytes is the maximum payload size in bytes that qualifies
	// for the fast lane. Tasks larger than this are routed to LaneFull.
	// Default: 65536 (64 KiB).
	MaxPayloadBytes int
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		MaxPayloadBytes: 64 * 1024,
	}
}
