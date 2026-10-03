package extractor

import (
	cuepath "path"
	"sort"
	"strings"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/literal"
	"cuelang.org/go/cue/parser"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// cueMaxDepth caps field nesting (ADR-021 §1 / LT-4): depth 1 is a top-level
// field (#Layer), depth 2 one level nested (#Layer.budgets). The walk is over
// a parsed tree, so it is finite whatever the cap; the cap bounds recursion
// on a pathologically nested file. LB-Q-cue's reading (2026-10-02) found real
// configuration nested far past the earlier cap of six (a CRD's OpenAPI
// schema imported into CUE: spec.versions.schema.openAPIV3Schema.properties…,
// past twenty levels), every level a field the cue binary declares, so the
// cap is set where no real file reaches.
const cueMaxDepth = 256

// kindCUEPackage is a file's package clause (LB-Q-cue): the root entity a
// cross-file reference to the file resolves to, named "package/<name>" so it
// never shares a symbol key with a top-level field of the package's own name
// (`package dashboards` beside `dashboards: {...}`, common in real modules).
// It has no connector-classification role, so it is local to the dialect.
const kindCUEPackage core.SourceKind = "CUEPackage"

// cuePackagePrefix names a package entity.
const cuePackagePrefix = "package/"

// CUEExtractor implements Extractor and DocResulter for CUE files. It uses the
// official cuelang.org/go parser, which also resolves each identifier to the
// declaration it names in the file (astutil.Resolve).
//
// Entities: every field a struct literal declares, wherever the literal sits
// in an expression (a conjunction or disjunction operand, a default, a list
// element or element type, a comprehension's source or body, a let's value, an
// embedding), named by the path of the static labels of the fields that
// enclose it (`#Kit.ports.port`, `alt.d1`) — the naming `cue lsp`'s document
// symbols use. A label starting with "#" (or "_#") is a KindCUEDefinition,
// any other a KindCUEField. A field with no static name (a pattern constraint
// `[string]: v`, an interpolated or parenthesized label) is no entity; the
// fields inside its value are, under its label as written. One KindCUEImport
// per import declaration, and one package entity (kindCUEPackage) per package
// clause. Tier is TierVerified (a real parse) throughout.
//
// References (VS-3, LB-Q-cue): each field's Calls carries every identifier and
// selector in its value that names a field or definition, its own nested
// fields' values excluded (they carry their own) and a pattern's or dynamic
// field's included (they have no entity). A name the parser resolves in the
// file is the full core.SymbolKey of every declaration of that name in its
// scope; a selector into a declaration whose value this file writes as a
// struct (through conjunctions, value aliases and lets) is that nested
// declaration's key; a selector into a value that unifies with a declaration
// the file does not write is that name with the label appended
// ("#Conv.result"); a selector on an import (not the standard library) is
// "seg.Name" (seg the package's identifier), deeper selectors appended
// ("seg.#A.b"); a name the file does not declare (another file of the
// package) is the bare name, deeper selectors appended ("#Kit.name"), and so
// is a path of the package value the file's struct lacks ("net.config"). Let,
// alias and comprehension variables, the standard library and predeclared
// identifiers are no reference. ExtractDoc gives each reference that may land
// in another file to fusion's cue-import and cue-package strategies.
type CUEExtractor struct{}

var (
	_ Extractor   = (*CUEExtractor)(nil)
	_ DocResulter = (*CUEExtractor)(nil)
)

// NewCUEExtractor returns a new CUEExtractor.
func NewCUEExtractor() *CUEExtractor { return &CUEExtractor{} }

// Extract parses CUE source bytes and returns the file's entities (see the
// type's doc). Parse errors are best-effort: a nil File yields nil, nil;
// otherwise whatever partial tree the parser produced is read.
//
// Package is read from the file's own `package` clause (ast.File.PackageName)
// — the authoritative source — falling back to pkgFromPath only when the file
// has no package clause at all (e.g. a plain data fragment meant to be
// unified into another file's package).
func (c *CUEExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	r, err := c.ExtractDoc(service, relPath, src)
	return r.Nodes, err
}

// cueDecl is one field declaration the walk found.
type cueDecl struct {
	field  *ast.Field
	label  string // its own label (as written, for a pattern or dynamic one)
	name   string // the dotted label path
	static bool   // a static label: an entity
	// tree: the field is a path of the package's value (every field around
	// it static, none reached through a list element, a call, a let or a
	// comprehension's source), so other files of the package may declare the
	// same path, and CUE unifies them
	tree     bool
	node     int // index into the walk's nodes, -1 when not an entity
	parent   *cueDecl
	children []*cueDecl
	calls    map[string]bool
	xrefs    []DocCrossFileRef
}

// cueImport is one import declaration.
type cueImport struct {
	path string // as written, unquoted
	seg  string // the canonical identifier: the path's last element
	std  bool   // the standard library: no first element with a dot
}

// cueWalk is one file's extraction in progress.
type cueWalk struct {
	service, pkg, relPath string
	src                   []byte
	nodes                 []core.SymbolNode
	decls                 []*cueDecl
	byValue               map[ast.Node]*cueDecl // a field's value (alias unwrapped) -> its declaration
	byField               map[*ast.Field]*cueDecl
	byName                map[string][]*cueDecl // static declarations by dotted name
	byStruct              map[*ast.StructLit][]*cueDecl
	imports               map[*ast.ImportSpec]cueImport
	lets                  map[ast.Node]ast.Expr // a let clause -> its value
	offTree               int                   // > 0 inside a container that is no path of the package value
}

// off walks fn in a container whose fields are no path of the package value.
func (w *cueWalk) off(fn func()) {
	w.offTree++
	fn()
	w.offTree--
}

// ExtractDoc implements DocResulter: Extract's entities, the file's package
// entity as its root, and the cross-file references fusion.ResolveDocXrefs
// resolves with the cue-import and cue-package strategies (cuedoc.go).
func (c *CUEExtractor) ExtractDoc(service, relPath string, src []byte) (DocResult, error) {
	return extractCUE(service, relPath, src), nil
}

// extractCUE is one CUE file's extraction (ExtractDoc's body, a function of
// its own so a test can call it directly: Extract and ExtractDoc are method
// names every extractor shares).
func extractCUE(service, relPath string, src []byte) DocResult {
	f, _ := parser.ParseFile(relPath, src)
	if f == nil {
		return DocResult{}
	}
	pkg := f.PackageName()
	if pkg == "" {
		pkg = pkgFromPath(relPath)
	}
	w := &cueWalk{
		service: service, pkg: pkg, relPath: relPath, src: src,
		byValue: map[ast.Node]*cueDecl{}, byField: map[*ast.Field]*cueDecl{}, byName: map[string][]*cueDecl{}, byStruct: map[*ast.StructLit][]*cueDecl{},
		imports: map[*ast.ImportSpec]cueImport{}, lets: map[ast.Node]ast.Expr{},
	}
	var res DocResult
	if name := f.PackageName(); name != "" && name != "_" {
		for _, d := range f.Decls {
			if p, ok := d.(*ast.Package); ok {
				w.nodes = append(w.nodes, core.SymbolNode{
					Service: service, Package: pkg, Name: cuePackagePrefix + name, Kind: kindCUEPackage, FilePath: relPath,
					Line: p.Pos().Line(), EndLine: p.End().Line(), Signature: "package " + name, Tier: core.TierVerified,
				})
				res.Roots = append(res.Roots, cuePackagePrefix+name)
				break
			}
		}
	}
	w.importDecls(f, &res)
	w.declare(f.Decls, nil, 1)
	for _, d := range w.decls {
		if d.static {
			w.references(d.field.Value, d, d)
		}
	}
	for _, d := range w.decls {
		if d.node < 0 || len(d.calls) == 0 {
			continue
		}
		calls := make([]string, 0, len(d.calls))
		for k := range d.calls {
			calls = append(calls, k)
		}
		sort.Strings(calls)
		w.nodes[d.node].Calls = calls
		res.CrossFile = append(res.CrossFile, d.xrefs...)
	}
	if strings.HasSuffix(relPath, "cue.mod/module.cue") {
		w.moduleSignature(f)
	}

	// VS-4 (O3(b)): a testdata/*.cue fixture that references a definition is
	// the CUE analogue of a Go test statically reaching a symbol. See
	// cueTestdataReaches's doc comment for the mechanism.
	w.nodes = append(w.nodes, cueTestdataReaches(service, pkg, relPath, w.nodes)...)

	sort.SliceStable(w.nodes, func(i, j int) bool {
		if w.nodes[i].Line != w.nodes[j].Line {
			return w.nodes[i].Line < w.nodes[j].Line
		}
		return w.nodes[i].Name < w.nodes[j].Name
	})
	sortCUECross(res.CrossFile)
	res.Nodes = w.nodes
	return res
}

// importDecls emits one KindCUEImport per import declaration and its
// cross-file reference to the package it loads.
func (w *cueWalk) importDecls(f *ast.File, res *DocResult) {
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
		// an import path is a string literal; anything else (`import 42`) is
		// no import, and the partial tree gives it no position either
		p := imp.Path.Value
		trimmed := strings.TrimLeft(p, "#")
		if len(trimmed) == 0 || (trimmed[0] != '"' && trimmed[0] != '\'') {
			continue
		}
		p, err := literal.Unquote(p)
		if err != nil || p == "" {
			continue
		}
		w.nodes = append(w.nodes, core.SymbolNode{
			Service: w.service, Package: w.pkg, Name: p, Kind: core.KindCUEImport, FilePath: w.relPath,
			Line: imp.Pos().Line(), EndLine: imp.End().Line(), Tier: core.TierVerified,
		})
		// the package's identifier: a `path:pkg` qualifier's name, else the
		// path's last element
		bare, qual, _ := strings.Cut(p, ":")
		seg := cuepath.Base(bare)
		if qual != "" {
			seg = qual
		}
		ci := cueImport{path: p, seg: seg, std: !strings.Contains(strings.SplitN(bare, "/", 2)[0], ".")}
		w.imports[imp] = ci
		if !ci.std {
			res.CrossFile = append(res.CrossFile, DocCrossFileRef{From: p, Edge: CUEImportsEdge, Raw: p, Strategy: CUEImportStrategy, Line: imp.Pos().Line()})
		}
	}
}

