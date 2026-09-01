// Design-token schema (D7). Authored once, validated everywhere: the kit's
// own tokens (cue/tokens/kit.cue) and every app's tokens.cue must conform.
//
// The schema is deliberately in the Go-representable CUE subset — no
// load-bearing disjunctions — so the handwritten Go types in tokens.go
// decode it directly (Spike 0.3 fallback posture; see spikes/0.3 verdict).
//
// Conventions (enforced by review, not schema): colors are OKLCH; never
// pure #000/#fff; neutrals carry a whisper of the brand hue (chroma
// 0.005–0.015); reduce chroma as lightness approaches the extremes.
package tokens

#Color: string & !=""

// #ColorPair: one semantic role, resolved per scheme.
#ColorPair: {
	light: #Color
	dark:  #Color
}

#Tokens: {
	// name identifies the token set (e.g. "appkit", "throughline").
	name: =~"^[a-z][a-z0-9-]*$"

	// description is for humans and the Gallery.
	description?: string

	fonts: {
		sans: string & !=""
		mono: string & !=""
		// display is optional brand/display type; falls back to sans.
		display?: string & !=""
	}

	// radius: CSS lengths, smallest to largest.
	radius: {
		sm:   string & !=""
		md:   string & !=""
		lg:   string & !=""
		xl:   string & !=""
		full: string & !=""
	}

	// colors: the semantic role → {light, dark} map. The kit's component
	// contract references roles by these names (utilities like bg-surface,
	// text-ink, border-line, bg-accent...). The set below is the REQUIRED
	// core every token set must define; additional app-specific roles are
	// allowed and emit utilities the same way.
	colors: {
		// canvases
		surface:           #ColorPair
		"surface-raised":  #ColorPair
		"surface-sunken":  #ColorPair
		"surface-overlay": #ColorPair
		// text
		ink:           #ColorPair
		"ink-muted":   #ColorPair
		"ink-faint":   #ColorPair
		"ink-inverse": #ColorPair
		// hairlines
		line:          #ColorPair
		"line-strong": #ColorPair
		// brand
		accent:         #ColorPair
		"accent-hover": #ColorPair
		"accent-ink":   #ColorPair
		"accent-soft":  #ColorPair
		// status
		positive:        #ColorPair
		"positive-soft": #ColorPair
		caution:         #ColorPair
		"caution-soft":  #ColorPair
		danger:          #ColorPair
		"danger-soft":   #ColorPair
		info:            #ColorPair
		"info-soft":     #ColorPair
		// affordances
		focus: #ColorPair

		// open: apps may add more roles
		[=~"^[a-z][a-z0-9-]*$"]: #ColorPair
	}
}
