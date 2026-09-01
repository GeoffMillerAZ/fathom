// Package card provides a generic token-driven Card primitive for kit UIs.
//
// # Slots
//
// A Card has three optional slot regions — Header, Body, and Footer — each
// accepted as [templ.Component]. Pass nil to omit a slot.
//
// # Theming
//
// All colors reference kit token roles via Tailwind utilities (bg-surface-raised,
// border-line, text-ink, text-ink-muted, rounded-lg). No hex values appear here.
// The consuming app's Tailwind build must import appkit/ui/tokens/theme.css.
//
// # Sizing / Padding
//
// The Card renders with full-width layout by default. Constrain width in the
// parent layout, not inside this component.
package card

import (
	"strings"

	"github.com/a-h/templ"
)

// Params configures a Card render.
type Params struct {
	// Header is an optional slot rendered above the body, inside the card border.
	// Nil omits the header section entirely.
	Header templ.Component
	// Body is the primary content slot. Nil renders an empty body.
	Body templ.Component
	// Footer is an optional slot rendered below the body.
	// Nil omits the footer section entirely.
	Footer templ.Component
	// Elevated uses surface-overlay (for modal/popover contexts) instead of
	// the default surface-raised.
	Elevated bool
	// NoBorder suppresses the border-line border. Use when nesting cards inside
	// a surface-sunken container that already provides visual separation.
	NoBorder bool
	// ExtraClass appends additional Tailwind utility classes to the outermost
	// element. Use sparingly — prefer token roles.
	ExtraClass string
}

// cardClass assembles the Tailwind class string for the outermost card div.
// All color and radius values reference kit token roles — no hex literals.
func cardClass(p Params) string {
	surface := "bg-surface-raised"
	if p.Elevated {
		surface = "bg-surface-overlay"
	}
	border := "border border-line"
	if p.NoBorder {
		border = ""
	}
	parts := []string{
		surface,
		"rounded-lg",
		border,
		"text-ink",
		"overflow-hidden",
	}
	if p.ExtraClass != "" {
		parts = append(parts, p.ExtraClass)
	}
	// Filter empty strings (e.g. NoBorder removes the border class string).
	out := make([]string, 0, len(parts))
	for _, s := range parts {
		if s != "" {
			out = append(out, s)
		}
	}
	return strings.Join(out, " ")
}
