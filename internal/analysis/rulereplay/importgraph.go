package rulereplay

import (
	"fmt"
	"go/build"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/depgraph"
	"github.com/geoffmilleraz/fathom/internal/guardspec"
)

// BuildImportGraphFromDir builds a guardspec.LayerRuleIndex from root's Go
// source via go/parser (package clause + import declarations only — no
// type-checking, no dependency resolution), keyed by the SAME convention
// internal/guardspec/layerrules.BuildIndex uses: a package's repo-relative
// declaring directory ("internal/store"). CrossService is always nil —
// this producer only ever sees one checkout.
//
// Deliberately not go/packages: `go list`/packages.Load needs a module
// whose requires actually resolve (network or a warm module cache) at
// EVERY historical commit it is pointed at, which a 200-commit replay
// cannot assume — a go.mod from months ago can name a version no longer
// fetchable offline. go/parser only reads the commit's own source text.
//
// A production file (not "_test.go", not under a "testdata" directory,
// matching the current GOOS/GOARCH build constraints) that fails to parse
// is skipped and named in warnings rather than failing the whole build —
// reported, never silently swallowed.
func BuildImportGraphFromDir(root string) (idx guardspec.LayerRuleIndex, modulePath string, warnings []string, err error) {
	modulePath, err = readModulePath(root)
	if err != nil {
		return guardspec.LayerRuleIndex{}, "", nil, err
	}

	g := depgraph.NewGraph()
	bctx := build.Default
	bctx.GOOS = runtime.GOOS
	bctx.GOARCH = runtime.GOARCH
	bctx.CgoEnabled = true

	walkErr := filepath.WalkDir(root, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if p == root {
				return nil
			}
			if skipDir(p, d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}

		dir := filepath.Dir(p)
		match, matchErr := bctx.MatchFile(dir, d.Name())
		if matchErr != nil {
			warnings = append(warnings, fmt.Sprintf("%s: build-constraint check failed: %v", relSlash(root, p), matchErr))
			return nil
		}
		if !match {
			return nil
		}

		rel, relErr := filepath.Rel(root, dir)
		if relErr != nil {
			return relErr
		}
		fromDir := filepath.ToSlash(rel)
		g.AddNode(fromDir)

		fset := token.NewFileSet()
		af, perr := parser.ParseFile(fset, p, nil, parser.ImportsOnly)
		if perr != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", relSlash(root, p), perr))
			return nil
		}
		for _, imp := range af.Imports {
			importPath, uqErr := strconv.Unquote(imp.Path.Value)
			if uqErr != nil {
				continue
			}
			toDir, ok := trimModule(importPath, modulePath)
			if !ok || toDir == fromDir {
				continue
			}
			g.AddEdge(fromDir, toDir)
		}
		return nil
	})
	if walkErr != nil {
		return guardspec.LayerRuleIndex{}, "", nil, fmt.Errorf("rulereplay: walking %s: %w", root, walkErr)
	}

	sort.Strings(warnings)
	return guardspec.LayerRuleIndex{PackageEdges: g}, modulePath, warnings, nil
}

// skipDir reports whether the tool should never descend into the
// directory named name at path p: a hidden or Go-tool-ignored directory
// (".git", "_anything", "testdata" — same convention `go build` itself
// uses), a vendor tree, or another Go module's root (its go.mod makes it
// a separate module boundary — never part of THIS module's package tree,
// matching what go/packages would report for "./...").
func skipDir(p, name string) bool {
	if name == "testdata" || name == "vendor" || name == "node_modules" {
		return true
	}
	if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
		return true
	}
	if _, err := os.Stat(filepath.Join(p, "go.mod")); err == nil {
		return true
	}
	return false
}

// readModulePath reads root's own go.mod and returns its module path.
func readModulePath(root string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("rulereplay: no go.mod at %s: %w", root, err)
	}
	modulePath, _, err := depgraph.ParseGoMod(data)
	if err != nil {
		return "", fmt.Errorf("rulereplay: %s/go.mod: %w", root, err)
	}
	return modulePath, nil
}

// trimModule reports importPath's directory relative to modulePath ("."
// for the module root package itself), and false when importPath is not
// local to modulePath (an external dependency) — mirrors
// internal/guardspec/layerrules.trimModule exactly; the two producers
// must agree on this convention (see that package's doc comment), so it
// is deliberately duplicated rather than shared across an internal/
// package boundary that would otherwise have no other reason to exist.
func trimModule(importPath, modulePath string) (string, bool) {
	if importPath == modulePath {
		return ".", true
	}
	prefix := modulePath + "/"
	if !strings.HasPrefix(importPath, prefix) {
		return "", false
	}
	return strings.TrimPrefix(importPath, prefix), true
}

// relSlash returns path relative to root, slash-separated, for warnings —
// falling back to the absolute path on error rather than failing.
func relSlash(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(rel)
}
