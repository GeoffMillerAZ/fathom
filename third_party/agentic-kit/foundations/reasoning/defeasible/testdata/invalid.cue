// Package defeasible — invalid test data showing what the schema rejects.
// This file is NOT vetted with `cue vet` (it is intentionally invalid CUE).
// It documents the constraints enforced by schema.cue. Corresponding Go-level
// enforcement is in Provision.Validate(), Modifier.Validate(), and Resolve().
//
// To verify the schema rejects a binding, temporarily uncomment it and run:
//
//	cue vet ./testdata/   (expect errors for that binding)
//
// The following errors would be produced by `cue vet ./testdata/`:
//
//	_badStatus: "ruled_out" is not a valid #Status.
//	_badEffect: "delete" is not a valid #Effect.
//	_badProvisionEmptyKey: "" violates string & !="" (Key must be non-empty).
//	_badProvisionConfHigh: BaseConfidence 1.5 violates <=1.
//	_badProvisionConfLow: BaseConfidence -0.1 violates >=0.
//	_badModifierEmptyID: "" violates string & !="" (ID must be non-empty).
//	_badModifierEmptyTarget: "" violates string & !="" (TargetKey must be non-empty).
//	_badModifierConfHigh: Confidence 1.2 violates <=1.
//	_badModifierConfLow: Confidence -0.05 violates >=0.
//	_badModifierBadStatus: "pending" is not a valid #Status.
//	_badModifierBadEffect: "append" is not a valid #Effect.
//	_badModifierNegativeLayer: -1 violates >=0.
//	_badEffectiveProvisionEmptyKey: "" violates string & !="" (Key must be non-empty).
//	_badEffectiveProvisionConfHigh: Confidence 1.1 violates <=1.
//	_badEffectiveProvisionConfLow: Confidence -0.1 violates >=0.
//	_badTraceEmptyKey: "" violates string & !="" (ProvisionKey must be non-empty).
//	_badTraceEmptyModifierID: "" violates string & !="" (ModifierID must be non-empty).
//	_badTraceEmptyDecision: "" violates string & !="" (Decision must be non-empty).
package defeasible

// // _badStatus: "ruled_out" is not one of the recognised Status values.
// _badStatus: #Status & "ruled_out"

// // _badEffect: "delete" is not one of the recognised Effect values.
// _badEffect: #Effect & "delete"

// // _badProvisionEmptyKey: Key must not be empty.
// _badProvisionEmptyKey: #Provision & {
// 	Key:            ""
// 	BaseValue:      "some text"
// 	BaseConfidence: 0.9
// }

// // _badProvisionConfHigh: BaseConfidence > 1 violates <=1.
// _badProvisionConfHigh: #Provision & {
// 	Key:            "§1"
// 	BaseValue:      "v"
// 	BaseConfidence: 1.5
// }

// // _badProvisionConfLow: BaseConfidence < 0 violates >=0.
// _badProvisionConfLow: #Provision & {
// 	Key:            "§1"
// 	BaseValue:      "v"
// 	BaseConfidence: -0.1
// }

// // _badModifierEmptyID: Modifier ID must not be empty.
// _badModifierEmptyID: #Modifier & {
// 	ID:           ""
// 	TargetKey:    "§1"
// 	Layer:        1
// 	Kind:         "amendment"
// 	NewValue:     "v"
// 	Effect:       "set"
// 	Confidence:   0.8
// 	EvidenceRefs: []
// 	Status:       "presumed"
// 	Supersedes:   []
// 	Priority:     0
// }

// // _badModifierEmptyTarget: Modifier TargetKey must not be empty.
// _badModifierEmptyTarget: #Modifier & {
// 	ID:           "m1"
// 	TargetKey:    ""
// 	Layer:        1
// 	Kind:         "amendment"
// 	NewValue:     "v"
// 	Effect:       "set"
// 	Confidence:   0.8
// 	EvidenceRefs: []
// 	Status:       "presumed"
// 	Supersedes:   []
// 	Priority:     0
// }

