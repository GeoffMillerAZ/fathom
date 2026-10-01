// Package decision defines the CUE-first contracts for the fast-decision
// port: the shape of a Question a caller may ask, the Request that
// bundles State with the Questions to ask about it, and the composition
// #Policy that ComposeFromConfig assembles decorators from. Go types in
// types.go and compose.go mirror these definitions; any drift is a bug.
//
// Kind-conditional requirements CUE cannot express cleanly across one open
// struct (Options required and bounded to 2..255 only for "choice", Max>Min
// only for "score", ID/Kind uniqueness across a Request) are enforced in
// Go's Validate — see that function's doc comment for the full contract.
package decision

import "list"

// _nonEmpty is a reusable constraint for non-empty strings.
_nonEmpty: string & !=""

// #Kind is the closed set of fast-decision question shapes: three generic
// kinds, "choice", "score" and "predicate" (a yes/no probability). "noul"
// is the decode alias for "predicate" (its former name and one vendor's
// term): stored rows and configuration that say "noul" still validate, and
// Go's ParseKind decodes them to KindPredicate. New data writes "predicate".
#Kind: "choice" | "score" | "predicate" | "noul"

// #Option is one named alternative for a #Kind "choice" Question.
// Criteria is optional free text describing when this option applies; it
// may be empty.
#Option: {
	Name:     _nonEmpty
	Criteria: string
}

// #Question is one typed fast-decision question. Kind selects which of the
// remaining fields are meaningful:
//   - "choice": Options (2..255 entries, enforced in Go — see Validate)
//   - "score":  Min, Max (Max > Min, enforced in Go), Rubric optional,
//     Levels optional (at least 2 ordered entries, no upper bound —
//     an adapter's own level limit is its declared Capabilities, not the
//     schema's; when set, Min/Max must be unset or exactly
//     0/len(Levels)-1, enforced in Go)
//   - "predicate" (alias "noul"): TrueCriteria/FalseCriteria optional
//
// Fields not meaningful for a given Kind are simply left at their zero
// value; CUE keeps them open here rather than modeling three disjoint
// struct shapes, matching Go's single-struct Question type field for field.
#Question: {
	ID:            _nonEmpty
	Kind:          #Kind
	Instructions:  string
	Options: [...#Option]
	Min:           number
	Max:           number
	Rubric:        string
	TrueCriteria:  string
	FalseCriteria: string
	Levels: [...string]
}

// #Request bundles a caller's State (the blob of state being asked about,
// rendered as text) with the Questions to ask about it.
#Request: {
	State: _nonEmpty
	Questions: [...#Question]
}

// #Policy is one node of a composition policy (Go: Policy, assembled by
// ComposeFromConfig): exactly one node type. "provider" names a leaf
// Decider by its Registry.Deciders key; every other type is a decorator
// whose children are #Policy nodes again. Each disjunct is closed, so an
// unknown node type, a second node type or an unknown field is rejected.
//
// Keys a node names (provider, store, transcript, observe) resolve from
// the caller's Registry in Go, which refuses an unknown key with the
// node's path. A decorator's "inner" is optional here only because CUE
// v0.16 reports a required recursive field as a structural cycle; Go
// refuses a cache or record node without one.
#Policy: {provider: _nonEmpty} | {cascade: #CascadePolicy} | {fallback: #FallbackPolicy} | {cache: #CachePolicy} | {record: #RecordPolicy} | {replay: #ReplayPolicy}

// #CascadePolicy is a cascade node (Go: CascadePolicy, NewCascade):
// stages cheapest first. An absent escalate_below means
// DefaultEscalateBelow; 0 never escalates on confidence.
#CascadePolicy: {
	name?: _nonEmpty
	stages!: [...#Policy] & list.MinItems(1)
	order_by_cost?:  bool
	escalate_below?: number & >=0 & <=1
	observe?:        _nonEmpty
}

// #FallbackPolicy is a fallback node (Go: FallbackPolicy, NewFallback):
// deciders tried in order. observe is required: a fallback is never
// silent.
#FallbackPolicy: {
	name?: _nonEmpty
	deciders!: [...#Policy] & list.MinItems(1)
	observe!: _nonEmpty
}

// #CachePolicy is a cache node (Go: CachePolicy, NewCached) over a store
// the caller names; template_version is part of every cache key.
#CachePolicy: {
	name?:             _nonEmpty
	store!:            _nonEmpty
	template_version!: _nonEmpty
	observe?:          _nonEmpty
	inner?:            #Policy
}

// #RecordPolicy is a record node (Go: RecordPolicy, NewRecorder); name is
// required so the transcript is reachable through Composed.Node.
#RecordPolicy: {
	name!:  _nonEmpty
	inner?: #Policy
}

// #ReplayPolicy is a replay node (Go: ReplayPolicy, NewReplay), a leaf
// over a transcript the caller names.
#ReplayPolicy: {
	name?:       _nonEmpty
	transcript!: _nonEmpty
}
