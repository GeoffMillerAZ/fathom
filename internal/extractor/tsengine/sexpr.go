package tsengine

import (
	"fmt"
	"strings"
)

// sexprLeafText caps a leaf's quoted source text in SExpr output.
const sexprLeafText = 40

// SExpr renders the tree as an S-expression of its named nodes, one node
// per line, indented by depth: `field: (type L<line> "leaf text")`. It is
// the query-writing aid behind `indexbench lang-probe` (LB-F20, design §10,
// §13 step 12) — a worker reads this instead of grammar.js or
// node-types.json. maxLines > 0 caps the output; truncated is the number of
// nodes left unrendered. Not on the extraction hot path: no guest budget is
// applied beyond the parse that produced the tree.
func (t *Tree) SExpr(maxLines int) (lines []string, truncated int, err error) {
	rootPtr, _, err := t.rootPtr()
	if err != nil {
		return nil, 0, err
	}
	defer t.p.free(rootPtr)
	w := &sexprWriter{t: t, max: maxLines}
	if err := w.walk(rootPtr, "", 0); err != nil {
		return nil, 0, err
	}
	return w.lines, w.dropped, nil
}

type sexprWriter struct {
	t       *Tree
	max     int
	lines   []string
	dropped int
}

func (w *sexprWriter) full() bool { return w.max > 0 && len(w.lines) >= w.max }

func (w *sexprWriter) walk(nodePtr uint32, field string, depth int) error {
	p := w.t.p
	if w.full() {
		w.dropped++
		return nil
	}
	node, err := p.readNode(nodePtr)
	if err != nil {
		return err
	}
	typ, err := w.cString("ts_node_type", uint64(nodePtr))
	if err != nil {
		return err
	}
	named, err := p.call("ts_node_named_child_count", uint64(nodePtr))
	if err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString(strings.Repeat("  ", depth))
	if field != "" {
		b.WriteString(field + ": ")
	}
	fmt.Fprintf(&b, "(%s L%d", typ, node.Line())
	if named[0] == 0 {
		text := node.Text(w.t.src)
		if len(text) > sexprLeafText {
			text = text[:sexprLeafText] + "…"
		}
		if text != "" {
			fmt.Fprintf(&b, " %q", text)
		}
		b.WriteString(")")
		w.lines = append(w.lines, b.String())
		return nil
	}
	w.lines = append(w.lines, b.String())
	count, err := p.call("ts_node_child_count", uint64(nodePtr))
	if err != nil {
		return err
	}
	child, err := p.malloc(nodeSize)
	if err != nil {
		return err
	}
	defer p.free(child)
	for i := uint32(0); i < uint32(count[0]); i++ {
		if _, err := p.call("ts_node_child", uint64(child), uint64(nodePtr), uint64(i)); err != nil {
			return err
		}
		isNamed, err := p.call("ts_node_is_named", uint64(child))
		if err != nil {
			return err
		}
		if isNamed[0] == 0 {
			continue
		}
		fieldName, err := w.cString("ts_node_field_name_for_child", uint64(nodePtr), uint64(i))
		if err != nil {
			return err
		}
		// The child's struct is reused by the next iteration; recurse on a
		// private copy.
		b, err := p.readBytes(child, nodeSize)
		if err != nil {
			return err
		}
		own, err := p.writeBytes(b)
		if err != nil {
			return err
		}
		err = w.walk(own, fieldName, depth+1)
		p.free(own)
		if err != nil {
			return err
		}
	}
	if !w.full() || w.dropped == 0 {
		w.lines[len(w.lines)-1] += ")"
	}
	return nil
}

// cString calls a guest function returning a const char* and reads the
// NUL-terminated string ("" for NULL).
func (w *sexprWriter) cString(fn string, args ...uint64) (string, error) {
	res, err := w.t.p.call(fn, args...)
	if err != nil {
		return "", err
	}
	ptr := uint32(res[0])
	if ptr == 0 {
		return "", nil
	}
	mem := w.t.p.mod.Memory()
	var out []byte
	for i := uint32(0); i < 256; i++ {
		c, ok := mem.ReadByte(ptr + i)
		if !ok || c == 0 {
			break
		}
		out = append(out, c)
	}
	return string(out), nil
}
