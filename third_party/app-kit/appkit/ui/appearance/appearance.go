// Package appearance shares a browser-local light/dark/system preference.
//
// Script and Handler are dependency-free L2 artifacts. BootstrapScript returns
// the same script plus configuration for an inline script before the theme CSS;
// it already includes the runtime, so a second script is unnecessary. Consumers
// own the inline script tag and any CSP nonce. RenderControl is an optional L3b
// native selector: scan kit Go sources when building its semantic utility CSS.
//
// A page has one storage key. Reuse the app's existing key to retain plain
// "light" and "dark" preferences. System removes the stored override and the
// root data-theme attribute; token CSS resolves the device theme. An empty key
// means page-session state only and never touches storage. Persistence failure
// does not prevent the current page from changing appearance.
//
// The sole global is window.AppKitAppearance:
//
//	init({storageKey: "ca-theme"}) // explicit configuration; conflicting keys throw
//	get()                        // current state; defaults to session-only if uninitialized
//	set("light" | "dark" | "system")
//	toggle()                     // choose the opposite effective theme explicitly
//	retry()                      // retry the failed load or save
//	mount(root)                  // synchronize controls added by app-owned rendering
//
// State is {preference, effective, storageKey, persistence, error}. Preference is
// light/dark/system, effective is light/dark. Persistence is saved/session/unsaved/
// unavailable/invalid; error is null or a stable diagnostic code. On initialization,
// selection, retry, matching cross-tab storage updates and device-theme changes
// while using system, window receives appkit:appearance-change with that state
// plus source (init/set/retry/storage/system). Apps redraw their own canvas or
// other non-CSS content from this event, and call get() for initial render.
// Cross-tab updates replace local unsaved state. Invalid stored values fall back
// to system and remain visibly invalid until a new preference is saved.
//
// Native select and button controls retain standard keyboard/pointer behavior.
// Same-key controls synchronize without replacement; a conflicting-key control
// is disabled with an explicit configuration error. Dynamic fragments call mount
// after insertion. Without the script, controls remain disabled and explanatory.
package appearance

import (
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"
)

// Options selects app-owned persistence. The zero value is session-only.
type Options struct {
	StorageKey string `json:"storageKey"`
}

//go:embed appearance.js
var appearanceJS string

// Script returns the classic browser script, without a wrapping script tag.
// It mounts existing controls at DOM readiness; loading it twice is harmless.
func Script() string { return appearanceJS }

// BootstrapScript configures and applies appearance before CSS is loaded. The
// returned body is safe within a script element, including hostile storage keys.
// It reuses Script so early application and later interaction cannot drift.
func BootstrapScript(o Options) string {
	config, _ := json.Marshal(o) // Options contains only a string; JSON also escapes HTML.
	return Script() + "\nwindow.AppKitAppearance.init(" + string(config) + ");\n"
}

// Handler serves the script from a Go HTTP server with revalidation at stable
// consumer-owned URLs, including GET, HEAD and conditional requests. Non-Go
// consumers can serve the same Script artifact through their own hosting path.
func Handler() http.Handler {
	etag := fmt.Sprintf(`"%x"`, sha256.Sum256([]byte(appearanceJS)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "public, max-age=0, must-revalidate")
		w.Header().Set("ETag", etag)
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		http.ServeContent(w, r, "appearance.js", time.Time{}, strings.NewReader(appearanceJS))
	})
}

// RenderControl returns a labeled native selector with save status and retry.
// It uses semantic role utilities and needs the L3b consumer CSS scan. A wrapping
// label avoids globally unique IDs, so multiple controls compose safely.
func RenderControl(o Options) string {
	return fmt.Sprintf(`<div data-appkit-appearance data-appearance-storage-key="%s" class="flex flex-col gap-2">
<label class="flex flex-wrap items-center gap-3 text-sm text-ink"><span class="font-medium">Appearance</span>
<select data-appkit-appearance-select disabled class="min-h-11 rounded-md border border-line bg-surface px-3 py-2 text-sm text-ink focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus disabled:opacity-60">
<option value="light">Light</option><option value="dark">Dark</option><option value="system" selected>System</option>
</select></label>
<p data-appkit-appearance-status role="status" aria-live="polite" aria-atomic="true" class="text-sm text-ink-muted">Appearance controls require JavaScript.</p>
<button data-appkit-appearance-retry type="button" hidden class="min-h-11 self-start rounded-md border border-line bg-surface px-3 py-2 text-sm font-medium text-ink hover:bg-surface-sunken focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus">Retry saving</button>
</div>`, html.EscapeString(o.StorageKey))
}
