// Package overlay renders the kit's overlay primitives — modal Dialog, slide-over
// Drawer, and Toast notifications — as token-themed, server-rendered HTML built
// on the native <dialog> element, enhanced by a dependency-free vanilla script
// (overlay.js, served via Script/Handler like appkit/ui/router). No Node, no
// framework bindings; everything stays within the loopback invariant (D5/ADR-0002):
// the markup is meaningful without JS (native <dialog> + Escape), and the script
// only adds open-by-id, light-dismiss, and client-spawned toasts.
//
// Toasts follow the server-first pattern: render the region with any flash
// messages after a POST-redirect (no JS needed); the JS toast() API is a
// progressive enhancement that appends to the same region.
package overlay

import (
	"fmt"
	"html"
	"html/template"
	"net/http"
	"strings"

	_ "embed"
)

//go:embed overlay.js
var overlayJS string

// Script returns overlay.js for inlining into a page (<script> tag).
func Script() string { return overlayJS }

// Handler serves overlay.js with a long-lived immutable cache header. Mount at a
// stable path, e.g. mux.Handle("/assets/overlay.js", overlay.Handler()).
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		_, _ = w.Write([]byte(overlayJS))
	})
}

func esc(s string) string { return html.EscapeString(s) }

func boolAttr(name string, on bool) string {
	if !on {
		return ""
	}
	return " " + name
}

// closeButton is the shared "×" dismiss control in a dialog/drawer header.
const closeButton = `<button type="button" class="ml-auto shrink-0 rounded-md p-1 text-ink-muted hover:bg-surface-sunken hover:text-ink focus:outline-none focus:ring-2 focus:ring-focus" data-overlay-dismiss aria-label="Close">` +
	`<svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true"><path d="M4 4l8 8M12 4l-8 8" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg></button>`

// header renders the shared title row (title + close), or just the close button
// when title is empty.
func header(title string) string {
	var b strings.Builder
	b.WriteString(`<header class="flex items-center gap-3 border-b border-line px-5 py-3">`)
	if title != "" {
		fmt.Fprintf(&b, `<h2 class="text-base font-medium text-ink">%s</h2>`, esc(title))
	}
	b.WriteString(closeButton)
	b.WriteString(`</header>`)
	return b.String()
}

func footer(f template.HTML) string {
	if f == "" {
		return ""
	}
	return `<footer class="flex flex-wrap items-center justify-end gap-2 border-t border-line px-5 py-3">` + string(f) + `</footer>`
}

// ── Dialog ────────────────────────────────────────────────────────────────────

// Dialog is a centered modal built on native <dialog>. Open it with a trigger
// (data-overlay-open="<ID>" via TriggerButton/OpenAttr) or AppKitOverlay.open(ID).
type Dialog struct {
	ID           string
	Title        string
	Body         template.HTML
	Footer       template.HTML
	Size         string // "sm" | "md" (default) | "lg"
	LightDismiss bool   // clicking the backdrop closes it
}

func dialogWidth(size string) string {
	switch size {
	case "sm":
		return "max-w-sm"
	case "lg":
		return "max-w-2xl"
	default:
		return "max-w-md"
	}
}

// RenderDialog returns the dialog as an HTML string.
//
// The backdrop veils the page with the scrim token role at 50%
// (`backdrop:bg-scrim/50`), so it follows the token set and scheme like every
// other kit color; a token set that omits the role gets one from tokens.Load.
//
// Centering is done with inset+translate utilities rather than the native
// margin:auto modal centering, so the dialog stays centered even when a host
// app ships an unlayered `* { margin: 0 }` reset (which beats Tailwind's
// layered utilities and would otherwise pin the modal to the top-left).
func RenderDialog(dlg Dialog) string {
	var b strings.Builder
	fmt.Fprintf(&b,
		`<dialog id="%s" class="fixed inset-auto left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2 max-h-[90dvh] w-full %s overflow-y-auto rounded-lg border border-line bg-surface-raised p-0 text-ink shadow-xl backdrop:bg-scrim/50"%s>`,
		esc(dlg.ID), dialogWidth(dlg.Size), boolAttr("data-overlay-light-dismiss", dlg.LightDismiss))
	b.WriteString(`<div data-overlay-panel class="flex flex-col">`)
	b.WriteString(header(dlg.Title))
	fmt.Fprintf(&b, `<div class="px-5 py-4 text-sm text-ink">%s</div>`, string(dlg.Body))
	b.WriteString(footer(dlg.Footer))
	b.WriteString(`</div></dialog>`)
	return b.String()
}