// // _badModifierConfHigh: Modifier Confidence > 1 violates <=1.
// _badModifierConfHigh: #Modifier & {
// 	ID:           "m2"
// 	TargetKey:    "§1"
// 	Layer:        1
// 	Kind:         "amendment"
// 	NewValue:     "v"
// 	Effect:       "set"
// 	Confidence:   1.2
// 	EvidenceRefs: []
// 	Status:       "presumed"
// 	Supersedes:   []
// 	Priority:     0
// }

// // _badModifierConfLow: Modifier Confidence < 0 violates >=0.
// _badModifierConfLow: #Modifier & {
// 	ID:           "m3"
// 	TargetKey:    "§1"
// 	Layer:        1
// 	Kind:         "amendment"
// 	NewValue:     "v"
// 	Effect:       "set"
// 	Confidence:   -0.05
// 	EvidenceRefs: []
// 	Status:       "presumed"
// 	Supersedes:   []
// 	Priority:     0
// }

// // _badModifierBadStatus: "pending" is not a valid #Status.
// _badModifierBadStatus: #Modifier & {
// 	ID:           "m4"
// 	TargetKey:    "§1"
// 	Layer:        1
// 	Kind:         "amendment"
// 	NewValue:     "v"
// 	Effect:       "set"
// 	Confidence:   0.8
// 	EvidenceRefs: []
// 	Status:       "pending"
// 	Supersedes:   []
// 	Priority:     0
// }

// // _badModifierBadEffect: "append" is not a valid #Effect.
// _badModifierBadEffect: #Modifier & {
// 	ID:           "m5"
// 	TargetKey:    "§1"
// 	Layer:        1
// 	Kind:         "amendment"
// 	NewValue:     "v"
// 	Effect:       "append"
// 	Confidence:   0.8
// 	EvidenceRefs: []
// 	Status:       "presumed"
// 	Supersedes:   []
// 	Priority:     0
// }

// // _badModifierNegativeLayer: Layer < 0 violates >=0.
// _badModifierNegativeLayer: #Modifier & {
// 	ID:           "m6"
// 	TargetKey:    "§1"
// 	Layer:        -1
// 	Kind:         "amendment"
// 	NewValue:     "v"
// 	Effect:       "set"
// 	Confidence:   0.8
// 	EvidenceRefs: []
// 	Status:       "presumed"
// 	Supersedes:   []
// 	Priority:     0
// }

// // _badEffectiveProvisionEmptyKey: Key must not be empty.
// _badEffectiveProvisionEmptyKey: #EffectiveProvision & {
// 	Key:                 ""
// 	Value:               "v"
// 	Void:                false
// 	Confidence:          0.8
// 	AppliedModifierIDs:  ["m1"]
// 	ShadowedModifierIDs: []
// }

// // _badEffectiveProvisionConfHigh: Confidence > 1 violates <=1.
// _badEffectiveProvisionConfHigh: #EffectiveProvision & {
// 	Key:                 "§1"
// 	Value:               "v"
// 	Void:                false
// 	Confidence:          1.1
// 	AppliedModifierIDs:  []
// 	ShadowedModifierIDs: []
// }

// // _badEffectiveProvisionConfLow: Confidence < 0 violates >=0.
// _badEffectiveProvisionConfLow: #EffectiveProvision & {
// 	Key:                 "§1"
// 	Value:               "v"
// 	Void:                false
// 	Confidence:          -0.1
// 	AppliedModifierIDs:  []
// 	ShadowedModifierIDs: []
// }

// // _badTraceEmptyKey: ProvisionKey must not be empty.
// _badTraceEmptyKey: #TraceEntry & {
// 	ProvisionKey: ""
// 	ModifierID:   "m1"
// 	Decision:     "applied"
// }

// // _badTraceEmptyModifierID: ModifierID must not be empty.
// _badTraceEmptyModifierID: #TraceEntry & {
// 	ProvisionKey: "§1"
// 	ModifierID:   ""
// 	Decision:     "applied"
// }

// // _badTraceEmptyDecision: Decision must not be empty.
// _badTraceEmptyDecision: #TraceEntry & {
// 	ProvisionKey: "§1"
// 	ModifierID:   "m1"
// 	Decision:     ""
// }
