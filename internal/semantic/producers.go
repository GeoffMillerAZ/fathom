package semantic

// producers.go — the findings-loop producer catalog (vision 30 §4,
// ADR-040 decision 4, plan FL-1/FL-2). schema/semantic/producers.cue is
// authoritative; producers.cue here is a byte-identical mirror (drift-gated
// by TestProducersSchemaMatchesSource, mirroring
// TestManifestSchemaMatchesSource) go:embed'd into the binary, loaded and
// validated once by LoadProducers.
//
// A producer catalog is deliberately simpler than the #Layer/layer.cue pack
// convention (manifest.go): one file, one list, no per-producer directory —
// there is no prompt, no corpus, nothing else a producer instance owns.

import (
	_ "embed"
	"fmt"
	"sort"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"

	"github.com/geoffmilleraz/fathom/internal/core"
)

//go:embed producers.cue
var producersSchema []byte

// Producer is the decoded runtime form of one #Producer catalog entry.
type Producer struct {
	ID       string
	Kind     string
	Origin   string
	TierCap  core.EvidenceTier
	Requires []string
}

// ProducerCatalog is the validated, loaded set of catalogued producers.
type ProducerCatalog struct {
	byID map[string]Producer
	// ids is the sorted producer id list — deterministic iteration/output.
	ids []string
}

// cueProducer mirrors #Producer's JSON shape for decoding.
type cueProducer struct {
	ID       string   `json:"id"`
	Kind     string   `json:"kind"`
	Origin   string   `json:"origin"`
	TierCap  string   `json:"tier_cap"`
	Requires []string `json:"requires"`
}

// validProducerKinds and validProducerOrigins mirror the #Producer
// disjunctions — re-validated in Go so an unknown value is a named refusal
// even though CUE unification already guards the embedded catalog (a
// project-local catalog is not on the table for v1; this defends the one
// loader path there is).
var validProducerKinds = map[string]bool{
	"deterministic": true, "model": true, "human": true, "external-tool": true,
}

var validProducerOrigins = map[string]bool{
	"fathom": true, "external": true,
}

// LoadProducers parses and validates raw CUE bytes against #Producer,
// returning the catalog. Every validation rule vision 30 §4 states is
// enforced here, at load — never deferred to the importer: an unknown
// kind/origin, a tier_cap above "verified", or a duplicate id is a named
// refusal.
func LoadProducers(raw []byte) (ProducerCatalog, error) {
	cctx := cuecontext.New()
	v := cctx.CompileBytes(raw, cue.Filename("schema/semantic/producers.cue"))
	if err := v.Err(); err != nil {
		return ProducerCatalog{}, fmt.Errorf("semantic: compile producers catalog: %w", err)
	}
	if err := v.Validate(cue.Concrete(true)); err != nil {
		return ProducerCatalog{}, fmt.Errorf("semantic: producers catalog does not satisfy #Producer: %w", err)
	}

	var raws []cueProducer
	if err := v.LookupPath(cue.ParsePath("producers")).Decode(&raws); err != nil {
		return ProducerCatalog{}, fmt.Errorf("semantic: decode producers catalog: %w", err)
	}

	byID := make(map[string]Producer, len(raws))
	for _, rp := range raws {
		if rp.ID == "" {
			return ProducerCatalog{}, fmt.Errorf("semantic: producers catalog: producer with empty id")
		}
		if _, dup := byID[rp.ID]; dup {
			return ProducerCatalog{}, fmt.Errorf("semantic: producers catalog: duplicate producer id %q", rp.ID)
		}
		if !validProducerKinds[rp.Kind] {
			return ProducerCatalog{}, fmt.Errorf("semantic: producer %q: unknown kind %q", rp.ID, rp.Kind)
		}
		if !validProducerOrigins[rp.Origin] {
			return ProducerCatalog{}, fmt.Errorf("semantic: producer %q: unknown origin %q", rp.ID, rp.Origin)
		}
		tier := core.EvidenceTier(rp.TierCap)
		if !tier.Valid() {
			return ProducerCatalog{}, fmt.Errorf("semantic: producer %q: unknown tier_cap %q", rp.ID, rp.TierCap)
		}
		if tier == core.TierVerified && rp.Kind != "deterministic" {
			// A cap above verified is impossible (verified is the ladder's
			// own ceiling); this instead catches the adjacent mistake vision
			// 30 §4 item 1 names explicitly — a non-deterministic producer
			// capped at the deterministic-oracle tier.
			return ProducerCatalog{}, fmt.Errorf("semantic: producer %q: tier_cap %q requires kind \"deterministic\" (got %q)", rp.ID, rp.TierCap, rp.Kind)
		}
		byID[rp.ID] = Producer{
			ID:       rp.ID,
			Kind:     rp.Kind,
			Origin:   rp.Origin,
			TierCap:  tier,
			Requires: append([]string(nil), rp.Requires...),
		}
	}

	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	return ProducerCatalog{byID: byID, ids: ids}, nil
}

// DefaultProducers returns the shipped catalog (schema/semantic/
// producers.cue, embedded). Panics on load failure — the embedded catalog
// is a build-time invariant, not a runtime possibility (mirrors
// internal/config's schema-embed convention: a corrupt built-in is a build
// bug, never a caller's problem to handle).
func DefaultProducers() ProducerCatalog {
	cat, err := LoadProducers(producersSchema)
	if err != nil {
		panic(fmt.Sprintf("semantic: embedded producers catalog failed to load: %v", err))
	}
	return cat
}

// Lookup returns the catalogued producer for id, if any.
func (c ProducerCatalog) Lookup(id string) (Producer, bool) {
	p, ok := c.byID[id]
	return p, ok
}

// IDs returns the sorted list of every catalogued producer id.
func (c ProducerCatalog) IDs() []string {
	return append([]string(nil), c.ids...)
}

// tierRank ranks the five-tier evidence ladder strongest-first — the same
// ordering vision 30 §1 and ADR-040 state ("verified > pre-scan >
// doc-claimed > sme-stated > assumed"). Kept local rather than imported: no
// exported rank helper exists on core.EvidenceTier itself (internal/memory's
// tierLadder/ladderIndex/weaker are unexported and internal/findings/
// external's declared import set does not include internal/memory), and this
// is a three-line, independently-tested piece of arithmetic — CapTier's own
// table test pins the ordering.
var tierRank = map[core.EvidenceTier]int{
	core.TierVerified:   4,
	core.TierPreScan:    3,
	core.TierDocClaimed: 2,
	core.TierSMEStated:  1,
	core.TierAssumed:    0,
}

// CapTier returns the weaker of event (the finding's own stated tier) and
// the producer's tier_cap — "served tier = min(event tier, producer cap)",
// vision 30 §3.3 item 5 / §4 item 1. Tier never rises on import: an
// unrecognized event tier ranks at the floor, same as
// internal/memory.ladderIndex's convention for an unrecognized tier.
func CapTier(event core.EvidenceTier, p Producer) core.EvidenceTier {
	er, eok := tierRank[event]
	if !eok {
		er = -1
	}
	pr, pok := tierRank[p.TierCap]
	if !pok {
		pr = -1
	}
	if er <= pr {
		return event
	}
	return p.TierCap
}
