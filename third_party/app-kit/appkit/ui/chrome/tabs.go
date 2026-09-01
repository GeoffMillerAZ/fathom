package chrome

import (
	"fmt"
	"strings"
)

// Tab is one link-based tab. Exactly one should be Active.
type Tab struct {
	Label  string
	Href   string
	Active bool
}

// Tabs renders a link-based tab bar (no JS — each tab is a plain link, so it
// works within the loopback invariant). The active tab is underlined in the
// accent role; the rest are muted.
func Tabs(tabs []Tab, ariaLabel string) string {
	if len(tabs) == 0 {
		return ""
	}
	if ariaLabel == "" {
		ariaLabel = "Tabs"
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<nav role="tablist" aria-label="%s" class="flex gap-1 border-b border-line">`, esc(ariaLabel))
	for _, t := range tabs {
		cls := "px-3 py-2 text-sm border-b-2 -mb-px "
		cur := `aria-current="false"`
		if t.Active {
			cls += "border-accent font-medium text-ink"
			cur = `aria-current="page"`
		} else {
			cls += "border-transparent text-ink-muted hover:text-ink"
		}
		fmt.Fprintf(&b, `<a role="tab" %s href="%s" class="%s">%s</a>`, cur, esc(t.Href), cls, esc(t.Label))
	}
	b.WriteString(`</nav>`)
	return b.String()
}