// labelText is a label as written in the source (a pattern's `[string]`, a
// dynamic label's `"\(k)"`), for the names of the fields inside its value.
func (w *cueWalk) labelText(l ast.Label) string {
	start, end := l.Pos().Offset(), l.End().Offset()
	if start >= 0 && end <= len(w.src) && start < end {
		return string(w.src[start:end])
	}
	return "_"
}

// declare walks declarations at one struct level: each field becomes a
// declaration under parent, and every struct literal its value holds is
// walked for its own fields.
func (w *cueWalk) declare(decls []ast.Decl, parent *cueDecl, depth int) {
	if depth > cueMaxDepth {
		return
	}
	for _, d := range decls {
		switch x := d.(type) {
		case *ast.Field:
			name, _, err := ast.LabelName(x.Label)
			static := err == nil && name != ""
			if !static {
				name = w.labelText(x.Label)
			}
			full := name
			if parent != nil {
				full = parent.name + "." + name
			}
			cd := &cueDecl{field: x, label: name, name: full, static: static, node: -1, parent: parent,
				tree: static && w.offTree == 0 && (parent == nil || parent.tree)}
			w.byField[x] = cd
			if static {
				kind := core.KindCUEField
				if strings.HasPrefix(name, "#") || strings.HasPrefix(name, "_#") {
					kind = core.KindCUEDefinition
				}
				cd.node = len(w.nodes)
				w.nodes = append(w.nodes, core.SymbolNode{
					Service: w.service, Package: w.pkg, Name: full, Kind: kind, FilePath: w.relPath,
					Line: x.Pos().Line(), EndLine: x.End().Line(), Tier: core.TierVerified,
				})
			}
			w.decls = append(w.decls, cd)
			if static {
				w.byName[full] = append(w.byName[full], cd)
			}
			if parent != nil {
				parent.children = append(parent.children, cd)
			}
			v := x.Value
			if a, ok := v.(*ast.Alias); ok {
				v = a.Expr
			}
			if v != nil {
				w.byValue[v] = cd
			}
			w.declareIn(x.Value, cd, depth+1)
		case *ast.Comprehension:
			w.comprehension(x, parent, depth)
		case *ast.EmbedDecl:
			w.declareIn(x.Expr, parent, depth)
		case *ast.LetClause:
			w.lets[x] = x.Expr
			w.off(func() { w.declareIn(x.Expr, parent, depth) })
		}
	}
}

