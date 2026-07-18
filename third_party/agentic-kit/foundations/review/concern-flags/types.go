// Package concernflags provides types and a registry for raising, listing,
// and querying concern flags surfaced during review.
//
// Concurrency: Register is safe for concurrent use via sync.RWMutex.
package concernflags

// Severity represents the urgency level of a concern flag.
type Severity string

const (
	// SeverityLow is an informational concern with minimal impact.
	SeverityLow Severity = "low"

	// SeverityMedium is a moderate concern that should be addressed.
	SeverityMedium Severity = "medium"

	// SeverityHigh is a serious concern requiring prompt attention.
	SeverityHigh Severity = "high"

	// SeverityCritical is an urgent concern that must be addressed immediately.
	SeverityCritical Severity = "critical"
)

// FlagType classifies the concern category.
type FlagType string

// ConcernFlag is a typed, severity-annotated concern raised against a target
// entity during review.
type ConcernFlag struct {
	// ID is the unique, stable identifier for this flag.
	ID string

	// Type classifies the concern (e.g. "accuracy", "dlp", "completeness").
	Type FlagType

	// Severity indicates how urgent the concern is.
	Severity Severity

	// Target identifies the entity (document section, claim, field, etc.)
	// this concern applies to.
	Target string

	// Rationale is a human-readable explanation of why the concern was raised.
	Rationale string
}
