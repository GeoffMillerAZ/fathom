// Package evidenceproducer defines the CUE-first contract for the producer
// catalog: for every producer contributing findings or claims to a
// consuming store, its kind, its origin relative to that store, and the
// highest evidence tier it may ever confer. Go types in types.go mirror
// these definitions; any drift is a bug.
//
// This package does not redefine any evidence-tier ladder — a Catalog's
// ladder field is caller-supplied (e.g. the engineering ladder in
// foundations/review/scoring: verified > pre-scan > doc-claimed >
// sme-stated > assumed).
package evidenceproducer

// _nonEmpty is a reusable constraint for non-empty strings.
_nonEmpty: string & !=""

// #ProducerKind classifies who or what produced a finding or claim.
#ProducerKind: "deterministic" | "model" | "human" | "external-tool"

// #Origin locates a producer relative to the consuming store. "self" means
// the producer's output originates from the consuming system's own prior
// analysis re-entering through another tool — refused on import (see
// Catalog.Admit in types.go): a system's own finding re-imported through
// another tool is not a second witness. "external" is a genuinely separate
// producer.
#Origin: "self" | "external"

// #Producer declares one producer's kind, origin, and the ceiling on the
// evidence tier it may ever confer. Served tier = min(claimed tier,
// tier_cap) (see Catalog.Cap in types.go) — tier never rises on import.
#Producer: {
	// id uniquely identifies the producer within its Catalog.
	id: _nonEmpty

	kind:   #ProducerKind
	origin: #Origin

	// tier_cap is the strongest tier this producer may ever confer. It must
	// be a member of the enclosing Catalog's ladder — enforced at the
	// #Catalog level (see #Catalog.producers below) via CUE's or() builtin
	// over the concrete ladder value; left open here because #Producer has
	// no visibility into a sibling ladder on its own.
	tier_cap: _nonEmpty

	// requires names preconditions (e.g. a capability flag or config key)
	// this producer's findings depend on. Entries must be non-empty.
	requires: [..._nonEmpty]

	// version is the producer's own version string, when one exists
	// (e.g. a model ID + snapshot, or a tool release tag).
	version?: string
}

// #Catalog is one tier-profile's producer roster: the tier vocabulary
// (ladder, strongest first) plus every registered producer, each
// constrained so its tier_cap is a member of that ladder.
#Catalog: {
	// profile names the tier-profile this catalog governs (e.g.
	// "engineering" for the ladder defined in foundations/review/scoring).
	profile: _nonEmpty

	// ladder is the tier vocabulary, strongest first. Must be non-empty.
	// This package supplies no default and defines no vocabulary of its
	// own — callers reuse the ladder their profile already uses.
	ladder: [..._nonEmpty] & [_, ...]

	// producers is the roster. Each producer's tier_cap must be one of the
	// literal values present in ladder.
	producers: [...#Producer & {tier_cap: or(ladder)}]
}
