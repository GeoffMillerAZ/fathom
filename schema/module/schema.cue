// Package modspec defines the CUE schema for an expert-module manifest
// (docs/spec/expert-module-v0.md §3). A module manifest describes a
// conforming store (docs/spec/store-format-v0.md) that has been packaged as
// a standalone expert module: its corpus selectors, the watermark it was
// compiled at, its tier vocabulary, its eval evidence, whether and how it
// may be recompiled, and its provenance.
//
// This file mirrors schema/semantic/schema.cue's #Layer shape and
// conventions deliberately — a module manifest and a layer definition are
// sibling concepts (both name a corpus + compile identity), and this
// package reuses #Layer's field patterns (name/version regexes, the
// corpus.include non-empty-list shape, tier_profile's default) rather than
// inventing parallel ones.
//
// Status: v0 (docs/spec/README.md's stability policy — additive within v0,
// never renamed/retyped/removed). No runtime loader for #Module ships in
// this repo yet (docs/spec/expert-module-v0.md §7's honest gap) — this
// schema validates the SHAPE; internal/modspec/schema_test.go asserts the
// negatives per repo convention (CLAUDE.md "New CUE schemas").
package modspec

// #Module is one expert-module manifest.
#Module: {
	// name identifies the module ("engineering-standards-module", …),
	// mirroring #Layer.name's convention (schema/semantic/schema.cue).
	name: string & =~"^[a-z][a-z0-9-]*$"

	// version of THIS MODULE DEFINITION itself — mirroring #Layer.version's
	// exact precedent (schema/semantic/schema.cue: "version of the layer
	// definition; part of every claim's provenance"), bumped when the
	// module's own corpus selection, prompt, or compile identity changes
	// meaningfully. NOT a version of the #Module CUE schema's own shape —
	// there is no separate field for that in v0 (matching #Layer, which
	// likewise has no schema-shape-version field of its own). See
	// docs/spec/expert-module-v0.md §3.1 for the explicit worked-example
	// note this choice depends on.
	version: string & =~"^v[0-9]+$"

	// description is a human-readable summary; no format constraint.
	description: string

	// corpus selects the module's source material, as repo-relative
	// gitignore-style globs evaluated against the compiled service's
	// indexed files — the exact shape #Layer.corpus already uses.
	corpus: {
		include: [...string] & [_, ...]
		exclude?: [...string]
	}

	// watermark is the corpus freshness token AT COMPILE TIME (ADR-017's
	// Watermark SPI, docs/spec/store-format-v0.md §6): a bare 40-hex git
	// HEAD SHA, or a "manifest:"-prefixed 64-hex content hash for a non-git
	// corpus. REQUIRED, no default — docs/spec/expert-module-v0.md §3.3's
	// freshness doctrine: "a module without a watermark is invalid." A
	// served claim from a module with no established watermark has nothing
	// to be honest relative to.
	watermark: string & =~"^([0-9a-f]{40}|manifest:[0-9a-f]{64})$"

	// tier_profile names the tier vocabulary this module's claims use
	// (docs/spec/store-format-v0.md §7 / ADR-017 §5: data now, machinery
	// later — v0 defines no interpretation for a profile other than the
	// default).
	tier_profile: string | *"engineering"

	// eval_evidence is the pack-taxonomy D3 trust seam
	// (docs/design/pack-taxonomy.md, "Platform" stratum) applied to a
	// module: a module shipped outside the fathom binary cannot rely on
	// "trust our own binary's authoring gate" the way an embedded layer
	// can, so an evaled:true claim MUST point at on-disk evidence.
	eval_evidence: {
		evaled: bool | *false
		path?:  string
		// A module that claims evaled:true MUST name where the evidence
		// lives — an empty/absent path with evaled:true is invalid
		// (schema/module/testdata/invalid.cue).
		if evaled {
			path: string & !=""
		}
	}

	// recompile states whether and how a consumer may recompile this
	// module against a refreshed corpus.
	recompile: {
		allowed: bool | *true
		prompt_identity?: string
		// Recompiling under a DIFFERENT prompt than the one that produced
		// the shipped claims silently breaks claim-identity reproducibility
		// (docs/spec/store-format-v0.md §9), so a manifest that permits
		// recompilation MUST name the exact prompt identity a compliant
		// recompile is expected to use.
		if allowed {
			prompt_identity: string & !=""
		}
	}

	// license names the module's license; "unspecified" is an honest
	// default, never silently omitted.
	license: string | *"unspecified"

	// provenance records who/what compiled this module and when.
	provenance: {
		// compiled_by is a model ID, or the literal "deterministic" for an
		// ADR-018 oracle-class (Model-empty) compile pass — mirroring
		// docs/spec/store-format-v0.md §7's "verified" tier admission rule.
		compiled_by: string & !=""
		// compiled_at is an RFC3339 timestamp.
		compiled_at: string & =~"^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}"
	}
}
