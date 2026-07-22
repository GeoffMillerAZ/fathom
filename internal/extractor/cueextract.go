package extractor

import (
	"sort"
	"strings"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/literal"
	"cuelang.org/go/cue/parser"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// cueMaxDepth caps struct-literal recursion (ADR-021 §1 / LT-4). Depth 1 is a
// top-level field (#Layer); depth 2 is one level nested (#Layer.budgets);
// recursion stops once depth would exceed cueMaxDepth so a pathological or
// deeply self-referential schema cannot make extraction unbounded. Six levels
// comfortably covers real manifests (schema/semantic/schema.cue's deepest
// field, #Layer.budgets.max_usd_per_run, is depth 3).
const cueMaxDepth = 6

// CUEExtractor implements Extractor for CUE language files. It uses the
// official cuelang.org/go parser to detect field declarations, recursing into
// nested struct literals so every field is visible — not just top-level ones.
// Fields whose label starts with "#" → KindCUEDefinition; all other fields →
// KindCUEField. Import declarations are captured as KindCUEImport reference
// observations. Tier is TierVerified (real parse) throughout.
type CUEExtractor struct{}

// NewCUEExtractor returns a new CUEExtractor.
func NewCUEExtractor() *CUEExtractor { return &CUEExtractor{} }

// Extract parses CUE source bytes and returns SymbolNodes for every field
// declaration (recursing into nested struct literals, dotted names —
// #Layer.budgets.max_usd_per_run) plus one node per import declaration. Parse
// errors are treated as best-effort: if the parser returns a nil File we
// return nil,nil; otherwise we use whatever partial AST the parser produced.
//
// Package is read from the file's own `package` clause (ast.File.PackageName)
// — the authoritative source — falling back to pkgFromPath only when the
// file has no package clause at all (e.g. a plain data fragment meant to be
// unified into another file's package). This fixes the prior filename-derived
// heuristic, which gave two files sharing `package config` (e.g.
// internal/config/config_schema.cue and schema/config/schema.cue) two
// different, both-wrong Package values.
func (c *CUEExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	f, _ := parser.ParseFile(relPath, src)
	if f == nil {
		return nil, nil
	}

	pkg := f.PackageName()
	if pkg == "" {
		pkg = pkgFromPath(relPath)
	}

	var nodes []core.SymbolNode

	for _, imp := range f.Imports {
		// Malformed imports (e.g. `import 42` in a mid-edit save) parse to a
		// partial AST whose ImportSpec carries a nil or empty Path — and
		// literal.Unquote's contract assumes a lexed string token: it indexes
		// the first byte after stripping leading '#'s, so an empty value (or
		// an all-'#' value) PANICS with index-out-of-range. Nothing in
		// fusion/mcp/cmd recovers, so without these guards one bad .cue file
		// crashes `fathom ingest` and the stdio server outright.
		if imp.Path == nil || imp.Path.Value == "" {
			continue
		}
		path := imp.Path.Value
		if trimmed := strings.TrimLeft(path, "#"); len(trimmed) > 0 && (trimmed[0] == '"' || trimmed[0] == '\'') {
			if unquoted, err := literal.Unquote(path); err == nil {
				path = unquoted
			}
		}
		if path == "" {
			continue
		}
		nodes = append(nodes, core.SymbolNode{
			Service:  service,
			Package:  pkg,
			Name:     path,
			Kind:     core.KindCUEImport,
			FilePath: relPath,
			Line:     imp.Pos().Line(),
			Tier:     core.TierVerified,
		})
	}

	for _, d := range f.Decls {
		walkCUEField(d, service, pkg, relPath, "", 1, &nodes)
	}

	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Line != nodes[j].Line {
			return nodes[i].Line < nodes[j].Line
		}
		return nodes[i].Name < nodes[j].Name
	})
	return nodes, nil
}

// walkCUEField emits a SymbolNode for one field declaration and, when its
// value is itself a struct literal, recurses into that struct's own
// declarations — building a dotted name (parent.child) at each level.
// depth is 1 for a top-level field; recursion is capped at cueMaxDepth (see
// its doc comment). Non-Field declarations (embeddings, comprehensions, …)
// and labels the parser cannot resolve to a plain name are silently skipped,
// matching the extractor's existing best-effort posture on partial ASTs.
//
// Recursion descends ONLY into a direct *ast.StructLit value. A struct
// nested inside a conjunction or disjunction (`a: {x: int} & #Base`,
// `b: {y: int} | {z: int}`) is an ast.BinaryExpr, not a StructLit, so its
// fields are not emitted — a documented best-effort limit, consistent with
// the posture above, rather than a promise of full expression evaluation.
func walkCUEField(d ast.Decl, service, pkg, relPath, parent string, depth int, nodes *[]core.SymbolNode) {
	if depth > cueMaxDepth {
		return
	}
	field, ok := d.(*ast.Field)
	if !ok {
		return
	}
	name, _, err := ast.LabelName(field.Label)
	if err != nil || name == "" {
		return
	}
	dotted := name
	if parent != "" {
		dotted = parent + "." + name
	}

	kind := core.KindCUEField
	if strings.HasPrefix(name, "#") {
		kind = core.KindCUEDefinition
	}

	*nodes = append(*nodes, core.SymbolNode{
		Service:  service,
		Package:  pkg,
		Name:     dotted,
		Kind:     kind,
		FilePath: relPath,
		Line:     field.Pos().Line(),
		Tier:     core.TierVerified,
	})

	if depth == cueMaxDepth {
		return
	}

	if lit, ok := field.Value.(*ast.StructLit); ok {
		for _, elt := range lit.Elts {
			walkCUEField(elt, service, pkg, relPath, dotted, depth+1, nodes)
		}
	}
}
