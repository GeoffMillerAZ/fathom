// tsimports.go — the SU-11 per-file import parser (ADR-041 decision 5,
// docs/plans/substrate.md §B). TSExtractor.ExtractImports turns the
// "import.*" captures added to javascript.scm/typescript.scm/python.scm
// into a structured ImportSpec list for internal/fusion.ResolveFileEdges's
// per-file resolver. It reuses the SAME cached (parser, compiled query)
// pair Extract uses for symbol extraction — no second grammar load, no
// second query compile — and is entirely additive: Extract's own capture
// switch (splitTSMatchesWith) only recognizes "name"/"reference.call"/
// "definition.*" and silently ignores every "import.*" capture, so nothing
// about existing symbol extraction changes.
package extractor

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
)

// ImportKind classifies how an ImportSpec's LocalName is bound in the
// importing file's scope.
type ImportKind int

const (
	// ImportNamed: `import {a} from './x'`, `const {a} = require('./x')`,
	// `from pkg.mod import a` — a SPECIFIC name is bound, resolvable to one
	// symbol in the target file via ExportedName (the name to look up
	// there, before any local alias — equal to LocalName when unaliased).
	ImportNamed ImportKind = iota
	// ImportWhole: the entire module is bound to LocalName as a prefix —
	// `import * as ns`, `import def from './y'`, `const mod =
	// require('./x')`, a side-effect-only `import './x'` (LocalName ""),
	// Python `import pkg.mod` (LocalName is the full dotted text: Python
	// only binds the top segment, but every real call site goes through
	// the full chain, so treating it as the bound prefix resolves member
	// access correctly). A call through LocalName itself resolves to the
	// target's sole top-level symbol when unambiguous (ResolveFileEdges'
	// job, not this parser's); a call through LocalName.member resolves to
	// a same-named top-level symbol in the target file.
	ImportWhole
	// ImportWildcard: every exported name of Module is bound unqualified
	// (`import a.c.*`, `from x import *`); LocalName is "*". Emitted only by
	// the generic @import.wildcard capture (LB-F7); ResolveFileEdges'
	// strategies decide what, if anything, it resolves.
	ImportWildcard
)

// ImportSpec is one name ResolveFileEdges can attribute a call site to,
// bound by a single import in one file.
type ImportSpec struct {
	LocalName    string // the name bound in THIS file's scope (post-alias); "" for a side-effect-only import
	ExportedName string // ImportNamed only: the name to look up in the target file (pre-alias)
	Module       string // raw module specifier text: unquoted JS string, Python dotted path, or a leading-dot relative Python path
	Kind         ImportKind
	Line         int
	// Binding (LB-F411): an ES `import` binding a declaration of its module
	// by name, a named (`import { a }`) or a default (`import a from`) one;
	// never `import * as`, a side-effect import or a CommonJS require. The
	// file resolver stores no module edge for it where the importing file
	// already draws a symbol edge into the module's file. Its stored key is
	// one letter: file_imports holds a row per import of every TS/JS file.
	Binding bool `json:"B,omitempty"`
}

// ExtractImports parses relPath's import statements using the SAME cached
// (parser, query) pair Extract uses for this variant — see the package doc.
func (e *TSExtractor) ExtractImports(relPath string, src []byte) ([]ImportSpec, error) {
	variant := e.variantFor(relPath)

	e.mu.Lock()
	defer e.mu.Unlock()

	ctx := context.Background()
	l, q, err := e.leaseFor(ctx, variant)
	if err != nil {
		return nil, err
	}
	defer l.Release() // a no-op after noteFault's Discard
	tree, err := l.Parser().Parse(src)
	if err != nil {
		e.noteFault(l)
		return nil, fmt.Errorf("tsextract[%s] %s: parse: %v: %w", e.spec.name, relPath, err, ErrTSEngineFault)
	}
	defer tree.Close()

	matches, err := q.Run(tree)
	if err != nil {
		e.noteFault(l)
		return nil, fmt.Errorf("tsextract[%s] %s: query: %v: %w", e.spec.name, relPath, err, ErrTSEngineFault)
	}

	return importSpecsFromCaptures(matches, src), nil
}

