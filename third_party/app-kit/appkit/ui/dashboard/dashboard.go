// Package dashboard renders a token-themed dashboard layout — an app Shell
// (sidebar + topbar + content region) and a WidgetGrid of cards — as
// deterministic, server-rendered HTML. It is the turnkey dashboard shell the kit
// otherwise lacks; charts inside it come from appkit/ui/viz, KPIs from
// appkit/ui/stat, tables from appkit/ui/table.
//
// The layout structure is adapted from MatDash (the purchased React/Tailwind
// dashboard template, data/matdash) — its FullLayout shell re-themed to kit role
// utilities and rendered in pure Go (no Node, D2; D13: the shared layout lives in
// the kit, not copied per-app). The app supplies the nav, brand, and actions as
// HTML slots; the kit owns no routing or transport (D5).
//
// Files: shell.go (the frame), widget.go (grid + cards).
package dashboard

import "html"

func esc(s string) string { return html.EscapeString(s) }
