// Package capability is fathom's single registry of what can be switched
// on and off, and — more importantly — of why anything is currently off.
//
// Phase T of docs/vision/20-ambient-fathom.md; design:
// docs/design/capability-registry.md.
//
// # Why this package exists
//
// Fathom grew four extension seams (expert modules, layer packs, guards,
// MCP profiles) with four unrelated on/off mechanisms. The cost was not
// inconsistency for its own sake: it was that nothing could answer the
// single most common adoption question — "why isn't X happening here?" —
// because nothing knew the full set of X's. Every onboarding defect of
// 2026-07/08 (the hook duplication, the CUE `?:` trust inversion, O22,
// O23) was a variant of that question going unanswered.
//
// # Two invariants
//
// Detection is a function of observed state, never of configuration. A
// capability is "operating" when it is observably working; a config file
// saying `guards: on` with no rules compiled reports OFF, with a reason.
// This is the observatory's value-or-named-reason primitive (law 3)
// applied to configuration.
//
// Every capability can say why it is off. Enforced by
// TestRegistryEveryEntryExplainsItsAbsence, which iterates the registry —
// so a capability cannot be added without a reason and a remedy.
//
// # Purity
//
// Detect functions take a [Probe], a plain value of already-gathered
// facts, and return a [State]. They perform no I/O, so the whole registry
// is testable in a table and the caller (cmd/fathom) owns every filesystem
// and HTTP touch. That separation is also what lets doctor, the
// observatory and the `capabilities` verb share one answer.
package capability
