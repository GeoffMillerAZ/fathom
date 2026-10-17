package mcp

// safe_delete_typeref.go — O1127: blast_radius kind:"delete" on a Go type
// or interface lists the symbols that USE the type, not only the ones that
// call it. A type written in a signature, a struct field, a composite
// literal or a conversion is in no Calls list (goextract reads function
// bodies only) and is no typed call edge, so the delete answer read those
// uses as absent. They are read here, at query time, from the service's Go
// sources: the bounded ScanSources read references already uses (no table,
// cache or background job), each file that contains the type's name parsed
// with go/parser, each reference attributed to the indexed symbol whose
// span encloses it.

import (
	"bytes"
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/corpus"
)

// goTypeTarget is one Go type a delete question names.
type goTypeTarget struct {
	key  string // the type's own key
	name string // its declared name
	dir  string // its package directory, repo-relative ("." at the root)
	pkg  string // its declared package name
}

// goTypeTargets is the Go types and interfaces symbol names (an ambiguous
// name unions its candidates), sorted by key; nil when it names none.
func goTypeTargets(g *navGraphs, byKey map[string]core.SymbolNode, symbol string) []goTypeTarget {
	if g == nil || g.Both == nil {
		return nil
	}
	var out []goTypeTarget
	for _, k := range callgraph.CandidateKeys(g.Both, symbol) {
		n, ok := byKey[k]
		if !ok || (n.Kind != core.KindGoType && n.Kind != core.KindGoInterface) {
			continue
		}
		out = append(out, goTypeTarget{key: k, name: n.Name, dir: path.Dir(n.FilePath), pkg: n.Package})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].key < out[j].key })
	return out
}

// goTypeUsers returns the keys of the indexed symbols whose span holds a
// reference to one of targets, sorted, and the scan's coverage. The
// targets themselves and their own methods are not users: deleting a type
// deletes them with it.
func goTypeUsers(ctx context.Context, backend QueryInterface, service string, nodes []core.SymbolNode, targets []goTypeTarget) ([]string, corpus.Coverage, error) {
	if len(targets) == 0 {
		return nil, corpus.Coverage{}, nil
	}
	own := map[string]bool{}
	for _, t := range targets {
		own[t.key] = true
	}
	for _, n := range nodes {
		for _, t := range targets {
			if n.Kind == core.KindGoMethod && n.Package == t.name && path.Dir(n.FilePath) == t.dir {
				own[n.Key()] = true
			}
		}
	}
	refLines := map[string][]int{} // file -> lines holding a reference
	b := corpus.Budget{Bytes: referencesScanBytes, Wall: referencesScanWall}
	cov, err := backend.ScanSources(ctx, service, b, func(f core.SourceFile) bool {
		if f.Lang != "go" || !strings.HasSuffix(f.Path, ".go") {
			return false
		}
		mentions := false
		for _, t := range targets {
			if bytes.Contains(f.Content, []byte(t.name)) {
				mentions = true
				break
			}
		}
		if !mentions {
			return false
		}
		if lines := goTypeRefLines(f.Path, f.Content, targets); len(lines) > 0 {
			refLines[f.Path] = lines
		}
		return false
	})
	if err != nil {
		return nil, cov, err
	}
	if len(refLines) == 0 {
		return nil, cov, nil
	}
	byFile := map[string][]core.SymbolNode{}
	for _, n := range nodes {
		if _, ok := refLines[n.FilePath]; ok && isGoDeclKind(n.Kind) {
			byFile[n.FilePath] = append(byFile[n.FilePath], n)
		}
	}
	seen := map[string]bool{}
	var users []string
	for file, lines := range refLines {
		for _, line := range lines {
			n, ok := enclosingNode(byFile[file], line)
			if !ok || own[n.Key()] || seen[n.Key()] {
				continue
			}
			seen[n.Key()] = true
			users = append(users, n.Key())
		}
	}
	sort.Strings(users)
	return users, cov, nil
}