// ImportSpan is one ImportSpec with the line span (1-based, inclusive) of
// the statement that binds it: the unit the language harness pairs an
// oracle's import references with (LB-F40, design §1.2 import_resolution)
// — a SCIP occurrence on a name inside a multi-line import, a dump's
// statement line. StartLine is never after the spec's Line and EndLine
// never before it.
type ImportSpan struct {
	ImportSpec
	StartLine, EndLine int
}

// ExtractImportSpans is ExtractImports with each spec's statement span: the
// same specs in the same order, each spanning the lines of the match that
// bound it (its statement capture, else its module and name captures).
func (e *TSExtractor) ExtractImportSpans(relPath string, src []byte) ([]ImportSpan, error) {
	variant := e.variantFor(relPath)

	e.mu.Lock()
	defer e.mu.Unlock()

	l, q, err := e.leaseFor(context.Background(), variant)
	if err != nil {
		return nil, err
	}
	defer l.Release()
	tree, err := l.Parser().Parse(src)
	if err != nil {
		e.noteFault(l)
		return nil, fmt.Errorf("tsextract[%s] %s: parse: %v: %w", e.spec.name, relPath, err, ErrTSEngineFault)
	}
	defer tree.Close()
	matches, err := q.Run(tree)
	if err != nil {
		e.noteFault(l)
		return nil, fmt.Errorf("tsextract[%s] %s: query: %v: %w", e.spec.name, relPath, err, ErrTSEngineFault)
	}
	specs, spans := importSpecsWithSpans(matches, src, true)
	out := make([]ImportSpan, len(specs))
	for i, s := range specs {
		lo := s.Line
		if sp := spans[i][0]; sp > 0 && sp < lo {
			lo = sp
		}
		out[i] = ImportSpan{ImportSpec: s, StartLine: lo, EndLine: max(spans[i][1], s.Line)}
	}
	if variant.rules.injectionHost {
		hosted, err := e.hostedImportSpansLocked(variant, l, tree, src) // F2: its injected ranges' imports are the file's (tsextract_host.go)
		if err != nil {
			return nil, err
		}
		out = append(out, hosted...)
	}
	return out, nil
}

// matchSpan is the first and last line (1-based) any of m's captures
// covers; newlines holds src's newline offsets, ascending.
func matchSpan(m tsengine.Match, newlines []int) [2]int {
	lineOf := func(off int) int { return sort.SearchInts(newlines, off) + 1 }
	span := [2]int{0, 0}
	for i, c := range m.Captures {
		lo := c.Node.Line()
		hi := lo
		if c.Node.EndByte > c.Node.StartByte {
			hi = max(lo, lineOf(int(c.Node.EndByte)-1))
		}
		if i == 0 || lo < span[0] {
			span[0] = lo
		}
		span[1] = max(span[1], hi)
	}
	return span
}

// capMap flattens one Match's captures into a name->Node lookup — every
// pattern below captures each name at most once per match.
func capMap(m tsengine.Match) map[string]tsengine.Node {
	out := make(map[string]tsengine.Node, len(m.Captures))
	for _, c := range m.Captures {
		out[c.Name] = c.Node
	}
	return out
}

// importSpecsFromCaptures turns any dialect's "import.*" captures into
// ImportSpecs (LB-F7, design §3.7). The generic vocabulary, for any
// imports.scm:
//
//   - @import.stmt: the statement (its line is the spec's line);
//
//   - @import.module: the module specifier (a quoted string is unquoted);
//
//   - @import.name: one specific imported name -> ImportNamed, bound as
//     @import.alias when present, else as the name;
//
//   - @import.wildcard: every name of the module -> ImportWildcard ("*");
//
//   - otherwise the whole module -> ImportWhole, bound as @import.alias when
//     present, else as the module text.
//
//   - @import.scope with an optional @import.prefix and an optional
//     @import.stmt: a scope record (LB-F83), no spec of its own. The node
//     @import.scope spans is a region in which every spec is prefixed by the
//     @import.prefix path (a Rust `a::b::{ .. }` group's list); a spec inside
//     several nested records takes every enclosing prefix, outermost first,
//     joined by "::" (composeImportPrefix), then its own @import.module, if
//     any. A record with no prefix and an @import.stmt scopes a whole
//     statement: it gives a spec its statement line and span when the spec's
//     own match, a pattern anchored at a nested group, cannot capture it.
//     @import.self: the whole composed prefix, bound to its last segment
//     (`use a::b::{self}` binds b).
//
// A dialect that writes no @import.scope compose nothing: its specs are
// exactly what the vocabulary above gives.
//
// The forms the JavaScript/TypeScript and Python queries already used are
// kept: import.default/namespace/named/bare_*/require* (JS) and
// import.from*/rel* (Python). Tree-sitter matches patterns in the
// query-text order they were written for a given start position, so for one
// JS import_statement the specific (default/namespace/named) pattern's match
// always precedes the bare-statement pattern's match — claimed below only
// needs to remember statement byte ranges already recorded.
func importSpecsFromCaptures(matches []tsengine.Match, src []byte) []ImportSpec {
	specs, _ := importSpecsWithSpans(matches, src, false)
	return specs
}

