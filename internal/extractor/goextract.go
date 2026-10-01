package extractor

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// GoExtractor implements Extractor for Go source files using go/parser.
// It is safe for concurrent use once constructed.
type GoExtractor struct{}

// NewGoExtractor returns a new GoExtractor.
func NewGoExtractor() *GoExtractor { return &GoExtractor{} }

// Extract parses a single Go source file and emits one core.SymbolNode for each
// top-level declaration:
//   - func   → KindGoFunc   (Package = file package name)
//   - method → KindGoMethod (Package = receiver type name, Name = "ReceiverType.MethodName")
//   - type struct → KindGoType
//   - type interface → KindGoInterface
//   - var → KindGoVar     (a package-level name, exported or not; never `_`)
//   - const → KindGoConst (likewise)
//
// For func/method nodes, Calls contains the sorted set of callee keys for calls
// that resolve to other symbols within the same service, plus unresolved callee
// names for calls outside the file. Unresolved callees take one of four forms:
//
//   - "Name"                      — bare identifier (builtin, or a same-package
//     symbol declared in another file of the package)
//   - "pkg.Name"                  — selector through an unaliased import (the
//     identifier in source IS the target's declared package name) or through a
//     receiver variable ("h.RunOnce")
//   - "seg.Name"                  — selector through an EXPLICIT import alias,
//     canonicalized to the import path's last segment (the alias itself is
//     file-local and meaningless to any resolver); e.g. a call written
//     `sig.PreconditionSatisfied()` under `import sig ".../evidence-signature"`
//     is recorded as "evidence-signature.PreconditionSatisfied"
//   - "<import-path>#.Method"     — a method call on a receiver variable whose
//     initializer, in the same function body, is a constructor-shaped selector
//     call into an imported package: `l := ladder.New(...)` then `l.Run(...)`
//     records "example.com/kit/.../evidence-ladder#.Run" ALONGSIDE the legacy
//     "l.Run" form. The "#." form carries the constructor package's import
//     path so a resolver can scope the method lookup to that package's
//     directory; the legacy form keeps the global unique-suffix method
//     fallback as the secondary path.
//
// All of these are derived from syntactically-present facts of the single file
// (its AST import table, assignment statements within the same function body) —
// never from type information or cross-file inference.
func (g *GoExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, relPath, src, 0)
	if err != nil {
		return nil, err
	}

	pkgName := f.Name.Name
	imports := importIdents(f)

	// Build index of top-level func/method names defined in this file so we can
	// resolve intra-service call edges.
	// key: simple function name → true (for funcs)
	localFuncs := map[string]bool{}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name == nil {
			continue
		}
		if fn.Recv != nil {
			// method — record under the qualified name too, but for resolution
			// within the same file we index the bare method name.
			localFuncs[fn.Name.Name] = true
		} else {
			localFuncs[fn.Name.Name] = true
		}
	}

	var nodes []core.SymbolNode

	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Name == nil {
				continue
			}
			node := g.funcNode(service, pkgName, relPath, fset, d, localFuncs, imports)
			nodes = append(nodes, node)

		case *ast.GenDecl:
			ns := g.genDeclNodes(service, pkgName, relPath, fset, d)
			nodes = append(nodes, ns...)
		}
	}

	// EW-15: a file no default build compiles still indexes, but is marked,
	// so the product wiring check never takes its main for an entry point.
	if defaultBuildExcludes(src) {
		for i := range nodes {
			nodes[i].ToolKind = core.ToolKindGoBuildExcluded
		}
	}

	// Sort nodes for determinism: by Line, then Name.
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Line != nodes[j].Line {
			return nodes[i].Line < nodes[j].Line
		}
		return nodes[i].Name < nodes[j].Name
	})

	return nodes, nil
}

// funcNode builds a SymbolNode for a *ast.FuncDecl.
func (g *GoExtractor) funcNode(
	service, pkgName, relPath string,
	fset *token.FileSet,
	fn *ast.FuncDecl,
	localFuncs map[string]bool,
	imports map[string]string,
) core.SymbolNode {
	pos := fset.Position(fn.Pos())
	endPos := fset.Position(fn.End())
	name := fn.Name.Name
	pkg := pkgName
	kind := core.KindGoFunc

	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		kind = core.KindGoMethod
		recv := receiverTypeName(fn.Recv.List[0].Type)
		pkg = recv
		name = recv + "." + fn.Name.Name
	}

	sig := renderFuncSignature(fset, fn)
	calls := collectCalls(service, pkgName, fn, localFuncs, imports)
	complexity := cyclomaticComplexity(fn)

	return core.SymbolNode{
		Service:    service,
		Package:    pkg,
		Name:       name,
		Kind:       kind,
		FilePath:   relPath,
		Line:       pos.Line,
		EndLine:    endPos.Line,
		Signature:  sig,
		Calls:      calls,
		Tier:       core.TierVerified,
		Complexity: complexity,
	}
}

