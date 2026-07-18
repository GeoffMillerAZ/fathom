// Package defeasible implements the generalized "effective state" engine: a
// base set of provisions plus layered, typed modifiers, each of which can be
// ruled valid or invalid. Resolve computes the effective state applying only
// active modifiers — with conflict/supersession handling — together with a
// full trace and confidence. Flipping a modifier's status and calling Resolve
// again instantly recomputes the outcome (contingency pivoting).
//
// The package is domain-agnostic: it does not know "contract" semantics.
// Callers supply provision keys and modifier effects. The confidence model
// is weakest-link: effective confidence ≤ min(base, each applied modifier
// confidence) along the applied chain, consistent with the evidence-grade
// bounding ethos.
//
// CUE contracts live in schema.cue; this file and resolve.go are the Go
// counterparts and must stay in sync with that schema. CUE is authoritative
// for shape; Go is authoritative for behaviour.
//
// Concurrency: all exported functions are pure and safe for concurrent use.
package defeasible

import "fmt"

// Status classifies the standing of a Modifier — whether it is active,
// overturned, or otherwise inapplicable.
//
// CUE mirror: #Status in schema.cue.
type Status string

const (
	// StatusPresumed is the default status: the modifier is presumed valid and
	// active unless overturned.
	StatusPresumed Status = "presumed"

	// StatusUpheld means the modifier's validity has been affirmatively confirmed
	// (e.g. by a court ruling or agreement). Active.
	StatusUpheld Status = "upheld"

	// StatusInvalidated means the modifier has been struck down or ruled void.
	// It is never applied in Resolve regardless of activeStatuses.
	StatusInvalidated Status = "invalidated"

	// StatusContested means the modifier's validity is disputed. Whether it is
	// treated as active is caller-controlled via the activeStatuses parameter
	// to Resolve. The default active set (presumed + upheld) does not include
	// contested.
	StatusContested Status = "contested"

	// StatusWaived means the modifier's right has been relinquished. It is
	// never applied in Resolve regardless of activeStatuses.
	StatusWaived Status = "waived"
)

// validStatuses is the canonical set for validation.
var validStatuses = map[Status]struct{}{
	StatusPresumed:    {},
	StatusUpheld:      {},
	StatusInvalidated: {},
	StatusContested:   {},
	StatusWaived:      {},
}

// Validate returns an error if s is not one of the recognised values.
func (s Status) Validate() error {
	if _, ok := validStatuses[s]; !ok {
		return fmt.Errorf("defeasible: unknown Status %q; must be one of presumed|upheld|invalidated|contested|waived", string(s))
	}
	return nil
}

// Effect describes how an active Modifier transforms its target provision.
//
// CUE mirror: #Effect in schema.cue.
type Effect string

const (
	// EffectSet replaces the provision's value with the modifier's NewValue.
	EffectSet Effect = "set"

	// EffectStrike makes the provision void (NewValue is ignored).
	EffectStrike Effect = "strike"

	// EffectReinterpret replaces the provision's value under a reinterpretation
	// frame (e.g. contra proferentem). Semantically equivalent to set from the
	// engine's perspective; callers use the distinct label for audit trails.
	EffectReinterpret Effect = "reinterpret"
)

// validEffects is the canonical set for validation.
var validEffects = map[Effect]struct{}{
	EffectSet:         {},
	EffectStrike:      {},
	EffectReinterpret: {},
}

// Validate returns an error if e is not one of the recognised values.
func (e Effect) Validate() error {
	if _, ok := validEffects[e]; !ok {
		return fmt.Errorf("defeasible: unknown Effect %q; must be one of set|strike|reinterpret", string(e))
	}
	return nil
}

// Provision is the Layer-0 base: a keyed text clause with a baseline confidence.
// Modifiers target provisions by Key.
//
// CUE mirror: #Provision in schema.cue.
type Provision struct {
	// Key uniquely identifies this provision within its set. Must be non-empty.
	Key string

	// BaseValue is the unmodified text of the provision.
	BaseValue string

	// BaseConfidence is the certainty that the base text is correct, in [0, 1].
	BaseConfidence float64
}

// Validate returns an error if any field is invalid.
func (p Provision) Validate() error {
	if p.Key == "" {
		return fmt.Errorf("defeasible: Provision.Key must not be empty")
	}
	if p.BaseConfidence < 0 || p.BaseConfidence > 1 {
		return fmt.Errorf("defeasible: Provision %q BaseConfidence %g is outside [0, 1]", p.Key, p.BaseConfidence)
	}
	return nil
}

