// Package semantic defines the CUE schema for semantic-layer definitions
// (ADR-016 §4.4). A layer definition names its corpus, prompt version, output
// kind, and budgets — the manifest SHAPE exists from day one so that when a
// second layer earns the generic runtime, definitions generalize instead of
// being reinvented.
//
// This file is authoritative AND live (M1, docs/plans/m1-manifest.md):
// internal/semantic embeds a byte-identical copy (internal/semantic/
// manifest_schema.cue, drift-tested against this file) and LoadLayer decodes
// it at runtime.
//
// LC-4 (docs/plans/layer-catalog.md, the pack-shape migration): this file now
// holds ONLY the #Layer definition — the shared shape every pack's instance
// is validated against. Layer INSTANCES live one per module pack, under
// schema/semantic/layers/<name>/layer.cue (plain data: no package clause, no
// "#Layer &" wrapper — a pack author edits values, not CUE), beside that
// pack's prompt.md, eval/, and KILL.md (docs/vision/02-shape.md §3's exact
// shape). internal/semantic's loader compiles #Layer from this file and each
// pack's layer.cue independently, then unifies and decodes — see
// internal/semantic/manifest.go's loadManifest.
package semantic

// #Layer is one semantic-layer definition.
#Layer: {
	// name identifies the layer ("obligations", "failure-mode", …).
	name: string & =~"^[a-z][a-z0-9-]*$"

	// version of the layer definition; part of every claim's provenance.
	version: string & =~"^v[0-9]+$"

	// kind of claim this layer emits (v1: "obligation").
	kind: string & =~"^[a-z][a-z0-9-]*$"

	// corpus selects the governing documents, as repo-relative
	// gitignore-style globs evaluated against the service's indexed files.
	corpus: {
		include: [...string] & [_, ...]
		exclude?: [...string]
	}

	// prompt_version is ADVISORY manifest metadata (M1 amendment, ADR-016).
	// For content-addressed layers (obligations), the runtime's real
	// compile-time prompt identity is derived from the Go
	// prompttemplate.Template text via TextHash — see
	// internal/semantic/prompts.go's obligationsPromptVersion — NOT this
	// field. It is kept so every #Layer instance satisfies the shape and a
	// human skimming the manifest sees a plausible label; LoadLayer decodes
	// it but callers must not treat it as authoritative for the digest gate.
	prompt_version: string

	// prompt_file is the repo-relative path to this layer's compile system
	// prompt (LC-1, docs/plans/layer-catalog.md): the text a build target
	// compiles under. Content-addressed identity is derived FROM this file's
	// bytes at runtime — internal/semantic.promptForLayer loads it and
	// computes PromptVersion = "<name>/" + sha256(bytes)[:8] — so a prompt
	// edit is automatically a new compile identity (the v1→v2 hand-bumped-
	// const bug class this whole mechanism exists to close). Every #Layer
	// instance MUST set this: an empty or unresolvable prompt_file is a loud
	// runtime error, never a silent fallback to another layer's prompt.
	// LC-4: for a pack-shape layer this is that pack's own prompt.md, i.e.
	// "schema/semantic/layers/<name>/prompt.md".
	prompt_file: string

	// tier every emitted claim carries. V1 layers are citation-grounded
	// restatements, hence doc-claimed; an LLM-inferred kind would need the
	// ai-derived tier and its revival evidence first (ADR-016 §4.2).
	// verified admits deterministic, Model-empty compile paths (ADR-018
	// oracle classes — index/cmd/lexical; ADR-016 M1 amendment). No layer
	// emits verified yet.
	tier: "doc-claimed" | "sme-stated" | "verified"

	// tier_profile names the tier vocabulary (ADR-017: data now).
	tier_profile: string | *"engineering"

	// stratum names which of the four pack-taxonomy strata this pack
	// instance belongs to (ADR-024, docs/design/pack-taxonomy.md) — a
	// vocabulary field, not a trust mechanism: it does not change
	// resolution order, enable/disable semantics, or the eval gate.
	// "concern" is a fathom-authored, eval-gated-embedded pack (the shipped
	// catalog); "platform" is an org's OWN pack authored and eval-gated
	// on-disk under a project's .fathom/layers/ (the D3 trust seam already
	// distinguishes these two cases mechanically — see
	// internal/semantic/packs.go's loadUnion). Defaults "concern": an
	// un-annotated pack (embedded or a project pack that doesn't set this)
	// is presumed fathom-shaped until declared otherwise. "dialect" and
	// "provider" are recorded strata (docs/design/pack-taxonomy.md) with no
	// #Layer instance today — dialects are code, not packs; provider packs
	// are a parked pack type (TX-3, docs/plans/pack-taxonomy-adoption.md) —
	// so this enum admits only the two strata a #Layer instance can
	// actually be.
	stratum: "concern" | "platform" | *"concern"

	// evaled declares whether this pack has a PASSING eval/RESULTS.md on
	// record (docs/layers.md "The gate": precision >= 0.90, recall >= 0.75,
	// scored by internal/semantic/eval_test.go's runLayerEval harness against
	// a real model) — self-declared DATA, not a live filesystem check,
	// because eval/ is deliberately excluded from the embedded pack mirror
	// (internal/semantic/manifest.go: "eval/ and KILL.md are dev-time/gate
	// artifacts, never needed by the compiled binary"), so the compiled
	// binary has no other way to know an embedded pack's gate status at
	// runtime (D3-1, docs/plans/expert-depth.md). A pack author flips this to
	// true only when eval/RESULTS.md records a passing run; `fathom compile`
	// refuses a non-dry-run compile of an evaled:false layer unless
	// --unsafe-skip-eval-gate overrides it (logged into compile_runs).
	// Defaults false: an un-evaled pack (or one whose author forgot to flip
	// this) fails closed, never silently passes the gate.
	evaled: bool | *false

	// budgets bound a compile run; the runner must refuse to exceed them.
	budgets: {
		max_tokens_per_run:  int & >0
		max_usd_per_run:     number & >=0
		max_rule_words:      int & >0 | *28
		max_claims_per_file: int & >0 | *40
	}

	// serving caps the decision-moment payload (zero-output-on-clean is
	// behavioral, not schematic).
	serving: {
		max_rules:  int & >0 & <=6 | *6
		max_tokens: int & >0 | *150

		// drift_tracked (LC-2, docs/plans/layer-catalog.md) opts a layer into
		// remediation-drift anchoring: verifyDraft persists each claim's
		// scope-matched files' content fingerprints as scope-fp ClaimLinks at
		// compile time, and served payloads carry code_drifted when a
		// scope-fp's live fingerprint disagrees (DriftedFindings, sweep.go).
		// Defaults false — most layers restate slow-churn governing docs, not
		// findings whose remediation code can drift out from under them.
		// Replaces a Go kind=="finding" hardcode with manifest data: a future
		// layer opts in by setting this field, no compiler/server Go edit.
		drift_tracked: bool | *false
	}
}