// genDeclNodes builds SymbolNodes for a *ast.GenDecl (type/var/const). A const or
// var is a symbol exported or not, as a func and a type are (LB-F60); the blank
// identifier declares nothing and is skipped.
func (g *GoExtractor) genDeclNodes(
	service, pkgName, relPath string,
	fset *token.FileSet,
	d *ast.GenDecl,
) []core.SymbolNode {
	var nodes []core.SymbolNode

	switch d.Tok {
	case token.TYPE:
		for _, spec := range d.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name == nil {
				continue
			}
			pos := fset.Position(ts.Pos())
			endPos := fset.Position(ts.End())
			kind := core.KindGoType
			if _, isIface := ts.Type.(*ast.InterfaceType); isIface {
				kind = core.KindGoInterface
			}
			nodes = append(nodes, core.SymbolNode{
				Service:  service,
				Package:  pkgName,
				Name:     ts.Name.Name,
				Kind:     kind,
				FilePath: relPath,
				Line:     pos.Line,
				EndLine:  endPos.Line,
				Tier:     core.TierVerified,
			})
		}

	case token.CONST:
		for _, spec := range d.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			endPos := fset.Position(vs.End())
			for _, name := range vs.Names {
				if name == nil || name.Name == "_" {
					continue
				}
				pos := fset.Position(name.Pos())
				nodes = append(nodes, core.SymbolNode{
					Service:  service,
					Package:  pkgName,
					Name:     name.Name,
					Kind:     core.KindGoConst,
					FilePath: relPath,
					Line:     pos.Line,
					EndLine:  endPos.Line,
					Tier:     core.TierVerified,
				})
			}
		}

	case token.VAR:
		for _, spec := range d.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			endPos := fset.Position(vs.End())
			for _, name := range vs.Names {
				if name == nil || name.Name == "_" {
					continue
				}
				pos := fset.Position(name.Pos())
				nodes = append(nodes, core.SymbolNode{
					Service:  service,
					Package:  pkgName,
					Name:     name.Name,
					Kind:     core.KindGoVar,
					FilePath: relPath,
					Line:     pos.Line,
					EndLine:  endPos.Line,
					Tier:     core.TierVerified,
				})
			}
		}
	}

	return nodes
}

// importIdents builds the file's import table: the identifier a package is
// referenced by in this file → its import path. Two entries per import class:
//
//   - explicit alias (`import sig "…/evidence-signature"`) → "sig" maps to the
//     path; "_" and "." imports are skipped (they introduce no selector ident).
//   - unaliased import → the path's last segment maps to the path. This is a
//     best-effort identity mapping: the identifier used in source is the
//     target's DECLARED package name, which we cannot see from this file; when
//     it differs from the last segment (e.g. ".../go-cmp" → package cmp) the
//     source ident simply misses this table and the callee is recorded as
//     written — which is already the declared-name form resolvers key on.
//
// If two imports would claim the same identifier the entry is dropped:
// genuine ambiguity resolves to nothing, never last-wins.
func importIdents(f *ast.File) map[string]string {
	idents := map[string]string{}
	dropped := map[string]bool{}
	add := func(ident, path string) {
		if dropped[ident] {
			return
		}
		if prev, ok := idents[ident]; ok && prev != path {
			delete(idents, ident)
			dropped[ident] = true
			return
		}
		idents[ident] = path
	}
	for _, imp := range f.Imports {
		if imp.Path == nil {
			continue
		}
		p, err := strconv.Unquote(imp.Path.Value)
		if err != nil || p == "" {
			continue
		}
		if imp.Name != nil {
			if imp.Name.Name == "_" || imp.Name.Name == "." {
				continue
			}
			add(imp.Name.Name, p)
			continue
		}
		add(path.Base(p), p)
	}
	return idents
}

// receiverTypeName extracts the base type name from a receiver type expression,
// stripping pointer indirection.
func receiverTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return receiverTypeName(t.X)
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr:
		// Generic receiver: Foo[T]
		return receiverTypeName(t.X)
	case *ast.IndexListExpr:
		// Generic receiver with multiple type params: Foo[T, U]
		return receiverTypeName(t.X)
	}
	return ""
}

