package catalog

import (
	"fmt"
	"sort"
	"strings"
)

// MarkdownOptions configures RenderMarkdown.
type MarkdownOptions struct {
	// Title is the document's leading heading, e.g. "# Package catalog".
	// Defaults to "# Catalog" if empty.
	Title string

	// RegenCommand is the exact command a reader should run to refresh
	// the file (e.g. "task docs-index"). The kit never invents a verb of
	// its own (design §6/§8): the consumer names its own regeneration
	// command. If empty, no DO-NOT-EDIT banner is emitted (useful for a
	// non-generated, ad-hoc render).
	RegenCommand string

	// IncludeHidden includes Hidden nodes. Default: rendered out (an
	// overlay's Hidden marks a node suppressed from output, not deleted
	// — see Build).
	IncludeHidden bool
}

// RenderMarkdown renders cat as a markdown document: an optional
// DO-NOT-EDIT banner naming RegenCommand (the convention this kit's own
// docs/catalog.md already follows), then the catalog as nested bullets
// depth-first from each top-level node. Same node set, same bytes,
// always: no wall-clock, no map iteration order, children sorted by
// Order then Slug, facet axes and values sorted.
func RenderMarkdown(cat Catalog, opts MarkdownOptions) string {
	var b strings.Builder
	if opts.RegenCommand != "" {
		fmt.Fprintf(&b, "<!-- Code generated. DO NOT EDIT. Regenerate: %s -->\n\n", opts.RegenCommand)
	}
	title := opts.Title
	if title == "" {
		title = "# Catalog"
	}
	b.WriteString(title)
	b.WriteString("\n\n")

	for _, root := range cat.Children("") {
		renderMarkdownNode(&b, cat, root, 0, opts.IncludeHidden)
	}
	return b.String()
}

func renderMarkdownNode(b *strings.Builder, cat Catalog, n Node, depth int, includeHidden bool) {
	if n.Hidden && !includeHidden {
		return
	}
	indent := strings.Repeat("  ", depth)
	fmt.Fprintf(b, "%s- `%s`", indent, n.ID())
	if n.Title != "" {
		fmt.Fprintf(b, " — %s", n.Title)
	}
	if n.Synopsis != "" {
		fmt.Fprintf(b, ": %s", n.Synopsis)
	}
	if len(n.Facets) > 0 {
		axes := make([]string, 0, len(n.Facets))
		for axis := range n.Facets {
			axes = append(axes, axis)
		}
		sort.Strings(axes)
		parts := make([]string, 0, len(axes))
		for _, axis := range axes {
			vals := append([]string(nil), n.Facets[axis]...)
			sort.Strings(vals)
			parts = append(parts, fmt.Sprintf("%s=[%s]", axis, strings.Join(vals, ",")))
		}
		fmt.Fprintf(b, " (%s)", strings.Join(parts, ", "))
	}
	b.WriteString("\n")

	for _, child := range cat.Children(n.ID()) {
		renderMarkdownNode(b, cat, child, depth+1, includeHidden)
	}
}
