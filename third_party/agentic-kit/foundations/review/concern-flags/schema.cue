// Package concernflags defines the CUE schema for concern-flag records.
// These constraints mirror the Go types and are evaluated with `cue vet ./...`.
package concernflags

// #Severity enumerates the valid urgency levels for a concern flag.
#Severity: "low" | "medium" | "high" | "critical"

// #ConcernFlag is a typed, severity-annotated concern raised against a target.
#ConcernFlag: {
	// ID is a non-empty stable identifier.
	ID: string & !=""

	// Type classifies the concern category (e.g. "accuracy", "dlp").
	Type: string & !=""

	// Severity is the urgency level of the concern.
	Severity: #Severity

	// Target identifies the entity this concern applies to.
	Target: string & !=""

	// Rationale is a human-readable explanation of the concern.
	Rationale: string
}
