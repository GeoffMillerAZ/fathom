// invalid.cue — demonstrates values that VIOLATE the schema.
//
// These cases are kept as commented documentation artifacts so reviewers
// understand what the schema rejects. Each case has a corresponding Go test
// in convergenceoracle_ext_test.go (see TestAssessWithThresholdsERejects*)
// that enforces the same contract at the Go level via AssessWithThresholdsE,
// so the negative contracts are executable — not just documentation.
package convergenceoracle

// _badNegativeFindings: Findings < 0 violates the >=0 constraint.
// Go enforcement: TestAssessWithThresholdsERejectsNegativeFindings.
// _badNegativeFindings: #AssessmentInput & {
// 	Findings:  -1
// 	Agreement: 0.5
// }

// _badAgreementHigh: Agreement > 1 violates the >=0 & <=1 constraint.
// Go enforcement: TestAssessWithThresholdsERejectsAgreementAboveOne.
// _badAgreementHigh: #AssessmentInput & {
// 	Findings:  5
// 	Agreement: 1.5
// }

// _badAgreementLow: Agreement < 0 violates the >=0 & <=1 constraint.
// Go enforcement: TestAssessWithThresholdsERejectsNegativeAgreement.
// _badAgreementLow: #AssessmentInput & {
// 	Findings:  5
// 	Agreement: -0.1
// }

// _badVerdict: unknown verdict string violates the #Verdict enum constraint.
// Go enforcement: the Verdict type is a string enum; no constructor accepts
// an arbitrary string — all Verdict values are returned from the oracle.
// _badVerdict: #AssessmentResult & {
// 	Input: {
// 		Findings:  10
// 		Agreement: 0.9
// 	}
// 	Verdict: "unknown"
// }

// _badNegativeContradictions: Contradictions < 0 violates the >=0 constraint.
// Go enforcement: TestAssessWithThresholdsERejectsNegativeContradictions.
// _badNegativeContradictions: #AssessmentInput & {
// 	Findings:       10
// 	Agreement:      0.9
// 	Contradictions: -1
// }
