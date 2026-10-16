// Package evidenceproducer defines the catalog of producers that
// contribute findings or claims to a consuming evidence store: for each
// producer, its kind (deterministic analyzer, model reviewer, human,
// external tool), its origin relative to the consuming store (self or
// external), and the highest evidence tier it may ever confer.
//
// Purpose. Findings and claims arriving from many producers need a catalog
// that declares, per producer, its kind, origin, and tier cap, so that:
//
//  1. Tier never rises on import — served tier = min(claimed tier,
//     producer cap). See Catalog.Cap.
//  2. Corroboration counts only across independent producers, never the
//     same producer twice. See Catalog.Independent.
//  3. A producer whose origin is the consuming system itself is refused on
//     import — circular proof: a system's own finding re-imported through
//     another tool is not a second witness. See Catalog.Admit.
//
// This package does not define an evidence-tier ladder of its own. A
// Catalog's Ladder is supplied by the caller from whatever vocabulary its
// profile already uses — e.g. the engineering ladder in
// foundations/review/scoring (TierVerified > TierPreScan > TierDocClaimed >
// TierSMEStated > TierAssumed), reused directly by DefaultKindCaps below
// rather than re-declared here.
//
// First consumer: fathom's internal/semantic/producers.go (the review-kit
// -> fathom findings loop, fathom ADR-040). Failure class closed: a
// model-authored review finding entering a store at "verified" because the
// transport lost the producer identity, and a native finding round-tripping
// through a review tool to be counted as a second corroborating witness of
// itself.
//
// CUE contracts live in schema.cue; this file is the Go counterpart and
// must stay in sync with that schema — drift is a bug.
//
// Concurrency contract: Producer and Catalog are plain immutable value
// types; every method is a pure function over its receiver and arguments,
// safe for unrestricted concurrent use.
package evidenceproducer

import (
	"fmt"

	scoring "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/review/scoring"
)

// Kind classifies who or what produced a finding or claim.
//
// CUE mirror: #ProducerKind in schema.cue.
type Kind string

const (
	// KindDeterministic is a deterministic analyzer (static analysis, a
	// linter, a fixed rule engine) — same input, same output.
	KindDeterministic Kind = "deterministic"

	// KindModel is a model-based reviewer (an LLM finding or judgment).
	KindModel Kind = "model"

	// KindHuman is a human producer.
	KindHuman Kind = "human"

	// KindExternalTool is a third-party tool outside this system's own
	// analyzers (e.g. a vendored scanner, a SaaS review product).
	KindExternalTool Kind = "external-tool"
)

// validKinds is the canonical set for validation.
var validKinds = map[Kind]struct{}{
	KindDeterministic: {},
	KindModel:         {},
	KindHuman:         {},
	KindExternalTool:  {},
}

// validate returns an error if k is not one of the four recognised kinds.
func (k Kind) validate() error {
	if _, ok := validKinds[k]; !ok {
		return fmt.Errorf("evidenceproducer: unknown kind %q; must be one of deterministic|model|human|external-tool", string(k))
	}
	return nil
}

// Origin locates a producer relative to the consuming store.
//
// CUE mirror: #Origin in schema.cue.
type Origin string

const (
	// OriginSelf means the producer's output originates from the consuming
	// system's own prior analysis re-entering through another tool.
	// Catalog.Admit refuses any producer with this origin.
	OriginSelf Origin = "self"

	// OriginExternal means the producer is genuinely separate from the
	// consuming system.
	OriginExternal Origin = "external"
)

// validOrigins is the canonical set for validation.
var validOrigins = map[Origin]struct{}{
	OriginSelf:     {},
	OriginExternal: {},
}

// validate returns an error if o is not one of the two recognised origins.
func (o Origin) validate() error {
	if _, ok := validOrigins[o]; !ok {
		return fmt.Errorf("evidenceproducer: unknown origin %q; must be one of self|external", string(o))
	}
	return nil
}

// Producer declares one producer's kind, origin, and the ceiling on the
// evidence tier it may ever confer.
//
// CUE mirror: #Producer in schema.cue.
type Producer struct {
	// ID uniquely identifies the producer within its Catalog.
	ID string

	Kind   Kind
	Origin Origin

	// TierCap is the strongest tier this producer may ever confer. It must
	// be a member of the enclosing Catalog's Ladder.
	TierCap string

	// Requires names preconditions (e.g. a capability flag or config key)
	// this producer's findings depend on. Entries must be non-empty.
	Requires []string

	// Version is the producer's own version string, when one exists (e.g.
	// a model ID + snapshot, or a tool release tag). Optional.
	Version string
}

// Catalog is one tier-profile's producer roster: the tier vocabulary
// (Ladder, strongest first) plus every registered producer.
//
// CUE mirror: #Catalog in schema.cue.
type Catalog struct {
	// Profile names the tier-profile this catalog governs (e.g.
	// "engineering" for the ladder in foundations/review/scoring).
	Profile string

	// Ladder is the tier vocabulary, strongest first. This package defines
	// no vocabulary of its own — callers reuse the ladder their profile
	// already uses.
	Ladder []string

	// Producers is the roster.
	Producers []Producer
}

