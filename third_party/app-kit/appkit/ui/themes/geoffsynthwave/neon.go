package geoffsynthwave

import (
	_ "embed"
	"net/http"
)

//go:embed neon.css
var neonCSS string

// NeonCSS is an opt-in, static title effect adapted from the personal site's
// white glow. Apply ak-neon-title deliberately; never to all data or body text.
// No animation, font request, framework or Tailwind build is required.
func NeonCSS() string { return neonCSS }

// NeonHandler serves the optional effect separately from theme selection.
func NeonHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		_, _ = w.Write([]byte(neonCSS))
	})
}
