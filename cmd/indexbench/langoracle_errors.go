package main

// langoracle_errors.go — LB-F59 (O203): files_with_error, the number §14.2's
// parse-error refusal rests on (a grammar that leaves more than 5% of a
// subject's files with an error is refused), counted by the harness instead
// of a worker's scratch tree-walker.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
)

// errorNodesQuery captures every node a tree-sitter parse could not fit to
// the grammar: the extractor's own has-error test (tsHasError). lang-probe's
// S-expression is not enough here: it renders named nodes only, so a
// MISSING token (a `;` or `}` the parser inserted) never shows in it.
const errorNodesQuery = "[(ERROR) (MISSING)] @error"

// filesWithError counts the files (subject-relative, under root) whose tree
// on the tags dialect's own pinned grammar — the variant the engine parses
// each file with, extractor.ProbeTargetFor — holds an ERROR or MISSING
// node. Only the file's own grammar is parsed, never an injected language's.
// unparsed counts the files the parser itself failed on (never counted as
// files with an error; a fault is reported where the extraction faults).
//
// A dialect that names a pre-parse step (cpp-macros, objc-macros) reads a file
// that does not parse as written as each text the step gives, the first that
// parses clean: such a file is no file with an error, because that is the tree
// the extractor reads. asWritten counts the files with an error before the step.
func filesWithError(ctx context.Context, dialect, root string, files []string) (n, unparsed, asWritten int, err error) {
	type grammar struct {
		p *tsengine.Parser
		q *tsengine.Query
	}
	parsers := map[[2]string]grammar{}
	defer func() {
		for _, g := range parsers {
			g.q.Close()
			_ = g.p.Close()
		}
	}()
	for _, rel := range files {
		target, err := extractor.ProbeTargetFor(dialect, rel)
		if err != nil {
			return 0, 0, 0, err
		}
		key := [2]string{target.Blob, target.EntryPoint}
		g, ok := parsers[key]
		if !ok {
			p, err := tsengine.NewParser(ctx, target.Blob, target.EntryPoint)
			if err != nil {
				return 0, 0, 0, fmt.Errorf("grammar %s: %w", target.Blob, err)
			}
			q, err := p.CompileQuery(errorNodesQuery)
			if err != nil {
				_ = p.Close()
				return 0, 0, 0, fmt.Errorf("grammar %s: %w", target.Blob, err)
			}
			g = grammar{p, q}
			parsers[key] = g
		}
		src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return 0, 0, 0, err
		}
		tree, err := g.p.Parse(src)
		if err != nil {
			unparsed++
			continue
		}
		ms, err := g.q.Run(tree)
		tree.Close()
		if err != nil {
			unparsed++
			continue
		}
		if len(ms) > 0 {
			asWritten++
			if target.Preprocess != nil && mendedBy(g.p, g.q, src, target.Preprocess) {
				continue
			}
			n++
		}
	}
	return n, unparsed, asWritten, nil
}

// mendedBy reports whether some text the dialect's pre-parse step gives for src
// parses with no ERROR or MISSING node (the first one the extractor would read).
func mendedBy(p *tsengine.Parser, q *tsengine.Query, src []byte, pre func([]byte) [][]byte) bool {
	for _, text := range pre(src) {
		t, err := p.Parse(text)
		if err != nil {
			return false
		}
		ms, err := q.Run(t)
		t.Close()
		if err == nil && len(ms) == 0 {
			return true
		}
	}
	return false
}