// Validate returns an error if the Catalog violates any invariant:
//   - Profile must be non-empty.
//   - Ladder must be non-empty with unique entries.
//   - Producer IDs must be non-empty and unique.
//   - Each Producer.Kind and Producer.Origin must be recognised.
//   - Each Producer.TierCap must be a member of Ladder.
//   - Each Producer.Requires entry must be non-empty.
func (c Catalog) Validate() error {
	if c.Profile == "" {
		return fmt.Errorf("evidenceproducer: Catalog.Profile must not be empty")
	}
	if len(c.Ladder) == 0 {
		return fmt.Errorf("evidenceproducer: Catalog.Ladder must not be empty")
	}
	tiers := make(map[string]struct{}, len(c.Ladder))
	for i, t := range c.Ladder {
		if t == "" {
			return fmt.Errorf("evidenceproducer: Catalog.Ladder[%d] must not be empty", i)
		}
		if _, dup := tiers[t]; dup {
			return fmt.Errorf("evidenceproducer: Catalog.Ladder has duplicate tier %q", t)
		}
		tiers[t] = struct{}{}
	}

	ids := make(map[string]struct{}, len(c.Producers))
	for i, p := range c.Producers {
		if p.ID == "" {
			return fmt.Errorf("evidenceproducer: Producers[%d].ID must not be empty", i)
		}
		if _, dup := ids[p.ID]; dup {
			return fmt.Errorf("evidenceproducer: duplicate producer id %q", p.ID)
		}
		ids[p.ID] = struct{}{}

		if err := p.Kind.validate(); err != nil {
			return fmt.Errorf("evidenceproducer: producer %q: %w", p.ID, err)
		}
		if err := p.Origin.validate(); err != nil {
			return fmt.Errorf("evidenceproducer: producer %q: %w", p.ID, err)
		}
		if _, ok := tiers[p.TierCap]; !ok {
			return fmt.Errorf("evidenceproducer: producer %q TierCap %q is not a member of Catalog.Ladder", p.ID, p.TierCap)
		}
		for j, r := range p.Requires {
			if r == "" {
				return fmt.Errorf("evidenceproducer: producer %q Requires[%d] must not be empty", p.ID, j)
			}
		}
	}
	return nil
}

// Lookup returns the producer registered under id, and whether it was found.
func (c Catalog) Lookup(id string) (Producer, bool) {
	for _, p := range c.Producers {
		if p.ID == id {
			return p, true
		}
	}
	return Producer{}, false
}

// Rank returns tier's position in Ladder (0 = strongest) and whether tier
// is a member of Ladder at all.
func (c Catalog) Rank(tier string) (int, bool) {
	for i, t := range c.Ladder {
		if t == tier {
			return i, true
		}
	}
	return 0, false
}

// Cap returns the weaker of claimed and p.TierCap — the tier actually
// served for an import from p — so tier can never rise on import (invariant
// 1). Returns an error if claimed or p.TierCap is not a member of Ladder.
func (c Catalog) Cap(claimed string, p Producer) (string, error) {
	claimedRank, ok := c.Rank(claimed)
	if !ok {
		return "", fmt.Errorf("evidenceproducer: claimed tier %q is not a member of Catalog.Ladder", claimed)
	}
	capRank, ok := c.Rank(p.TierCap)
	if !ok {
		return "", fmt.Errorf("evidenceproducer: producer %q TierCap %q is not a member of Catalog.Ladder", p.ID, p.TierCap)
	}
	// Higher rank number = weaker tier (Ladder is strongest first). The
	// weaker of the two wins.
	if capRank > claimedRank {
		return p.TierCap, nil
	}
	return claimed, nil
}

// Independent reports whether a and b are independent producers — i.e.
// different IDs. Corroboration must count only across independent
// producers (invariant 2): the same producer counted twice is not a second
// witness.
func (c Catalog) Independent(a, b Producer) bool {
	return a.ID != b.ID
}

// Admit returns an error if p's origin is the consuming system itself
// (invariant 3): a system's own finding re-imported through another tool is
// not a second witness, and is refused on import.
func (c Catalog) Admit(p Producer) error {
	if p.Origin == OriginSelf {
		return fmt.Errorf("circular: producer %q originates from the consuming system", p.ID)
	}
	return nil
}

// DefaultKindCaps documents the recommended TierCap for each Kind under the
// engineering evidence-tier ladder in foundations/review/scoring
// (TierVerified > TierPreScan > TierDocClaimed > TierSMEStated >
// TierAssumed). It is a helper for callers assembling a Catalog under that
// profile; Validate never enforces it — a Catalog's Ladder and each
// producer's TierCap are the caller's own contract, so a catalog is free to
// set stricter or looser caps than recommended here, and a catalog under a
// different profile's ladder does not use this helper at all.
//
// Recommended caps:
//   - KindDeterministic: TierPreScan — a deterministic analyzer's raw
//     output is unverified until a human or a declared oracle confirms it.
//     A producer that DOES have a declared oracle may set a stronger
//     TierCap explicitly; this default assumes none is declared.
//   - KindModel: TierSMEStated — a model reviewer's finding is an
//     assertion, not independent verification.
//   - KindHuman: TierSMEStated — an unverified human statement carries the
//     same epistemic weight as any subject-matter-expert assertion.
//   - KindExternalTool: TierPreScan — an external tool's output is treated
//     the same as an in-house deterministic pass: a pre-scan, not verified
//     truth.
func DefaultKindCaps() map[Kind]string {
	return map[Kind]string{
		KindDeterministic: scoring.TierPreScan,
		KindModel:         scoring.TierSMEStated,
		KindHuman:         scoring.TierSMEStated,
		KindExternalTool:  scoring.TierPreScan,
	}
}
