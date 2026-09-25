// Package geoffindigo ships a CUE-authored semantic adaptation of the personal site's
// dark gray/indigo Engineering Console. It contains no website components or
// font assets. The Inter stack is a declaration, not a font-loading guarantee.
package geoffindigo

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

// CSS returns the generated stylesheet. Import it after Tailwind in a build,
// or link it after compiled semantic utilities to bind their runtime variables.
// Both light and dark scheme branches intentionally use the same dark palette.
func CSS() string { return themeCSS }

// CSSHandler serves the current embedded bytes at a caller-selected route.
// Stable routes must revalidate after upgrades, so they are not marked immutable.
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