// importSpecsWithSpans is importSpecsFromCaptures, and with withSpans each
// spec's match span (matchSpan), index for index.
func importSpecsWithSpans(matches []tsengine.Match, src []byte, withSpans bool) ([]ImportSpec, [][2]int) {
	var specs []ImportSpec
	var spans [][2]int
	var newlines []int
	if withSpans {
		for i, b := range src {
			if b == '\n' {
				newlines = append(newlines, i)
			}
		}
	}
	claimed := map[[2]uint32]bool{}
	scopes := importScopes(matches, src)

	for _, m := range matches {
		// A case below that skips its match (`continue`) has appended
		// nothing, so every spec before this match already has its span.
		before := len(specs)
		caps := capMap(m)
		if has(caps, "import.scope") {
			continue // a scope record, composed into the specs inside it
		}
		composed := ""
		if len(scopes) > 0 {
			var stmt tsengine.Node
			var haveStmt bool
			composed, stmt, haveStmt = composeImportPrefix(caps, scopes)
			if haveStmt && !has(caps, "import.stmt") {
				m.Captures = append(append([]tsengine.Capture(nil), m.Captures...), tsengine.Capture{Name: "import.stmt", Node: stmt})
				caps = capMap(m)
			}
		}
		switch {
		case has(caps, "import.default"):
			stmt := caps["import.stmt"]
			specs = append(specs, ImportSpec{
				LocalName: caps["import.default"].Text(src),
				Module:    unquoteJS(caps["import.source"].Text(src)),
				Kind:      ImportWhole,
				Line:      stmt.Line(),
				Binding:   true,
			})
			claimed[stmtKey(stmt)] = true

		case has(caps, "import.namespace"):
			stmt := caps["import.stmt"]
			specs = append(specs, ImportSpec{
				LocalName: caps["import.namespace"].Text(src),
				Module:    unquoteJS(caps["import.source"].Text(src)),
				Kind:      ImportWhole,
				Line:      stmt.Line(),
			})
			claimed[stmtKey(stmt)] = true

		case has(caps, "import.named"):
			stmt := caps["import.stmt"]
			name := caps["import.named"].Text(src)
			local := name
			if alias, ok := caps["import.alias"]; ok {
				local = alias.Text(src)
			}
			specs = append(specs, ImportSpec{
				LocalName: local, ExportedName: name,
				Module:  unquoteJS(caps["import.source"].Text(src)),
				Kind:    ImportNamed,
				Line:    stmt.Line(),
				Binding: true,
			})
			claimed[stmtKey(stmt)] = true

		case has(caps, "import.bare_stmt"):
			stmt := caps["import.bare_stmt"]
			if claimed[stmtKey(stmt)] {
				continue // already recorded by a more specific pattern above
			}
			specs = append(specs, ImportSpec{
				Module: unquoteJS(caps["import.bare_source"].Text(src)),
				Kind:   ImportWhole,
				Line:   stmt.Line(),
			})

		case has(caps, "import.requirestmt"):
			if caps["import.requirecallee"].Text(src) != "require" {
				continue // some other single-string-arg call assigned to a variable
			}
			stmt := caps["import.requirestmt"]
			specs = append(specs, ImportSpec{
				LocalName: caps["import.requirename"].Text(src),
				Module:    unquoteJS(caps["import.requiresource"].Text(src)),
				Kind:      ImportWhole,
				Line:      stmt.Line(),
			})

		case has(caps, "import.requirestmt2"):
			if caps["import.requirecallee2"].Text(src) != "require" {
				continue
			}
			stmt := caps["import.requirestmt2"]
			name := caps["import.requirenamed"].Text(src)
			specs = append(specs, ImportSpec{
				LocalName: name, ExportedName: name,
				Module: unquoteJS(caps["import.requiresource2"].Text(src)),
				Kind:   ImportNamed,
				Line:   stmt.Line(),
			})

		case has(caps, "import.fromstmt"):
			mod := caps["import.frommodule"].Text(src)
			name := caps["import.fromname"].Text(src)
			local := name
			if alias, ok := caps["import.fromalias"]; ok {
				local = alias.Text(src)
			}
			specs = append(specs, ImportSpec{
				LocalName: local, ExportedName: name, Module: mod,
				Kind: ImportNamed, Line: caps["import.fromstmt"].Line(),
			})

		case has(caps, "import.relstmt"):
			mod := caps["import.relmodule"].Text(src)
			name := caps["import.relname"].Text(src)
			local := name
			if alias, ok := caps["import.relalias"]; ok {
				local = alias.Text(src)
			}
			specs = append(specs, ImportSpec{
				LocalName: local, ExportedName: name, Module: mod,
				Kind: ImportNamed, Line: caps["import.relstmt"].Line(),
			})

		case has(caps, "import.module"), has(caps, "import.self"), composed != "":
			if spec, ok := genericImportSpec(caps, src, composed); ok {
				if path, ok := joinedModule(m, src); ok && composed == "" {
					spec.Module = path // LB-F438: a path written as sibling nodes
				}
				specs = append(specs, spec)
			}
		}
		if withSpans && len(specs) > before {
			span := matchSpan(m, newlines)
			for len(spans) < len(specs) {
				spans = append(spans, span)
			}
		}
	}
	return specs, spans
}

