package publicloc

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"

	"golang.org/x/tools/go/packages"
)

// gotruth.go holds the go/types walker that used to live in cmd/loctruth
// (bench-pyramid BP-12). It moved here — beside CallEdge, CallersFile and
// QualifiedFuncName, the types it has always produced — because it now has
// two consumers: cmd/loctruth (the public-corpus ground truth, PB-16) and
// internal/indexbench (the L0 index oracle, which scores fathom's own
// answers against it). A walker whose only home is a main package cannot be
// the independent oracle two benches both cite.
//
// Independence is the whole point: this file never opens a fathom store and
// never asks a daemon. A benchmark whose truth comes from the instrument
// under test measures nothing.

// GoTruth is the go/types ground truth for one module tree, in the three
// shapes the benches ask for.
//
// Scope, so each number means one thing: non-test packages only; a call is
// counted when its callee resolves through go/types to a concrete function
// or method (a call through an interface method is dynamic dispatch and is
// not a static caller); a call inside a closure belongs to the enclosing
// declared function; a call in a package-level initializer belongs to
// "init". Every map value is sorted and duplicate-free, so two runs over the
// same tree are byte-identical.
//
// The by-NAME maps aggregate across packages on purpose: fathom's index is
// queried by symbol name (find_callers takes a name, not a package path), so
// name-level truth is the only truth the two are comparable at. In a module
// where two packages both declare New, the truth for "New" is the union.
type GoTruth struct {
	Module             string
	Packages           int
	PackagesWithErrors int

	// Edges is every direct static intra-module call, sorted.
	Edges []CallEdge

	// CallersByName maps a callee's qualified name (Name, or Recv.Method for
	// a concrete method) to the sorted set of qualified names that call it.
	CallersByName map[string][]string

	// DeclsByFile maps a repo-relative file to the sorted set of qualified
	// function names declared in it.
	DeclsByFile map[string][]string

	// UsesByName maps a qualified function name to the sorted set of
	// repo-relative files that mention it — every identifier that resolves
	// to the function through go/types, plus its declaring file. This is the
	// truth the `references` tool is scored against.
	UsesByName map[string][]string
}

// CallersTruth type-checks the module rooted at root and lists every direct
// static call whose callee is a function or method declared in the module
// (docs/plans/bench-public.md PB-16). It is cmd/loctruth's output shape and
// the find_callers ground truth for the public corpus.
func CallersTruth(root string) (CallersFile, error) {
	t, err := LoadGoTruth(root)
	if err != nil {
		return CallersFile{}, err
	}
	return CallersFile{
		Tool:               "cmd/loctruth",
		Module:             t.Module,
		Packages:           t.Packages,
		PackagesWithErrors: t.PackagesWithErrors,
		Edges:              t.Edges,
	}, nil
}

// LoadGoTruth type-checks the module rooted at root once and derives every
// truth shape from that single load. Scope: non-test packages only (see
// GoTruth's doc comment) — this is cmd/loctruth's public-corpus ground
// truth and internal/indexbench's L0 index-oracle truth, both of which
// score a subject's PRODUCTION symbols and must not have a scope wider
// than either's own answers. A caller whose claim is scored against
// answers that correctly include test files (VB-20 receipts — the daemon's
// find_callers/references answers do) wants LoadGoTruthTests instead.
func LoadGoTruth(root string) (GoTruth, error) {
	return loadGoTruth(root, false)
}

// LoadGoTruthTests is LoadGoTruth with test files (`_test.go`, both
// internal and external `_test` packages) included in the load and
// therefore in every truth shape's callers/decls/uses. internal/
// receiptverify is the one consumer: `fathom receipt verify`'s independent
// oracle must cover exactly the scope the live answer it re-derives does
// (docs/bench/readings/vb21-receipts-m11b.md) — the daemon's find_callers
// and references answers correctly include `_test.go` callers and
// occurrences, so the truth a receipt is checked against must too.
func LoadGoTruthTests(root string) (GoTruth, error) {
	return loadGoTruth(root, true)
}

func loadGoTruth(root string, tests bool) (GoTruth, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return GoTruth{}, err
	}
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo | packages.NeedModule,
		Dir: abs,
		// The subject is its own module; a go.work above it (the
		// developer's, never the subject's) must not redirect the load.
		Env:   append(os.Environ(), "GOWORK=off"),
		Tests: tests,
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return GoTruth{}, fmt.Errorf("loading %s: %w", abs, err)
	}
	if len(pkgs) == 0 {
		return GoTruth{}, fmt.Errorf("loading %s: no packages (is there a go.mod?)", abs)
	}
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].PkgPath < pkgs[j].PkgPath })

	out := GoTruth{Packages: len(pkgs)}
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

	callers := map[string]map[string]bool{}
	decls := map[string]map[string]bool{}
	uses := map[string]map[string]bool{}
	seen := map[CallEdge]bool{}

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
					caller = QualifiedFuncName(fd)
					addTo(decls, callerFile, caller)
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
					edge := CallEdge{
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
					addTo(callers, name, caller)
					return true
				})
			}
			// Uses: every identifier in the file that resolves to a
			// module-local function or method. Declarations are added
			// separately — go/types records a declaration in Defs, not Uses.
			for ident, obj := range p.TypesInfo.Uses {
				fn, ok := obj.(*types.Func)
				if !ok || fn.Pkg() == nil || !local[fn.Pkg().Path()] {
					continue
				}
				name, ok := qualifiedTypesName(fn)
				if !ok {
					continue
				}
				addTo(uses, name, rel(abs, p.Fset, ident.Pos()))
			}
		}
	}
	for f, names := range decls {
		for n := range names {
			addTo(uses, n, f)
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
		out.Edges = []CallEdge{}
	}
	out.CallersByName = flatten(callers)
	out.DeclsByFile = flatten(decls)
	out.UsesByName = flatten(uses)
	return out, nil
}

// addTo records v in the set at m[k], creating the set on first use.
func addTo(m map[string]map[string]bool, k, v string) {
	if m[k] == nil {
		m[k] = map[string]bool{}
	}
	m[k][v] = true
}

// flatten turns the set-of-sets into sorted slices — the determinism
// contract (internal/CLAUDE.md) applied to every map this file returns.
func flatten(m map[string]map[string]bool) map[string][]string {
	out := make(map[string][]string, len(m))
	for k, set := range m {
		vals := make([]string, 0, len(set))
		for v := range set {
			vals = append(vals, v)
		}
		sort.Strings(vals)
		out[k] = vals
	}
	return out
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
