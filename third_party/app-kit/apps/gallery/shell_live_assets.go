package gallery

import (
	"crypto/sha256"
	_ "embed"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// These scripts belong only to the explicitly connected Gallery observer.
// The pinned upstream client does not establish consumer or host compatibility.
//
//go:embed shell_live.js
var shellLiveJS string

//go:embed shell_live_entry.js
var shellLiveEntryJS string

//go:embed assets/datastar/v1.0.3/datastar.js
var shellLiveDatastarJS string

//go:embed assets/datastar/v1.0.3/LICENSE.md
var shellLiveDatastarLicense string

func registerShellLiveAssets(mux *http.ServeMux) {
	for _, asset := range []struct{ path, name, body, mime string }{
		{"/static/shell-live.js", "shell-live.js", shellLiveJS, "application/javascript; charset=utf-8"},
		{"/static/shell-live-entry.js", "shell-live-entry.js", shellLiveEntryJS, "application/javascript; charset=utf-8"},
		{"/static/datastar-v1.0.3.js", "datastar-v1.0.3.js", shellLiveDatastarJS, "application/javascript; charset=utf-8"},
		{"/static/datastar-v1.0.3.LICENSE.txt", "datastar-v1.0.3.LICENSE.txt", shellLiveDatastarLicense, "text/plain; charset=utf-8"},
	} {
		etag := fmt.Sprintf(`"%x"`, sha256.Sum256([]byte(asset.body)))
		mux.HandleFunc(asset.path, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", asset.mime)
			if asset.path == "/static/datastar-v1.0.3.js" {
				w.Header().Set("Link", `</static/datastar-v1.0.3.LICENSE.txt>; rel="license"`)
			}
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Cache-Control", "public, max-age=0, must-revalidate")
			w.Header().Set("ETag", etag)
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				w.Header().Set("Allow", "GET, HEAD")
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			http.ServeContent(w, r, asset.name, time.Time{}, strings.NewReader(asset.body))
		})
	}
}