// isGoDeclKind reports whether k is a top-level Go declaration's kind.
func isGoDeclKind(k core.SourceKind) bool {
	switch k {
	case core.KindGoFunc, core.KindGoMethod, core.KindGoType, core.KindGoInterface, core.KindGoVar, core.KindGoConst:
		return true
	}
	return false
}

// enclosingNode is the node of nodes (one file's) whose span holds line,
// the tightest when spans nest (the latest start wins, then the key).
func enclosingNode(nodes []core.SymbolNode, line int) (core.SymbolNode, bool) {
	var best core.SymbolNode
	found := false
	for _, n := range nodes {
		end := n.EndLine
		if end < n.Line {
			end = n.Line
		}
		if line < n.Line || line > end {
			continue
		}
		if !found || n.Line > best.Line || (n.Line == best.Line && n.Key() < best.Key()) {
			best, found = n, true
		}
	}
	return best, found
}

// goTypeRefLines parses one Go file and returns the sorted lines that
// reference a target type: the bare name in a file of the type's own
// package (or through a dot import), pkg.Name through an import of the
// type's directory elsewhere. A name in a declaring position (a field, a
// parameter, a variable, a selector's field or method, a struct literal's
// key, a label) is no reference. Without type information a local
// variable that shadows the type's name is read as the type; a type of a
// nested module's package is matched by its own directory's import path
// suffix only.
func goTypeRefLines(file string, src []byte, targets []goTypeTarget) []int {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, src, parser.SkipObjectResolution)
	if err != nil {
		return nil
	}
	dir := path.Dir(file)
	bare := map[string]bool{}     // names that refer to a target unqualified
	qual := map[string][]string{} // import local name -> target names
	for _, t := range targets {
		if dir == t.dir && f.Name.Name == t.pkg {
			bare[t.name] = true
		}
		if t.dir == "." {
			continue
		}
		for _, imp := range f.Imports {
			p, err := strconv.Unquote(imp.Path.Value)
			if err != nil || (p != t.dir && !strings.HasSuffix(p, "/"+t.dir)) {
				continue
			}
			local := t.pkg
			if imp.Name != nil {
				local = imp.Name.Name
			}
			switch local {
			case "_":
			case ".":
				bare[t.name] = true
			default:
				qual[local] = append(qual[local], t.name)
			}
		}
	}
	if len(bare) == 0 && len(qual) == 0 {
		return nil
	}
	skip := map[*ast.Ident]bool{}
	lineSet := map[int]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.ImportSpec:
			return false
		case *ast.TypeSpec:
			skip[x.Name] = true
		case *ast.FuncDecl:
			skip[x.Name] = true
		case *ast.Field:
			for _, id := range x.Names {
				skip[id] = true
			}
		case *ast.ValueSpec:
			for _, id := range x.Names {
				skip[id] = true
			}
		case *ast.KeyValueExpr:
			if id, ok := x.Key.(*ast.Ident); ok {
				skip[id] = true
			}
		case *ast.AssignStmt:
			if x.Tok == token.DEFINE {
				for _, l := range x.Lhs {
					if id, ok := l.(*ast.Ident); ok {
						skip[id] = true
					}
				}
			}
		case *ast.RangeStmt:
			if x.Tok == token.DEFINE {
				for _, e := range []ast.Expr{x.Key, x.Value} {
					if id, ok := e.(*ast.Ident); ok {
						skip[id] = true
					}
				}
			}
		case *ast.LabeledStmt:
			skip[x.Label] = true
		case *ast.BranchStmt:
			if x.Label != nil {
				skip[x.Label] = true
			}
		case *ast.SelectorExpr:
			skip[x.Sel] = true
			if id, ok := x.X.(*ast.Ident); ok {
				for _, name := range qual[id.Name] {
					if x.Sel.Name == name {
						lineSet[fset.Position(x.Sel.Pos()).Line] = true
					}
				}
			}
		case *ast.Ident:
			if bare[x.Name] && !skip[x] {
				lineSet[fset.Position(x.Pos()).Line] = true
			}
		}
		return true
	})
	lines := make([]int, 0, len(lineSet))
	for l := range lineSet {
		lines = append(lines, l)
	}
	sort.Ints(lines)
	return lines
}
