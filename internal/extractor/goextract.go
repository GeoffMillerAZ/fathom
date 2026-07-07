package extractor

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"sort"
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
//   - exported var → KindGoVar
//   - exported const → KindGoConst
//
// For func/method nodes, Calls contains the sorted set of callee keys for calls
// that resolve to other symbols within the same service, plus unresolved callee
// names (bare or "pkg.Name") for calls outside the service.
func (g *GoExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, relPath, src, 0)
	if err != nil {
		return nil, err
	}

	pkgName := f.Name.Name

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
			node := g.funcNode(service, pkgName, relPath, fset, d, localFuncs)
			nodes = append(nodes, node)

		case *ast.GenDecl:
			ns := g.genDeclNodes(service, pkgName, relPath, fset, d)
			nodes = append(nodes, ns...)
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
) core.SymbolNode {
	pos := fset.Position(fn.Pos())
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
	calls := collectCalls(service, pkgName, fn, localFuncs)
	complexity := cyclomaticComplexity(fn)

	return core.SymbolNode{
		Service:    service,
		Package:    pkg,
		Name:       name,
		Kind:       kind,
		FilePath:   relPath,
		Line:       pos.Line,
		Signature:  sig,
		Calls:      calls,
		Tier:       core.TierVerified,
		Complexity: complexity,
	}
}

// genDeclNodes builds SymbolNodes for a *ast.GenDecl (type/var/const).
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
				Tier:     core.TierVerified,
			})
		}

	case token.CONST:
		for _, spec := range d.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, name := range vs.Names {
				if name == nil || !ast.IsExported(name.Name) {
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
			for _, name := range vs.Names {
				if name == nil || !ast.IsExported(name.Name) {
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
					Tier:     core.TierVerified,
				})
			}
		}
	}

	return nodes
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
//     looks like a package-qualified call).
//  3. All callee names are deduplicated and sorted.
func collectCalls(service, pkgName string, fn *ast.FuncDecl, localFuncs map[string]bool) []string {
	if fn.Body == nil {
		return nil
	}

	seen := map[string]bool{}

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fun := call.Fun.(type) {
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
				qualified := ident.Name + "." + sel
				seen[qualified] = true
			}
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
