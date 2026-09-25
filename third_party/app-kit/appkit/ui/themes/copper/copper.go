// Package copper ships the generated first-party Copper token stylesheet.
// It is an L1 asset: plain CSS, embeddable for Go consumers and vendorable for
// other consumers without a framework or font download.
package copper

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

// CSS returns the generated Copper stylesheet.
func CSS() string { return themeCSS }

// CSSHandler serves the embedded stylesheet with content revalidation.
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
