// Package modelcatalog provides a CUE-contracted home for per-model capability
// and classification facts, together with the pure checks that consume them.
//
// # What the kit ships
//
// The kit ships the schema (schema.cue), the loader, the validators, and
// EXAMPLE testdata — never authoritative rows. Model capabilities, family
// membership, and deprecation status are caller assertions: the kit cannot know
// whether a caller's table is current, and a stale embedded catalog silently
// mis-routes requests or mis-gates verdicts. Every Catalog is caller-supplied
// config validated against #ModelCatalog; the entries in testdata/valid.cue are
// illustrative examples only, never defaults. (Precedent: cost-tracker ships no
// prices for the same reason.)
//
// # Non-guarantees
//
// Family separation mitigates self-preference bias in LLM-as-judge settings,
// but it is not a complete solution. Position bias, verbosity bias, and
// stylistic bias persist across family boundaries; these are well documented in
// LLM-as-judge evaluations and are not addressed by catalog independence checks.
// Treat ValidateIndependence as a necessary, not sufficient, safeguard.
//
// # Concurrency contract
//
// A Catalog is immutable after Load and safe to share across goroutines without
// additional locking. The Load function itself is not re-entrant, but each Load
// call operates on independent state and may be called concurrently.
//
// CUE contracts live in schema.cue; this file is the Go counterpart and must
// stay in sync with that schema. No LLM is involved; every decision is fully
// reproducible given the same inputs.
package modelcatalog

// Caps holds the wire-capability flags for one model. Each flag is a caller
// assertion about what the model accepts on the wire.
//
// CUE mirror: #Caps in schema.cue.
type Caps struct {
	// Thinking indicates the model accepts a "thinking" / extended-reasoning
	// control block on the request.
	Thinking bool `json:"thinking"`

	// Effort indicates the model accepts an effort-level control.
	Effort bool `json:"effort"`

	// Temperature indicates the model accepts a temperature parameter.
	// The CUE default is true; callers must set this field explicitly in JSON.
	Temperature bool `json:"temperature"`

	// Seed indicates the model accepts a seed parameter.
	Seed bool `json:"seed"`

	// StructuredOutputs indicates the model can produce structured output.
	StructuredOutputs bool `json:"structured_outputs"`
}

// ModelEntry is the Go mirror of #ModelEntry in schema.cue. It holds the
// capability and classification facts for a single model. All fields are
// required; Load rejects entries with an empty id or family, a negative
// cost_tier, or a mismatched id/key.
//
// CUE mirror: #ModelEntry in schema.cue.
type ModelEntry struct {
	// ID is the canonical model identifier as used on the wire.
	ID string `json:"id"`

	// Family is an opaque, org-defined tag grouping related models. The kit
	// imposes no vocabulary; callers define and maintain their own taxonomy.
	Family string `json:"family"`

	// Lineage lists style-correlated families whose members share training
	// heritage (e.g. a distilled model may list its teacher's family here).
	// May be nil or empty.
	Lineage []string `json:"lineage"`

	// Caps holds wire-capability flags for this model.
	Caps Caps `json:"caps"`

	// CostTier is an ordinal within the caller's catalog (0 = least
	// expensive). Meaning is relative within one catalog.
	CostTier int `json:"cost_tier"`

	// Deprecated marks a model the caller considers retired.
	Deprecated bool `json:"deprecated"`
}

// catalogJSON is the JSON wire shape for a Catalog, mirroring #ModelCatalog.
type catalogJSON struct {
	Models map[string]ModelEntry `json:"models"`
}

// Problem is a structural or semantic issue found by Catalog.Validate.
// Problems are informational and do not prevent a Catalog from being used;
// callers decide whether to treat problems as fatal.
type Problem struct {
	// Kind is a machine-readable category (e.g. "dangling_lineage_ref",
	// "all_deprecated_family").
	Kind string

	// Detail is a human-readable description of the specific problem.
	Detail string
}

// IndependenceOpts carries caller-supplied assertions for ValidateIndependence.
// Each flag, when true, asserts that the named condition holds; if it does
// hold, no Violation is added for it. If the flag is false (the default),
// the corresponding condition is not checked.
//
// These opts allow callers to surface session-level coupling (e.g. the judge
// and producer ran in the same session) as explicit assertions rather than
// silent omissions. Violations derived from opts are construction-time fatal
// in consumers — check them before spending any tokens.
type IndependenceOpts struct {
	// SameSession, when true, asserts that the judge and producer share the
	// same session context. This is always a Violation: a shared session can
	// carry implicit priming that undermines independence.
	SameSession bool

	// SharedHistory, when true, asserts that the judge has access to the
	// producer's generation history. This is always a Violation.
	SharedHistory bool
}

// Violation is an independence constraint that is not satisfied.
// Violations are designed to be construction-time fatal: callers should
// treat any non-empty []Violation as a hard error before routing any
// requests or accepting any verdicts.
type Violation struct {
	// Kind is a machine-readable category (e.g. "shared_family",
	// "shared_lineage", "unknown_model", "same_session", "shared_history").
	Kind string

	// Detail is a human-readable description of the specific violation.
	Detail string
}
