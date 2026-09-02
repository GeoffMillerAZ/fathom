// Package toolregistry — valid test data that must unify with schema.cue.
package toolregistry

// The explicit ":toolregistry" qualifier is required: CUE cannot derive a
// package name from the hyphenated final path element.
import "github.com/geoffmilleraz/agentic-kit/foundations/agent/tool-registry:toolregistry"

// validReadOnly is a pure read — the shape a file reader or a search tool
// declares. A strict harness dispatches it without an approval.
validReadOnly: toolregistry.#Effect & {
	readOnlyHint:    true
	destructiveHint: false
	idempotentHint:  true
	openWorldHint:   false
}

// validDestructive is an irreversible action — a delete, a rollback, a
// send. A strict harness routes it through a human approval before dispatch.
validDestructive: toolregistry.#Effect & {
	readOnlyHint:    false
	destructiveHint: true
	idempotentHint:  false
	openWorldHint:   true
}

// validAdditive is the explicit "every hint false" declaration: additive
// updates only, closed world. Distinct from an ABSENT declaration (Go nil),
// which a strict policy treats as destructive.
validAdditive: toolregistry.#Effect & {
	readOnlyHint:    false
	destructiveHint: false
	idempotentHint:  false
	openWorldHint:   false
}

// validSparse omits every hint, which on the wire means the MCP defaults
// apply (destructiveHint=true, openWorldHint=true): a sparse object is a
// DESTRUCTIVE declaration, not a permissive one.
validSparse: toolregistry.#Effect & {}

// Negative cases (kept as documentation; the schema rejects them and the Go
// side never constructs them):
//
//   invalidStringHint: #Effect & {readOnlyHint: "yes"}   — hints are bool, not string
//   invalidUnknownHint: #Effect & {reversibleHint: true} — not an MCP annotation;
//                                                          the kit invents no fifth hint
