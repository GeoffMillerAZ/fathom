// Package modelcatalog defines the CUE-first contract for per-model capability
// and classification facts. Go types in types.go mirror these definitions; any
// drift between the two is a bug. Run `cue vet ./...` from this directory (or
// the repo root) to validate testdata/valid.cue against these constraints.
//
// NO AUTHORITATIVE ROWS ARE SHIPPED in this package — not in Go, not in CUE.
// Model capabilities and deprecation status change without notice, and a stale
// embedded catalog silently mis-routes requests or mis-gates verdicts. A
// Catalog is always caller-supplied config validated against #ModelCatalog; the
// entries in testdata/valid.cue are illustrative EXAMPLES only, never defaults.
package modelcatalog

// #Caps holds the wire-capability flags for one model. Each flag is a caller
// assertion about what the model accepts on the wire. The kit cannot verify
// these claims; callers are responsible for keeping them current.
#Caps: {
	// thinking indicates the model accepts a "thinking" / extended-reasoning
	// control block on the request.
	thinking: bool | *false

	// effort indicates the model accepts an effort-level control (e.g. a
	// "low"/"medium"/"high" scheduling hint).
	effort: bool | *false

	// temperature indicates the model accepts a temperature parameter.
	// Defaults to true: the majority of models accept temperature, so an
	// absent field is a safe default; set false to suppress the field.
	temperature: bool | *true

	// seed indicates the model accepts a seed parameter for reproducibility.
	seed: bool | *false

	// structured_outputs indicates the model can produce structured output
	// (e.g. JSON mode or a schema-constrained generation mode).
	structured_outputs: bool | *false
}

// #ModelEntry is the CUE contract for a single catalog entry. Every field is
// required (no CUE defaults survive unmarshalling unless the caller supplies
// them); the Go Load function mirrors these constraints in validation.
#ModelEntry: {
	// id is the canonical model identifier string as used on the wire.
	// Must match the map key in #ModelCatalog.
	id: string & !=""

	// family is an opaque, org-defined tag grouping related models
	// (e.g. "alpha", "beta"). It is never an enum — the kit imposes no
	// vocabulary; callers define and maintain their own family taxonomy.
	family: string & !=""

	// lineage lists style-correlated families whose members share training
	// heritage (e.g. distilled models). May be empty. The kit uses lineage
	// to extend independence checks: sharing a lineage entry is treated as a
	// weaker form of sharing a family.
	lineage: [...string]

	// caps holds wire-capability flags for this model.
	caps: #Caps

	// cost_tier is an ordinal within the caller's catalog (0 = least
	// expensive). Its meaning is purely relative within one catalog; the
	// kit never interprets the magnitude.
	cost_tier: int & >=0

	// deprecated marks a model the caller considers retired. Callers should
	// not route new requests to deprecated models; the kit flags catalogs
	// where every member of a family is deprecated.
	deprecated: bool | *false
}

// #ModelCatalog is a map from model ID to its entry. The constraint
// `#ModelEntry & {id: ID}` ensures the id field inside each entry matches the
// map key it lives under — a mismatch is a CUE constraint violation and a Go
// validation error.
#ModelCatalog: {
	models: [ID=string]: #ModelEntry & {id: ID}
}
