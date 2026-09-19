// tsimports.go — the SU-11 per-file import parser (ADR-041 decision 5,
// docs/plans/substrate.md §B). TSExtractor.ExtractImports turns the
// "import.*" captures added to javascript.scm/typescript.scm/python.scm
// into a structured ImportSpec list for internal/fusion.ResolveFileEdges's
// per-file resolver. It reuses the SAME cached (parser, compiled query)
// pair Extract uses for symbol extraction — no second grammar load, no
// second query compile — and is entirely additive: Extract's own capture
// switch (splitTSMatches) only recognizes "name"/"reference.call"/
// "definition.*" and silently ignores every "import.*" capture, so nothing
// about existing symbol extraction changes.
package extractor

import (
	"context"
	"fmt"

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
)

// ImportSpec is one name ResolveFileEdges can attribute a call site to,
// bound by a single import in one file.
type ImportSpec struct {
	LocalName    string // the name bound in THIS file's scope (post-alias); "" for a side-effect-only import
	ExportedName string // ImportNamed only: the name to look up in the target file (pre-alias)
	Module       string // raw module specifier text: unquoted JS string, Python dotted path, or a leading-dot relative Python path
	Kind         ImportKind
	Line         int
}

// ExtractImports parses relPath's import statements using the SAME cached
// (parser, query) pair Extract uses for this variant — see the package doc.
func (e *TSExtractor) ExtractImports(relPath string, src []byte) ([]ImportSpec, error) {
	variant := e.variantFor(relPath)

	e.mu.Lock()
	defer e.mu.Unlock()

	ctx := context.Background()
	p, q, err := e.parserFor(ctx, variant)
	if err != nil {
		return nil, err
	}
	tree, err := p.Parse(src)
	if err != nil {
		e.noteFault(variant)
		return nil, fmt.Errorf("tsextract[%s] %s: parse: %v: %w", e.spec.name, relPath, err, ErrTSEngineFault)
	}
	defer tree.Close()

	matches, err := q.Run(tree)
	if err != nil {
		e.noteFault(variant)
		return nil, fmt.Errorf("tsextract[%s] %s: query: %v: %w", e.spec.name, relPath, err, ErrTSEngineFault)
	}

	if e.spec.name == "python" {
		return pythonImports(matches, src), nil
	}
	return jsImports(matches, src), nil
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

// jsImports turns javascript.scm/typescript.scm's "import.*" captures into
// ImportSpecs. Tree-sitter matches patterns in the query-text order they
// were written for a given start position, so for one import_statement the
// specific (default/namespace/named) pattern's match always precedes the
// generic bare-statement pattern's match — bareSeen below only needs to
// remember statement byte ranges already claimed, never reorder anything.
func jsImports(matches []tsengine.Match, src []byte) []ImportSpec {
	var specs []ImportSpec
	claimed := map[[2]uint32]bool{}

	for _, m := range matches {
		caps := capMap(m)
		switch {
		case has(caps, "import.default"):
			stmt := caps["import.stmt"]
			specs = append(specs, ImportSpec{
				LocalName: caps["import.default"].Text(src),
				Module:    unquoteJS(caps["import.source"].Text(src)),
				Kind:      ImportWhole,
				Line:      stmt.Line(),
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
				Module: unquoteJS(caps["import.source"].Text(src)),
				Kind:   ImportNamed,
				Line:   stmt.Line(),
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
		}
	}
	return specs
}

// pythonImports turns python.scm's "import.*" captures into ImportSpecs.
func pythonImports(matches []tsengine.Match, src []byte) []ImportSpec {
	var specs []ImportSpec
	for _, m := range matches {
		caps := capMap(m)
		switch {
		case has(caps, "import.stmt"):
			mod := caps["import.module"].Text(src)
			local := mod
			if alias, ok := caps["import.alias"]; ok {
				local = alias.Text(src)
			}
			specs = append(specs, ImportSpec{LocalName: local, Module: mod, Kind: ImportWhole, Line: caps["import.stmt"].Line()})

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
		}
	}
	return specs
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
