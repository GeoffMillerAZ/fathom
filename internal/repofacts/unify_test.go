package repofacts

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// retiredRules are the eight product test rules and two vendor
// classifiers of vision 42 section 3.1 (RF-2), by file and the
// declaration that held the rule. After RF-2 each is gone, or a delegate
// whose whole body returns one repofacts call (vendored.Path, which the
// typed pass calls, and routes' isTestPath, which five route readers call).
var retiredRules = []struct{ file, decl string }{
	{"internal/mcp/search_tool.go", "isTestPath"},
	{"internal/mcp/search_tool.go", "searchVendorSegments"},
	{"internal/certify/risk.go", "isTestSymbol"},
	{"internal/certify/risk.go", "isTestFile"},
	{"internal/certify/portfolio.go", "isTestFunc"},
	{"internal/conserve/plane_integrity.go", "looksLikeTestFile"},
	{"internal/analysis/routes/laravel.go", "isTestPath"},
	{"internal/vendored/vendored.go", "Path"},
	{"internal/custodian/audit.go", "vendoredPath"},
}

// ruleScopes are the declarations that applied a retired rule, or held one
// inline (the _test.go gate of ClassifyWith, the Test prefix of the impact
// walks): none may spell a rule of its own again. Declarations beside them
// in the same file that are out of RF-2's scope (custodian's walk skip
// list, conserve's Test-named reach over guarded tests) are not scanned:
// vision 42 D2 leaves directory-walk skip lists as they are, and the other
// Test-prefix readers are counted in O879.
var ruleScopes = []struct{ file, decl string }{
	{"internal/mcp/search_tool.go", "searchFlags"},
	{"internal/certify/risk.go", "RiskWeightedCoverage"},
	{"internal/certify/portfolio.go", "ClassifyTests"},
	{"internal/conserve/plane_integrity.go", "Check"},
	{"internal/analysis/routes/laravel.go", "readLaravel"},
	{"internal/testlayer/classify.go", "ClassifyWith"},
	{"internal/analysis/impact/impact.go", "blastRadiusOn"},
	{"internal/analysis/impact/bounded.go", "BlastRadiusBounded"},
	{"internal/custodian/audit.go", "gitAgentConfigFiles"},
	{"internal/custodian/propose.go", "indexRepo"},
}

// ruleLiterals are the string literals a test or vendor rule is spelled
// with. None may appear as code (comments are free) in a unified file.
var ruleLiterals = map[string]bool{
	"_test.go": true, "_test.py": true, "test_": true, ".test.": true, ".spec.": true, "_spec.rb": true,
	"Test.java": true, "Tests.cs": true, "Test.php": true, "conftest.py": true, "testdata": true,
	"tests": true, "Tests": true, "node_modules": true, "third_party": true, "vendor": true,
}

// TestNoSecondTestOrVendorRuleInProductCode (RF-2): every named site calls
// the one classifier and keeps no copy: no rule literal, no `Test` prefix
// test, and no rule declaration but a one-line repofacts delegate. The
// classifier itself answers what the retired rules answered between them.
func TestNoSecondTestOrVendorRuleInProductCode(t *testing.T) {
	root := filepath.Join("..", "..")
	parsed := map[string]*ast.File{}
	fset := token.NewFileSet()
	parse := func(file string) *ast.File {
		if f, ok := parsed[file]; ok {
			return f
		}
		f, err := parser.ParseFile(fset, filepath.Join(root, file), nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		if !importsPath(f, "github.com/geoffmilleraz/fathom/internal/repofacts") &&
			!importsPath(f, "github.com/geoffmilleraz/fathom/internal/vendored") {
			t.Errorf("%s reaches neither repofacts nor its vendored delegate", file)
		}
		parsed[file] = f
		return f
	}
	for _, r := range retiredRules {
		checkDelegateOrGone(t, r.file, parse(r.file), r.decl)
	}
	for _, sc := range ruleScopes {
		f := parse(sc.file)
		found := false
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Name.Name != sc.decl {
				continue
			}
			found = true
			scanRuleCopies(t, fset, sc.file, fd)
		}
		if !found {
			t.Errorf("%s: %s not found: update ruleScopes with the declaration that applies the rule now", sc.file, sc.decl)
		}
	}

	// The one classifier answers what the retired rules answered.
	tests := []string{"a/b_test.go", "pkg/test_x.py", "pkg/x_test.py", "web/a.test.ts", "web/a.spec.js",
		"spec/models/user_spec.rb", "src/test/java/a/FooTest.java", "Tests/FooTests.cs", "tests/Unit/FooTest.php"}
	for _, p := range tests {
		if !IsTest(p) || !IsTestMaterial(p) {
			t.Errorf("IsTest(%q) = false, want true", p)
		}
	}
	material := []string{"internal/x/testdata/in.txt", "tests/helpers.py", "conftest.py", "test/e2e/util.go", "Tests/Feature/routes.php"}
	for _, p := range material {
		if IsTest(p) || !IsTestMaterial(p) {
			t.Errorf("%q: IsTest %v IsTestMaterial %v, want false true", p, IsTest(p), IsTestMaterial(p))
		}
	}
	for _, p := range []string{"internal/x/x.go", "src/latest.ts", "contest.py", "cmd/fathom/tests_select.go"} {
		if IsTestMaterial(p) {
			t.Errorf("IsTestMaterial(%q) = true, want false", p)
		}
	}
	if !IsTestEntry("TestRole", "internal/repofacts/role_test.go") || IsTestEntry("TestImpact", "internal/analysis/callgraph/graph.go") {
		t.Error("IsTestEntry: a Test-named function counts only in a test file")
	}
	for p, want := range map[string]bool{"vendor/a/b.go": true, "web/node_modules/x/i.js": true, "third_party/k/CLAUDE.md": true,
		"kits/a/third_party/cli": true, "vendor": true, "internal/fusion": false, "": false} {
		if IsVendored(p) != want {
			t.Errorf("IsVendored(%q) = %v, want %v", p, !want, want)
		}
	}
}

