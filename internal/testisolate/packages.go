package testisolate

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// ResolverPackages are the module-relative packages that define a default
// reaching the operator: the daemon URL (cmd/fathom's defaultFathomURL,
// custodian.DefaultDaemonURL), the state directory (statedir.Resolve) and
// the call log / daemon record (console.ResolveCallLog,
// console.DaemonRecordPath), and the operator ledger a package appends to
// by default (intent.LedgerPath's ~/.fathom/intent-ledger.jsonl). Not
// listed: internal/scope (DefaultUserPath is resolved only by cmd/fathom's
// main; the package takes explicit paths) and internal/metrics
// (DefaultArmStatePath is only read — and listing it would rope in the
// store and every package above it). Adding a new such default means adding its
// package here.
var ResolverPackages = []string{
	"cmd/fathom",
	"internal/console",
	"internal/custodian",
	"internal/intent",
	"internal/statedir",
}

// skippedDirs are never walked: vendored or mirrored code, fixtures, and
// the gitignored artifact trees.
var skippedDirs = map[string]bool{
	"third_party": true, "testdata": true, "vendor": true, "workspace": true,
	"ref": true, "tmp": true, "node_modules": true,
}

// packagesNeedingIsolation returns, sorted, the module-relative directories
// under root that hold tests and are, or import (transitively, through the
// module's own packages, test imports included), a ResolverPackages entry.
// Only import clauses are parsed; no file body is read past them.
func packagesNeedingIsolation(root, module string) ([]string, error) {
	// A test binary for dir compiles dir's own files and its tests, and
	// only the PRODUCTION files of everything they import: a dependency's
	// tests are not compiled into it. prod is therefore the graph walked
	// transitively (Go forbids cycles in it); a test import is only a
	// root. Test imports make per-directory cycles normal (q_test imports
	// p, p imports q), which is why merging them into one graph was wrong.
	prod := map[string]map[string]bool{} // dir -> module-relative imports of non-test files
	test := map[string]map[string]bool{} // dir -> module-relative imports of _test.go files
	hasTests := map[string]bool{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if p != root && (skippedDirs[name] || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") {
			return nil
		}
		rel, err := filepath.Rel(root, filepath.Dir(p))
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		into := prod
		if strings.HasSuffix(p, "_test.go") {
			hasTests[rel] = true
			into = test
		}
		if into[rel] == nil {
			into[rel] = map[string]bool{}
		}
		f, err := parser.ParseFile(fset, p, nil, parser.ImportsOnly)
		if err != nil {
			return fmt.Errorf("testisolate: parsing imports of %s: %w", p, err)
		}
		for _, is := range f.Imports {
			ip, err := strconv.Unquote(is.Path.Value)
			if err != nil {
				continue
			}
			if dep, ok := strings.CutPrefix(ip, module+"/"); ok {
				into[rel][dep] = true
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// reaching: every package whose production code is, or imports
	// (transitively), a resolver — a fixpoint over the reversed prod
	// graph, so neither map order nor a cycle can change the answer.
	reaching := map[string]bool{}
	for _, r := range ResolverPackages {
		reaching[r] = true
	}
	for changed := true; changed; {
		changed = false
		for dir, deps := range prod {
			if reaching[dir] {
				continue
			}
			for dep := range deps {
				if reaching[dep] {
					reaching[dir], changed = true, true
					break
				}
			}
		}
	}

	var need []string
	for dir := range hasTests {
		hit := reaching[dir]
		for dep := range test[dir] {
			hit = hit || reaching[dep]
		}
		if hit {
			need = append(need, dir)
		}
	}
	sort.Strings(need)
	return need, nil
}

// usesEntryPoint reports whether a _test.go file in dir has a TestMain
// that calls Main or IsolateAndRun from this package (under whatever name
// the file imports it). A mention in a comment or a string does not count.
func usesEntryPoint(dir string) (bool, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	fset := token.NewFileSet()
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, 0)
		if err != nil {
			return false, err
		}
		local := ""
		for _, is := range f.Imports {
			if ip, _ := strconv.Unquote(is.Path.Value); ip == importPath {
				local = "testisolate"
				if is.Name != nil {
					local = is.Name.Name
				}
			}
		}
		if local == "" {
			continue
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Name.Name != "TestMain" || fn.Body == nil {
				continue
			}
			found := false
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return !found
				}
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
					if x, ok := sel.X.(*ast.Ident); ok && x.Name == local && (sel.Sel.Name == "Main" || sel.Sel.Name == "IsolateAndRun") {
						found = true
					}
				}
				return !found
			})
			if found {
				return true, nil
			}
		}
	}
	return false, nil
}

// importPath is this package's import path.
const importPath = "github.com/geoffmilleraz/fathom/internal/testisolate"
