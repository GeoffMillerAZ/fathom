// guard-pack.cue — agentic-kit's hard rules as pre-edit guards.
// from agentic-kit fathom-pack v0.1 — copy the group that matches your
// repo (kit-self vs consumer) into .fathom/guard.cue; keep provenance
// comments for /fathom:activate re-sync.

guards: [
	// ── KIT-SELF group: working inside agentic-kit ──

	{
		// from agentic-kit fathom-pack v0.1 (kit-self)
		scopes: ["go.mod"]
		note: "stdlib only — ZERO external Go dependencies, ever. go.mod has no require block; keep it that way. A dependency you're about to add is either unnecessary, or it belongs behind a port with a stdlib default, or this change belongs in a different repo."
		max_rules: 2
	},
	{
		// from agentic-kit fathom-pack v0.1 (kit-self)
		scopes: ["foundations/**"]
		note: "Foundations import ONLY stdlib + sibling foundations — imports point downward, nothing reaches back up from kits/runtimes/surfaces. Every foundation sub-package keeps the shape: schema.cue (CUE owns the data's shape) + types.go mirroring it + mock.go where an interface exists + testdata/*.cue positive fixtures; drift between CUE and Go is a bug. State the concurrency contract in the package doc comment. foundations/governance/* stays WASM-safe: no file I/O, no goroutine leaks."
		max_rules: 4
	},
	{
		// from agentic-kit fathom-pack v0.1 (kit-self)
		scopes: ["foundations/**/*.go", "kits/**/*.go", "runtimes/**/*.go"]
		note: "Determinism: never a bare time.Now() without a seam — inject clocks (now func() time.Time); a nil-clock default to time.Now is legal only when documented on the seam. Seedable randomness, table-driven tests, no sleeps, no network in tests (httptest only), everything passes -race."
		max_rules: 3
	},
	{
		// from agentic-kit fathom-pack v0.1 (kit-self)
		scopes: ["TRACKING.md"]
		note: "Single source of truth for build status: every capability gets a checkbox, and [x] means tests pass AND the full gate is green — updated in the SAME change that lands the work. Never check a box on a claim the gate hasn't proven."
		max_rules: 2
	},
	{
		// from agentic-kit fathom-pack v0.1 (kit-self)
		scopes: ["**/_reference/**"]
		note: "_reference/ dirs are reconstructed scanned sources: Go ignores them, you NEVER import them — port from them, matching the sibling packages' shape exactly. If you're adding an import path containing _reference, stop."
		max_rules: 2
	},
	{
		// from agentic-kit fathom-pack v0.1 (kit-self)
		command_scopes: ["git commit --no-verify*", "git push --no-verify*"]
		note: "Never --no-verify. Opt out only with LEFTHOOK=0, and only when the hook itself is broken — and if it is broken, that's an observation to file, not a workaround to normalize. Exported-API changes: run task downstream-check against the real consumers (fathom, throughline) before tagging."
	},

	// ── CONSUMER group: repos that import the kit ──

	{
		// from agentic-kit fathom-pack v0.1 (consumer)
		scopes: ["**/go.mod"]
		note: "If you're adding a utility that agentic-kit plausibly should own (retry, telemetry heartbeats, evidence types, governance shapes), check the kit's catalog first and extend the KIT, not this app — two apps needing it means it lives in the kit. Forking kit code into an app is the drift the kit exists to prevent."
		max_rules: 3
	},
	{
		// from agentic-kit fathom-pack v0.1 (consumer)
		scopes: ["**/*_test.go"]
		note: "The mock IS an adapter, never a mode flag: where the kit exposes an interface, use/extend its mock.go double and run the conformance suite against both real and fake. Tests inject the kit's clock seams (fixedClock/advancingClock patterns) — a bare time.Now() in decision logic is the named bug class."
		max_rules: 3
	},
	{
		// from agentic-kit fathom-pack v0.1 (consumer)
		scopes: ["**/*.cue"]
		note: "Org-agnostic means org identity is DATA validated against the kit's schemas, never code: no company/product names, no hard-coded catalogs (sensitivity rules, taxonomies, gateways) in Go. If you're writing an org-specific value into a .go file, it belongs in CUE config the schema validates."
		max_rules: 3
	},
	{
		// from agentic-kit fathom-pack v0.1 (consumer)
		scopes: ["**/telemetry/**", "**/metrics/**"]
		note: "Instruments get existence checks (the heartbeat lesson: a hook channel once died silently for 4 days while its metric read zero). Every measurement channel ships an is-anything-flowing assertion via the kit's heartbeat foundations — a metric whose source can silently vanish is theater, and an absent instrument prints a named reason, never a fabricated zero."
		max_rules: 3
	},
]
