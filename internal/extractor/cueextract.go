package extractor

import (
	cuepath "path"
	"sort"
	"strings"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/literal"
	"cuelang.org/go/cue/parser"
	"cuelang.org/go/cue/token"

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
//
// VS-3 (docs/plans/verify-spine.md, .fathom/observations.md O3): each field
// node's Calls carries the CUE reference edges cueReferenceEdges finds in its
// value expression — unification, field type references, list element types,
// and cross-package selectors — mirroring goextract.go's collectCalls
// contract exactly (core.SymbolKey for same-file definitions, bare/"pkg.Name"
// forms for the callgraph package's generic resolver) so find_callers /
// references / test_reaches work for CUE with no new machinery.
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

	// cueImports maps a source identifier used at a selector base (the
	// explicit import alias, or — unaliased — the import path's last
	// segment) to that same canonical segment, mirroring goextract.go's
	// importIdents. Built alongside the existing KindCUEImport emission loop
	// so the unquoting logic (and its malformed-import guards) is not
	// duplicated.
	cueImports := map[string]string{}

	// f.Imports is deprecated (removal slated mid-2026); ImportSpecs is the
	// supported iterator over the same decls.
	for imp := range f.ImportSpecs() {
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
			EndLine:  imp.End().Line(),
			Tier:     core.TierVerified,
		})

		seg := cuepath.Base(path)
		ident := seg
		if imp.Name != nil && imp.Name.Name != "" && imp.Name.Name != "_" {
			ident = imp.Name.Name
		}
		if ident != "" && seg != "" {
			cueImports[ident] = seg
		}
	}

	// localDefs is the set of "#Name" definitions declared at the TOP LEVEL
	// of THIS file — the same per-file, top-level-only scope goextract.go's
	// localFuncs uses. A reference to one of these resolves to the full
	// core.SymbolKey; a reference to anything else (another file, another
	// package) is emitted as a bare/qualified name and left to the
	// callgraph package's generic resolver, exactly as unresolved Go calls
	// are.
	localDefs := map[string]bool{}
	for _, d := range f.Decls {
		field, ok := d.(*ast.Field)
		if !ok {
			continue
		}
		name, _, err := ast.LabelName(field.Label)
		if err == nil && strings.HasPrefix(name, "#") {
			localDefs[name] = true
		}
	}

	for _, d := range f.Decls {
		walkCUEField(d, service, pkg, relPath, "", 1, localDefs, cueImports, &nodes)
	}

	// VS-4 (O3(b)): a testdata/*.cue fixture that references a definition is
	// the CUE analogue of a Go test statically reaching a symbol. See
	// cueTestdataReaches's doc comment for the mechanism.
	nodes = append(nodes, cueTestdataReaches(service, pkg, relPath, nodes)...)

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
//
// Each emitted node's Calls carries cueReferenceEdges(field.Value) (VS-3):
// the same field-value expression this function inspects for nested
// StructLits also carries the field's outgoing reference edges.
func walkCUEField(d ast.Decl, service, pkg, relPath, parent string, depth int, localDefs map[string]bool, imports map[string]string, nodes *[]core.SymbolNode) {
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
		EndLine:  field.End().Line(),
		Tier:     core.TierVerified,
		Calls:    cueReferenceEdges(service, pkg, field.Value, localDefs, imports),
	})

	if depth == cueMaxDepth {
		return
	}

	if lit, ok := field.Value.(*ast.StructLit); ok {
		for _, elt := range lit.Elts {
			walkCUEField(elt, service, pkg, relPath, dotted, depth+1, localDefs, imports, nodes)
		}
	}
}

