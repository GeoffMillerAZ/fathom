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
	evidenceproducer "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/evidence-producer"
)

//go:embed producers.cue
var producersSchema []byte

// engineeringLadder is the evidence-tier vocabulary, strongest first, that
// CatalogFromKit hands to agentic-kit's evidenceproducer.Catalog as its
// Ladder — the same ordering vision 30 §1 / ADR-040 state ("verified >
// pre-scan > doc-claimed > sme-stated > assumed"). SU-25 (ADR-041 decision
// 8): this is the only place that ordering is declared now that the
// formerly-local tierRank map is gone — CapTier, ProducerCatalog.Lookup,
// Admit and Independent all run over the kit Catalog built from it.
var engineeringLadder = []string{
	string(core.TierVerified),
	string(core.TierPreScan),
	string(core.TierDocClaimed),
	string(core.TierSMEStated),
	string(core.TierAssumed),
}

// Producer is the decoded runtime form of one #Producer catalog entry.
type Producer struct {
	ID       string
	Kind     string
	Origin   string
	TierCap  core.EvidenceTier
	Requires []string
}

// ProducerCatalog is the validated, loaded set of catalogued producers. kit
// is the agentic-kit evidenceproducer.Catalog built from the same producers
// by CatalogFromKit — Lookup, CapTier, Admit and Independent all delegate
// to it rather than reimplementing tier arithmetic or origin checks here.
type ProducerCatalog struct {
	kit evidenceproducer.Catalog
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

	producers := make([]Producer, 0, len(ids))
	for _, id := range ids {
		producers = append(producers, byID[id])
	}

	return ProducerCatalog{kit: CatalogFromKit(producers), ids: ids}, nil
}

// toKitOrigin maps fathom's origin vocabulary ("fathom"/"external") to
// agentic-kit's ("self"/"external") — SU-25, ADR-041 decision 8. fathom's
// "fathom" means the same thing as the kit's "self": the producer's output
// originates from the consuming system's own prior analysis.
func toKitOrigin(origin string) evidenceproducer.Origin {
	if origin == "fathom" {
		return evidenceproducer.OriginSelf
	}
	return evidenceproducer.OriginExternal
}

// fromKitOrigin is toKitOrigin's inverse, used when translating a kit
// Producer back into fathom's own wire shape (ProducerCatalog.Lookup).
func fromKitOrigin(origin evidenceproducer.Origin) string {
	if origin == evidenceproducer.OriginSelf {
		return "fathom"
	}
	return "external"
}

// CatalogFromKit builds an agentic-kit evidenceproducer.Catalog from
// fathom's own producers (schema/semantic/producers.cue, decoded by
// LoadProducers), under the shared engineeringLadder and with origin
// mapped by toKitOrigin — SU-25, ADR-041 decision 8. LoadProducers calls
// this once at load time; ProducerCatalog.Lookup, CapTier, Admit and
// Independent all delegate to the result's Lookup/Cap/Admit/Independent
// rather than reimplementing tier arithmetic or origin checks locally.
func CatalogFromKit(producers []Producer) evidenceproducer.Catalog {
	kp := make([]evidenceproducer.Producer, len(producers))
	for i, p := range producers {
		kp[i] = evidenceproducer.Producer{
			ID:       p.ID,
			Kind:     evidenceproducer.Kind(p.Kind),
			Origin:   toKitOrigin(p.Origin),
			TierCap:  string(p.TierCap),
			Requires: append([]string(nil), p.Requires...),
		}
	}
	return evidenceproducer.Catalog{
		Profile:   "engineering",
		Ladder:    engineeringLadder,
		Producers: kp,
	}
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

// Lookup returns the catalogued producer for id, if any — delegated to the
// kit Catalog's own Lookup (SU-25, ADR-041 decision 8), translating the
// result back into fathom's wire shape (fromKitOrigin).
func (c ProducerCatalog) Lookup(id string) (Producer, bool) {
	kp, ok := c.kit.Lookup(id)
	if !ok {
		return Producer{}, false
	}
	return Producer{
		ID:       kp.ID,
		Kind:     string(kp.Kind),
		Origin:   fromKitOrigin(kp.Origin),
		TierCap:  core.EvidenceTier(kp.TierCap),
		Requires: append([]string(nil), kp.Requires...),
	}, true
}

// IDs returns the sorted list of every catalogued producer id.
func (c ProducerCatalog) IDs() []string {
	return append([]string(nil), c.ids...)
}

// CapTier returns the weaker of event (the finding's own stated tier) and
// the producer's tier_cap — "served tier = min(event tier, producer cap)",
// vision 30 §3.3 item 5 / §4 item 1 — delegated to agentic-kit's
// evidenceproducer.Catalog.Cap over engineeringLadder (SU-25, ADR-041
// decision 8; deletes the formerly-local tierRank map). Tier never rises on
// import: an event tier or producer cap that is not on the ladder cannot be
// ranked, so Cap is not called and event passes through unchanged — the
// same floor convention the deleted tierRank map gave, and unreachable in
// practice since both ParseProjection (event) and LoadProducers (producer)
// already refuse anything off the five-tier ladder before CapTier ever
// sees it.
func CapTier(event core.EvidenceTier, p Producer) core.EvidenceTier {
	kc := evidenceproducer.Catalog{Ladder: engineeringLadder}
	if _, ok := kc.Rank(string(event)); !ok {
		return event
	}
	if _, ok := kc.Rank(string(p.TierCap)); !ok {
		return event
	}
	served, err := kc.Cap(string(event), evidenceproducer.Producer{TierCap: string(p.TierCap)})
	if err != nil {
		return event
	}
	return core.EvidenceTier(served)
}

// Admit returns an error if p's origin is fathom itself — the fathom-origin
// refusal vision 30 §4 / ADR-040 decision 3 requires (importing a native
// finding back through another tool is circular proof) — delegated to the
// kit Catalog's own Admit (SU-25, ADR-041 decision 8). Callers name their
// own rejection reason; this only decides whether to refuse.
func (c ProducerCatalog) Admit(p Producer) error {
	return c.kit.Admit(evidenceproducer.Producer{ID: p.ID, Origin: toKitOrigin(p.Origin)})
}

// Independent reports whether a and b are independent producers (different
// ids) — corroboration must count only across independent producers, never
// the same one twice (vision 30 §4 / ADR-040 decision 4) — delegated to the
// kit Catalog's own Independent (SU-25, ADR-041 decision 8).
func (c ProducerCatalog) Independent(a, b Producer) bool {
	return c.kit.Independent(evidenceproducer.Producer{ID: a.ID}, evidenceproducer.Producer{ID: b.ID})
}
