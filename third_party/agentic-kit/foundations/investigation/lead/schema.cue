// Package lead defines the CUE-first contracts for investigative lead tracking.
// Go types in types.go and log.go mirror these definitions; any drift is a bug.
//
// The #Author shape is imported directly from the ai-provenance package so that
// any upstream authorship constraint change is caught at `cue vet` time rather
// than silently diverging from the canonical definition.
package lead

import ap "github.com/geoffmilleraz/agentic-kit/foundations/provenance/ai-provenance:aiprovenance"

// _nonEmpty is a reusable constraint for non-empty strings.
_nonEmpty: string & !=""

// #LeadStatus enumerates the four valid lifecycle states of a Lead.
#LeadStatus: "open" | "pursuing" | "closed" | "abandoned"

// #Lead represents a single provenance-bearing investigative thread. Every
// field is carried in the immutable snapshot stored in each #LeadEntry.
#Lead: {
	// ID is a non-empty stable identifier.
	ID: _nonEmpty

	// Prompt is the human-readable question or hypothesis the lead addresses.
	Prompt: _nonEmpty

	// HypothesisID optionally links this lead to a parent hypothesis.
	HypothesisID?: string

	// TriggerSpanIDs holds the IDs of evidence spans that triggered this lead.
	TriggerSpanIDs: [...string]

	// Assignee is the identifier of the assigned person or agent. May be empty.
	Assignee?: string

	// Status is the current lifecycle state.
	Status: #LeadStatus

	// Author records who or what originated this lead.
	Author: ap.#Author

	// CreatedAt is the RFC 3339 timestamp when the lead was first opened.
	CreatedAt: string

	// UpdatedAt is the RFC 3339 timestamp of the most recent status change.
	UpdatedAt: string

	// Outcome is a human-readable summary of the resolution. Empty on open or
	// pursuing leads.
	Outcome?: string

	// ResultSpanIDs holds the IDs of evidence spans produced by investigation.
	ResultSpanIDs: [...string]
}

// #LeadEntry is one immutable record in the append-only LeadLog. It carries
// the full Lead snapshot after each transition, the author of the transition,
// and the wall-clock time at which the entry was appended.
#LeadEntry: {
	// Lead is the complete Lead snapshot after this transition.
	Lead: #Lead

	// TransitionAuthor records who authorised this specific transition.
	TransitionAuthor: ap.#Author

	// RecordedAt is the RFC 3339 timestamp set by the log's injected clock.
	RecordedAt: string
}
