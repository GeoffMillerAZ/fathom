package view

import (
	"slices"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/fathom/pkg/lang"
)

// OutlineOptions bounds an outline.
type OutlineOptions struct {
	// MaxNodes is the most nodes the outline holds. 0 (or below) means no limit.
	MaxNodes int
}

// OutlineDoc is the shape of one file: its symbols nested by span containment. Every string is a field of the
// Result it came from, or the path the caller passed. The counts are those of the whole Result, not of what
// MaxNodes kept. Omitted is the number of nodes MaxNodes dropped.
type OutlineDoc struct {
	Path       string        `json:"path"`
	Dialect    string        `json:"dialect"`
	Language   string        `json:"language"`
	Level      string        `json:"level"`
	Skipped    string        `json:"skipped"`
	Incomplete bool          `json:"incomplete"`
	Nodes      []OutlineNode `json:"nodes"`
	Symbols    int           `json:"symbols"`
	References int           `json:"references"`
	Unresolved int           `json:"unresolved"`
	Facts      int           `json:"facts"`
	Notes      int           `json:"notes"`
	Omitted    int           `json:"omitted"`
}

// OutlineNode is one symbol. SpanKnown is false when the symbol has no known span (EndLine is then not an end).
// Children are the symbols its span strictly holds, never null.
type OutlineNode struct {
	Key       string        `json:"key"`
	Name      string        `json:"name"`
	Kind      string        `json:"kind"`
	Canonical string        `json:"canonical"`
	Signature string        `json:"signature"`
	Line      int           `json:"line"`
	EndLine   int           `json:"end_line"`
	SpanKnown bool          `json:"span_known"`
	Children  []OutlineNode `json:"children"`
}

// Outline nests the Result's symbols by span containment, the rule of contain.go: a node is a child of the
// innermost known-span symbol whose span holds its own, and among equal spans the first in listing order
// (Line, EndLine descending, Key) is the parent. A symbol without a known span is a top-level node with SpanKnown
// false. The input order of r.Symbols does not matter and r is not changed. An Incomplete or Skipped read still
// produces a document that says so.
func Outline(path string, r lang.Result, o OutlineOptions) OutlineDoc {
	d := OutlineDoc{
		Path: path, Dialect: r.Dialect, Language: r.Language, Level: r.Level, Skipped: r.Skipped, Incomplete: r.Incomplete,
		Symbols: len(r.Symbols), References: len(r.References), Unresolved: len(r.Unresolved), Facts: len(r.Facts), Notes: len(r.Notes),
	}
	syms := slices.Clone(r.Symbols)
	slices.SortFunc(syms, order)
	// parents[i] is the index of the parent of syms[i], or -1. The stack holds the chain of known-span
	// symbols that hold the current one, outermost first, so its top is the innermost.
	parents := make([]int, len(syms))
	var stack []int
	for i, s := range syms {
		parents[i] = -1
		if !known(s) {
			continue
		}
		for len(stack) > 0 && !covers(syms[stack[len(stack)-1]], s.Line, s.EndLine) {
			stack = stack[:len(stack)-1]
		}
		if len(stack) > 0 {
			parents[i] = stack[len(stack)-1]
		}
		stack = append(stack, i)
	}
	kids := make([][]int, len(syms))
	var top []int
	for i, p := range parents {
		if p >= 0 {
			kids[p] = append(kids[p], i)
		} else {
			top = append(top, i)
		}
	}
	var build func(idx []int) []OutlineNode
	build = func(idx []int) []OutlineNode {
		out := make([]OutlineNode, 0, len(idx))
		for _, i := range idx {
			s := syms[i]
			out = append(out, OutlineNode{Key: s.Key, Name: s.Name, Kind: s.Kind, Canonical: s.Canonical, Signature: s.Signature,
				Line: s.Line, EndLine: s.EndLine, SpanKnown: known(s), Children: build(kids[i])})
		}
		return out
	}
	d.Nodes = build(top)
	if o.MaxNodes > 0 {
		d.Nodes, d.Omitted = limit(d.Nodes, o.MaxNodes)
	}
	return d
}

func countNodes(ns []OutlineNode) int {
	n := len(ns)
	for _, c := range ns {
		n += countNodes(c.Children)
	}
	return n
}

func depth(ns []OutlineNode) int {
	max := 0
	for _, n := range ns {
		if d := 1 + depth(n.Children); d > max {
			max = d
		}
	}
	return max
}