// cueReferenceEdges (VS-3, .fathom/observations.md O3) walks a field's value
// expression and returns the sorted, deduplicated set of callee keys for the
// CUE definitions ("#Name") it references — the edges the extractor
// previously discarded entirely:
//
//   - unification: `#Kit: #Block & {...}` → an edge from #Kit to #Block (the
//     BinaryExpr's "&" operands).
//   - field type references: `profile: #Profile` → an edge from profile to
//     #Profile (the value is the reference itself).
//   - list element types: `uses: [...#Block]` → an edge from uses to #Block
//     (the Ellipsis element type inside a ListLit).
//   - cross-package imports: `blocks.#Block` → a bare "seg.#Block" edge
//     (seg is the import's canonical last-path-segment identifier), left for
//     the callgraph package's generic resolver exactly as an unresolved Go
//     selector call is.
//
// A same-file top-level definition (localDefs) resolves to its full
// core.SymbolKey; anything else resolves to a bare or "seg.Name" form. Only
// references to a "#"-prefixed definition are edges — a plain field or
// builtin type reference (`profile: string`) is not.
func cueReferenceEdges(service, pkg string, value ast.Expr, localDefs map[string]bool, imports map[string]string) []string {
	if value == nil {
		return nil
	}
	seen := map[string]bool{}

	var walk func(e ast.Expr)
	walk = func(e ast.Expr) {
		switch v := e.(type) {
		case *ast.Ident:
			addCUEDefRef(seen, service, pkg, v.Name, localDefs)
		case *ast.SelectorExpr:
			addCUESelectorRef(seen, v.X, v.Sel, imports)
		case *ast.BinaryExpr:
			if v.Op == token.AND {
				walk(v.X)
				walk(v.Y)
			}
		case *ast.ListLit:
			for _, elt := range v.Elts {
				if ell, ok := elt.(*ast.Ellipsis); ok && ell.Type != nil {
					walk(ell.Type)
				}
			}
		}
	}
	walk(value)

	if len(seen) == 0 {
		return nil
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// addCUEDefRef records a reference to a bare "#Name" identifier: a same-file
// top-level definition resolves to its full core.SymbolKey (exact-match
// resolution, mirroring collectCalls's localFuncs branch); anything else is
// left as the bare name for the callgraph package's byName fallback. Only
// "#"-prefixed names are definitions — everything else (`profile: string`)
// is not a reference edge.
func addCUEDefRef(seen map[string]bool, service, pkg, name string, localDefs map[string]bool) {
	if !strings.HasPrefix(name, "#") {
		return
	}
	if localDefs[name] {
		seen[core.SymbolKey(service, pkg, name)] = true
		return
	}
	seen[name] = true
}

// addCUESelectorRef records a cross-package reference (`seg.#Name`): the
// selector base must be a known import identifier and the selector label
// must resolve to a "#"-prefixed definition name. Anything else — a local
// struct path segment, a non-definition field selector — is a best-effort
// skip, matching the extractor's existing posture on partial/ambiguous
// expressions elsewhere in this file.
func addCUESelectorRef(seen map[string]bool, x ast.Expr, sel ast.Label, imports map[string]string) {
	name, _, err := ast.LabelName(sel)
	if err != nil || !strings.HasPrefix(name, "#") {
		return
	}
	ident, ok := x.(*ast.Ident)
	if !ok {
		return
	}
	if seg, ok := imports[ident.Name]; ok {
		seen[seg+"."+name] = true
	}
}

// cueTestdataReaches (VS-4, .fathom/observations.md O3(b)) is the CUE
// analogue of Go's test-reaches: internal/analysis/callgraph.TestImpact walks
// the call graph's reverse edges from a symbol and reports any caller whose
// Name has the prefix "Test" — the same convention go test gives Test*
// functions. That classification is by NODE NAME, not by file or edge kind,
// and it is entirely generic (it inspects core.SymbolNode.Calls, indifferent
// to which extractor produced it) — so CUE joins it by supplying a
// Test-prefixed caller, not by changing it.
//
// For every top-level field in a testdata/*.cue fixture that carries a
// reference edge (cueReferenceEdges, VS-3), cueTestdataReaches synthesizes
// one sibling node named "Test"+the field's own name (stripped of a leading
// "#"), carrying the SAME Calls edges — a caller TestImpact's reverse walk
// will find, satisfying `reachable_from`/`test_reaches` for a CUE plan row
// with no new plan grammar and no change to the reaches machinery itself.
func cueTestdataReaches(service, pkg, relPath string, nodes []core.SymbolNode) []core.SymbolNode {
	if !isCUETestdataPath(relPath) {
		return nil
	}
	var synth []core.SymbolNode
	for _, n := range nodes {
		if strings.Contains(n.Name, ".") || len(n.Calls) == 0 {
			continue // only top-level fields (dotted = nested) carrying a reference edge
		}
		synth = append(synth, core.SymbolNode{
			Service:  service,
			Package:  pkg,
			Name:     "Test" + strings.TrimPrefix(n.Name, "#"),
			Kind:     n.Kind,
			FilePath: relPath,
			Line:     n.Line,
			EndLine:  n.EndLine,
			Tier:     core.TierVerified,
			Calls:    append([]string(nil), n.Calls...),
		})
	}
	return synth
}

// isCUETestdataPath reports whether relPath lives under a testdata/
// directory — the same convention Go's own tooling (and `go test`, which
// never treats testdata/ as buildable source) uses to mark fixtures as
// test-only.
func isCUETestdataPath(relPath string) bool {
	return strings.HasPrefix(relPath, "testdata/") || strings.Contains(relPath, "/testdata/")
}
