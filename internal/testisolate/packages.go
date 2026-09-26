package testisolate

import (
	"fmt"
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
// console.DaemonRecordPath). Adding a new such default means adding its
// package here.
var ResolverPackages = []string{
	"cmd/fathom",
	"internal/console",
	"internal/custodian",
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
	imports := map[string]map[string]bool{} // dir -> module-relative imports
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
		if imports[rel] == nil {
			imports[rel] = map[string]bool{}
		}
		if strings.HasSuffix(p, "_test.go") {
			hasTests[rel] = true
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
				imports[rel][dep] = true
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	resolver := map[string]bool{}
	for _, r := range ResolverPackages {
		resolver[r] = true
	}
	memo := map[string]bool{}
	visiting := map[string]bool{}
	var reaches func(dir string) bool
	reaches = func(dir string) bool {
		if v, ok := memo[dir]; ok {
			return v
		}
		if resolver[dir] {
			memo[dir] = true
			return true
		}
		if visiting[dir] {
			return false
		}
		visiting[dir] = true
		deps := make([]string, 0, len(imports[dir]))
		for dep := range imports[dir] {
			deps = append(deps, dep)
		}
		sort.Strings(deps)
		out := false
		for _, dep := range deps {
			if reaches(dep) {
				out = true
				break
			}
		}
		visiting[dir] = false
		memo[dir] = out
		return out
	}

	var need []string
	for dir := range hasTests {
		if reaches(dir) {
			need = append(need, dir)
		}
	}
	sort.Strings(need)
	return need, nil
}

// usesEntryPoint reports whether any _test.go file in dir calls Main or
// IsolateAndRun from this package.
func usesEntryPoint(dir string) (bool, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return false, err
		}
		s := string(raw)
		if strings.Contains(s, "testisolate.Main(") || strings.Contains(s, "testisolate.IsolateAndRun(") {
			return true, nil
		}
	}
	return false, nil
}
