package main

// console_errors.go — OC-5 (docs/design/observatory-console.md §2): error
// pages render on the shell with the reason and the remedy. The survey found
// 404, 400 and unknown-service answers as bare text/plain outside the shell:
// a reader who mistyped a project name lost the navigation, the search and
// the next step all at once.

import (
	"encoding/json"
	"html"
	"html/template"
	"net/http"
	"net/url"
	"strings"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/callout"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/chrome"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/form"
)

// consoleError is a page failure with its status, reason and remedy.
type consoleError struct {
	Status int
	Title  string // "Unknown project", "Not found", "Store unreadable"
	Reason string // what happened, in words
	// Remedy is the exact command (or step) that fixes it; RemedyHref, when
	// set, is a page that helps (the fleet, a search).
	Remedy      string
	RemedyHref  string
	RemedyLabel string
}

func (e *consoleError) Error() string { return e.Title + ": " + e.Reason }

// renderConsoleError answers r with e on the shell — or as JSON when the
// request asked for JSON, so a script sees the same reason and remedy.
func renderConsoleError(w http.ResponseWriter, r *http.Request, reg *consoleRegistry, e *consoleError) {
	if e.Status == 0 {
		e.Status = http.StatusInternalServerError
	}
	if wantsJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(e.Status)
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{"status": e.Status, "error": e.Title, "reason": e.Reason, "remedy": e.Remedy, "remedy_href": e.RemedyHref})
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(e.Status)
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write([]byte(renderConsoleErrorPage(reg, r, e)))
}

// renderConsoleErrorPage is the error's HTML document.
func renderConsoleErrorPage(reg *consoleRegistry, r *http.Request, e *consoleError) string {
	kind := callout.Danger
	if e.Status >= 400 && e.Status < 500 {
		kind = callout.Caution
	}
	var remedy strings.Builder
	if e.Remedy != "" {
		remedy.WriteString(`<p class="mt-3 text-ink">Remedy:</p>`)
		remedy.WriteString(`<div class="mt-1 rounded-md bg-surface-sunken px-2 py-1 font-mono text-xs text-ink [overflow-wrap:anywhere]" data-console-remedy>` + html.EscapeString(e.Remedy) + `</div>`)
	}
	if e.RemedyHref != "" {
		label := e.RemedyLabel
		if label == "" {
			label = "Open the fleet"
		}
		remedy.WriteString(`<p class="mt-3 [&>a]:min-h-[44px]">` + form.RenderButton(form.Button{Label: label, Href: e.RemedyHref, Variant: form.ButtonGhost}) + `</p>`)
	}
	body := callout.Render(callout.Callout{
		Kind:     kind,
		Title:    e.Title,
		BodyHTML: template.HTML(`<p class="max-w-prose [overflow-wrap:anywhere]">` + html.EscapeString(e.Reason) + `</p>` + remedy.String()),
	})
	path := ""
	if r != nil && r.URL != nil {
		path = r.URL.Path
	}
	return renderConsoleShell(reg, consoleShell{
		Title:    e.Title,
		Crumbs:   []chrome.Crumb{{Label: "Observatory", Href: "/"}, {Label: e.Title}},
		Question: statusLine(e.Status, path),
		Body:     body,
	})
}

func statusLine(status int, path string) string {
	s := itoa(status) + " " + http.StatusText(status)
	if path != "" {
		s += " — " + path
	}
	return s
}

// consoleNotFound answers every path no route claims: on the shell, with a
// search for what was asked and a way home.
func consoleNotFound(reg *consoleRegistry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := strings.Trim(strings.ReplaceAll(r.URL.Path, "/", " "), " ")
		renderConsoleError(w, r, reg, &consoleError{
			Status:      http.StatusNotFound,
			Title:       "Not found",
			Reason:      "No Observatory page lives at " + r.URL.Path + ".",
			Remedy:      "search the console for what you were looking for",
			RemedyHref:  "/search?q=" + url.QueryEscape(q),
			RemedyLabel: "Search for “" + q + "”",
		})
	}
}

// consoleMethodNotAllowed is the shell's 405.
func consoleMethodNotAllowed(w http.ResponseWriter, r *http.Request, reg *consoleRegistry) {
	w.Header().Set("Allow", "GET, HEAD")
	renderConsoleError(w, r, reg, &consoleError{
		Status: http.StatusMethodNotAllowed, Title: "Method not allowed",
		Reason: r.Method + " is not served here — console pages are read-only (GET). Only wiring preview, apply and undo mutate, behind the local gate.",
	})
}
