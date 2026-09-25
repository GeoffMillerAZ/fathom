// Package geoffsynthwave ships the owner's older white/pink/purple personal-site
// palette as a CUE-authored semantic adaptation. It is distinct from the existing
// synthwave package. CSS excludes font assets and website components; the static
// title effect is separately available through NeonCSS, never mandatory.
package geoffsynthwave

import (
	"crypto/sha256"
	_ "embed"
	"fmt"
	"net/http"
	"strings"
	"time"
)

//go:embed theme.css
var themeCSS string

var themeETag = fmt.Sprintf("\"%x\"", sha256.Sum256([]byte(themeCSS)))

// CSS returns the generated dark-only token stylesheet. Import it after Tailwind
// in a build, or link it after compiled semantic utilities to rebind their runtime
// colors, fonts and radii. Declaring Inter does not load it.
func CSS() string { return themeCSS }

// CSSHandler serves the embedded stylesheet with content-based revalidation.
// Stable routes are deliberately not immutable across module upgrades.
func CSSHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=0, must-revalidate")
		w.Header().Set("ETag", themeETag)
		http.ServeContent(w, r, "theme.css", time.Time{}, strings.NewReader(themeCSS))
	})
}