func (w *cueWalk) comprehension(x *ast.Comprehension, parent *cueDecl, depth int) {
	for _, cl := range x.Clauses {
		switch c := cl.(type) {
		case *ast.ForClause:
			w.off(func() { w.declareIn(c.Source, parent, depth) })
		case *ast.IfClause:
			w.off(func() { w.declareIn(c.Condition, parent, depth) })
		case *ast.LetClause:
			w.lets[c] = c.Expr
			w.off(func() { w.declareIn(c.Expr, parent, depth) })
		case *ast.TryClause:
			w.off(func() { w.declareIn(c.Expr, parent, depth) })
		}
	}
	w.declareIn(x.Value, parent, depth)
	if x.Fallback != nil && x.Fallback.Body != nil {
		w.declareIn(x.Fallback.Body, parent, depth)
	}
}

// declareIn finds the struct literals of an expression; their fields are
// declared under parent (the field whose value holds them).
func (w *cueWalk) declareIn(e ast.Expr, parent *cueDecl, depth int) {
	if e == nil || depth > cueMaxDepth {
		return
	}
	switch x := e.(type) {
	case *ast.StructLit:
		before := len(w.decls)
		w.declare(x.Elts, parent, depth)
		for _, d := range w.decls[before:] {
			if d.parent == parent {
				w.byStruct[x] = append(w.byStruct[x], d)
			}
		}
	case *ast.ListLit:
		w.off(func() {
			for _, el := range x.Elts {
				if ell, ok := el.(*ast.Ellipsis); ok {
					w.declareIn(ell.Type, parent, depth)
					continue
				}
				w.declareIn(el, parent, depth)
			}
		})
	case *ast.BinaryExpr:
		w.declareIn(x.X, parent, depth)
		w.declareIn(x.Y, parent, depth)
	case *ast.UnaryExpr:
		w.declareIn(x.X, parent, depth)
	case *ast.ParenExpr:
		w.declareIn(x.X, parent, depth)
	case *ast.Alias:
		w.declareIn(x.Expr, parent, depth)
	case *ast.CallExpr:
		w.off(func() {
			w.declareIn(x.Fun, parent, depth)
			for _, a := range x.Args {
				w.declareIn(a, parent, depth)
			}
		})
	case *ast.IndexExpr:
		w.off(func() {
			w.declareIn(x.X, parent, depth)
			w.declareIn(x.Index, parent, depth)
		})
	case *ast.SelectorExpr:
		w.off(func() { w.declareIn(x.X, parent, depth) }) // `(#A & {f: 1}).out` declares f
	case *ast.SliceExpr:
		w.off(func() { w.declareIn(x.X, parent, depth) })
	case *ast.PostfixExpr:
		w.declareIn(x.X, parent, depth)
	case *ast.Interpolation:
		w.off(func() {
			for _, el := range x.Elts {
				w.declareIn(el, parent, depth)
			}
		})
	case *ast.Comprehension:
		w.comprehension(x, parent, depth)
	}
}