// renderFuncSignature renders the function signature (without body) using go/format
// so it matches canonical Go style: "func Name(params) results".
func renderFuncSignature(fset *token.FileSet, fn *ast.FuncDecl) string {
	// Clone a minimal FuncDecl to format — we only want signature, not the body.
	clone := &ast.FuncDecl{
		Name: fn.Name,
		Type: fn.Type,
	}
	// go/format.Node requires a complete file; build a synthetic one.
	file := &ast.File{
		Name:  ast.NewIdent("p"),
		Decls: []ast.Decl{clone},
	}
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, file); err != nil {
		// Fallback: just the function name.
		return "func " + fn.Name.Name
	}
	s := buf.String()
	// format.Node wraps in "package p\n\n", strip that prefix.
	s = strings.TrimPrefix(s, "package p\n\n")
	// The result is "func Name(...) ...\n" — strip trailing newline and braces.
	s = strings.TrimRight(s, "\n")
	// Strip empty body "{}" if format appended it.
	s = strings.TrimSuffix(s, " {}")
	s = strings.TrimRight(s, " ")
	return s
}

// cyclomaticComplexity computes the cyclomatic complexity of a function.
// Formula: 1 + count of decision points.
// Decision points counted:
//   - if statement (each *ast.IfStmt)
//   - for statement (each *ast.ForStmt)
//   - range statement (each *ast.RangeStmt with body)
//   - case clause inside switch (each *ast.CaseClause)
//   - comm clause inside select (each *ast.CommClause)
//   - binary logical && operator (each *ast.BinaryExpr with token.LAND)
//   - binary logical || operator (each *ast.BinaryExpr with token.LOR)
func cyclomaticComplexity(fn *ast.FuncDecl) int {
	if fn.Body == nil {
		return 1
	}
	count := 1 // base complexity
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.IfStmt:
			count++
		case *ast.ForStmt:
			count++
		case *ast.RangeStmt:
			count++
		case *ast.CaseClause:
			count++
		case *ast.CommClause:
			count++
		case *ast.BinaryExpr:
			be := n.(*ast.BinaryExpr)
			if be.Op == token.LAND || be.Op == token.LOR {
				count++
			}
		}
		return true
	})
	return count
}

