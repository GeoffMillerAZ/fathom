package docs

import "html"

// esc escapes text for interpolation into HTML text or a quoted attribute —
// the same helper every other ui/* package in the kit defines locally
// (chrome.esc, callout.esc): html.EscapeString, not template.HTMLEscapeString,
// so escaping is identical across the kit regardless of which package a
// consumer pulled in.
func esc(s string) string { return html.EscapeString(s) }
