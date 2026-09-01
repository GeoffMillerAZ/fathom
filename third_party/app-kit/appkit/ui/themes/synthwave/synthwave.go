// Package synthwave ships the synthwave token set's generated stylesheet as a
// Go artifact so it travels with the module.
//
// It exists for one concrete reason. `go mod vendor` copies only directories
// that contain Go files — a directory holding nothing but theme.css is dropped
// on the floor. Before this file, a consuming app could not vendor the theme:
// resume_as_code's input.css reached four levels up
// (`../../../../app-kit/appkit/ui/themes/synthwave/theme.css`) into a sibling
// checkout, so its CSS build hard-failed on any machine that had not cloned
// app-kit next to it at exactly that depth. Six lines of //go:embed delete that
// assumption.
//
// theme.css is generated (`task gen:synthwave`) and committed; nothing here
// edits it. The theme is **L1** in ADR-0017's layering: plain CSS with no Go in
// it, valid in a <link>, consumable by anything — a Go server-rendered app, a
// React app's Tailwind entry, or a static page.
//
//	@import ".../appkit/ui/themes/synthwave/theme.css";   // Tailwind entry
//	http.Handle("GET /static/theme.css", synthwave.CSSHandler())  // or serve it
package synthwave

import (
	_ "embed"
	"net/http"
)

//go:embed theme.css
var themeCSS string

// CSS returns the generated synthwave stylesheet: the `@theme inline` block,
// the `--ak-color-*` custom properties per scheme, and the `dark:` custom
// variant. Import it after `@import "tailwindcss"` in a Tailwind app, or inline
// it into a <style> tag in an app with no CSS build.
func CSS() string { return themeCSS }

// CSSHandler serves theme.css with a long-lived immutable cache header. Mount
// at a stable path, e.g. mux.Handle("GET /static/theme.css", synthwave.CSSHandler()).
func CSSHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		_, _ = w.Write([]byte(themeCSS))
	})
}
