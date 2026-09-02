package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"

	"golang.org/x/tools/go/packages"

	"github.com/geoffmilleraz/fathom/internal/contextbench/publicloc"
)

// CallersTruth type-checks the module rooted at root and lists every
// direct static call whose callee is a function or method declared in the
// module (docs/plans/bench-public.md PB-16). It is the find_callers ground
// truth for the public corpus, and it is deliberately independent of
// fathom's own index: a benchmark whose truth comes from the instrument
// under test measures nothing.
//
// Scope, so the number means one thing: non-test packages only; a call is
// counted when its callee resolves through go/types to a concrete function
// or method (a call through an interface method is dynamic dispatch and
// is not a static caller); a call inside a closure belongs to the enclosing
// declared function; a call in a package-level initializer belongs to
// "init". Output is sorted and duplicate-free, so two runs over the same
// tree are byte-identical.
func CallersTruth(root string) (publicloc.CallersFile, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return publicloc.CallersFile{}, err
	}
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo | packages.NeedModule,
		Dir: abs,
		// The subject is its own module; a go.work above it (the
		// developer's, never the subject's) must not redirect the load.
		Env:   append(os.Environ(), "GOWORK=off"),
		Tests: false,
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return publicloc.CallersFile{}, fmt.Errorf("loading %s: %w", abs, err)
	}
	if len(pkgs) == 0 {
		return publicloc.CallersFile{}, fmt.Errorf("loading %s: no packages (is there a go.mod?)", abs)
	}
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].PkgPath < pkgs[j].PkgPath })

	out := publicloc.CallersFile{Tool: "cmd/loctruth", Packages: len(pkgs)}
	local := map[string]bool{}
	for _, p := range pkgs {
		local[p.PkgPath] = true
		if len(p.Errors) > 0 {
			out.PackagesWithErrors++
		}
		if out.Module == "" && p.Module != nil {
			out.Module = p.Module.Path
		}
	}

	seen := map[publicloc.CallEdge]bool{}
	for _, p := range pkgs {
		if p.TypesInfo == nil {
			continue
		}
		for _, file := range p.Syntax {
			callerFile := rel(abs, p.Fset, file.Pos())
			for _, decl := range file.Decls {
				caller := "init"
				var body ast.Node = decl
				if fd, ok := decl.(*ast.FuncDecl); ok {
					caller = publicloc.QualifiedFuncName(fd)
					if fd.Body == nil {
						continue
					}
					body = fd.Body
				}
				ast.Inspect(body, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					fn := calleeFunc(p.TypesInfo, call.Fun)
					if fn == nil || fn.Pkg() == nil || !local[fn.Pkg().Path()] {
						return true
					}
					name, ok := qualifiedTypesName(fn)
					if !ok {
						return true
					}
					edge := publicloc.CallEdge{
						CalleeFile: rel(abs, p.Fset, fn.Pos()),
						Callee:     name,
						CallerFile: callerFile,
						Caller:     caller,
						Line:       p.Fset.Position(call.Lparen).Line,
					}
					if !seen[edge] {
						seen[edge] = true
						out.Edges = append(out.Edges, edge)
					}
					return true
				})
			}
		}
	}
	sort.Slice(out.Edges, func(i, j int) bool {
		a, b := out.Edges[i], out.Edges[j]
		switch {
		case a.CalleeFile != b.CalleeFile:
			return a.CalleeFile < b.CalleeFile
		case a.Callee != b.Callee:
			return a.Callee < b.Callee
		case a.CallerFile != b.CallerFile:
			return a.CallerFile < b.CallerFile
		case a.Line != b.Line:
			return a.Line < b.Line
		default:
			return a.Caller < b.Caller
		}
	})
	if out.Edges == nil {
		out.Edges = []publicloc.CallEdge{}
	}
	return out, nil
}

// calleeFunc resolves the callee of a call expression to its declared
// *types.Func, or nil when the call is not a direct static call to one
// (a func value, a conversion, a builtin, an unresolved name).
func calleeFunc(info *types.Info, fun ast.Expr) *types.Func {
	switch e := fun.(type) {
	case *ast.ParenExpr:
		return calleeFunc(info, e.X)
	case *ast.IndexExpr:
		return calleeFunc(info, e.X)
	case *ast.IndexListExpr:
		return calleeFunc(info, e.X)
	case *ast.Ident:
		if fn, ok := info.Uses[e].(*types.Func); ok {
			return fn.Origin()
		}
	case *ast.SelectorExpr:
		if fn, ok := info.Uses[e.Sel].(*types.Func); ok {
			return fn.Origin()
		}
	}
	return nil
}

// qualifiedTypesName is Recv.Method for a concrete method, Name for a
// function, and not-ok for an interface method (dynamic dispatch).
func qualifiedTypesName(fn *types.Func) (string, bool) {
	sig, ok := fn.Type().(*types.Signature)
	if !ok {
		return "", false
	}
	recv := sig.Recv()
	if recv == nil {
		return fn.Name(), true
	}
	t := types.Unalias(recv.Type())
	if ptr, ok := t.(*types.Pointer); ok {
		t = types.Unalias(ptr.Elem())
	}
	named, ok := t.(*types.Named)
	if !ok {
		return "", false
	}
	if _, isIface := named.Underlying().(*types.Interface); isIface {
		return "", false
	}
	return named.Obj().Name() + "." + fn.Name(), true
}

func rel(root string, fset *token.FileSet, pos token.Pos) string {
	p := fset.Position(pos).Filename
	if r, err := filepath.Rel(root, p); err == nil {
		return filepath.ToSlash(r)
	}
	return filepath.ToSlash(p)
}
