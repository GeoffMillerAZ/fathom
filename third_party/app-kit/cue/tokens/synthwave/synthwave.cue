// Synthwave — a shipped alternative token set for app-kit (Geoff's brand).
//
// Neon white + purple on near-black violet. Unlike the calm kit default, this
// is a *dark-only* theme: the `light` and `dark` values are intentionally the
// same so the brand reads identically regardless of the page's color scheme.
// Conforms to appkit/ui/tokens/schema.cue (#Tokens) and adds one open role,
// `neon-pink`, used by the kanban card-title hover.
//
// Regenerate the committed CSS with:
//   go run ./cmd/appkit gen \
//     --tokens cue/tokens/synthwave/synthwave.cue \
//     --css appkit/ui/themes/synthwave/theme.css --dtcg "" --go ""
package synthwave

// mono pairs a value for both schemes (synthwave is dark in either).
mono: {v: string, out: {light: v, dark: v}}

tokens: {
	name:        "synthwave"
	description: "Neon white + purple on near-black — Geoff's brand. Dark-only."

	fonts: {
		sans: "ui-sans-serif, system-ui, -apple-system, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif"
		mono: "ui-monospace, 'SF Mono', Menlo, Consolas, 'Liberation Mono', monospace"
	}

	radius: {
		sm:   "0.25rem"
		md:   "0.375rem"
		lg:   "0.5rem"
		xl:   "0.75rem"
		full: "9999px"
	}

	colors: {
		// canvases — near-black violet rising toward the viewer
		surface: (mono & {v:           "#0e0b1a"}).out
		"surface-raised": (mono & {v:  "#1b1640"}).out
		"surface-sunken": (mono & {v:  "#141029"}).out
		"surface-overlay": (mono & {v: "#221b52"}).out

		// text — neon white fading to muted lavender
		ink: (mono & {v:           "#ece9ff"}).out
		"ink-muted": (mono & {v:   "#9b93c9"}).out
		"ink-faint": (mono & {v:   "#6f679b"}).out
		"ink-inverse": (mono & {v: "#0e0b1a"}).out

		// hairlines
		line: (mono & {v:          "#2c2552"}).out
		"line-strong": (mono & {v: "#3a3270"}).out

		// brand — neon purple
		accent: (mono & {v:         "#7c4dff"}).out
		"accent-hover": (mono & {v: "#8f66ff"}).out
		"accent-ink": (mono & {v:   "#f4f1ff"}).out
		"accent-soft": (mono & {v:  "#2a2150"}).out

		// status
		positive: (mono & {v:        "#36e0a0"}).out
		"positive-soft": (mono & {v: "#103a30"}).out
		caution: (mono & {v:         "#ffd166"}).out
		"caution-soft": (mono & {v:  "#3a3216"}).out
		danger: (mono & {v:          "#ff5d8f"}).out
		"danger-soft": (mono & {v:   "#3a1626"}).out
		info: (mono & {v:            "#6cb8ff"}).out
		"info-soft": (mono & {v:     "#15294a"}).out

		// affordances
		focus: (mono & {v: "#b388ff"}).out

		// open role: neon pink (kanban card-title hover)
		"neon-pink": (mono & {v: "#ff5d8f"}).out
	}
}
