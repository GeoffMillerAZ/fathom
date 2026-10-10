package extractor

// nativespecs.go — the dialect's file-spec queries over a go-tree-sitter tree
// (EW-53, O361). A CGO incumbent (C#, Java, Rust, PHP and C++ when EW-53
// landed; Rust alone since LB-F393 deleted the others) parses each file
// natively to extract; the same grammar commit is pinned for its wasm dialect
// (tsengine/grammars/PROVENANCE.md), so the dialect's tags and references
// queries can run over the tree the incumbent already holds instead of a
// second, wasm parse. nativeFileSpecs is that run: its matches are handed, as
// tsengine matches, to the capture->spec code ExtractFileSpecs uses. The
// one-parse contract itself (SpecExtractor) lives in specextractor.go; this
// file leaves with the last CGO walker (LB-F19).

import (
	"sync"

	ts "github.com/tree-sitter/go-tree-sitter"

	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
)

type nativeQueryKey struct {
	lang *ts.Language
	text string
}

var nativeQueries sync.Map // nativeQueryKey -> *nativeQueryEntry

type nativeQueryEntry struct {
	once sync.Once
	q    *ts.Query
	err  error
}

// nativeQuery compiles text for lang once per process; a compiled query is
// read-only and shared by every cursor.
func nativeQuery(lang *ts.Language, text string) (*ts.Query, error) {
	v, _ := nativeQueries.LoadOrStore(nativeQueryKey{lang, text}, &nativeQueryEntry{})
	e := v.(*nativeQueryEntry)
	e.once.Do(func() {
		q, qerr := ts.NewQuery(lang, text)
		if qerr != nil {
			e.err = qerr
			return
		}
		e.q = q
	})
	return e.q, e.err
}

// nativeMatches runs q over root and returns its matches as tsengine matches
// (capture names and byte ranges, start rows), in the engine's match order.
func nativeMatches(q *ts.Query, root *ts.Node, src []byte) []tsengine.Match {
	qc := ts.NewQueryCursor()
	defer qc.Close()
	names := q.CaptureNames()
	var out []tsengine.Match
	it := qc.Matches(q, root, src)
	for m := it.Next(); m != nil; m = it.Next() {
		caps := make([]tsengine.Capture, 0, len(m.Captures))
		for _, c := range m.Captures {
			n := c.Node
			caps = append(caps, tsengine.Capture{
				Name: names[c.Index],
				Node: tsengine.Node{
					StartByte: uint32(n.StartByte()),
					EndByte:   uint32(n.EndByte()),
					StartRow:  uint32(n.StartPosition().Row),
				},
			})
		}
		out = append(out, tsengine.Match{PatternIndex: uint16(m.PatternIndex), Captures: caps})
	}
	return out
}

// nativeFileSpecs is ExtractFileSpecs over root, a native tree of src parsed
// with the grammar rules' dialect is pinned to.
func nativeFileSpecs(lang *ts.Language, root *ts.Node, rules *tsDialectRules, src []byte) (FileSpecs, error) {
	tq, err := nativeQuery(lang, rules.query)
	if err != nil {
		return FileSpecs{}, err
	}
	matches := nativeMatches(tq, root, src)
	out := FileSpecs{Imports: importSpecsFromCaptures(matches, src)}
	if rules.refsQuery == "" {
		return out, nil
	}
	rq, err := nativeQuery(lang, rules.refsQuery)
	if err != nil {
		return FileSpecs{}, err
	}
	rmatches := nativeMatches(rq, root, src)
	keep := refKeeper(out.Imports, rules)
	out.Refs = refSpecsFromCaptures(rmatches, src, definitionNames(matches), importRanges(matches), keep)
	out.Exports = exportSpecsFromCaptures(rmatches, src)
	return out, nil
}

// nativeSpecsOut receives the FileSpecs an incumbent reads from its own tree.
type nativeSpecsOut struct {
	specs FileSpecs
	ok    bool
}

// nativeRulesFor is the wasm dialect's rules for relPath: what
// TSExtractor.ExtractFileSpecs would read the file with.
func nativeRulesFor(dialect, relPath string) *tsDialectRules {
	reg, err := tsRegistry()
	if err != nil {
		return nil
	}
	spec, ok := reg[dialect]
	if !ok {
		return nil
	}
	if v, ok := spec.variantForExt[extOf(relPath)]; ok {
		return v.rules
	}
	return spec.defaultVariant.rules
}

// take reads the dialect's specs over root, a native tree of src as written.
// A nil o is a caller that wants none; a failure leaves ok false.
func (o *nativeSpecsOut) take(dialect string, lang *ts.Language, root *ts.Node, relPath string, src []byte) {
	if o == nil {
		return
	}
	rules := nativeRulesFor(dialect, relPath)
	if rules == nil {
		return
	}
	if fs, err := nativeFileSpecs(lang, root, rules, src); err == nil {
		o.specs, o.ok = fs, true
	}
}
