package main

// tests_select_srcread.go — TQ-24 (docs/plans/test-intelligence.md, O783):
// a test that reads another package's source through a literal path
// (filepath.Glob("../store/*.go") then go/parser) never imports that package,
// so a diff touching it selected nothing under --safe. A changed file a
// literal read names stands for the function making the read.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/analysis/impact"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// srcReadStats counts the source reads found in the repo's test files.
type srcReadStats struct {
	Reads    int // calls to a file-reading function
	Literal  int // path folded to a constant
	CrossPkg int // literal paths resolving outside the test's own package directory
	Dynamic  int // path not a constant: ignored
	// LiteralFuncs counts the functions (tests or helpers) holding at least one literal read.
	LiteralFuncs int
}

// srcReadArg reports the index of the path argument of a recognized reader
// (a switch, not a package-level map: the tenancy lint's pkg-map rule).
func srcReadArg(call string) (int, bool) {
	switch call {
	case "os.ReadFile", "os.Open", "os.ReadDir", "filepath.Glob":
		return 0, true
	case "fs.Glob", "parser.ParseFile", "parser.ParseDir":
		return 1, true
	}
	return 0, false
}

// foldSrcPath constant-folds a string literal, a "+" of folded operands, or
// a filepath.Join/path.Join of folded operands.
func foldSrcPath(e ast.Expr) (string, bool) {
	switch v := e.(type) {
	case *ast.BasicLit:
		if v.Kind != token.STRING {
			return "", false
		}
		s, err := strconv.Unquote(v.Value)
		return s, err == nil
	case *ast.ParenExpr:
		return foldSrcPath(v.X)
	case *ast.BinaryExpr:
		if v.Op != token.ADD {
			return "", false
		}
		l, ok1 := foldSrcPath(v.X)
		r, ok2 := foldSrcPath(v.Y)
		return l + r, ok1 && ok2
	case *ast.CallExpr:
		sel, ok := v.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Join" {
			return "", false
		}
		if id, ok := sel.X.(*ast.Ident); !ok || (id.Name != "filepath" && id.Name != "path") {
			return "", false
		}
		parts := make([]string, 0, len(v.Args))
		for _, a := range v.Args {
			s, ok := foldSrcPath(a)
			if !ok {
				return "", false
			}
			parts = append(parts, s)
		}
		return path.Join(filepath.ToSlash(filepath.Join(parts...))), true
	}
	return "", false
}

// srcReadMatches reports whether a changed file is named by the resolved
// literal p: equal, matched as a glob, or under the directory p names.
func srcReadMatches(p, cf string) bool {
	for f := cf; ; f = path.Dir(f) {
		if f == p {
			return true
		}
		if ok, _ := path.Match(p, f); ok {
			return true
		}
		if f == "." || f == "/" || f == "" {
			return false
		}
	}
}

// srcReadChangedSymbols returns the keys of the functions in the repo's
// _test.go files that read a changed file through a literal path, plus the
// census of every read found (changed or not).
func srcReadChangedSymbols(root string, nodes []core.SymbolNode, changedFiles []string) ([]string, srcReadStats) {
	var st srcReadStats
	nodesByFile := map[string][]core.SymbolNode{}
	var testFiles []string
	for _, n := range nodes {
		f := filepath.ToSlash(n.FilePath)
		if !strings.HasSuffix(f, "_test.go") {
			continue
		}
		if _, seen := nodesByFile[f]; !seen {
			testFiles = append(testFiles, f)
		}
		nodesByFile[f] = append(nodesByFile[f], n)
	}
	sort.Strings(testFiles)
	changed := make([]string, 0, len(changedFiles))
	for _, cf := range changedFiles {
		changed = append(changed, path.Clean(filepath.ToSlash(cf)))
	}
	keys := map[string]bool{}
	fset := token.NewFileSet()
	for _, f := range testFiles {
		af, err := parser.ParseFile(fset, filepath.Join(root, filepath.FromSlash(f)), nil, 0)
		if err != nil {
			continue
		}
		dir := path.Dir(f)
		for _, decl := range af.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			hit, lit1 := false, false
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				pk, ok := sel.X.(*ast.Ident)
				if !ok {
					return true
				}
				idx, ok := srcReadArg(pk.Name + "." + sel.Sel.Name)
				if !ok || idx >= len(call.Args) {
					return true
				}
				st.Reads++
				lit, ok := foldSrcPath(call.Args[idx])
				lit = filepath.ToSlash(lit)
				if !ok || path.IsAbs(lit) {
					st.Dynamic++
					return true
				}
				st.Literal++
				lit1 = true
				resolved := path.Join(dir, lit)
				if strings.HasPrefix(lit, "../") || lit == ".." {
					st.CrossPkg++
				}
				for _, cf := range changed {
					if srcReadMatches(resolved, cf) {
						hit = true
					}
				}
				return true
			})
			if lit1 {
				st.LiteralFuncs++
			}
			if !hit {
				continue
			}
			start, end := fset.Position(fd.Pos()).Line, fset.Position(fd.End()).Line
			for _, n := range nodesByFile[f] {
				name := n.Name
				if i := strings.LastIndex(name, "."); i >= 0 {
					name = name[i+1:]
				}
				if name == fd.Name.Name && n.Line >= start && n.Line <= end {
					keys[n.Key()] = true
				}
			}
		}
	}
	out := make([]string, 0, len(keys))
	for k := range keys {
		out = append(out, k)
	}
	sort.Strings(out)
	if len(out) == 0 {
		out = nil
	}
	return out, st
}

// isTestKey reports whether a canonical symbol key names a Test function.
func isTestKey(k string) bool {
	if i := strings.LastIndex(k, "."); i >= 0 {
		k = k[i+1:]
	}
	return strings.HasPrefix(k, "Test")
}

// splitSrcReadSeeds separates the seeds that are helpers (they feed the
// impact walk, which finds the tests reaching them) from the Test functions
// that make the read themselves (the walk skips a Test-named seed, so these
// are added to the selection directly).
func splitSrcReadSeeds(seeds []string) (helpers, tests []string) {
	for _, k := range seeds {
		if isTestKey(k) {
			tests = append(tests, k)
		} else {
			helpers = append(helpers, k)
		}
	}
	return helpers, tests
}

// addSrcReadTests appends a TestReach for each directly-reading test the
// impact walk did not already reach, so it ranks and prints like any other.
func addSrcReadTests(nodes []core.SymbolNode, reach []impact.TestReach, tests []string) []impact.TestReach {
	have := map[string]bool{}
	for _, r := range reach {
		have[r.Test] = true
	}
	byKey := map[string]core.SymbolNode{}
	for _, n := range nodes {
		byKey[n.Key()] = n
	}
	for _, k := range tests {
		if have[k] {
			continue
		}
		n := byKey[k]
		reach = append(reach, impact.TestReach{Test: k, File: n.FilePath, Line: n.Line, Citation: core.CiteRef(n.FilePath, n.Line, n.Line), Reaches: []string{k}})
	}
	sort.Slice(reach, func(i, j int) bool { return reach[i].Test < reach[j].Test })
	return reach
}

// unionSorted is a sorted, de-duplicated union of two key lists.
func unionSorted(a, b []string) []string {
	set := map[string]bool{}
	for _, k := range a {
		set[k] = true
	}
	for _, k := range b {
		set[k] = true
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
