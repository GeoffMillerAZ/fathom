package docs

// anchor.go — "anchor nav": the permalink a reader clicks to copy a stable
// link to one heading.
//
// This is deliberately the whole of what this file does. Assigning the id a
// heading gets is the caller's job (see the package doc's "anchors are
// given" section) — this file only lays out the small "#" once an id
// exists.

import (
	"fmt"
)

// RenderPermalink renders the permalink for one heading. It renders nothing
// for an empty id: a heading the caller's render pass could not anchor gets
// no permalink, rather than one pointing at the bare "#" (which would
// silently jump to the top of the page instead of doing nothing).
func RenderPermalink(id string) string {
	if id == "" {
		return ""
	}
	return fmt.Sprintf(`<a class="doc-heading__anchor" href="#%s" aria-label="Permalink to this section">#</a>`, esc(id))
}