// collectCalls inspects the body of fn and returns a sorted slice of callee keys.
//
// Resolution strategy:
//  1. If the callee name exists in localFuncs (defined in the same file/package),
//     emit core.SymbolKey(service, pkgName, callee).
//  2. For selector expressions (pkg.Func or receiver.Method), emit "pkg.Func" as a
//     bare unresolved key when the selector target is not a local variable (i.e., it
//     looks like a package-qualified call). When the selector base is an EXPLICIT
//     import alias from the file's import table, the callee is recorded under the
//     import path's last segment instead of the file-local alias (see the Extract
//     doc comment). A local variable that shadows an import identifier is an
//     accepted AST-level approximation — scope tracking is out of budget here, and
//     a wrong "seg.Name" form can only fail to resolve (ambiguity → nothing), never
//     invent an edge to an unrelated symbol the way a raw alias string could.
//  3. All callee names are deduplicated and sorted.
//
// VS-6 (docs/plans/verify-spine.md): a function referenced as a VALUE, not
// called at this site — mux.HandleFunc(p, h), a route-table literal
// {p, h}, an http.HandlerFunc(h) conversion, middleware(h) — is recorded
// too, additively, using the same key forms: a package-local ident
// (localFuncs) or a "pkg.Func" import-table selector. Only these two
// precise, already-established-safe forms are recorded in value position —
// unlike call position, an arbitrary bare ident here is usually just a
// plain variable, and the syntax pass has no type information to tell the
// two apart, so recall is deliberately traded for precision (the typed
// pass, which DOES have type information, covers the rest — e.g. a method
// value on a local receiver var). Three positions are excluded because they
// are not references at all: a CallExpr's own Fun (already classified
// above), a composite-literal KEY (a field NAME), and the left side of an
// assignment (the ASSIGNED-TO variable).
func collectCalls(service, pkgName string, fn *ast.FuncDecl, localFuncs map[string]bool, imports map[string]string) []string {
	if fn.Body == nil {
		return nil
	}

	seen := map[string]bool{}

	// skip marks AST nodes the value-ref pass below must not treat as a
	// reference (see the doc comment). Registered at the parent node's
	// visit — ast.Inspect is pre-order, so a child is marked before Inspect
	// itself reaches it.
	skip := map[ast.Expr]bool{}

	// varPkg tracks receiver variables whose initializer is a
	// constructor-shaped selector call into an imported package:
	// `l := ladder.New(...)` maps "l" → the ladder import path. AST-local
	// only: same function body, pre-order traversal (an assignment statement
	// is visited before any later statement that uses the variable). A
	// variable re-assigned from a DIFFERENT package is dropped — genuine
	// ambiguity resolves to nothing, never last-wins.
	varPkg := map[string]string{}
	varPkgDropped := map[string]bool{}

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			for _, lhs := range node.Lhs {
				skip[lhs] = true
			}
			if node.Tok == token.ASSIGN && allBlank(node.Lhs) {
				skipBlankSinkRefs(skip, node.Rhs)
			}
			if len(node.Lhs) != len(node.Rhs) {
				return true
			}
			for i, lhs := range node.Lhs {
				lhsIdent, ok := lhs.(*ast.Ident)
				if !ok || lhsIdent.Name == "_" {
					continue
				}
				rhsCall, ok := node.Rhs[i].(*ast.CallExpr)
				if !ok {
					continue
				}
				sel, ok := rhsCall.Fun.(*ast.SelectorExpr)
				if !ok {
					continue
				}
				pkgIdent, ok := sel.X.(*ast.Ident)
				if !ok {
					continue
				}
				imp, ok := imports[pkgIdent.Name]
				if !ok || varPkgDropped[lhsIdent.Name] {
					continue
				}
				if prev, ok := varPkg[lhsIdent.Name]; ok && prev != imp {
					delete(varPkg, lhsIdent.Name)
					varPkgDropped[lhsIdent.Name] = true
					continue
				}
				varPkg[lhsIdent.Name] = imp
			}
			return true

		case *ast.ValueSpec:
			// `var _ = f` in a body: a blank sink is no reference (EW-15).
			for i, name := range node.Names {
				if name.Name == "_" && i < len(node.Values) && len(node.Names) == len(node.Values) {
					skipBlankSinkRefs(skip, node.Values[i:i+1])
				}
			}
			return true

		case *ast.CompositeLit:
			for _, elt := range node.Elts {
				if kv, ok := elt.(*ast.KeyValueExpr); ok {
					if keyIdent, ok := kv.Key.(*ast.Ident); ok {
						skip[keyIdent] = true
					}
				}
			}
			return true

		case *ast.CallExpr:
			skip[node.Fun] = true
			switch fun := node.Fun.(type) {
			case *ast.Ident:
				// Simple call: Foo()
				name := fun.Name
				if localFuncs[name] {
					key := core.SymbolKey(service, pkgName, name)
					seen[key] = true
				} else {
					// Record bare name for unresolved builtins/other symbols.
					seen[name] = true
				}
			case *ast.SelectorExpr:
				// Selector call: pkg.Func() or receiver.Method()
				sel := fun.Sel.Name
				if ident, ok := fun.X.(*ast.Ident); ok {
					if imp, ok := varPkg[ident.Name]; ok {
						// Method call on a constructor-initialized receiver:
						// record the constructor-scoped form alongside the
						// legacy "<var>.Method" form (checked before the import
						// table — a local variable shadows an import ident).
						seen[imp+"#."+sel] = true
						seen[ident.Name+"."+sel] = true
					} else if imp, ok := imports[ident.Name]; ok {
						// Canonical package identity: the import path's last
						// segment, never the file-local alias.
						seen[path.Base(imp)+"."+sel] = true
					} else {
						seen[ident.Name+"."+sel] = true
					}
				}
			}
			return true

		case *ast.Ident:
			// Value position: only a known local func/method name counts
			// (see the doc comment) — everything else is silently ignored,
			// same as an unresolvable call target would be.
			if !skip[node] && localFuncs[node.Name] {
				seen[core.SymbolKey(service, pkgName, node.Name)] = true
			}
			return true

		case *ast.SelectorExpr:
			// Value position: only an explicit "pkg.Func" import-table
			// selector counts (see the doc comment).
			if !skip[node] {
				if ident, ok := node.X.(*ast.Ident); ok {
					if imp, ok := imports[ident.Name]; ok {
						seen[path.Base(imp)+"."+node.Sel.Name] = true
					}
				}
			}
			return true
		}
		return true
	})

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

// allBlank reports whether every expression is the blank identifier.
func allBlank(exprs []ast.Expr) bool {
	for _, e := range exprs {
		if id, ok := e.(*ast.Ident); !ok || id.Name != "_" {
			return false
		}
	}
	return len(exprs) > 0
}

// skipBlankSinkRefs marks each value that is a bare function reference
// (an identifier or a selector, parentheses aside) assigned to the blank
// identifier: `_ = f` and `var _ = f` keep a name alive for the compiler
// or a linter and never call it, so they are no reference (EW-15). A call
// in the same position (`_ = f()`) still runs and still counts.
func skipBlankSinkRefs(skip map[ast.Expr]bool, values []ast.Expr) {
	for _, v := range values {
		for {
			p, ok := v.(*ast.ParenExpr)
			if !ok {
				break
			}
			v = p.X
		}
		switch v.(type) {
		case *ast.Ident, *ast.SelectorExpr:
			skip[v] = true
		}
	}
}
