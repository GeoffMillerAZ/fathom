package catalog

import (
	"fmt"
	"strings"
)

// RenderTree renders the whole catalog as a compact ASCII tree for
// terminals (├──, └──), depth-first, children sorted by Order then Slug
// — the same determinism contract as every other renderer.
func RenderTree(cat Catalog) string {
	var b strings.Builder
	roots := cat.Children("")
	for i, root := range roots {
		renderTreeNode(&b, cat, root, "", i == len(roots)-1, 0, 0)
	}
	return b.String()
}

// RenderSubtree renders the subtree rooted at id (id itself plus its
// descendants), depth-limited to maxDepth levels below id (0 or negative
// means unlimited). id == "" is equivalent to RenderTree. Returns "" if
// id is not in cat.
func RenderSubtree(cat Catalog, id string, maxDepth int) string {
	if id == "" {
		return RenderTree(cat)
	}
	root, ok := cat.Get(id)
	if !ok {
		return ""
	}
	var b strings.Builder
	renderTreeNode(&b, cat, root, "", true, 0, maxDepth)
	return b.String()
}

func renderTreeNode(b *strings.Builder, cat Catalog, n Node, prefix string, last bool, depth, maxDepth int) {
	connector := "├── "
	nextPrefix := prefix + "│   "
	if last {
		connector = "└── "
		nextPrefix = prefix + "    "
	}
	label := n.Slug
	if n.Title != "" {
		label = fmt.Sprintf("%s (%s)", n.Slug, n.Title)
	}
	fmt.Fprintf(b, "%s%s%s\n", prefix, connector, label)

	if maxDepth > 0 && depth+1 > maxDepth {
		return
	}
	children := cat.Children(n.ID())
	for i, c := range children {
		renderTreeNode(b, cat, c, nextPrefix, i == len(children)-1, depth+1, maxDepth)
	}
}