// ── Drawer ────────────────────────────────────────────────────────────────────

// Drawer is a full-height slide-over panel anchored to a side, built on native
// <dialog>. Same triggers as Dialog.
type Drawer struct {
	ID           string
	Title        string
	Body         template.HTML
	Footer       template.HTML
	Side         string // "right" (default) | "left"
	LightDismiss bool
}

// RenderDrawer returns the drawer as an HTML string.
//
// Like RenderDialog, the drawer is edge-anchored with inset utilities rather
// than margin:auto, so it survives a host `* { margin: 0 }` reset.
func RenderDrawer(dr Drawer) string {
	anchor := "right-0 left-auto" // right
	if dr.Side == "left" {
		anchor = "left-0 right-auto"
	}
	var b strings.Builder
	fmt.Fprintf(&b,
		`<dialog id="%s" class="fixed inset-y-0 %s h-dvh max-h-dvh w-full max-w-md rounded-none border-0 bg-surface-raised p-0 text-ink shadow-xl backdrop:bg-scrim/50"%s>`,
		esc(dr.ID), anchor, boolAttr("data-overlay-light-dismiss", dr.LightDismiss))
	b.WriteString(`<div data-overlay-panel class="flex h-full flex-col">`)
	b.WriteString(header(dr.Title))
	fmt.Fprintf(&b, `<div class="flex-1 overflow-y-auto px-5 py-4 text-sm text-ink">%s</div>`, string(dr.Body))
	b.WriteString(footer(dr.Footer))
	b.WriteString(`</div></dialog>`)
	return b.String()
}

// ── Trigger ───────────────────────────────────────────────────────────────────

// OpenAttr returns the attribute that makes any element open the dialog/drawer
// with the given id when clicked. Append it inside your own button/link tag.
func OpenAttr(targetID string) template.HTMLAttr {
	return template.HTMLAttr(fmt.Sprintf(`data-overlay-open="%s"`, esc(targetID)))
}

// TriggerButton renders a primary-styled button that opens the target overlay.
// A convenience for the common case; for custom controls use OpenAttr.
func TriggerButton(targetID, label string) string {
	return fmt.Sprintf(
		`<button type="button" class="inline-flex items-center justify-center rounded-md bg-accent px-3 py-2 text-sm font-medium text-accent-ink hover:bg-accent-hover focus:outline-none focus:ring-2 focus:ring-focus" data-overlay-open="%s">%s</button>`,
		esc(targetID), esc(label),
	)
}

// ── Toast ─────────────────────────────────────────────────────────────────────

// Toast is one notification. Role is "info" (default) | "success" | "caution" |
// "danger". The same role→class map is mirrored in overlay.js.
type Toast struct {
	Message string
	Role    string
}

const toastBase = "rounded-md border px-4 py-3 text-sm shadow-lg max-w-sm"

func toastRoleClass(role string) string {
	switch role {
	case "success":
		return "border-positive bg-positive-soft text-ink"
	case "caution":
		return "border-caution bg-caution-soft text-ink"
	case "danger":
		return "border-danger bg-danger-soft text-ink"
	default:
		return "border-info bg-info-soft text-ink"
	}
}

// RenderToast returns a single toast element (used inside a region, or standalone).
func RenderToast(t Toast) string {
	aria := "status"
	if t.Role == "danger" {
		aria = "alert"
	}
	return fmt.Sprintf(`<div class="%s %s" role="%s">%s</div>`,
		toastBase, toastRoleClass(t.Role), aria, esc(t.Message))
}

// RenderToastRegion returns the fixed, polite live-region that holds toasts, with
// any server-rendered initial toasts (the flash-after-redirect pattern — no JS
// required). AppKitOverlay.toast() appends client-spawned toasts to this same
// region (id "ak-toast-region").
func RenderToastRegion(initial []Toast) string {
	var b strings.Builder
	b.WriteString(`<div id="ak-toast-region" role="status" aria-live="polite" class="fixed bottom-4 right-4 z-50 flex flex-col gap-2">`)
	for _, t := range initial {
		b.WriteString(RenderToast(t))
	}
	b.WriteString(`</div>`)
	return b.String()
}