// scanRuleCopies fails on a rule literal or a Test-prefix test in n.
func scanRuleCopies(t *testing.T, fset *token.FileSet, file string, n ast.Node) {
	t.Helper()
	ast.Inspect(n, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.BasicLit:
			if x.Kind == token.STRING {
				if v, err := strconv.Unquote(x.Value); err == nil && ruleLiterals[v] {
					t.Errorf("%s: rule literal %q at %s: a second copy of a test or vendor rule", file, v, fset.Position(x.Pos()))
				}
			}
		case *ast.CallExpr:
			if sel, ok := x.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "HasPrefix" && len(x.Args) == 2 {
				if lit, ok := x.Args[1].(*ast.BasicLit); ok && lit.Value == `"Test"` {
					t.Errorf("%s: a Test-prefix test at %s: call repofacts.IsTestEntry", file, fset.Position(x.Pos()))
				}
			}
		}
		return true
	})
}

func importsPath(f *ast.File, p string) bool {
	for _, im := range f.Imports {
		if v, _ := strconv.Unquote(im.Path.Value); v == p {
			return true
		}
	}
	return false
}

// checkDelegateOrGone: decl is absent, or a one-statement func returning a
// repofacts call; a package-level table of that name fails.
func checkDelegateOrGone(t *testing.T, file string, f *ast.File, decl string) {
	t.Helper()
	for _, d := range f.Decls {
		switch x := d.(type) {
		case *ast.FuncDecl:
			if x.Name.Name != decl || x.Recv != nil {
				continue
			}
			if len(x.Body.List) != 1 {
				t.Errorf("%s: %s keeps a rule body of %d statements; delegate to repofacts", file, decl, len(x.Body.List))
				return
			}
			ret, ok := x.Body.List[0].(*ast.ReturnStmt)
			if !ok || len(ret.Results) != 1 || !callsPackage(ret.Results[0], "repofacts") {
				t.Errorf("%s: %s is not a one-line repofacts delegate", file, decl)
			}
			return
		case *ast.GenDecl:
			for _, sp := range x.Specs {
				if vs, ok := sp.(*ast.ValueSpec); ok {
					for _, n := range vs.Names {
						if n.Name == decl {
							t.Errorf("%s: %s still declares a rule table", file, decl)
						}
					}
				}
			}
		}
	}
}

func callsPackage(e ast.Expr, pkg string) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == pkg {
				found = true
			}
		}
		return !found
	})
	return found
}

// TestRepofactsIsAPureLeaf: repofacts imports no fathom package, so fusion
// (through internal/vendored) and every other consumer may import it.
func TestRepofactsIsAPureLeaf(t *testing.T) {
	entries, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), e, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, im := range f.Imports {
			if v, _ := strconv.Unquote(im.Path.Value); strings.HasPrefix(v, "github.com/geoffmilleraz/fathom/") {
				t.Errorf("%s imports %s: repofacts must stay a leaf", e, v)
			}
		}
	}
}