// genericImportSpec is one match of the generic vocabulary (see
// importSpecsFromCaptures).
//
// prefix is the composed enclosing-scope path (composeImportPrefix, "" for
// every dialect that writes no @import.scope): the module is prefix, "::",
// then the match's own @import.module when it has one.
func genericImportSpec(caps map[string]tsengine.Node, src []byte, prefix string) (ImportSpec, bool) {
	own := ""
	if m, ok := caps["import.module"]; ok {
		own = unquoteJS(m.Text(src))
	}
	mod := own
	if prefix != "" {
		mod = prefix
		if own != "" {
			mod = prefix + "::" + own
		}
	}
	if mod == "" {
		return ImportSpec{}, false
	}
	if has(caps, "import.self") {
		// `use a::b::{self}`: the module the group names, bound to its last segment.
		last := mod
		if i := strings.LastIndex(mod, "::"); i >= 0 {
			last = mod[i+2:]
		}
		line := caps["import.self"].Line()
		if stmt, ok := caps["import.stmt"]; ok {
			line = stmt.Line()
		}
		return ImportSpec{LocalName: last, Module: mod, Kind: ImportWhole, Line: line}, true
	}
	line := caps["import.name"].Line()
	if m, ok := caps["import.module"]; ok {
		line = m.Line()
	}
	if stmt, ok := caps["import.stmt"]; ok {
		line = stmt.Line()
	}
	alias := ""
	if a, ok := caps["import.alias"]; ok {
		alias = a.Text(src)
	}
	switch {
	case has(caps, "import.name"):
		name := caps["import.name"].Text(src)
		local := name
		if alias != "" {
			local = alias
		}
		return ImportSpec{LocalName: local, ExportedName: name, Module: mod, Kind: ImportNamed, Line: line}, true
	case has(caps, "import.wildcard"):
		return ImportSpec{LocalName: "*", Module: mod, Kind: ImportWildcard, Line: line}, true
	default:
		local := mod
		if alias != "" {
			local = alias
		}
		return ImportSpec{LocalName: local, Module: mod, Kind: ImportWhole, Line: line}, true
	}
}

