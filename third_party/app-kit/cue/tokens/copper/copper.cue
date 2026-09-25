// Copper is a warm mineral identity for focused planning at a sunlit desk.
// It keeps product surfaces restrained while using a burnished orange action
// color. Both schemes are first-party semantic mappings, not a source import.
package copper

tokens: {
	name:        "copper"
	description: "Warm mineral copper with restrained orange actions, in distinct light and dark schemes."

	fonts: {
		sans: "system-ui, -apple-system, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif"
		mono: "ui-monospace, 'SF Mono', Menlo, Consolas, 'Liberation Mono', monospace"
	}
	radius: {sm: "0.25rem", md: "0.375rem", lg: "0.5rem", xl: "0.75rem", full: "9999px"}

	colors: {
		surface:          {light: "oklch(0.985 0.006 70)", dark: "oklch(0.17 0.013 70)"}
		"surface-raised": {light: "oklch(0.995 0.004 70)", dark: "oklch(0.21 0.015 70)"}
		"surface-sunken": {light: "oklch(0.960 0.009 70)", dark: "oklch(0.13 0.011 70)"}
		"surface-overlay": {light: "oklch(0.995 0.004 70)", dark: "oklch(0.23 0.016 70)"}
		ink:              {light: "oklch(0.23 0.020 60)", dark: "oklch(0.93 0.008 70)"}
		"ink-muted":      {light: "oklch(0.44 0.025 60)", dark: "oklch(0.72 0.014 70)"}
		"ink-faint":      {light: "oklch(0.54 0.018 60)", dark: "oklch(0.64 0.013 70)"}
		"ink-inverse":    {light: "oklch(0.985 0.006 70)", dark: "oklch(0.17 0.013 70)"}
		line:             {light: "oklch(0.89 0.012 70)", dark: "oklch(0.29 0.018 70)"}
		"line-strong":    {light: "oklch(0.60 0.018 70)", dark: "oklch(0.56 0.025 70)"}
		accent:           {light: "oklch(0.50 0.150 55)", dark: "oklch(0.72 0.130 55)"}
		"accent-hover":   {light: "oklch(0.43 0.150 55)", dark: "oklch(0.79 0.120 55)"}
		"accent-ink":     {light: "oklch(0.985 0.006 70)", dark: "oklch(0.17 0.013 70)"}
		"accent-soft":    {light: "oklch(0.94 0.040 55)", dark: "oklch(0.30 0.045 55)"}
		positive:         {light: "oklch(0.47 0.120 150)", dark: "oklch(0.72 0.110 150)"}
		"positive-soft":  {light: "oklch(0.93 0.045 150)", dark: "oklch(0.29 0.040 150)"}
		caution:          {light: "oklch(0.49 0.120 85)", dark: "oklch(0.80 0.110 85)"}
		"caution-soft":   {light: "oklch(0.94 0.050 85)", dark: "oklch(0.31 0.040 85)"}
		danger:           {light: "oklch(0.50 0.160 25)", dark: "oklch(0.72 0.140 25)"}
		"danger-soft":    {light: "oklch(0.94 0.050 25)", dark: "oklch(0.30 0.050 25)"}
		info:             {light: "oklch(0.48 0.110 235)", dark: "oklch(0.75 0.100 235)"}
		"info-soft":      {light: "oklch(0.94 0.040 235)", dark: "oklch(0.30 0.040 235)"}
		focus:            {light: "oklch(0.52 0.160 55)", dark: "oklch(0.74 0.120 55)"}
	}
}
