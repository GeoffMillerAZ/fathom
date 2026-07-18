// valid.cue — concrete values that satisfy the #AssessmentInput and
// #AssessmentResult constraints defined in schema.cue. Running `cue vet ./...`
// from the package root validates these against the schema.
package convergenceoracle

// An assessment that produced a converged verdict.
_result1: #AssessmentResult & {
	Input: {
		Findings:          10
		Agreement:         0.9
		Contradictions:    0
		MaxContradictions: 0
	}
	Verdict: "converged"
}

// An assessment that is still exploring.
_result2: #AssessmentResult & {
	Input: {
		Findings:  5
		Agreement: 0.4
	}
	Verdict: "exploring"
}

// An assessment that needs more sources.
_result3: #AssessmentResult & {
	Input: {
		Findings:  1
		Agreement: 1.0
	}
	Verdict: "needs_sources"
}

// An assessment where material contradictions blocked convergence.
// Converging on a contradiction is not convergence.
_result4: #AssessmentResult & {
	Input: {
		Findings:          10
		Agreement:         0.9
		Contradictions:    3
		MaxContradictions: 2
	}
	Verdict: "contradicted"
}

// An input at boundary values.
_input1: #AssessmentInput & {
	Findings:  0
	Agreement: 0.0
}

// An input at maximum boundary values.
_input2: #AssessmentInput & {
	Findings:  100
	Agreement: 1.0
}

// An input with contradictions at tolerance boundary (not blocked).
_input3: #AssessmentInput & {
	Findings:          10
	Agreement:         0.9
	Contradictions:    2
	MaxContradictions: 2
}