// joinedModule is a module path a grammar writes as sibling nodes (Scala's
// `import cats.data.X`: the import's own `path` identifiers, and a selector's
// name), captured as @import.module once per node: their texts in source
// order joined by ".". ok is false for one capture or none, which
// genericImportSpec reads as written.
func joinedModule(m tsengine.Match, src []byte) (string, bool) {
	var parts []tsengine.Node
	for _, c := range m.Captures {
		if c.Name == "import.module" {
			parts = append(parts, c.Node)
		}
	}
	if len(parts) < 2 {
		return "", false
	}
	sort.SliceStable(parts, func(i, j int) bool { return parts[i].StartByte < parts[j].StartByte })
	texts := make([]string, 0, len(parts))
	for _, n := range parts {
		texts = append(texts, strings.Join(strings.Fields(n.Text(src)), ""))
	}
	return strings.Join(texts, "."), true
}

func has(caps map[string]tsengine.Node, name string) bool {
	_, ok := caps[name]
	return ok
}

func stmtKey(n tsengine.Node) [2]uint32 { return [2]uint32{n.StartByte, n.EndByte} }

// unquoteJS strips a JS/TS string literal's surrounding quote characters
// (', ", or `) — tree-sitter's (string) node text includes them. A
// template literal with interpolation is returned with its raw text
// (interpolated module specifiers are not statically resolvable, and
// ResolveFileEdges' later path-resolution step simply won't find a file
// matching it, which is the correct "stays unresolved" outcome).
func unquoteJS(s string) string {
	if len(s) < 2 {
		return s
	}
	first, last := s[0], s[len(s)-1]
	if (first == '\'' || first == '"' || first == '`') && first == last {
		return s[1 : len(s)-1]
	}
	return s
}

// importScope is one scope record (see importSpecsFromCaptures): the byte
// range it spans, its prefix path when it has one, and its statement node
// when it has one.
type importScope struct {
	lo, hi    uint32
	prefix    string
	stmt      tsengine.Node
	hasStmt   bool
	hasPrefix bool
}

// importScopes collects every scope record of matches, in match order.
func importScopes(matches []tsengine.Match, src []byte) []importScope {
	var out []importScope
	for _, m := range matches {
		caps := capMap(m)
		sc, ok := caps["import.scope"]
		if !ok {
			continue
		}
		r := importScope{lo: sc.StartByte, hi: sc.EndByte}
		if p, ok := caps["import.prefix"]; ok {
			r.prefix, r.hasPrefix = strings.Join(strings.Fields(p.Text(src)), ""), true
		}
		if st, ok := caps["import.stmt"]; ok {
			r.stmt, r.hasStmt = st, true
		}
		out = append(out, r)
	}
	return out
}

// composeImportPrefix is the prefix a spec match inherits from the scope
// records enclosing it: every prefix whose scope contains the spec's anchor
// (its name, else its module, else its self capture), outermost first,
// joined by "::" — and the statement of the innermost record that has one.
// A match with no anchor, or inside no record, composes nothing.
func composeImportPrefix(caps map[string]tsengine.Node, scopes []importScope) (prefix string, stmt tsengine.Node, haveStmt bool) {
	anchor, ok := caps["import.name"]
	if !ok {
		if anchor, ok = caps["import.module"]; !ok {
			if anchor, ok = caps["import.self"]; !ok {
				return "", stmt, false
			}
		}
	}
	var in []importScope
	for _, sc := range scopes {
		if sc.lo <= anchor.StartByte && anchor.EndByte <= sc.hi {
			in = append(in, sc)
		}
	}
	// Outermost first: a scope that starts earlier, or spans more at the same start.
	sort.SliceStable(in, func(i, j int) bool {
		if in[i].lo != in[j].lo {
			return in[i].lo < in[j].lo
		}
		return in[i].hi > in[j].hi
	})
	var parts []string
	for _, sc := range in {
		if sc.hasPrefix && sc.prefix != "" {
			parts = append(parts, sc.prefix)
		}
		if sc.hasStmt {
			stmt, haveStmt = sc.stmt, true
		}
	}
	return strings.Join(parts, "::"), stmt, haveStmt
}