// cut returns ns with every node deeper than keep levels removed (keep 1 leaves the top level).
func cut(ns []OutlineNode, keep int) []OutlineNode {
	out := make([]OutlineNode, len(ns))
	for i, n := range ns {
		out[i] = n
		if keep <= 1 {
			out[i].Children = []OutlineNode{}
		} else {
			out[i].Children = cut(n.Children, keep-1)
		}
	}
	return out
}

// limit drops the deepest level, then the next, until at most max nodes remain. If the top level alone is over
// max, the first max top-level nodes are kept. It returns the kept nodes and how many were dropped.
func limit(ns []OutlineNode, max int) ([]OutlineNode, int) {
	total := countNodes(ns)
	if total <= max {
		return ns, 0
	}
	kept := ns
	for levels := depth(ns) - 1; levels >= 1; levels-- {
		kept = cut(ns, levels)
		if countNodes(kept) <= max {
			return kept, total - countNodes(kept)
		}
	}
	kept = cut(ns, 1)
	if len(kept) > max {
		kept = kept[:max]
	}
	return kept, total - len(kept)
}

// Text is the line-oriented form: a header, one line per node, two spaces of indent per depth, and a footer.
//
//	# <path> dialect=<d> level=<l>[ incomplete][ skipped=<reason>]
//	<indent><line>-<end or ?> <canonical or kind> <name>[  <signature>]
//	# symbols=<n> references=<n> unresolved=<n> facts=<n> notes=<n>[ omitted=<n>]
//
// Every line ends with a newline. A string of the Result can span lines (a Go Signature keeps its raw newlines
// and tabs), so the text form writes each run of whitespace that holds a newline or CR as one space and drops such
// a run at either end of a string. A run without a newline is written as it is. Only Text does this: the nodes and
// the JSON form keep every string unchanged, so a node is always exactly one line and the outline holds one more
// line per node than it has nodes besides the header and the footer. The footer counts are those of the whole Result and omitted appears only when
// MaxNodes dropped nodes, so a truncated outline always says so on its last line.
func (d OutlineDoc) Text() string {
	var b strings.Builder
	b.WriteString("# " + oneLine(d.Path) + " dialect=" + oneLine(d.Dialect) + " level=" + oneLine(d.Level))
	if d.Incomplete {
		b.WriteString(" incomplete")
	}
	if d.Skipped != "" {
		b.WriteString(" skipped=" + oneLine(d.Skipped))
	}
	b.WriteByte('\n')
	var write func(ns []OutlineNode, depth int)
	write = func(ns []OutlineNode, depth int) {
		for _, n := range ns {
			b.WriteString(strings.Repeat("  ", depth))
			b.WriteString(strconv.Itoa(n.Line) + "-")
			if n.SpanKnown {
				b.WriteString(strconv.Itoa(n.EndLine))
			} else {
				b.WriteByte('?')
			}
			label := n.Canonical
			if label == "" {
				label = n.Kind
			}
			b.WriteString(" " + oneLine(label) + " " + oneLine(n.Name))
			if sig := oneLine(n.Signature); sig != "" {
				b.WriteString("  " + sig)
			}
			b.WriteByte('\n')
			write(n.Children, depth+1)
		}
	}
	write(d.Nodes, 0)
	b.WriteString("# symbols=" + strconv.Itoa(d.Symbols) + " references=" + strconv.Itoa(d.References) +
		" unresolved=" + strconv.Itoa(d.Unresolved) + " facts=" + strconv.Itoa(d.Facts) + " notes=" + strconv.Itoa(d.Notes))
	if d.Omitted > 0 {
		b.WriteString(" omitted=" + strconv.Itoa(d.Omitted))
	}
	b.WriteByte('\n')
	return b.String()
}

// oneLine returns s with every run of whitespace that holds a newline or CR written as one space, and such a run
// at the start or end of s dropped. Other whitespace is unchanged.
func oneLine(s string) string {
	if !strings.ContainsAny(s, "\n\r") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		if !isSpace(s[i]) {
			b.WriteByte(s[i])
			i++
			continue
		}
		j, nl := i, false
		for j < len(s) && isSpace(s[j]) {
			nl = nl || s[j] == '\n' || s[j] == '\r'
			j++
		}
		switch {
		case !nl:
			b.WriteString(s[i:j])
		case i > 0 && j < len(s):
			b.WriteByte(' ')
		}
		i = j
	}
	return b.String()
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\v' || c == '\f'
}
