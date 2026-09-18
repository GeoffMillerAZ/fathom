// producers.cue — the findings-loop producer catalog (vision 30 §4,
// ADR-040 decision 4): every producer that may emit a finding into the
// external import path (`fathom findings import`) is named here, with the
// kind of check it runs, whether it lives inside fathom or outside it, and
// the highest evidence tier a claim from it may ever carry. Served tier is
// always min(event tier, this cap) — internal/semantic.CapTier applies it,
// never the importer improvising a number.
//
// #Producer is the shape; producers is the shipped list, loaded and
// validated at start by internal/semantic.LoadProducers (mirrors the
// #Layer/layer.cue pack-loading convention in internal/semantic/manifest.go
// — a producer catalog is simpler: one file, one list, no per-producer
// directory). internal/semantic/producers.cue is a byte-identical mirror,
// go:embed'd into the binary; TestProducersSchemaMatchesSource is the drift
// gate (mirrors TestManifestSchemaMatchesSource).
package semantic

// #Producer is one catalogued source of external findings.
#Producer: {
	// id is the producer's stable identity, namespaced by its owner
	// ("reviewkit.system-review", "human.review") — the exact string an
	// import event's producer field and `fathom findings import --producer`
	// must match.
	id: string & =~"^[a-z][a-z0-9]*(\\.[a-z][a-z0-9-]*)*$"

	// kind names WHAT KIND OF CHECK produced the finding: a deterministic
	// tool, a model, a human reviewer, or an external-tool integration
	// (e.g. a linter fathom did not author).
	kind: "deterministic" | "model" | "human" | "external-tool"

	// origin is "fathom" for a producer whose findings already exist as
	// native fathom claims (review-verb/v1) — importing one back in would
	// be circular proof, and the importer refuses it outright — or
	// "external" for every other producer.
	origin: "fathom" | "external"

	// tier_cap is the highest evidence tier a claim from this producer may
	// ever be served at: served tier = min(event tier, tier_cap). Never
	// above "verified", and a "verified" cap is reserved for a producer
	// whose method is itself a deterministic oracle.
	tier_cap: "verified" | "pre-scan" | "doc-claimed" | "sme-stated" | "assumed"

	// requires lists event fields that must be present for this producer's
	// events to import cleanly (e.g. "rule.closure" for a producer whose
	// findings are meaningless without a stated remediation).
	requires: [...string]
}

// producers is the shipped catalog.
producers: [...#Producer] & [
	{
		id:       "fathom.review"
		kind:     "deterministic"
		origin:   "fathom"
		tier_cap: "verified"
		requires: []
	},
	{
		// origin "fathom": this producer's findings already exist as native
		// fathom claims — a projection of them back through the external
		// importer is circular proof and is refused (vision 30 §2, ADR-040
		// decision 3).
		id:       "reviewkit.review-import"
		kind:     "deterministic"
		origin:   "fathom"
		tier_cap: "verified"
		requires: []
	},
	{
		id:       "reviewkit.system-review"
		kind:     "deterministic"
		origin:   "external"
		tier_cap: "pre-scan"
		requires: ["rule.closure"]
	},
	{
		id:       "reviewkit.security-review"
		kind:     "deterministic"
		origin:   "external"
		tier_cap: "pre-scan"
		requires: ["rule.closure"]
	},
	{
		id:       "human.review"
		kind:     "human"
		origin:   "external"
		tier_cap: "sme-stated"
		requires: []
	},
	{
		id:       "model.review"
		kind:     "model"
		origin:   "external"
		tier_cap: "sme-stated"
		requires: []
	},
]
