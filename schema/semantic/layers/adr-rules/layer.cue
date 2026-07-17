// adr-rules is the layer-2 proof (docs/plans/layer-catalog.md LC-5,
// docs/vision/02-shape.md §3, docs/vision/08-roadmap.md): architectural
// constraints extracted from this repo's own decision records and design
// docs, scoped to the code they govern — the same compile engine as
// obligations, a different corpus and prompt, ZERO new Go.
//
// LC-4 pack shape: plain data, unified against #Layer (schema/semantic/
// schema.cue) by internal/semantic's loader — see obligations/layer.cue's
// doc comment for the convention.
name:    "adr-rules"
version: "v1"
kind:    "adr-rule"
corpus: include: ["ADR.md", "docs/design/*.md"]
// Advisory only — see #Layer.prompt_version's doc comment in schema.cue. The
// real identity is content-addressed at runtime (internal/semantic/prompts.go).
prompt_version: "adr-rules/content-addressed"
prompt_file:    "schema/semantic/layers/adr-rules/prompt.md"
tier:           "doc-claimed"
budgets: {
	max_tokens_per_run: 200000
	max_usd_per_run:    1.0
}
