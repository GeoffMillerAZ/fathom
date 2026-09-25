// Geoff Synthwave: the owner's requested older white-neon, pink/purple reference.
// Source: geoff/src/styles/global.css at commit
// 375defbac2c9e5c3fa01cada634464b95936f877 (not the newer indigo working tree).
// Source SHA-256: cb2d45d42bd73a3862ee01e7a9efec1be52272cceb7f2c315c21c22c0ebb51b3
// Canonical variables: lines 7-16; body font:26; focus:77; white glow:395-400.
// White #ffffff and exact source hex values are explicit owner-requested
// exceptions to the usual tinted-neutral/OKLCH convention. This token adaptation
// copies no website component, asset, dependency, global heading rule or animation.
// Static glow belongs to a separately opted-in composition, not global token CSS.
package geoffsynthwave

mono: {v: string, out: {light: v, dark: v}}

tokens: {
	name:        "geoff-synthwave"
	description: "Geoff's older white-neon, pink/purple reference, preserved as a dark-only semantic palette. Glow is a separate opt-in effect."

	fonts: {
		// Actual committed body declaration, not the unused YAML Orbitron stack.
		// Inter is declared only; no automatic font loading or bundled font bytes.
		sans: "Inter, system-ui, sans-serif"
		mono: "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace"
	}
	// Kit-assigned Tailwind-compatible control/panel scale, not a measured
	// original DaisyUI radius contract. The source did not define a radius scale.
	radius: {sm: "0.125rem", md: "0.375rem", lg: "0.5rem", xl: "0.75rem", full: "9999px"}

	colors: {
		// Source --synth-bg. Its same-RGB translucent cards over that canvas do
		// not introduce a new opaque color; retain it for panels and overlays.
		surface: (mono & {v: "#241b2f"}).out
		"surface-raised": (mono & {v: "#241b2f"}).out
		"surface-overlay": (mono & {v: "#241b2f"}).out
		// Source pre background black/30 over the canvas, rounded to opaque sRGB.
		"surface-sunken": (mono & {v: "#191321"}).out

		// Source --synth-glow-text and --synth-text. No low-contrast dimming for
		// semantic faint text, and no text-shadow in this portable palette.
		ink: (mono & {v: "#ffffff"}).out
		"ink-muted": (mono & {v: "#c7c7c7"}).out
		"ink-faint": (mono & {v: "#c7c7c7"}).out
		"ink-inverse": (mono & {v: "#241b2f"}).out

		// Source mint/20 border composited over canvas. Decorative only.
		line: (mono & {v: "#34464a"}).out
		// Kit adaptation: mint/50 on canvas for a measured strong boundary.
		"line-strong": (mono & {v: "#4b8674"}).out

		// Source --synth-pink and --synth-purple. Dark ink follows the source
		// selection treatment and replaces white on filled bright-pink actions.
		accent: (mono & {v: "#f92aad"}).out
		"accent-hover": (mono & {v: "#ff7edb"}).out
		"accent-ink": (mono & {v: "#241b2f"}).out
		// Soft-role backgrounds are kit adaptations: each source role at 10%
		// over surface-sunken, rounded to opaque sRGB. The darker base preserves
		// pink text's tight contrast margin; these are not original portal tokens.
		"accent-soft": (mono & {v: "#2f152f"}).out
		focus: (mono & {v: "#72f1b8"}).out

		// Kit-assigned semantic roles using the source palette, not source-verified
		// domain meanings. Status still needs a label; pink is also the brand accent.
		positive: (mono & {v: "#72f1b8"}).out
		"positive-soft": (mono & {v: "#222930"}).out
		caution: (mono & {v: "#fede5d"}).out
		"caution-soft": (mono & {v: "#302727"}).out
		danger: (mono & {v: "#f92aad"}).out
		"danger-soft": (mono & {v: "#2f152f"}).out
		info: (mono & {v: "#36f9f6"}).out
		"info-soft": (mono & {v: "#1c2a36"}).out
	}
}
