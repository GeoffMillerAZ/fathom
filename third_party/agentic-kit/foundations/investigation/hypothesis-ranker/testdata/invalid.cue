// invalid.cue — demonstrates #Hypothesis values that VIOLATE the schema.
// This file is intentionally NOT included in the `cue vet` path (only valid.cue
// is vetted in CI). It is kept here as executable documentation so reviewers
// understand what the schema rejects.
//
// To verify the schema rejects a binding, temporarily uncomment it and run:
//
//	cue vet ./testdata/   (expect errors for that binding)
//
// All constraints are also enforced at runtime in Go via Hypothesis.Validate().
// Corresponding Go-level rejection tests live in hypothesisranker_audit_test.go.
//
// The following errors would be produced by `cue vet ./testdata/`:
//
//	_badSupportHigh:        Support 1.5 violates <=1.
//	_badSupportLow:         Support -0.1 violates >=0.
//	_badEmptyID:            "" violates string & !="" (ID must be non-empty).
//	_badStatusUnknown:      "unknown" is not a valid #HypothesisStatus value.
//	_badGradeTierUnknown:   "expert" is not a valid #GradeTierLabel value.
//	_badAuthorKindUnknown:  "robot" is not a valid #AuthorKind value.
//
// The following are enforced in Go Validate() but cannot be expressed as CUE
// structural constraints (they are cross-field rules, not type constraints):
//
//	_aiAuthorEmptyModel:      Author.Kind=="ai" with empty Model — rejected (Invariant 3).
//	_aiAuthorEmptyPromptHash: Author.Kind=="ai" with empty PromptHash — rejected (Invariant 3).
package hypothesisranker

// _badSupportHigh: Support > 1 violates the >=0 & <=1 constraint.
// _badSupportHigh: #Hypothesis & {
// 	ID:       "h-bad-high"
// 	Statement: "Some hypothesis."
// 	Support:  1.5
// 	SpanRefs: []
// }

// _badSupportLow: Support < 0 violates the >=0 & <=1 constraint.
// _badSupportLow: #Hypothesis & {
// 	ID:       "h-bad-low"
// 	Statement: "Some hypothesis."
// 	Support:  -0.1
// 	SpanRefs: []
// }

// _badEmptyID: empty ID violates the non-empty string constraint.
// _badEmptyID: #Hypothesis & {
// 	ID:       ""
// 	Statement: "Some hypothesis."
// 	Support:  0.5
// 	SpanRefs: []
// }

// _badStatusUnknown: "unknown" is not one of the four recognised status values.
// _badStatusUnknown: #Hypothesis & {
// 	ID:       "h-status-bad"
// 	Statement: "Some hypothesis."
// 	Support:  0.5
// 	SpanRefs: []
// 	Status:   "unknown"
// }

// _badGradeTierUnknown: "expert" is not a recognised #GradeTierLabel value.
// _badGradeTierUnknown: #Hypothesis & {
// 	ID:        "h-tier-bad"
// 	Statement: "Some hypothesis."
// 	Support:   0.5
// 	SpanRefs:  []
// 	GradeTier: "expert"
// }

// _badAuthorKindUnknown: "robot" is not a recognised #AuthorKind value.
// _badAuthorKindUnknown: #Hypothesis & {
// 	ID:       "h-author-bad"
// 	Statement: "Some hypothesis."
// 	Support:  0.5
// 	SpanRefs: []
// 	Author: {
// 		Kind:        "robot"
// 		Model:       ""
// 		PromptHash:  ""
// 		InputDigest: ""
// 	}
// }

// _aiAuthorEmptyModel: AI author with empty Model violates Invariant 3.
// CUE cannot express cross-field rules; rejected by Go Validate().
// _aiAuthorEmptyModel: #Hypothesis & {
// 	ID:       "h-ai-no-model"
// 	Statement: "Some AI hypothesis."
// 	Support:  0.5
// 	SpanRefs: []
// 	Author: {
// 		Kind:        "ai"
// 		Model:       ""       // empty — Go Validate() rejects this
// 		PromptHash:  "sha256:abc"
// 		InputDigest: ""
// 	}
// }

// _aiAuthorEmptyPromptHash: AI author with empty PromptHash violates Invariant 3.
// CUE cannot express cross-field rules; rejected by Go Validate().
// _aiAuthorEmptyPromptHash: #Hypothesis & {
// 	ID:       "h-ai-no-prompt"
// 	Statement: "Some AI hypothesis."
// 	Support:  0.5
// 	SpanRefs: []
// 	Author: {
// 		Kind:        "ai"
// 		Model:       "claude-sonnet-4"
// 		PromptHash:  ""       // empty — Go Validate() rejects this
// 		InputDigest: ""
// 	}
// }
