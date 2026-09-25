package form

import (
	"fmt"
	"html/template"
	"strings"
	"unicode"
	"unicode/utf8"
)

// SettingsSection is an experimental description/control composition for an
// app-owned profile or settings page. Its h3 belongs beneath the page's h2.
// HeadingID, Title and Description are escaped. Content is reviewed HTML, like
// Field.Control, and may contain controls or read-only app-owned information.
//
// The caller owns form tags, field labels, actions, authorization, persistence,
// save/recovery state and document-wide ID uniqueness, including IDs in Content.
type SettingsSection struct {
	HeadingID, Title, Description string
	Content                       template.HTML
}

// RenderSettingsSections lays out ordered sections with description and controls
// side by side at sm and above, stacked at narrower widths. It owns one leading
// border and a separator after each section; no wrapper or client runtime is added.
// Scan appkit/ui/form/*.go with the consumer's Tailwind build (L3b).
// Both columns allow long words to wrap. Reviewed content can override that
// inherited text flow; native control markup and behavior are not rewritten.
//
// A heading ID must be nonempty valid UTF-8, contain no whitespace/control characters and be
// unique within this call; titles must not be blank. Invalid input returns no
// partial markup, so callers can handle errors before writing a response. An empty
// slice renders nothing. This is a presentation check, not HTML-slot validation or
// an accessibility certification of the surrounding page.
func RenderSettingsSections(sections []SettingsSection) (string, error) {
	seen := make(map[string]bool, len(sections))
	for i, section := range sections {
		if section.HeadingID == "" || !utf8.ValidString(section.HeadingID) || strings.ContainsFunc(section.HeadingID, func(r rune) bool {
			return unicode.IsSpace(r) || unicode.IsControl(r)
		}) {
			return "", fmt.Errorf("settings section %d: heading ID must be nonempty valid UTF-8 without whitespace or controls", i)
		}
		if seen[section.HeadingID] {
			return "", fmt.Errorf("settings section %d: duplicate heading ID", i)
		}
		seen[section.HeadingID] = true
		if strings.TrimSpace(section.Title) == "" {
			return "", fmt.Errorf("settings section %d: title is required", i)
		}
	}

	var out strings.Builder
	for i, section := range sections {
		border := "border-b"
		if i == 0 {
			border = "border-y"
		}
		fmt.Fprintf(&out, `<section class="grid gap-x-8 gap-y-6 border-line py-7 sm:grid-cols-2 %s" aria-labelledby="%s"><div class="min-w-0 [overflow-wrap:anywhere]"><h3 id="%s" class="font-semibold">%s</h3>`, border, esc(section.HeadingID), esc(section.HeadingID), esc(section.Title))
		if section.Description != "" {
			fmt.Fprintf(&out, `<p class="mt-2 max-w-prose text-sm text-ink-muted">%s</p>`, esc(section.Description))
		}
		fmt.Fprintf(&out, `</div><div class="min-w-0 [overflow-wrap:anywhere]">%s</div></section>`, section.Content)
	}
	return out.String(), nil
}