// Modifier is a layered, typed change that can alter a provision's effective
// value. A Modifier is active only if its Status appears in the activeStatuses
// map passed to Resolve.
//
// Layer controls application precedence (0=base context, 1=legal, 2=operational).
// Within a layer, Priority (ascending) breaks ties; identical Priority uses
// Confidence (descending) and then alphabetical ID as the final tiebreak.
//
// Supersedes names modifier IDs that this modifier defeats. The superseded
// modifiers are removed from contention before conflict resolution. Supersession
// forms a DAG; cycles are detected and returned as errors by Resolve.
//
// CUE mirror: #Modifier in schema.cue.
type Modifier struct {
	// ID uniquely identifies this modifier. Must be non-empty.
	ID string

	// TargetKey is the Provision.Key this modifier targets. Must be non-empty.
	TargetKey string

	// Layer is the application tier: 0=base context, 1=legal, 2=operational.
	// Higher layer beats lower layer in conflict resolution.
	Layer int

	// Kind is a domain-defined label for the type of modifier (e.g. "amendment",
	// "unenforceable", "statutory_override"). The engine does not interpret Kind;
	// it is for caller audit trails and display.
	Kind string

	// NewValue is the replacement value. Used by EffectSet and EffectReinterpret;
	// ignored by EffectStrike.
	NewValue string

	// Effect describes how this modifier transforms the target provision.
	Effect Effect

	// Confidence is the certainty of this modifier in [0, 1].
	Confidence float64

	// EvidenceRefs are opaque references to supporting evidence (source IDs,
	// span IDs, document locators). The engine stores and returns them as-is.
	EvidenceRefs []string

	// Status is the current standing of this modifier.
	Status Status

	// Supersedes is the list of modifier IDs that this modifier defeats.
	// Those IDs are removed from contention regardless of their own Status.
	// Must not contain this modifier's own ID. Forms a DAG; cycles cause errors.
	Supersedes []string

	// Priority breaks ties within the same Layer. Lower value = lower precedence;
	// higher value = applied last (wins). When equal, Confidence (descending) is
	// used, then alphabetical ID.
	Priority int
}

// Validate returns an error if any field is invalid.
func (m Modifier) Validate() error {
	if m.ID == "" {
		return fmt.Errorf("defeasible: Modifier.ID must not be empty")
	}
	if m.TargetKey == "" {
		return fmt.Errorf("defeasible: Modifier %q TargetKey must not be empty", m.ID)
	}
	if m.Confidence < 0 || m.Confidence > 1 {
		return fmt.Errorf("defeasible: Modifier %q Confidence %g is outside [0, 1]", m.ID, m.Confidence)
	}
	if err := m.Status.Validate(); err != nil {
		return fmt.Errorf("defeasible: Modifier %q: %w", m.ID, err)
	}
	if err := m.Effect.Validate(); err != nil {
		return fmt.Errorf("defeasible: Modifier %q: %w", m.ID, err)
	}
	for _, sup := range m.Supersedes {
		if sup == m.ID {
			return fmt.Errorf("defeasible: Modifier %q: Supersedes must not include own ID", m.ID)
		}
	}
	return nil
}

// EffectiveProvision is the resolved state of a single provision after all
// active modifiers have been applied.
//
// CUE mirror: #EffectiveProvision in schema.cue.
type EffectiveProvision struct {
	// Key matches the originating Provision.Key.
	Key string

	// Value is the effective text after applying modifiers. Empty when Void=true.
	Value string

	// Void is true when an active EffectStrike modifier was applied.
	Void bool

	// Confidence is the effective certainty after weakest-link bounding across
	// the base and all applied modifiers.
	Confidence float64

	// AppliedModifierIDs lists the IDs of modifiers that were applied, in
	// application order (ascending by Layer, Priority, then ID tiebreak).
	AppliedModifierIDs []string

	// ShadowedModifierIDs lists the IDs of modifiers that were active for this
	// provision but defeated — either by supersession or by losing a conflict.
	// Each entry is accompanied by a reason in the Trace.
	ShadowedModifierIDs []string
}

// TraceEntry records a single resolution decision for audit purposes.
//
// Every active modifier that participates in resolution for a provision appears
// in the Trace exactly once: either as "applied" or with a "shadowed:*" reason.
// No active modifier is silently dropped.
//
// CUE mirror: #TraceEntry in schema.cue.
type TraceEntry struct {
	// ProvisionKey is the key of the provision this decision concerns.
	ProvisionKey string

	// ModifierID is the modifier involved in the decision.
	ModifierID string

	// Decision describes what happened:
	//   "applied"                        — modifier was applied to the chain
	//   "shadowed:superseded"            — removed from contention by supersession
	//   "shadowed:conflict:<reason>"     — lost a same-(Layer,Priority) conflict
	Decision string
}

// Resolution is the output of Resolve. It contains the effective state of
// every provision and the full decision trace.
//
// CUE mirror: #Resolution in schema.cue.
type Resolution struct {
	// Effective is the ordered list of effective provisions, sorted by Key.
	Effective []EffectiveProvision

	// Trace is the ordered list of resolution decisions across all provisions,
	// useful for auditing and explaining the outcome.
	Trace []TraceEntry
}
