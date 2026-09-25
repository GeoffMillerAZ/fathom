package gallery

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// staticCSSHandler owns an immutable snapshot of the built stylesheet. Revalidate
// every reuse: a replacement build at the same URL must never get a freshness TTL.
// This does not apply to theme documents, candidate CSS or private source assets.
func staticCSSHandler(css []byte) http.HandlerFunc {
	body := string(css)
	etag := fmt.Sprintf(`"%x"`, sha256.Sum256(css))
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			// Preserve existing status/body behavior outside read methods without
			// treating a conditional POST/OPTIONS as a conditional GET.
			w.Header().Set("Cache-Control", "no-store")
			_, _ = io.WriteString(w, body)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=0, must-revalidate")
		w.Header().Set("ETag", etag)
		// No invented modification time. net/http owns conditional and HEAD
		// semantics, including weak/list validators and body-free 304 replies.
		http.ServeContent(w, r, "app.css", time.Time{}, strings.NewReader(body))
	}
}
