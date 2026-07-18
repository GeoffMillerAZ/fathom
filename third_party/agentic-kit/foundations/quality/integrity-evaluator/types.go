// Package integrityevaluator checks output integrity against a set of
// invariants. Currently enforced invariants:
//
//   - No fabricated references: every reference cited in the output must appear
//     in the provided knownRefs set.
//   - All references resolve: every reference cited must be non-empty and
//     present in knownRefs (same as above; surfaces as a distinct violation
//     kind for traceability).
//
// Evaluation is fully deterministic — no LLM is involved.
//
// Concurrency: values are immutable and safe for concurrent use.
package integrityevaluator

// ViolationKind categorises the type of integrity violation found.
type ViolationKind string

const (
	// ViolationFabricatedRef means a reference appears in the output but is
	// not present in the knownRefs set.
	ViolationFabricatedRef ViolationKind = "fabricated_ref"

	// ViolationUnresolvableRef means a reference in the output is present in
	// knownRefs but its resolved value is empty.
	ViolationUnresolvableRef ViolationKind = "unresolvable_ref"
)

// Violation is a single integrity breach found during evaluation.
type Violation struct {
	// Kind identifies the category of violation.
	Kind ViolationKind

	// Ref is the reference string that triggered the violation.
	Ref string

	// Detail is a human-readable explanation.
	Detail string
}

// Report is the result of an integrity evaluation.
type Report struct {
	// Passed is true when no violations were found.
	Passed bool

	// Violations lists every integrity breach found. Empty when Passed is true.
	Violations []Violation
}