// cuePredeclared are CUE's predeclared identifiers: a name the file leaves
// unresolved that is one of these is no reference.
var cuePredeclared = map[string]bool{
	"_": true, "null": true, "true": true, "false": true,
	"bool": true, "string": true, "bytes": true, "rune": true, "number": true, "int": true, "float": true, "uint": true,
	"int8": true, "int16": true, "int32": true, "int64": true, "int128": true,
	"uint8": true, "uint16": true, "uint32": true, "uint64": true, "uint128": true,
	"float32": true, "float64": true, "byte": true,
	"len": true, "close": true, "and": true, "or": true, "div": true, "mod": true, "quo": true, "rem": true,
	"matchN": true, "matchIf": true, "error": true,
}

// references records the references in e for caller (the innermost static
// declaration around them). owner is the declaration whose value is being
// walked: a nested field of owner with a static label is skipped (its own
// value is walked for itself).
func (w *cueWalk) references(e ast.Expr, caller, owner *cueDecl) {
	if e == nil {
		return
	}
	switch x := e.(type) {
	case *ast.Ident:
		w.ident(x, caller)
	case *ast.SelectorExpr, *ast.IndexExpr:
		w.chain(e, caller, owner)
	case *ast.StructLit:
		w.referencesIn(x.Elts, caller, owner)
	case *ast.ListLit:
		for _, el := range x.Elts {
			if ell, ok := el.(*ast.Ellipsis); ok {
				w.references(ell.Type, caller, owner)
				continue
			}
			w.references(el, caller, owner)
		}
	case *ast.BinaryExpr:
		w.references(x.X, caller, owner)
		w.references(x.Y, caller, owner)
	case *ast.UnaryExpr:
		w.references(x.X, caller, owner)
	case *ast.PostfixExpr:
		w.references(x.X, caller, owner)
	case *ast.ParenExpr:
		w.references(x.X, caller, owner)
	case *ast.Alias:
		w.references(x.Expr, caller, owner)
	case *ast.CallExpr:
		w.references(x.Fun, caller, owner)
		for _, a := range x.Args {
			w.references(a, caller, owner)
		}
	case *ast.SliceExpr:
		w.references(x.X, caller, owner)
		w.references(x.Low, caller, owner)
		w.references(x.High, caller, owner)
	case *ast.Interpolation:
		for _, el := range x.Elts {
			w.references(el, caller, owner)
		}
	case *ast.Comprehension:
		w.comprehensionRefs(x, caller, owner)
	}
}

