// Package defeasible defines the CUE-first contracts for the defeasible
// resolution engine. Go types in types.go and resolve.go mirror these
// definitions; any drift is a bug.
//
// Design invariants encoded here:
//   - Status is a closed enum: presumed | upheld | invalidated | contested | waived.
//   - Effect is a closed enum: set | strike | reinterpret.
//   - Confidence values are always in [0, 1].
//   - Provision.Key, Modifier.ID, and Modifier.TargetKey are non-empty strings.
//   - Modifier.Supersedes must not contain the modifier's own ID (enforced in Go).
//   - EffectiveProvision.Confidence ≤ min(BaseConfidence, each applied modifier's
//     Confidence) — weakest-link bounding (enforced in Go; documented here).
package defeasible

// _nonEmpty is a reusable constraint for non-empty strings.
_nonEmpty: string & !=""

// #Status classifies the standing of a Modifier.
// Active statuses (by default): presumed | upheld.
// Inactive regardless: invalidated | waived.
// Caller-configurable: contested (default inactive).
#Status: "presumed" | "upheld" | "invalidated" | "contested" | "waived"

// #Effect describes how a Modifier transforms its target Provision.
#Effect: "set" | "strike" | "reinterpret"

// #Provision is the Layer-0 base: a keyed text clause with a baseline confidence.
//
//   - Key uniquely identifies this provision within its set.
//   - BaseValue is the unmodified text.
//   - BaseConfidence is the certainty of the base text in [0, 1].
#Provision: {
	Key:            _nonEmpty
	BaseValue:      string
	BaseConfidence: >=0 & <=1
}

// #Modifier is a layered, typed change that may alter a provision's effective value.
//
// Layer controls precedence: 0=base context, 1=legal, 2=operational.
// Within a layer, Priority (ascending) breaks ties; Confidence (descending)
// and then alphabetical ID are further tiebreaks.
//
// Supersedes names modifier IDs that this modifier defeats. Forms a DAG;
// cycles are an error in Go. Must not contain the modifier's own ID.
//
// EvidenceRefs are opaque references to supporting evidence; the engine
// stores them as-is for audit purposes.
#Modifier: {
	ID:           _nonEmpty
	TargetKey:    _nonEmpty
	Layer:        int & >=0
	Kind:         string
	NewValue:     string
	Effect:       #Effect
	Confidence:   >=0 & <=1
	EvidenceRefs: [...string]
	Status:       #Status
	Supersedes:   [...string]
	Priority:     int
}

// #EffectiveProvision is the resolved state of a single provision.
//
//   - Void is true when an active EffectStrike modifier was applied.
//   - Confidence reflects weakest-link bounding across the base and applied chain.
//   - AppliedModifierIDs lists the IDs of modifiers that were applied, in
//     application order.
//   - ShadowedModifierIDs lists modifiers that were active but defeated by
//     supersession or conflict.
#EffectiveProvision: {
	Key:                 _nonEmpty
	Value:               string
	Void:                bool
	Confidence:          >=0 & <=1
	AppliedModifierIDs:  [...string]
	ShadowedModifierIDs: [...string]
}

// #TraceEntry records a single resolution decision for audit purposes.
//
//   - Decision is one of:
//       "applied"                         — modifier was applied to the chain
//       "shadowed:superseded"             — removed from contention by supersession
//       "shadowed:conflict:<reason>"      — lost a same-(Layer,Priority) conflict
//
// Every active modifier that participates in resolution appears in the Trace:
// either as "applied" or with a "shadowed:*" reason. No active modifier is
// silently dropped.
#TraceEntry: {
	ProvisionKey: _nonEmpty
	ModifierID:   _nonEmpty
	Decision:     _nonEmpty
}

// #Resolution is the output of Resolve.
//
//   - Effective is sorted by Key for deterministic output.
//   - Trace records all resolution decisions across all provisions.
#Resolution: {
	Effective: [...#EffectiveProvision]
	Trace:     [...#TraceEntry]
}
