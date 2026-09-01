// svg.go — shared SVG helpers used by all viz components.
package viz

import (
	"fmt"
	"html"
	"regexp"
	"strings"
)

// roleRe validates a token role name: lowercase letters, digits, hyphens only.
var roleRe = regexp.MustCompile(`^[a-z0-9-]+$`)

const fallbackRole = "ink-muted"

// cssColor resolves a role name to var(--ak-color-<role>).
// Invalid roles silently fall back to fallbackRole — never inject unsanitized
// strings into attributes.
func cssColor(role string) string {
	if role == "" {
		return fmt.Sprintf("var(--ak-color-%s)", fallbackRole)
	}
	if !roleRe.MatchString(role) {
		return fmt.Sprintf("var(--ak-color-%s)", fallbackRole)
	}
	return fmt.Sprintf("var(--ak-color-%s)", role)
}

// esc HTML-escapes a string for safe inclusion in SVG text content or attribute
// values. Always call this on any caller-supplied string.
func esc(s string) string {
	return html.EscapeString(s)
}

// fmtNum formats a float64 with up to 2 decimal places, trimming trailing zeros.
//
//	fmtNum(1.0)   → "1"
//	fmtNum(1.5)   → "1.5"
//	fmtNum(1.50)  → "1.5"
//	fmtNum(1.23)  → "1.23"
func fmtNum(v float64) string {
	s := fmt.Sprintf("%.2f", v)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s
}

// svgOpen returns an opening <svg> tag with common accessibility attributes.
//   - role is always "img"
//   - a <title> element is injected as the first child (returned separately)
func svgOpen(w, h int, ariaLabel string, extraAttrs string) string {
	label := esc(ariaLabel)
	if label == "" {
		label = "chart"
	}
	base := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" role="img" aria-label="%s" viewBox="0 0 %d %d"`,
		w, h, label, w, h)
	if extraAttrs != "" {
		base += " " + extraAttrs
	}
	return base + ">"
}

// svgTitle returns a <title> element for screen-reader accessibility.
func svgTitle(label string) string {
	return "<title>" + esc(label) + "</title>"
}
