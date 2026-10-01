// Package decision — invalid test data showing what the schema rejects.
// This file is NOT vetted with `cue vet` (it is intentionally invalid CUE,
// entirely commented out). It documents the constraints enforced by
// schema.cue. Kind-conditional and cross-field enforcement (2..255 unique
// choice options, score Max>Min, unique question IDs within a Request) is
// Go-level, in Validate — see validate.go and validate_test.go.
//
// To verify the schema rejects a binding, temporarily uncomment it and run:
//
//	cue vet ./testdata/   (expect errors for that binding)
//
// The following errors would be produced by `cue vet ./testdata/`:
//
//	_badKind: "maybe" is not a valid #Kind.
//	_badQuestionEmptyID: "" violates string & !="" (ID must be non-empty).
//	_badOptionEmptyName: "" violates string & !="" (Name must be non-empty).
//	_badRequestEmptyState: "" violates string & !="" (State must be non-empty).
//	badPolicyUnknownNodeType: stages.1.ensemble: field not allowed.
//	badPolicyTwoNodeTypes: provider and cascade: field not allowed (one node type).
//	badPolicyEmptyStages: stages does not satisfy list.MinItems(1).
//	badPolicyEmptyFallback: deciders does not satisfy list.MinItems(1).
//	badPolicyThresholdAbove1: escalate_below 1.5 out of bound <=1.
//	badPolicyThresholdBelow0: escalate_below -0.1 out of bound >=0.
//	badPolicyUnknownField: cascade.escalate_belw: field not allowed.
//	badPolicyCacheNoTemplateVersion: incomplete (with -c: template_version
//	  is required but not present).
//	badPolicyFallbackNoObserve: incomplete (with -c: observe is required).
//
// The #Policy cases are regular fields, not hidden ones, so a missing
// required field is reported (as an incomplete instance) and not only a
// conflict. Go refuses every one of them too, with the node's path, plus
// what CUE cannot see (an unknown Registry key, a cache or record node
// without inner, a duplicate name): TestComposeFromConfigRefusesWithNodePath.
package decision

// // _badKind: "maybe" is not one of the recognised #Kind values.
// _badKind: #Kind & "maybe"

// // _badQuestionEmptyID: ID must not be empty.
// _badQuestionEmptyID: #Question & {
// 	ID:           ""
// 	Kind:         "noul"
// 	Instructions: "some statement"
// 	Options: []
// 	Min:            0
// 	Max:            0
// 	Rubric:         ""
// 	TrueCriteria:   ""
// 	FalseCriteria:  ""
// }

// // _badOptionEmptyName: Option.Name must not be empty.
// _badOptionEmptyName: #Option & {
// 	Name:     ""
// 	Criteria: "some criteria"
// }

// // _badRequestEmptyState: Request.State must not be empty.
// _badRequestEmptyState: #Request & {
// 	State:     ""
// 	Questions: []
// }

// // badPolicyUnknownNodeType: a node type ComposeFromConfig does not have.
// badPolicyUnknownNodeType: #Policy & {cascade: {stages: [{provider: "cheap"}, {ensemble: {}}]}}

// // badPolicyTwoNodeTypes: a node sets exactly one node type.
// badPolicyTwoNodeTypes: #Policy & {provider: "cheap", cascade: {stages: [{provider: "a"}]}}

// // badPolicyEmptyStages: a cascade needs at least one stage.
// badPolicyEmptyStages: #Policy & {cascade: {stages: []}}

// // badPolicyEmptyFallback: a fallback needs at least one Decider.
// badPolicyEmptyFallback: #Policy & {fallback: {observe: "fallback-log", deciders: []}}

// // badPolicyThresholdAbove1: escalate_below lies in [0,1].
// badPolicyThresholdAbove1: #Policy & {cascade: {escalate_below: 1.5, stages: [{provider: "cheap"}]}}

// // badPolicyThresholdBelow0: escalate_below lies in [0,1].
// badPolicyThresholdBelow0: #Policy & {cascade: {escalate_below: -0.1, stages: [{provider: "cheap"}]}}

// // badPolicyUnknownField: a misspelt field is refused, never ignored.
// badPolicyUnknownField: #Policy & {cascade: {escalate_belw: 0.2, stages: [{provider: "cheap"}]}}

// // badPolicyCacheNoTemplateVersion: a cache node names its template version.
// badPolicyCacheNoTemplateVersion: #Policy & {cache: {store: "memory", inner: {provider: "cheap"}}}

// // badPolicyFallbackNoObserve: a fallback is never silent.
// badPolicyFallbackNoObserve: #Policy & {fallback: {deciders: [{provider: "a"}]}}
