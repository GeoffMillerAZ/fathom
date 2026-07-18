// Package operatorelicitation defines the CUE contract for operator elicitation records.
// These constraints mirror the Go types and are evaluated with `cue vet ./...`.
package operatorelicitation

// #Request is a single elicitation request directed at a human operator.
#Request: {
	// ID is a stable, non-empty identifier for this request.
	ID: string & !=""

	// Prompt is the question or instruction presented to the operator.
	Prompt: string & !=""

	// Answer is the operator's response, or "" if not yet answered.
	Answer: string

	// Resolved is true once the operator has provided an answer.
	Resolved: bool
}
