// guard-pack.cue — app-kit's UI doctrine as pre-edit guards.
// from app-kit fathom-pack v0.1 — consumers copy the CONSUMER group into
// .fathom/guard.cue and narrow scopes to their real paths; keep the
// provenance comments so /fathom:activate can re-sync.
//
// Every guard exists because the mistake already happened in a real repo:
// hand-written components that duplicated the licensed corpus, raw palette
// colors that broke theming, missing @source lines that shipped unstyled
// markup, and a package.json that violated the no-Node covenant.

guards: [
	// ── CONSUMER group: for any repo that renders UI with app-kit ──

	{
		// from app-kit fathom-pack v0.1 (consumer)
		scopes: ["**/ui/**/*.go", "**/*.templ", "**/views/**/*.go", "**/components/**/*.go"]
		note: "UI markup is never hand-written here (app-kit D10). BEFORE writing any component: search the licensed corpus via the appkit MCP (search_components, framework html, tailwind 4) — it is already bought and indexed, and hand-writing a component it contains is the most expensive mistake available. Adapt via get_component + get_kit_theming: palette families become ROLES (indigo→accent, red→danger, emerald→positive), neutrals resolve by CSS property, dark: color variants are DROPPED (the role carries its dark value), focus rings become ring-focus. Never an inline color, never a runtime JS dep, never a CDN. If this component would serve a second app, it belongs UP in appkit/ui/ (D13) — build it there and consume it, don't fork it here."
		max_rules: 4
	},
	{
		// from app-kit fathom-pack v0.1 (consumer)
		scopes: ["**/input.css", "**/tokens.cue", "**/*.css"]
		note: "Tailwind emits only utilities it can SEE: any kit package or file whose classes you render needs an @source line in input.css, or the component renders unstyled — the single most common failure in adopting repos. CSS comes from the checksum-pinned Tailwind binary (appkit css / get_kit_css_setup), never from Node. Theming is data: brand lives in tokens.cue; light/dark resolves server-side with no JavaScript."
		max_rules: 3
	},
	{
		// from app-kit fathom-pack v0.1 (consumer)
		scopes: ["**/package.json", "**/package-lock.json", "**/node_modules/**"]
		note: "No package.json in a kit repo (app-kit ADR-0017 §3, refining D2): 'kit repo' means the app-kit monorepo and any repo whose primary artifact is a Go binary or http.Handler tree — there it is a build failure by policy. The purchased templates are PATTERN SOURCES, not runtime deps: pull the HTML variant, strip hardcoded colors to kit roles, express as a pure-Go render fn. A repo that is NOT a Go app (React/Vite, static site) may keep its package.json and consume the kit's generated artifacts (theme.css, motion.css, L2 scripts) — that is compliant consumption, not a violation."
		max_rules: 2
	},
	{
		// from app-kit fathom-pack v0.1 (consumer)
		command_scopes: ["npm *", "npx *", "yarn *", "pnpm *", "bun *"]
		note: "No Node toolchain in a kit repo (ADR-0017 §3: the app-kit monorepo, and any repo whose primary artifact is a Go binary or http.Handler tree). CSS builds via the pinned Tailwind binary (appkit css); components come from the corpus as pure Go; kit-authored browser JS only at the L1 escape-hatch rung, third-party JS only vendored + SHA-256-recorded. If this repo is actually a Node-built app (check for an existing package.json/Vite config), its own toolchain is fine and this guard does not apply — it consumes the kit as generated artifacts."
	},
	{
		// from app-kit fathom-pack v0.1 (consumer)
		scopes: ["**/dashboard/**", "**/dash/**"]
		note: "Dashboards = MatDash shells (sidebar/topbar/widget grid, via search_components source=matdash) + kit ui/viz SVG charts, re-themed to kit tokens. TailwindPlus and MatDash ship NO charting — the charts are the kit's SVG (ui/viz). Don't import a JS charting library; don't hand-draw what ui/viz already renders."
		max_rules: 3
	},
	{
		// from app-kit fathom-pack v0.1 (consumer)
		command_scopes: ["git push*"]
		note: "A 200 is not a rendering. Before reporting UI work done: run the repo's screenshot sweep if it has one, and LOOK at the surface in a browser — status codes and passing tests cannot see ugly, raw markdown in snippets, or a missing stylesheet. This rule exists because an entire UI was once reviewed by HTTP 200s."
	},

	// ── KIT-SELF group: for work inside app-kit itself ──

	{
		// from app-kit fathom-pack v0.1 (kit-self)
		scopes: ["appkit/ui/**"]
		note: "This is the kit: every component here serves EVERY consuming app. Token-themed roles only (no literal colors anywhere); each component documents its @source contract; breaking a render fn's signature breaks the fleet — check consumers (portal, throughline) before changing exported shapes. New components arriving from an app (D13 uplift) get generalized here, not pasted."
		max_rules: 4
	},
]