func (w *cueWalk) referencesIn(decls []ast.Decl, caller, owner *cueDecl) {
	for _, d := range decls {
		switch x := d.(type) {
		case *ast.Field:
			if cd := w.declOf(x); cd != nil && cd.static {
				continue // its value is its own
			}
			// a pattern or dynamic field: no entity, so its label's and its
			// value's references belong to the caller
			if _, _, err := ast.LabelName(x.Label); err != nil {
				l := ast.Node(x.Label)
				if a, ok := l.(*ast.Alias); ok {
					l = a.Expr
				}
				if e, ok := l.(ast.Expr); ok {
					w.references(e, caller, owner)
				}
			}
			w.references(x.Value, caller, owner)
		case *ast.Comprehension:
			w.comprehensionRefs(x, caller, owner)
		case *ast.EmbedDecl:
			w.references(x.Expr, caller, owner)
		case *ast.LetClause:
			w.references(x.Expr, caller, owner)
		}
	}
}

func (w *cueWalk) comprehensionRefs(x *ast.Comprehension, caller, owner *cueDecl) {
	for _, cl := range x.Clauses {
		switch c := cl.(type) {
		case *ast.ForClause:
			w.references(c.Source, caller, owner)
		case *ast.IfClause:
			w.references(c.Condition, caller, owner)
		case *ast.LetClause:
			w.references(c.Expr, caller, owner)
		case *ast.TryClause:
			w.references(c.Expr, caller, owner)
		}
	}
	w.references(x.Value, caller, owner)
	if x.Fallback != nil && x.Fallback.Body != nil {
		w.references(x.Fallback.Body, caller, owner)
	}
}

// declOf is the declaration a field node was walked as.
func (w *cueWalk) declOf(f *ast.Field) *cueDecl {
	v := f.Value
	if a, ok := v.(*ast.Alias); ok {
		v = a.Expr
	}
	return w.byField[f]
}

