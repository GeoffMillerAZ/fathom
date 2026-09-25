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
	"net/url"
)

// fragmentHref preserves the caller-owned heading ID while making it safe for
// both URL-fragment and HTML-attribute contexts, in that order.
func fragmentHref(id string) string {
	if id == "" {
		return "#" // Preserve the ToC's existing top-of-document link.
	}
	return esc((&url.URL{Fragment: id}).String())
}

// RenderPermalink renders the permalink for one heading. It renders nothing
// for an empty id: a heading the caller's render pass could not anchor gets
// no permalink, rather than one pointing at the bare "#" (which would
// silently jump to the top of the page instead of doing nothing).
func RenderPermalink(id string) string {
	if id == "" {
		return ""
	}
	return fmt.Sprintf(`<a class="doc-heading__anchor" href="%s" aria-label="Permalink to this section">#</a>`, fragmentHref(id))
}
