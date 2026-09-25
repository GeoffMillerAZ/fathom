// Reviewed guidance, not an enforcement engine. ADR-0022/0023 govern fidelity and profiles.
// Consumer group: first four entries. Kit-self group: final six, mirrored locally by tests.
guards: [
	// from app-kit fathom-pack v0.2 (consumer)
	{
		"scopes": [
			"**/ui/**/*.go",
			"**/*.templ",
			"**/views/**/*.go",
			"**/components/**/*.go",
		]
		"note":      "ADR-0022/ADR-0023: inspect first-party capabilities and a complete licensed design reference before composing UI. Preserve original hierarchy and interaction states; review semantic roles contextually, never infer fidelity from palette replacement. Choose the app's declared renderer/runtime profile; originals remain valid alternatives. Consumer identity, domain policy and authorization stay app-owned."
		"max_rules": 2
	},
	// from app-kit fathom-pack v0.2 (consumer)
	{
		"scopes": [
			"**/input.css",
			"**/tokens.cue",
		]
		"note":      "I2/I3/I5 and INC-5/INC-9: CUE authors tokens; use the checksum-pinned CSS recipe. L3b/L4 utility markup needs explicit source scanning; L3a SVG does not. Custom CSS token generation must not overwrite the kit terminal palette. Fonts and asset delivery remain app-owned; generated CSS alone does not prove rendered quality."
		"max_rules": 2
	},
	// from app-kit fathom-pack v0.2 (consumer)
	{
		"scopes": [
			"**/dashboard/**",
			"**/analytics/**",
		]
		"note":      "I11 and INC-8: metrics need units, period, source and missingness. Missing is not zero. Verify chart/table/export agreement against an independent fixture; source presence and HTTP success do not prove geometry, accessibility or useful analysis. Choose a measured renderer under ADR-0023."
		"max_rules": 2
	},
	// from app-kit fathom-pack v0.2 (consumer)
	{
		"command_scopes": [
			"git push*",
		]
		"note":      "I12/I14/I15: UI completion needs the relevant browser task, keyboard/narrow/theme/recovery evidence and reviewed consumer propagation. Static Fathom reachability is not runtime or visual proof. This reminder does not authorize a push or publication."
		"max_rules": 1
	},
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
