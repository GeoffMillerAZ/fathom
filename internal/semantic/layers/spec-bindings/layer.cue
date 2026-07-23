// spec-bindings is the SDD pack (docs/plans/layer-catalog.md LC-6): binding
// requirement lines from gated plans (docs/plans/**), scoped to the package
// each task governs — feeding the same serving channel the edit-hook already
// renders kind-aware. Excludes README.md: it describes the grammar, it
// doesn't carry gated tasks.
//
// LC-4 pack shape: plain data, unified against #Layer (schema/semantic/
// schema.cue) by internal/semantic's loader — see obligations/layer.cue's
// doc comment for the convention.
name:    "spec-bindings"
version: "v1"
kind:    "spec-binding"
corpus: {
	include: ["docs/plans/**"]
	exclude: ["docs/plans/README.md"]
}
// Advisory only — see #Layer.prompt_version's doc comment in schema.cue. The
// real identity is content-addressed at runtime (internal/semantic/prompts.go).
prompt_version: "spec-bindings/content-addressed"
prompt_file:    "schema/semantic/layers/spec-bindings/prompt.md"
tier:           "doc-claimed"
// eval/RESULTS.md: 1.00 precision / 1.00 recall, 2 consecutive runs on the
// production compile model. Result: PASS.
evaled: true
budgets: {
	max_tokens_per_run: 200000
	max_usd_per_run:    1.0
}
