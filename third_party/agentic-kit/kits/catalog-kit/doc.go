// Package catalogkit composes the catalog and decision foundations into an
// offline catalog workflow. CUE validates authored shape; Resolve applies an
// explicit dependency graph and receiver-owned override grants. It never treats
// CUE unification as last-writer-wins inheritance.
//
// Resolve is additive to catalog.Build: it does not change that API's semantics.
// Its stricter path requires stable keys and sources, refuses implicit scalar
// replacement, checks patch preconditions, and records changes. Grants establish
// mechanical permission only; a domain adapter must still enforce semantic floors
// and authorization. A digest establishes byte integrity, not source truth.
//
// PrepareDecision assembles a bounded, evidence-bound request for the existing
// decision port. It makes no model call and grants no execution or review closure
// authority. Providers remain optional, and missing evidence returns an error.
//
// Concurrency: functions are stateless and safe for concurrent use when callers
// do not mutate their arguments during a call. Returned maps and slices are owned
// by the caller; separate Resolve calls do not share mutable state.
package catalogkit
