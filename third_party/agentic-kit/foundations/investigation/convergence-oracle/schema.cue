// Package convergenceoracle defines the CUE-first contracts for the
// convergence oracle. Go types in types.go mirror these definitions; any drift
// is a bug.
package convergenceoracle

// #Verdict enumerates the valid outcomes of a convergence assessment.
// "contradicted" is returned when unresolved contradictions exceed the
// configured tolerance — converging on a contradiction is not convergence.
#Verdict: "converged" | "exploring" | "needs_sources" | "contradicted"

// #AssessmentInput captures the inputs to the convergence oracle so that
// assessments can be recorded and replayed.
#AssessmentInput: {
	// Findings is the total number of independent findings collected.
	// Must be non-negative.
	Findings: >=0

	// Agreement is the proportion of findings in agreement with the leading
	// hypothesis. Must be in the closed interval [0, 1].
	// CALLER CONTRACT: agreement must be computed only from findings whose
	// evidence grade meets the minimum required grade for this investigation.
	// The oracle does not enforce grade-bounding internally (Invariant 4).
	Agreement: >=0 & <=1

	// Contradictions is the number of unresolved contradictions detected
	// during the investigation. Must be non-negative.
	Contradictions?: >=0

	// MaxContradictions is the maximum tolerated contradictions before the
	// Contradicted verdict is returned. Must be non-negative.
	MaxContradictions?: >=0
}

// #AssessmentResult captures one oracle decision for audit purposes.
#AssessmentResult: {
	// Input is the assessment input that produced this result.
	Input: #AssessmentInput

	// Verdict is the oracle's decision.
	Verdict: #Verdict
}
