package topology

import (
	"fmt"
	"strings"

	"github.com/geoffmilleraz/app-kit/appkit/ui/callout"
)

// Node is one element in the topology tree — a region, cluster, service, or
// workload. A node with Children renders as a collapsible <details> group; a
// leaf renders as a single row. The tree is deep-nestable to any level.
type Node struct {
	// Name is the node label (e.g. "us-east-1", "checkout-svc"). Plain text;
	// escaped.
	Name string
	// Status is the node's health, shown as a token-colored badge. The zero
	// value ("") renders as Unknown.
	Status Status
	// Metric is an optional at-a-glance figure shown after the name (e.g.
	// "p99 142ms", "3/4 pods", "12% err"). Plain text; escaped. Empty hides it.
	Metric string
	// Href is an optional drill-through target for the node — the consuming app
	// supplies a loopback URL (D5); the kit imposes no transport or JS. When set,
	// the node name becomes a link. Rendered as an href attribute value
	// (escaped); pass only URLs you construct.
	Href string
	// Children are nested nodes. A node with children is a collapsible group; a
	// node without is a leaf row.
	Children []Node
}

// isLeaf reports whether the node has no children.
func (n Node) isLeaf() bool { return len(n.Children) == 0 }

// Options configures the rendered tree.
type Options struct {
	// AriaLabel labels the tree region for assistive tech. Default "Topology".
	AriaLabel string
	// Collapsed, when true, renders every group collapsed by default (no `open`
	// attribute). Default false: groups render expanded so the full hierarchy is
	// visible on first paint, then the user can collapse branches. Either way the
	// disclosure is pure CSS/HTML — no JS (D5).
	Collapsed bool
}

// Render returns the topology tree as a server-rendered HTML string. nodes are
// the top-level entries (e.g. global regions). The output is a single
// role="tree" region; each node is a row (leaf) or a <details> group
// (has children) whose <summary> carries the name, status badge, and metric.
//
// Determinism: nodes are rendered in the order given (no map iteration); status
// roles and classes are literal strings so the Tailwind scanner emits them.
func Render(nodes []Node, o Options) string {
	aria := o.AriaLabel
	if aria == "" {
		aria = "Topology"
	}

	var b strings.Builder
	fmt.Fprintf(&b,
		`<div role="tree" aria-label=%q class="rounded-lg border border-line bg-surface-raised text-sm text-ink">`,
		esc(aria))
	if len(nodes) == 0 {
		b.WriteString(`<p class="px-3 py-2 text-ink-muted">No nodes.</p>`)
	} else {
		for _, n := range nodes {
			renderNode(&b, n, 0, o)
		}
	}
	b.WriteString(`</div>`)
	return b.String()
}

// renderNode writes one node (and, recursively, its children) at the given
// depth. depth drives a left indent so nesting reads visually; it is rendered
// as an inline padding-left so it works without bespoke utility classes.
func renderNode(b *strings.Builder, n Node, depth int, o Options) {
	// 0.75rem base gutter + 1rem per level of nesting.
	indent := fmt.Sprintf("padding-left:%.2frem", 0.75+float64(depth))

	if n.isLeaf() {
		fmt.Fprintf(b,
			`<div role="treeitem" class="flex items-center gap-2 border-t border-line px-3 py-2 first:border-t-0" style="%s">`,
			indent)
		writeLabel(b, n)
		b.WriteString(`</div>`)
		return
	}

	open := " open"
	if o.Collapsed {
		open = ""
	}
	fmt.Fprintf(b, `<details role="treeitem" class="border-t border-line first:border-t-0"%s>`, open)
	fmt.Fprintf(b,
		`<summary class="flex cursor-pointer select-none items-center gap-2 px-3 py-2 text-ink hover:bg-surface-sunken" style="%s">`,
		indent)
	// A small disclosure caret (rotates via the open state with the marker
	// hidden); pure CSS, no JS.
	b.WriteString(`<span aria-hidden="true" class="shrink-0 text-ink-muted">▸</span>`)
	writeLabel(b, n)
	b.WriteString(`</summary>`)
	for _, c := range n.Children {
		renderNode(b, c, depth+1, o)
	}
	b.WriteString(`</details>`)
}

// writeLabel writes the shared row body: name (optionally a drill-through link),
// status badge, and optional metric. Used by both leaf rows and group summaries
// so a node renders identically whether or not it has children.
func writeLabel(b *strings.Builder, n Node) {
	name := esc(n.Name)
	if n.Href != "" {
		// Drill-through: a plain anchor — the app owns the loopback URL (D5),
		// the kit adds no JS. relative class keeps it above the summary's click
		// target so the link is independently focusable.
		fmt.Fprintf(b,
			`<a href=%q class="relative font-medium text-ink hover:text-accent hover:underline">%s</a>`,
			esc(n.Href), name)
	} else {
		fmt.Fprintf(b, `<span class="font-medium text-ink">%s</span>`, name)
	}

	s := n.Status
	if s == "" {
		s = Unknown
	}
	b.WriteString(callout.Badge(statusLabel(s), statusRole(s)))

	if n.Metric != "" {
		fmt.Fprintf(b, `<span class="ml-auto tabular-nums text-ink-muted">%s</span>`, esc(n.Metric))
	}
}
