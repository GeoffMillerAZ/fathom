// Geoff Indigo: the current site's Engineering Console, not legacy synthwave84.
// Source: geoff/apps/web/src/app/{layout.jsx,globals.css}, tailwind.config.js,
// components/react/dashboard/{ResumeDashboard,ContextRelatedContent}.jsx and
// geoff/packages/ui/src/components/application-ui/{shells/SidebarLayout,
// filters/AdvancedFilterBar}.jsx. These are uncommitted working-tree sources;
// geoff HEAD 375defbac2c9e5c3fa01cada634464b95936f877 is not their revision.
// ResumeDashboard SHA-256: 21ab1c35df43e5d2dfae7458cbaa04b580d85becf9bce47e1814bce9df8a4800
// SidebarLayout SHA-256: 436a2cc7368b454fdf347f70a46b9fff27c0841b36556c11d2d0b7c400b2d66a
// Tailwind config SHA-256: 19044ad6bca723d8a9fdfb3d6eb2a18ab4dfdc785f06d8a3a5222f8db86feabb
// Hex values preserve the source's Tailwind 3.4.17 colors, a deliberate exception
// to the kit's usual OKLCH authoring convention. This is a semantic adaptation,
// not a copied component, font bundle, portal baseline or visual certification.
package geoffindigo

// The source is dark-only. Scheme switches do not invent an unreviewed light UI.
mono: {v: string, out: {light: v, dark: v}}

tokens: {
	name:        "geoff-indigo"
	description: "Geoff's Engineering Console: dark gray and indigo, with explicit semantic contrast adaptations. Dark-only."

	fonts: {
		// Declared by the site's Tailwind config; actual Inter loading is unproven.
		// No font files, downloads or remote URLs are provided by this token set.
		sans: "Inter, system-ui, sans-serif"
		mono: "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace"
	}

	// Source Tailwind radii: controls md, panels lg, context switcher xl.
	radius: {sm: "0.125rem", md: "0.375rem", lg: "0.5rem", xl: "0.75rem", full: "9999px"}

	colors: {
		// gray-900 canvas and recessed regions; gray-800 panels and menu flyouts.
		surface: (mono & {v: "#111827"}).out
		"surface-raised": (mono & {v: "#1f2937"}).out
		"surface-sunken": (mono & {v: "#111827"}).out
		"surface-overlay": (mono & {v: "#1f2937"}).out

		// Adapt white to gray-50. Keep secondary/faint semantic text at gray-400;
		// source gray-500/600 labels are not carried into small semantic text.
		ink: (mono & {v: "#f9fafb"}).out
		"ink-muted": (mono & {v: "#9ca3af"}).out
		"ink-faint": (mono & {v: "#9ca3af"}).out
		"ink-inverse": (mono & {v: "#111827"}).out

		// Solid gray-700 substitutes for context-dependent white/10 hairlines.
		// It is decorative, not a sufficient sole control boundary. gray-500 is
		// the semantic strong boundary, measured on both admitted surface values.
		line: (mono & {v: "#374151"}).out
		"line-strong": (mono & {v: "#6b7280"}).out

		// Source indigo-400 links, indigo-300 hover text and indigo-500 focus.
		// Dark accent ink replaces source white to make filled actions readable.
		accent: (mono & {v: "#818cf8"}).out
		"accent-hover": (mono & {v: "#a5b4fc"}).out
		"accent-ink": (mono & {v: "#111827"}).out
		// Rounded opaque equivalent of indigo-500/10 over the gray-900 canvas;
		// explicitly fixed to that canvas, not a claim about every source overlay.
		"accent-soft": (mono & {v: "#19203b"}).out
		focus: (mono & {v: "#6366f1"}).out

		// Kit-assigned semantics from Tailwind dark palettes, not source-verified
		// meanings or a complete original-site status vocabulary. Source green
		// availability informed positive; caution/danger/info and soft pairs fill
		// the kit contract. Never infer a domain status from these palette names.
		positive: (mono & {v: "#4ade80"}).out
		"positive-soft": (mono & {v: "#052e16"}).out
		caution: (mono & {v: "#fcd34d"}).out
		"caution-soft": (mono & {v: "#451a03"}).out
		danger: (mono & {v: "#f87171"}).out
		"danger-soft": (mono & {v: "#450a0a"}).out
		info: (mono & {v: "#38bdf8"}).out
		"info-soft": (mono & {v: "#082f49"}).out
	}
}