// target is what one step of a reference names: the declarations of this
// file it reaches (decls), else the name it is written as (name: bare, or
// "seg.Name" through an import), and the import it crosses (path).
type cueTarget struct {
	decls    []*cueDecl
	name     string
	path     string // a non-standard import the reference goes through
	below    string // through an import: the name below the package ("#A.b")
	none     bool   // no reference at all (a let, alias or loop variable, the standard library)
	viaValue ast.Expr
}

// joinBelow appends a label to a name below an import's package.
func joinBelow(below, label string) string {
	if below == "" {
		return label
	}
	return below + "." + label
}

func (w *cueWalk) ident(x *ast.Ident, caller *cueDecl) {
	t := w.resolveIdent(x)
	w.record(t, caller, x.Pos().Line())
}

// resolveIdent resolves a bare identifier.
func (w *cueWalk) resolveIdent(x *ast.Ident) cueTarget {
	if x.Node == nil {
		if cuePredeclared[x.Name] || x.Name == "" {
			return cueTarget{none: true}
		}
		return cueTarget{name: x.Name}
	}
	switch n := x.Node.(type) {
	case *ast.ImportSpec:
		return cueTarget{none: true} // a package alias alone names no entity
	case *ast.LetClause:
		return cueTarget{none: true, viaValue: w.lets[n]}
	case *ast.Alias:
		// a value alias (`f: V={...}`): no entity of its own; a selector
		// through it reads the value
		return cueTarget{none: true, viaValue: n.Expr}
	case *ast.Field:
		// a label alias (`X=name: v`) names its field
		if d := w.byField[n]; d != nil && d.static {
			return cueTarget{decls: []*cueDecl{d}}
		}
		return cueTarget{none: true}
	}
	if d := w.byValue[x.Node]; d != nil {
		if !d.static {
			return cueTarget{none: true}
		}
		// the parser binds a name to the last of its declarations in a
		// scope; CUE unifies them all (`netmeta: images: ...` and
		// `netmeta: config: ...`), so a selector reads every one
		return cueTarget{decls: w.byName[d.name]}
	}
	return cueTarget{none: true} // a loop variable, an alias of a value
}

// chain resolves a selector chain `a.b[i].c`, recording a reference for its
// base and for each selector.
func (w *cueWalk) chain(e ast.Expr, caller, owner *cueDecl) {
	// unwind to the base, collecting the selectors in order
	var sels []*ast.SelectorExpr
	base := e
	for {
		switch x := base.(type) {
		case *ast.SelectorExpr:
			sels = append(sels, x)
			base = x.X
			continue
		case *ast.IndexExpr:
			w.references(x.Index, caller, owner)
			base = x.X
			continue
		}
		break
	}
	for i, j := 0, len(sels)-1; i < j; i, j = i+1, j-1 {
		sels[i], sels[j] = sels[j], sels[i]
	}
	var t cueTarget
	switch b := base.(type) {
	case *ast.Ident:
		if spec, ok := b.Node.(*ast.ImportSpec); ok {
			ci, known := w.imports[spec]
			if !known || ci.std {
				return // the standard library: no entity of the subject
			}
			t = cueTarget{path: ci.path, name: ci.seg}
		} else {
			t = w.resolveIdent(b)
			w.record(t, caller, b.Pos().Line())
		}
	default:
		// `(#A & {f: x}).out`: the base's own references, and the selectors
		// read through its value
		w.references(base, caller, owner)
		t = cueTarget{none: true, viaValue: base}
	}
	for i, s := range sels {
		label, _, err := ast.LabelName(s.Sel)
		if err != nil || label == "" {
			return
		}
		t = w.selectStep(t, label, i == 0 && t.path != "")
		w.record(t, caller, s.Sel.Pos().Line())
	}
}

