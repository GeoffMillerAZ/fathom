// Lagoon is a clear-water identity for a bright studio and a dim evening desk.
// Its teal is reserved for active choices and links; the rest stays quietly cool.
package lagoon

tokens: {
	name:        "lagoon"
	description: "Cool lagoon teal with restrained blue-green actions, in distinct light and dark schemes."

	fonts: {
		sans: "system-ui, -apple-system, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif"
		mono: "ui-monospace, 'SF Mono', Menlo, Consolas, 'Liberation Mono', monospace"
	}
	radius: {sm: "0.25rem", md: "0.375rem", lg: "0.5rem", xl: "0.75rem", full: "9999px"}

	colors: {
		surface:          {light: "oklch(0.985 0.007 205)", dark: "oklch(0.16 0.014 205)"}
		"surface-raised": {light: "oklch(0.995 0.004 205)", dark: "oklch(0.20 0.016 205)"}
		"surface-sunken": {light: "oklch(0.960 0.010 205)", dark: "oklch(0.12 0.012 205)"}
		"surface-overlay": {light: "oklch(0.995 0.004 205)", dark: "oklch(0.22 0.017 205)"}
		ink:              {light: "oklch(0.22 0.020 205)", dark: "oklch(0.93 0.008 205)"}
		"ink-muted":      {light: "oklch(0.43 0.025 205)", dark: "oklch(0.72 0.014 205)"}
		"ink-faint":      {light: "oklch(0.53 0.018 205)", dark: "oklch(0.64 0.014 205)"}
		"ink-inverse":    {light: "oklch(0.985 0.007 205)", dark: "oklch(0.16 0.014 205)"}
		line:             {light: "oklch(0.89 0.013 205)", dark: "oklch(0.28 0.020 205)"}
		"line-strong":    {light: "oklch(0.60 0.020 205)", dark: "oklch(0.56 0.028 205)"}
		accent:           {light: "oklch(0.45 0.130 205)", dark: "oklch(0.73 0.110 205)"}
		"accent-hover":   {light: "oklch(0.39 0.130 205)", dark: "oklch(0.80 0.100 205)"}
		"accent-ink":     {light: "oklch(0.985 0.007 205)", dark: "oklch(0.16 0.014 205)"}
		"accent-soft":    {light: "oklch(0.94 0.040 205)", dark: "oklch(0.29 0.040 205)"}
		positive:         {light: "oklch(0.46 0.120 155)", dark: "oklch(0.72 0.110 155)"}
		"positive-soft":  {light: "oklch(0.93 0.045 155)", dark: "oklch(0.28 0.040 155)"}
		caution:          {light: "oklch(0.49 0.120 85)", dark: "oklch(0.80 0.110 85)"}
		"caution-soft":   {light: "oklch(0.94 0.050 85)", dark: "oklch(0.31 0.040 85)"}
		danger:           {light: "oklch(0.50 0.160 25)", dark: "oklch(0.72 0.140 25)"}
		"danger-soft":    {light: "oklch(0.94 0.050 25)", dark: "oklch(0.30 0.050 25)"}
		info:             {light: "oklch(0.47 0.110 240)", dark: "oklch(0.75 0.100 240)"}
		"info-soft":      {light: "oklch(0.94 0.040 240)", dark: "oklch(0.30 0.040 240)"}
		focus:            {light: "oklch(0.48 0.140 205)", dark: "oklch(0.75 0.110 205)"}
	}
}
