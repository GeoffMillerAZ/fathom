// Package event defines the CUE-first contracts for the telemetry event.
// Go types in types.go mirror these definitions; any drift between the two
// is a bug.
//
// Run `cue vet ./...` from this directory (or the repo root) to validate
// testdata/valid.cue against these constraints.
package event

// _nonEmpty is a reusable constraint for non-empty strings.
_nonEmpty: string & !=""

// #Kind enumerates the valid telemetry event categories.
// Mirrors the KindXxx constants in types.go.
#Kind: "tool_call" | "llm_call" | "error" | "info"

// #Attributes constrains the per-event key–value metadata bag.
// All keys and values are strings; the map may be empty.
//
// The "trace." key namespace is reserved for run/trace correlation stamped
// by event.Apply in trace.go: "trace.run_id", "trace.span_id", and
// "trace.parent_span_id". They are ordinary string attributes — the map
// stays open and unconstrained — but emitters must not set them directly.
#Attributes: {[string]: string}

// #Event is the atomic telemetry unit emitted by every agentic component.
// Every field except Attributes is required and non-empty after normalisation.
#Event: {
	// ID uniquely identifies this event within the telemetry stream.
	ID: _nonEmpty

	// Kind classifies the event.
	Kind: #Kind

	// Source identifies the emitting component (lower_snake_case path).
	Source: _nonEmpty

	// Timestamp is an RFC 3339 timestamp for when the event occurred.
	Timestamp: _nonEmpty

	// Attributes holds arbitrary normalised key–value metadata.
	// The field is optional; an absent Attributes map is valid.
	Attributes?: #Attributes
}