// selectStep is the target `prev.label` names: the declarations of that
// label the struct literals of prev's values write (in this file), else, when
// prev is written by name (another file, an import), or its value unifies with
// a declaration so written (`x: #Remote & {...}`, `(xml.#Marshal & {...})`),
// that name with the label appended.
func (w *cueWalk) selectStep(prev cueTarget, label string, firstThroughImport bool) cueTarget {
	if firstThroughImport {
		return cueTarget{path: prev.path, name: prev.name + "." + label, below: label}
	}
	var exprs []ast.Expr
	for _, d := range prev.decls {
		exprs = append(exprs, d.field.Value)
	}
	if prev.viaValue != nil {
		exprs = append(exprs, prev.viaValue)
	}
	var found []*cueDecl
	seen := map[*cueDecl]bool{}
	for _, e := range exprs {
		for _, d := range w.fieldsOf(e, label, 0) {
			if !seen[d] {
				seen[d] = true
				found = append(found, d)
			}
		}
	}
	if prev.name != "" {
		return cueTarget{decls: found, path: prev.path, name: prev.name + "." + label, below: joinBelow(prev.below, label)}
	}
	// a declaration the value unifies with but this file does not write
	// declares the label too (`(v.#V & {x: 1}).x` is #V.x and the literal's x)
	for _, e := range exprs {
		if o, ok := w.opaqueOperand(e, 0); ok {
			return cueTarget{decls: found, path: o.path, name: o.name + "." + label, below: joinBelow(o.below, label)}
		}
	}
	if len(found) > 0 {
		return cueTarget{decls: found}
	}
	// a path of the package value that this file's struct does not write
	// may be written by another file of the package (`net: config: x` there):
	// named as the path
	for _, d := range prev.decls {
		if d.tree && structish(d.field.Value) {
			return cueTarget{name: d.name + "." + label}
		}
	}
	return cueTarget{none: true}
}

// structish reports whether e is a value other files can add fields to by
// unification: a struct, a reference, or an operator over those (never a
// list, an index, a call, a literal or a comprehension's result).
func structish(e ast.Expr) bool {
	switch x := e.(type) {
	case *ast.StructLit, *ast.Ident, *ast.SelectorExpr:
		return true
	case *ast.BinaryExpr:
		return structish(x.X) || structish(x.Y)
	case *ast.UnaryExpr:
		return structish(x.X)
	case *ast.ParenExpr:
		return structish(x.X)
	case *ast.Alias:
		return structish(x.Expr)
	}
	return false
}

// opaqueOperand is the first operand of e's unification (through
// conjunctions, disjunctions, defaults, parentheses, aliases and references to
// declarations of this file) that names a declaration this file does not
// write: another file's name, or a name through an import.
func (w *cueWalk) opaqueOperand(e ast.Expr, hops int) (cueTarget, bool) {
	if e == nil || hops > 8 {
		return cueTarget{}, false
	}
	switch x := e.(type) {
	case *ast.BinaryExpr:
		if o, ok := w.opaqueOperand(x.X, hops); ok {
			return o, true
		}
		return w.opaqueOperand(x.Y, hops)
	case *ast.UnaryExpr:
		return w.opaqueOperand(x.X, hops)
	case *ast.ParenExpr:
		return w.opaqueOperand(x.X, hops)
	case *ast.Alias:
		return w.opaqueOperand(x.Expr, hops)
	case *ast.Ident:
		t := w.resolveIdent(x)
		if t.name != "" {
			return t, true
		}
		for _, d := range t.decls {
			if o, ok := w.opaqueOperand(d.field.Value, hops+1); ok {
				return o, true
			}
		}
	case *ast.SelectorExpr:
		if id, ok := x.X.(*ast.Ident); ok {
			if spec, ok := id.Node.(*ast.ImportSpec); ok {
				ci, known := w.imports[spec]
				label, _, err := ast.LabelName(x.Sel)
				if known && !ci.std && err == nil && label != "" {
					return cueTarget{path: ci.path, name: ci.seg + "." + label, below: label}, true
				}
			}
		}
	}
	return cueTarget{}, false
}

