// Package treesitter provides a thin, concurrency-safe wrapper around the
// go-tree-sitter CGO bindings. Each exported helper creates its own parser so
// callers can invoke them from multiple goroutines simultaneously.
package treesitter

import (
	"sort"
	"strings"

	ts "github.com/tree-sitter/go-tree-sitter"
)

// WithTree creates a fresh parser, sets the supplied language, parses src, and
// calls fn with the root node. The parser and tree are closed before returning.
//
// A new parser is created per call so multiple goroutines may safely call
// WithTree concurrently with any language — the tree-sitter parser is not
// thread-safe when shared.
//
// If SetLanguage returns an error it is returned immediately. If Parse yields a
// nil tree (malformed source) fn is not called and nil is returned (best-effort,
// no error). Any non-nil tree whose root node is nil is treated the same way.
func WithTree(lang *ts.Language, src []byte, fn func(root *ts.Node)) error {
	p := ts.NewParser()
	defer p.Close()

	if err := p.SetLanguage(lang); err != nil {
		return err
	}

	tree := p.Parse(src, nil)
	if tree == nil {
		return nil
	}
	defer tree.Close()

	root := tree.RootNode()
	if root == nil {
		return nil
	}

	fn(root)
	return nil
}

// FieldText returns the UTF-8 text of the child node at the named field, or ""
// if the field is absent or the node pointer is nil.
func FieldText(n *ts.Node, field string, src []byte) string {
	if n == nil {
		return ""
	}
	child := n.ChildByFieldName(field)
	if child == nil {
		return ""
	}
	return child.Utf8Text(src)
}

// Line converts a node's zero-based start row to a 1-based line number.
func Line(n *ts.Node) int {
	return int(n.StartPosition().Row) + 1
}

// EndLine converts a node's zero-based end row to a 1-based line number —
// the last line the node's own span covers (core.SymbolNode.EndLine, T5-5).
// Callers building a SymbolNode from a definition node already have that
// node in scope, so this is a one-line addition alongside Line(n).
func EndLine(n *ts.Node) int {
	return int(n.EndPosition().Row) + 1
}

// CollectCalls walks the subtree rooted at root and returns the sorted,
// de-duplicated callee texts. For each descendant node whose Kind() is a key
// in callFields, it reads the child at the named field callFields[kind] (when
// the field name is "", it uses the node's own text) and records that text.
// Call it with a function/method node to get that function's outgoing calls.
//
// Returns nil when no calls are found. Empty/whitespace callee texts are
// skipped.
func CollectCalls(root *ts.Node, src []byte, callFields map[string]string) []string {
	if root == nil {
		return nil
	}
	seen := make(map[string]struct{})
	collectCallsRec(root, src, callFields, seen)
	if len(seen) == 0 {
		return nil
	}
	out := make([]string, 0, len(seen))
	for text := range seen {
		out = append(out, text)
	}
	sort.Strings(out)
	return out
}

// collectCallsRec is the recursive inner walk for CollectCalls.
func collectCallsRec(n *ts.Node, src []byte, callFields map[string]string, seen map[string]struct{}) {
	if n == nil {
		return
	}
	kind := n.Kind()
	if field, ok := callFields[kind]; ok {
		var text string
		if field == "" {
			text = n.Utf8Text(src)
		} else {
			child := n.ChildByFieldName(field)
			if child != nil {
				text = child.Utf8Text(src)
			}
		}
		text = strings.TrimSpace(text)
		if text != "" {
			seen[text] = struct{}{}
		}
		// Still descend: a call expression may be nested inside another call
		// expression (e.g. foo(bar())).
	}
	count := n.NamedChildCount()
	for i := uint(0); i < count; i++ {
		collectCallsRec(n.NamedChild(i), src, callFields, seen)
	}
}
