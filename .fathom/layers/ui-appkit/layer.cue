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
version: "v1"
kind:    "adr-rule"
corpus: include: ["README.md", "AGENTS.md", "docs/**"]
prompt_version: "ui-appkit/content-addressed"
prompt_file:    "fathom-pack/layers/ui-appkit/prompt.md"
tier:           "doc-claimed"
// platform stratum: authored by app-kit (the org's own kit), from the kit's
// own corpus — not fathom's embedded catalog. D3 requires on-disk eval
// evidence before this may be BOUND by a consumer repo (only servable+
// packs are bindable); per KILL.md this pack is "not yet compiled
// anywhere", so evaled is honestly false. It compiles and serves locally
// once installed; it is not yet bindable.
stratum: "platform"
evaled:  false
budgets: {
	max_tokens_per_run: 200000
	max_usd_per_run:    1.0
}
