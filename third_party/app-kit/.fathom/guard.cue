// Repo-scoped guards from app-kit fathom-pack v0.2 (kit-self).
// Guards guide; tests/gates enforce. No implicit module bindings or LLM compilation.
guards: [
	// from app-kit fathom-pack v0.2 (kit-self)
	{
		"scopes": [
			"appkit/ui/**",
		]
		"note":      "I4/I5/I7 and ADR-0017: shared rendering uses semantic roles and escapes untrusted labels. Generated token definitions own literal values; do not hand-edit them. L3b/L4 declare scanner requirements; L3a stays scanner-free. Preserve public contracts, app identity and layer-specific promotion evidence."
		"max_rules": 2
	},
	// from app-kit fathom-pack v0.2 (kit-self)
	{
		"scopes": [
			"cue/tokens/**",
			"appkit/ui/tokens/**",
			"apps/gallery/ui/input.css",
			"cmd/appkit/main.go",
			"cmd/appkit-mcp/setup.go",
		]
		"note":      "I2/I3 and INC-5/INC-9: regenerate from CUE with pinned tools. CSS-only custom generation cannot write the kit terminal palette. Verify actual selected utility sources and fail on missing paths; do not bypass binary integrity checks or assume a served recipe works."
		"max_rules": 2
	},
	// from app-kit fathom-pack v0.2 (kit-self)
	{
		"scopes": [
			"internal/designassets/**",
			"internal/designconvert/**",
			"internal/designattempts/**",
			"internal/matdash/**",
			"apps/gallery/pricing*",
			"apps/gallery/conversions*",
			"cmd/appkit-mcp/theming.go",
		]
		"note":      "ADR-0022/ADR-0023 and I17: preserve private originals, design states and exact source identity. Refuse unsupported conversion explicitly. Do not weaken the oracle, silently change runtime or promote compiler/DOM checks into browser/behavior certification. MatDash compatibility follows its source manifest, not a desired target."
		"max_rules": 2
	},
	// from app-kit fathom-pack v0.2 (kit-self)
	{
		"scopes": [
			"appkit/ui/analytics/**",
			"appkit/ui/vizcheck/**",
			"apps/gallery/analytics*",
		]
		"note":      "I11 and INC-8: independently expected values must agree across chart, table, detail and export. Preserve missing/zero/partial/stale distinctions, units and source. Static output checks do not prove calculation lineage, geometry, accessibility or task performance."
		"max_rules": 1
	},
	// from app-kit fathom-pack v0.2 (kit-self)
	{
		"scopes": [
			"spikes/wails-gallery/**",
			"apps/gallery/shell*",
		]
		"note":      "ADR-0026/ADR-0033 and I9/I19: browser core tasks remain independent of Wails. Preserve auth, CSRF, bounded streams and draft recovery. Native navigation/bridge containment needs actual macOS evidence; browser/HTTP success is not native proof. Windows, remote hosting and signing are separately scoped."
		"max_rules": 2
	},
	// from app-kit fathom-pack v0.2 (kit-self)
	{
		"scopes": [
			"AGENTS.md",
			"CLAUDE.md",
			"fathom-pack/**",
			"cmd/appkit-mcp/kitdocs*",
			"docs/intent/**",
			"docs/adr/**",
			"docs/plans/**",
			"docs/design/adoption*",
		]
		"note":      "INC-6/INC-11 and I8/I15/I18: current intent governs historical slogans. Keep agent/MCP/pack guidance aligned with ADR-0022/ADR-0023. Distinguish implemented, selected and proven; preserve exact browser/consumer evidence limits. Never turn a plan checkbox or static contract into goal completion."
		"max_rules": 2
	},
]
