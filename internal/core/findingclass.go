package core

// findingclass.go — the findings-as-evidence typed failure-class vocabulary
// (docs/design/findings-as-evidence.md §3.2, §6 item 2). A finding-kind
// claim's Class is a CLOSED vocabulary, deliberately coarse (a taxonomy
// nobody applies consistently is worse than none): assigned by the reviewer
// at emit time — a deterministic dimension in internal/analysis/review, or
// the LLM findings-layer compiler (internal/semantic) — and validated at
// compile/persist time. An unrecognized class is REJECTED, never silently
// bucketed as "other" (mirrors O19's rejection-cause taxonomy,
// internal/semantic/compile.go's classifyRejectionCause).
//
// The Go vocabulary here is the single source of truth; schema/semantic/
// findingclass.cue derives its #FindingClass disjunction FROM the same
// literal list via CUE's or() builtin, and
// internal/semantic.TestFindingClassVocabularyMatchesSchema cross-checks the
// two never drift apart (mirrors internal/analysis/review's own
// TestClaimsVocabularyMatchesSemanticLayer precedent).

// FindingClass is one closed-vocabulary failure class a finding-kind claim
// carries. The zero value "" is valid ONLY for a claim compiled before this
// vocabulary existed (additive migration — see internal/store's conditional
// column migration) or for a non-finding claim (Class is meaningless
// outside kind=="finding"); a finding-kind claim compiled or emitted AFTER
// this vocabulary exists must carry a non-empty, valid Class.
type FindingClass string

const (
	// FindingClassCorrectness: the code produces a wrong result — a logic
	// error, off-by-one, wrong condition — as opposed to a crash, leak, or
	// missing check.
	FindingClassCorrectness FindingClass = "correctness"
	// FindingClassContractViolation: an API, interface, or documented
	// contract (pre/post-condition, invariant) is violated by the
	// implementation or a caller.
	FindingClassContractViolation FindingClass = "contract-violation"
	// FindingClassResourceLeak: a resource (file handle, goroutine,
	// connection, memory) is acquired without a guaranteed release on every
	// path.
	FindingClassResourceLeak FindingClass = "resource-leak"
	// FindingClassErrorHandling: an error is ignored, swallowed,
	// miscategorized, or handled in a way that loses information a caller
	// needed.
	FindingClassErrorHandling FindingClass = "error-handling"
	// FindingClassConcurrency: a race, deadlock, or unsynchronized shared
	// state.
	FindingClassConcurrency FindingClass = "concurrency"
	// FindingClassSecurity: an injection, auth/authz gap, secret exposure,
	// or other trust-boundary violation.
	FindingClassSecurity FindingClass = "security"
	// FindingClassConfigDrift: runtime behavior diverges from its
	// documented/intended configuration, or configuration itself has
	// drifted from its source of truth.
	FindingClassConfigDrift FindingClass = "config-drift"
	// FindingClassTestGap: a code path, contract, or spec obligation has no
	// covering test or verification — includes spec-coverage findings
	// (a referenced spec ID with no covering symbol).
	FindingClassTestGap FindingClass = "test-gap"
	// FindingClassPerformance: correct but needlessly slow or resource-
	// heavy — an algorithmic or I/O inefficiency, not a leak.
	FindingClassPerformance FindingClass = "performance"
	// FindingClassConformance: an architectural rule (a layering boundary,
	// an import-direction constraint) is broken — the review-verb
	// arch_conformance dimension's own class.
	FindingClassConformance FindingClass = "conformance"
)

// ValidFindingClasses is the closed vocabulary, in the canonical order the
// CUE schema and every rendering of the vocabulary uses.
var ValidFindingClasses = []FindingClass{
	FindingClassCorrectness,
	FindingClassContractViolation,
	FindingClassResourceLeak,
	FindingClassErrorHandling,
	FindingClassConcurrency,
	FindingClassSecurity,
	FindingClassConfigDrift,
	FindingClassTestGap,
	FindingClassPerformance,
	FindingClassConformance,
}

// Valid reports whether c is a member of the closed vocabulary. The zero
// value "" is NOT valid by this method — callers that must admit an unset
// legacy class (a pre-vocabulary claim) check for "" explicitly before
// calling Valid, exactly like ClaimStatus.Valid's own convention of only
// recognizing real values.
func (c FindingClass) Valid() bool {
	for _, v := range ValidFindingClasses {
		if c == v {
			return true
		}
	}
	return false
}
