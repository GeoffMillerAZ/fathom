// agentic-kit-standards — a PLATFORM-stratum pack (ADR-024): agentic-kit's
// own hard-rule doctrine, compiled from the kit's own corpus. Ships inside
// the kit repo (fathom-pack/layers/agentic-kit-standards/) rather than
// under a consuming repo's own .fathom/ — a consumer adopts it via
// `fathom layers install <agentic-kit>/fathom-pack/layers/agentic-kit-standards`,
// at which point it resolves as that consumer's own project pack
// (docs/design/pack-taxonomy.md's platform stratum: "authored by the
// org... trusted via... on-disk eval evidence").
//
// Derived from fathom's shipped adr-rules pack (same engine, same prompt
// shape, different corpus + stratum) — same lineage as this org's own
// platform-standards pack (.fathom/layers/platform-standards/layer.cue).
name:    "agentic-kit-standards"
version: "v1"
kind:    "adr-rule"
corpus: include: ["CLAUDE.md", "VISION.md", "docs/*.md", "specs/**"]
prompt_version: "agentic-kit-standards/content-addressed"
prompt_file:    "fathom-pack/layers/agentic-kit-standards/prompt.md"
tier:           "doc-claimed"
// platform stratum: authored by agentic-kit (the org's own kit), from the
// kit's own corpus — not fathom's embedded catalog. D3 requires on-disk
// eval evidence before this may be BOUND by a consumer repo (only
// servable+ packs are bindable); per KILL.md this pack is "not yet
// compiled anywhere", so evaled is honestly false. It compiles and serves
// locally once installed; it is not yet bindable.
stratum: "platform"
evaled:  false
budgets: {
	max_tokens_per_run: 200000
	max_usd_per_run:    1.0
}
