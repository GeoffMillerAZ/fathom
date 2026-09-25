// Orchard is a leaf-and-bark identity for long planning sessions near daylight.
// Olive-green carries the active state while status colors remain semantic roles.
package orchard

tokens: {
	name:        "orchard"
	description: "Olive orchard green with restrained leaf actions, in distinct light and dark schemes."

	fonts: {
		sans: "system-ui, -apple-system, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif"
		mono: "ui-monospace, 'SF Mono', Menlo, Consolas, 'Liberation Mono', monospace"
	}
	radius: {sm: "0.25rem", md: "0.375rem", lg: "0.5rem", xl: "0.75rem", full: "9999px"}

	colors: {
		surface:          {light: "oklch(0.985 0.007 120)", dark: "oklch(0.17 0.014 120)"}
		"surface-raised": {light: "oklch(0.995 0.004 120)", dark: "oklch(0.21 0.016 120)"}
		"surface-sunken": {light: "oklch(0.960 0.010 120)", dark: "oklch(0.13 0.012 120)"}
		"surface-overlay": {light: "oklch(0.995 0.004 120)", dark: "oklch(0.23 0.017 120)"}
		ink:              {light: "oklch(0.23 0.020 120)", dark: "oklch(0.93 0.008 120)"}
		"ink-muted":      {light: "oklch(0.44 0.025 120)", dark: "oklch(0.72 0.014 120)"}
		"ink-faint":      {light: "oklch(0.53 0.018 120)", dark: "oklch(0.64 0.014 120)"}
		"ink-inverse":    {light: "oklch(0.985 0.007 120)", dark: "oklch(0.17 0.014 120)"}
		line:             {light: "oklch(0.89 0.013 120)", dark: "oklch(0.29 0.020 120)"}
		"line-strong":    {light: "oklch(0.60 0.020 120)", dark: "oklch(0.56 0.028 120)"}
		accent:           {light: "oklch(0.45 0.120 135)", dark: "oklch(0.73 0.100 135)"}
		"accent-hover":   {light: "oklch(0.39 0.120 135)", dark: "oklch(0.80 0.090 135)"}
		"accent-ink":     {light: "oklch(0.985 0.007 120)", dark: "oklch(0.17 0.014 120)"}
		"accent-soft":    {light: "oklch(0.94 0.040 135)", dark: "oklch(0.29 0.040 135)"}
		positive:         {light: "oklch(0.45 0.120 150)", dark: "oklch(0.72 0.110 150)"}
		"positive-soft":  {light: "oklch(0.93 0.045 150)", dark: "oklch(0.28 0.040 150)"}
		caution:          {light: "oklch(0.49 0.120 85)", dark: "oklch(0.80 0.110 85)"}
		"caution-soft":   {light: "oklch(0.94 0.050 85)", dark: "oklch(0.31 0.040 85)"}
		danger:           {light: "oklch(0.50 0.160 25)", dark: "oklch(0.72 0.140 25)"}
		"danger-soft":    {light: "oklch(0.94 0.050 25)", dark: "oklch(0.30 0.050 25)"}
		info:             {light: "oklch(0.47 0.110 240)", dark: "oklch(0.75 0.100 240)"}
		"info-soft":      {light: "oklch(0.94 0.040 240)", dark: "oklch(0.30 0.040 240)"}
		focus:            {light: "oklch(0.48 0.130 135)", dark: "oklch(0.75 0.100 135)"}
	}
}
