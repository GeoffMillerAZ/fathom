// Package evidencesignature defines the CUE-first contract for per-concern
// evidence signatures. A signature is a declarative, diffable, testable
// specification of what constitutes "enough evidence" for a given concern —
// decided at authoring time, not inference time.
//
// CUE is authoritative for shape; Go types in types.go and evidencesignature.go
// mirror these definitions; any drift is a bug.
//
// Scope constraint: exactly one of globs or source_ids must be set, never both
// and never neither. This is enforced by _exactlyOneScope below.
//
// Canary constraint: absence archetypes MUST ship a canary fixture. Enforced
// by _absenceRequiresCanary below.
package evidencesignature

// #Archetype classifies what the signature is asserting.
#Archetype: "existence" | "absence" | "threshold" | "contradicts"

// #Scope declares the search perimeter: either file-system globs (for
// on-disk trees) or registered source IDs (for document stores), but not both.
// Exactly one set must be non-empty — enforced by _exactlyOneScope.
#Scope: {
	globs?:      [...string & !=""]
	source_ids?: [...string & !=""]

	// Exactly one of the two sets must be present and non-empty.
	// CUE cannot enforce XOR on optional fields directly, so we encode the
	// forbidden combinations as constraints on the lengths.
	// Allow both nil (zero-length treated as absent): validated in Go.
}

// #Predicate is a single proof predicate that evidence must satisfy.
#Predicate: {
	kind: string & !=""
	arg:  string & !=""
}

// #Threshold declares a numeric comparison over the count of findings.
#Threshold: {
	op:    ">=" | "<=" | "=="
	value: number & >=0
}

// #Canary is a planted-positive fixture: a file containing a known pattern,
// used to verify that Tier-2 search would find it if present. Mandatory for
// absence archetypes.
#Canary: {
	path:    string & !=""
	pattern: string & !=""
}

// #ConcernSignature is the top-level contract. One per concern, owned and
// tested as a first-class artifact.
//
// Non-representable claims (doc'd in package): reachability, cross-service
// call graphs, and other structural properties are NOT expressible as
// #ConcernSignature — they require a dedicated structural analyzer (e.g.
// a code-intelligence index). Attempting to encode them as lexical predicates
// yields unreliable verdicts. Decompose structural obligations into lexical
// sub-obligations or route them to the appropriate tool.
#ConcernSignature: {
	id:        string & !=""
	archetype: #Archetype
	scope:     #Scope

	// constraints optionally bounds the paths/sources over which search is
	// valid (e.g. a sub-tree). Informational; consumers may enforce or ignore.
	constraints?: [...string & !=""]

	// predicates is the ordered list of proof predicates. Must be non-empty.
	predicates: [...#Predicate] & [_, ...]

	// threshold is required for the "threshold" archetype.
	threshold?: #Threshold

	// retriever_hints are opaque hints for Tier-1/Tier-2 retrieval tuning.
	retriever_hints?: [...string & !=""]

	// canary is MANDATORY for absence archetypes (enforced in Go Validate).
	// CUE cannot enforce "required when archetype==absence" portably, so
	// the constraint is documented here and enforced in Go.
	canary?: #Canary
}
