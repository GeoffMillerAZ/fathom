// The kit's default token set — the neutral, calm baseline every kit app
// starts from and the set the Gallery renders. Apps override by authoring
// their own tokens.cue against the same schema (appkit/ui/tokens/schema.cue)
// and running `appkit gen` with their paths.
//
// Palette notes: OKLCH throughout; neutrals tinted toward the accent hue
// (260°, chroma 0.004–0.02 — never gray-gray, never #fff/#000); accent is a
// restrained violet-blue; status hues keep chroma moderate so they read as
// information, not alarm. Dark scheme lifts lightness of accents and drops
// chroma at the extremes.
package tokens

tokens: {
	name:        "appkit"
	description: "Kit default theme: calm, information-dense, trustworthy."

	fonts: {
		sans: "system-ui, -apple-system, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif"
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
		surface: {light: "oklch(0.985 0.005 260)", dark: "oklch(0.16 0.012 260)"}
		"surface-raised": {light: "oklch(0.995 0.003 260)", dark: "oklch(0.20 0.014 260)"}
		"surface-sunken": {light: "oklch(0.965 0.006 260)", dark: "oklch(0.13 0.010 260)"}
		"surface-overlay": {light: "oklch(0.995 0.003 260)", dark: "oklch(0.22 0.015 260)"}

		ink: {light: "oklch(0.21 0.015 260)", dark: "oklch(0.93 0.008 260)"}
		"ink-muted": {light: "oklch(0.45 0.020 260)", dark: "oklch(0.70 0.015 260)"}
		"ink-faint": {light: "oklch(0.60 0.015 260)", dark: "oklch(0.55 0.015 260)"}
		"ink-inverse": {light: "oklch(0.98 0.005 260)", dark: "oklch(0.18 0.012 260)"}

		line: {light: "oklch(0.90 0.008 260)", dark: "oklch(0.28 0.015 260)"}
		"line-strong": {light: "oklch(0.80 0.012 260)", dark: "oklch(0.38 0.018 260)"}

		accent: {light: "oklch(0.50 0.19 262)", dark: "oklch(0.68 0.16 262)"}
		"accent-hover": {light: "oklch(0.44 0.19 262)", dark: "oklch(0.73 0.15 262)"}
		"accent-ink": {light: "oklch(0.98 0.010 262)", dark: "oklch(0.15 0.020 262)"}
		"accent-soft": {light: "oklch(0.94 0.040 262)", dark: "oklch(0.28 0.060 262)"}

		positive: {light: "oklch(0.51 0.14 155)", dark: "oklch(0.70 0.13 155)"}
		"positive-soft": {light: "oklch(0.94 0.05 155)", dark: "oklch(0.30 0.05 155)"}
		caution: {light: "oklch(0.70 0.14 80)", dark: "oklch(0.78 0.13 85)"}
		"caution-soft": {light: "oklch(0.95 0.06 90)", dark: "oklch(0.32 0.06 90)"}
		danger: {light: "oklch(0.55 0.19 25)", dark: "oklch(0.68 0.17 25)"}
		"danger-soft": {light: "oklch(0.94 0.05 25)", dark: "oklch(0.30 0.06 25)"}
		info: {light: "oklch(0.58 0.12 230)", dark: "oklch(0.72 0.11 230)"}
		"info-soft": {light: "oklch(0.94 0.04 230)", dark: "oklch(0.30 0.05 230)"}

		focus: {light: "oklch(0.55 0.19 262)", dark: "oklch(0.72 0.15 262)"}
	}
}
