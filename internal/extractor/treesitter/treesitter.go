// Package treesitter provides a thin, concurrency-safe wrapper around the
// go-tree-sitter CGO bindings. Each exported helper creates its own parser so
// callers can invoke them from multiple goroutines simultaneously.
package treesitter

import (
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
