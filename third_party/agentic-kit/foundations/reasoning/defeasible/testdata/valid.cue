// valid.cue — concrete values that satisfy the constraints defined in
// schema.cue. Running `cue vet ./...` from the package directory validates
// these against the schema.
package defeasible

// A minimal provision at full confidence.
_provision1: #Provision & {
	Key:            "§1"
	BaseValue:      "payment due in 30 days"
	BaseConfidence: 0.9
}

// A provision with zero base confidence.
_provision2: #Provision & {
	Key:            "§2"
	BaseValue:      "governing law: New York"
	BaseConfidence: 0.0
}

// A provision at maximum confidence.
_provision3: #Provision & {
	Key:            "§3"
	BaseValue:      "arbitration clause"
	BaseConfidence: 1.0
}

// A presumed amendment at layer 1.
_modAmendment: #Modifier & {
	ID:           "mod-001"
	TargetKey:    "§1"
	Layer:        1
	Kind:         "amendment"
	NewValue:     "payment due in 45 days"
	Effect:       "set"
	Confidence:   0.85
	EvidenceRefs: ["ev-001", "ev-002"]
	Status:       "presumed"
	Supersedes:   []
	Priority:     0
}

// A statutory override at layer 2, upheld.
_modStatutory: #Modifier & {
	ID:           "mod-002"
	TargetKey:    "§1"
	Layer:        2
	Kind:         "statutory_override"
	NewValue:     "payment due in 60 days per statute"
	Effect:       "set"
	Confidence:   0.95
	EvidenceRefs: ["statute-ref-42"]
	Status:       "upheld"
	Supersedes:   ["mod-001"]
	Priority:     10
}

// An invalidated modifier — never applied by Resolve.
_modInvalidated: #Modifier & {
	ID:           "mod-003"
	TargetKey:    "§2"
	Layer:        1
	Kind:         "contra_proferentem"
	NewValue:     "governing law: California"
	Effect:       "reinterpret"
	Confidence:   0.6
	EvidenceRefs: []
	Status:       "invalidated"
	Supersedes:   []
	Priority:     0
}

// A strike modifier making a provision void.
_modStrike: #Modifier & {
	ID:           "mod-004"
	TargetKey:    "§3"
	Layer:        2
	Kind:         "unenforceable"
	NewValue:     ""
	Effect:       "strike"
	Confidence:   0.8
	EvidenceRefs: ["case-law-007"]
	Status:       "upheld"
	Supersedes:   []
	Priority:     0
}

// A contested modifier — active only when caller includes contested in activeStatuses.
_modContested: #Modifier & {
	ID:           "mod-005"
	TargetKey:    "§2"
	Layer:        1
	Kind:         "course_of_performance"
	NewValue:     "governing law: Delaware"
	Effect:       "set"
	Confidence:   0.7
	EvidenceRefs: []
	Status:       "contested"
	Supersedes:   []
	Priority:     5
}

// An effective provision — clause amended, confidence bounded to 0.85.
_effective1: #EffectiveProvision & {
	Key:                 "§1"
	Value:               "payment due in 45 days"
	Void:                false
	Confidence:          0.85
	AppliedModifierIDs:  ["mod-001"]
	ShadowedModifierIDs: []
}

// A void effective provision after a strike.
_effective2: #EffectiveProvision & {
	Key:                 "§3"
	Value:               ""
	Void:                true
	Confidence:          0.8
	AppliedModifierIDs:  ["mod-004"]
	ShadowedModifierIDs: []
}

// A trace entry for an applied modifier.
_trace1: #TraceEntry & {
	ProvisionKey: "§1"
	ModifierID:   "mod-001"
	Decision:     "applied"
}

// A trace entry for a superseded modifier.
_trace2: #TraceEntry & {
	ProvisionKey: "§1"
	ModifierID:   "mod-001"
	Decision:     "shadowed:superseded"
}

// A representative resolution: one provision amended, one void.
_resolution1: #Resolution & {
	Effective: [
		{
			Key:                 "§1"
			Value:               "payment due in 45 days"
			Void:                false
			Confidence:          0.85
			AppliedModifierIDs:  ["mod-001"]
			ShadowedModifierIDs: []
		},
		{
			Key:                 "§3"
			Value:               ""
			Void:                true
			Confidence:          0.8
			AppliedModifierIDs:  ["mod-004"]
			ShadowedModifierIDs: []
		},
	]
	Trace: [
		{
			ProvisionKey: "§1"
			ModifierID:   "mod-001"
			Decision:     "applied"
		},
		{
			ProvisionKey: "§3"
			ModifierID:   "mod-004"
			Decision:     "applied"
		},
	]
}