// fieldsOf finds the static declarations named label that the struct
// literals of e declare directly, looking through conjunctions, disjunctions,
// defaults, parentheses and aliases, and through a reference to a field or
// let of this file (bounded).
func (w *cueWalk) fieldsOf(e ast.Expr, label string, hops int) []*cueDecl {
	if e == nil || hops > 8 {
		return nil
	}
	switch x := e.(type) {
	case *ast.StructLit:
		var out []*cueDecl
		for _, d := range w.byStruct[x] {
			if d.static && d.label == label {
				out = append(out, d)
			}
		}
		return out
	case *ast.BinaryExpr:
		return append(w.fieldsOf(x.X, label, hops), w.fieldsOf(x.Y, label, hops)...)
	case *ast.UnaryExpr:
		return w.fieldsOf(x.X, label, hops)
	case *ast.ParenExpr:
		return w.fieldsOf(x.X, label, hops)
	case *ast.Alias:
		return w.fieldsOf(x.Expr, label, hops)
	case *ast.ListLit:
		var out []*cueDecl
		for _, el := range x.Elts {
			if ell, ok := el.(*ast.Ellipsis); ok {
				out = append(out, w.fieldsOf(ell.Type, label, hops)...)
				continue
			}
			out = append(out, w.fieldsOf(el, label, hops)...)
		}
		return out
	case *ast.Ident:
		t := w.resolveIdent(x)
		var out []*cueDecl
		for _, d := range t.decls {
			out = append(out, w.fieldsOf(d.field.Value, label, hops+1)...)
		}
		if t.viaValue != nil {
			out = append(out, w.fieldsOf(t.viaValue, label, hops+1)...)
		}
		return out
	}
	return nil
}

// record adds one reference target to caller's calls (and, when it leaves the
// file, to its cross-file references).
func (w *cueWalk) record(t cueTarget, caller *cueDecl, line int) {
	if t.none || caller == nil {
		return
	}
	if caller.calls == nil {
		caller.calls = map[string]bool{}
	}
	if len(t.decls) > 0 {
		w.recordDecls(t, caller, line)
	}
	if t.name == "" {
		return
	}
	caller.calls[t.name] = true
	if t.path != "" {
		// "seg.Name…" through an import: the entity is Name… in the package
		// the import loads
		if t.below != "" {
			caller.xrefs = append(caller.xrefs, DocCrossFileRef{From: caller.name, Edge: CUEReferencesEdge, Raw: t.below, Target: t.path, Strategy: CUEImportStrategy, Line: line})
		}
		return
	}
	caller.xrefs = append(caller.xrefs, DocCrossFileRef{From: caller.name, Edge: CUEReferencesEdge, Raw: t.name, Strategy: CUEPackageStrategy, Line: line})
}

// recordDecls adds a reference to declarations of this file.
func (w *cueWalk) recordDecls(t cueTarget, caller *cueDecl, line int) {
	tree := false
	for _, d := range t.decls {
		if d == caller {
			continue
		}
		caller.calls[core.SymbolKey(w.service, w.pkg, d.name)] = true
		tree = tree || d.tree
	}
	if tree {
		// a path of the package value: the package's other files may declare
		// it too (CUE unifies every declaration)
		caller.xrefs = append(caller.xrefs, DocCrossFileRef{From: caller.name, Edge: CUEReferencesEdge, Raw: t.decls[0].name, Strategy: CUEPackageStrategy, Line: line})
	}
}

// moduleSignature signs a cue.mod/module.cue's `module` field with the module
// path it declares, the one value the cue-import strategy reads from the file.
func (w *cueWalk) moduleSignature(f *ast.File) {
	for _, d := range f.Decls {
		field, ok := d.(*ast.Field)
		if !ok {
			continue
		}
		if name, _, err := ast.LabelName(field.Label); err != nil || name != "module" {
			continue
		}
		lit, ok := field.Value.(*ast.BasicLit)
		if !ok {
			return
		}
		v, err := literal.Unquote(lit.Value)
		if err != nil || v == "" {
			return
		}
		for i := range w.nodes {
			if w.nodes[i].Name == "module" && w.nodes[i].Line == field.Pos().Line() {
				w.nodes[i].Signature = v
			}
		}
		return
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
// reference edge (VS-3), cueTestdataReaches synthesizes one sibling node
// named "Test"+the field's own name (stripped of a leading "#"), carrying the
// SAME Calls edges — a caller TestImpact's reverse walk will find, satisfying
// `reachable_from`/`test_reaches` for a CUE plan row with no new plan grammar
// and no change to the reaches machinery itself.
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
