package callout

import "fmt"

// softRoles are the status roles that have a -soft tint (used for the badge
// background). Other roles fall back to a neutral pill.
var softRoles = map[string]bool{
	"accent": true, "positive": true, "caution": true,
	"danger": true, "info": true,
}

// Badge renders an inline status pill — "Strong", "Filed", "At risk" — tinted by
// a token role. It is the at-a-glance label for a status next to a heading or in
// a table cell. Supported roles: accent, positive, caution, danger, info; any
// other value renders a neutral pill. Text is escaped.
func Badge(text, role string) string {
	bg, fg := "surface-sunken", "ink-muted" // neutral fallback
	if softRoles[role] {
		bg, fg = role+"-soft", role
	}
	return fmt.Sprintf(
		`<span class="inline-flex items-center rounded-full bg-%s px-2 py-0.5 text-xs font-medium text-%s">%s</span>`,
		bg, fg, esc(text),
	)
}
