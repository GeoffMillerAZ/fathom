// ui-appkit — a PLATFORM-stratum pack (ADR-024): app-kit's own UI doctrine,
// compiled from the kit's own decision docs. Ships inside the kit repo
// (fathom-pack/layers/ui-appkit/) rather than under a consuming repo's own
// .fathom/ — a consumer adopts it via `fathom layers install
// <app-kit>/fathom-pack/layers/ui-appkit`, at which point it resolves as
// that consumer's own project pack (docs/design/pack-taxonomy.md's platform
// stratum: "authored by the org... trusted via... on-disk eval evidence").
//
// Derived from fathom's shipped adr-rules pack (same engine, same prompt
// shape, different corpus + stratum) — same lineage as this org's own
// platform-standards pack (.fathom/layers/platform-standards/layer.cue).
name:    "ui-appkit"
version: "v2"
kind:    "adr-rule"
corpus: {
	include: ["AGENTS.md", "CLAUDE.md", "docs/intent/invariants.md", "docs/intent/principles.md", "docs/design/adoption.md", "docs/design/runtime-selection.md", "docs/design/conversion-pipeline.md", "docs/design/web-desktop-parity.md"]
	exclude: ["data/**", "vendor/**", "docs/assessments/**", "docs/adr/**", "docs/plans/**"]
}
prompt_version: "ui-appkit/content-addressed"
prompt_file:    "fathom-pack/layers/ui-appkit/prompt.md"
tier:           "doc-claimed"
// platform stratum: authored by app-kit (the org's own kit), from the kit's
// own corpus — not fathom's embedded catalog. D3 requires on-disk eval
// evidence before this may be BOUND by a consumer repo (only servable+
// packs are bindable). This v2 candidate has not been evaluated, compiled
// or served. Installation only stages files. Historical served obligations
// may exist with stale citations; staging v2 does not refresh them.
stratum: "platform"
evaled:  false
budgets: {
	max_tokens_per_run:  40000
	max_usd_per_run:     1.0
	max_rule_words:      35
	max_claims_per_file: 8
}
serving: {max_rules: 3, max_tokens: 150}
