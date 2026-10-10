package fusion

// typed_nestedmodules.go — EW-59 (docs/plans/efficiency-wave.md, O779): the
// typed pass loads every Go module of the repository, not only the root's.
//
// "./..." from the repository root never enters a directory that holds its
// own go.mod, so before this a repository whose binaries live in nested
// modules (kits/*/go.mod, tools/*/go.mod) had typed edges for the root
// module alone. The syntax pass could not make up for it: a call written
// "cmd.NewRoot()" binds only when exactly one package named cmd declares
// NewRoot, and two kits each with an internal/cmd.NewRoot leave every such
// main reaching nothing — `fathom verify` read the whole module
// library-only. Each nested module is now loaded on its own, from its own
// directory, one after another (never concurrently: the daemon's footprint
// is one go/packages load at a time), under the same environment as the
// root load (GOWORK=off, GOTOOLCHAIN=local last — a module whose go
// directive needs a newer toolchain is a disclosed load failure, never a
// toolchain fetch, K22).

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"
)

// goModule is one Go module the typed pass loads.
type goModule struct {
	// rel is the module root's repo-relative, forward-slash directory; "."
	// for the repository root.
	rel string
	// path is the module path its go.mod declares ("" when unreadable).
	path string
	// expected counts the connector-allowed Go files "./..." from the
	// module root would load: zeroPackagesDisclosed's honesty input for
	// this module. 0 when the allowed set is nil (whole-module scope).
	expected int
}

// nested reports whether m is a module below the repository root.
func (m goModule) nested() bool { return m.rel != "." }

// goModFile is m's go.mod, repo-relative ("go.mod" for the root), the name
// a load failure is disclosed under.
func (m goModule) goModFile() string { return path.Join(m.rel, "go.mod") }

// typedStoreKey is the typed-edge store key (the cache and replace
// granularity) of a package this module's load produced. The root module
// keeps the bare go/packages ID every stored row already carries; a nested
// module's IDs are prefixed with its directory, because two nested modules
// may declare the same module path (two vendored copies of one framework)
// and their packages would otherwise overwrite each other's rows and
// fingerprints. The "//" separator never occurs in a clean relative path,
// so one module's prefix is never a prefix of another's.
func (m goModule) typedStoreKey(pkgID string) string {
	if !m.nested() {
		return pkgID
	}
	return m.storeKeyPrefix() + pkgID
}

// storeKeyPrefix is the prefix every typedStoreKey of a nested module
// carries; "" for the root.
func (m goModule) storeKeyPrefix() string {
	if !m.nested() {
		return ""
	}
	return nestedModuleKeyPrefix + m.rel + "//"
}

// nestedModuleKeyPrefix starts every nested module's typed-edge store key.
const nestedModuleKeyPrefix = "gomod:"

// excludedFromPattern reports whether a directory segment is one "./..."
// never enters: testdata, vendor, and names starting with "_" or ".".
func excludedFromPattern(seg string) bool {
	return seg == "testdata" || seg == "vendor" || strings.HasPrefix(seg, "_") || strings.HasPrefix(seg, ".")
}

// typedModules returns the modules the typed pass loads, in load order:
// the root module first when repoPath holds a go.mod, then every nested
// module, sorted by directory.
//
// With an allowed set (the pipeline's Discover result), a nested module is
// one that governs at least one allowed Go file: the nearest go.mod above
// the file, below the root, with no testdata, vendor, "_" or "." segment on
// the way (the directories loadableByRootPattern excludes) — so a module
// whose files the connector ignores is never loaded. With a nil allowed set
// (whole-module scope, for direct callers and tests) every go.mod below
// the root outside those directories counts.
func typedModules(repoPath string, allowed map[string]bool) []goModule {
	var mods []goModule
	if hasGoModule(repoPath) {
		mods = append(mods, goModule{rel: ".", path: goModulePath(filepath.Join(repoPath, "go.mod"))})
	}
	nested := map[string]int{}
	if allowed == nil {
		for _, rel := range walkNestedGoMods(repoPath) {
			nested[rel] = 0
		}
	} else {
		hasMod := map[string]bool{} // repo-relative dir -> holds a go.mod
		modAt := func(dir string) bool {
			v, ok := hasMod[dir]
			if !ok {
				_, err := os.Stat(filepath.Join(repoPath, filepath.FromSlash(dir), "go.mod"))
				v = err == nil
				hasMod[dir] = v
			}
			return v
		}
		for rel := range allowed {
			if !strings.HasSuffix(rel, ".go") {
				continue
			}
			dir := path.Dir(rel)
			if dir == "." {
				continue
			}
			segs := strings.Split(dir, "/")
			skip := false
			for _, sg := range segs {
				if excludedFromPattern(sg) {
					skip = true
					break
				}
			}
			if skip {
				continue
			}
			for i := len(segs); i > 0; i-- {
				if d := strings.Join(segs[:i], "/"); modAt(d) {
					nested[d]++
					break
				}
			}
		}
	}
	rels := make([]string, 0, len(nested))
	for rel := range nested {
		rels = append(rels, rel)
	}
	sort.Strings(rels)
	for _, rel := range rels {
		mods = append(mods, goModule{
			rel:      rel,
			path:     goModulePath(filepath.Join(repoPath, filepath.FromSlash(rel), "go.mod")),
			expected: nested[rel],
		})
	}
	return mods
}

// walkNestedGoMods lists, sorted, the repo-relative directories below
// repoPath that hold a go.mod, skipping the directories "./..." never
// enters.
func walkNestedGoMods(repoPath string) []string {
	var out []string
	_ = filepath.WalkDir(repoPath, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() && p != repoPath {
				return fs.SkipDir
			}
			return nil
		}
		if !d.IsDir() || p == repoPath {
			return nil
		}
		if excludedFromPattern(d.Name()) {
			return fs.SkipDir
		}
		if _, statErr := os.Stat(filepath.Join(p, "go.mod")); statErr == nil {
			out = append(out, repoRelSlash(repoPath, p))
		}
		return nil
	})
	sort.Strings(out)
	return out
}

// goModulePath reads the module path a go.mod declares; "" when the file
// cannot be read or names none.
func goModulePath(gomod string) string {
	data, err := os.ReadFile(gomod) //nolint:gosec — a go.mod inside the repository being ingested
	if err != nil {
		return ""
	}
	return modfile.ModulePath(data)
}

// hasTypedGoModule reports whether the typed pass has a module to load: a
// go.mod at repoPath's root, or a nested module governing an allowed file.
func hasTypedGoModule(repoPath string, allowed map[string]bool) bool {
	return hasGoModule(repoPath) || len(typedModules(repoPath, allowed)) > 0
}

// typedFirstParty decides whether a resolved target's package has
// SymbolNodes in the index: a package this module's load produced, a
// package the root module's load produced, or a package of an in-repo
// nested module — the last two are another module's directory reached
// through a replace directive (=> ../shared, => ./third_party/x). For a
// package outside this load, the declaring position still decides scope:
// a dependency fetched from a module proxy under the same path declares
// outside the repository and is never an edge target.
type typedFirstParty struct {
	loaded map[string]bool // this module's load
	root   map[string]bool // the root module's load (nil before it ran)
	nested []string        // in-repo nested module paths
	memo   map[string]bool
}

func newTypedFirstParty(loaded, root map[string]bool, mods []goModule) *typedFirstParty {
	fp := &typedFirstParty{loaded: loaded, root: root, memo: map[string]bool{}}
	for _, m := range mods {
		if m.nested() && m.path != "" {
			fp.nested = append(fp.nested, m.path)
		}
	}
	return fp
}

// has reports whether pkgPath is first-party.
func (fp *typedFirstParty) has(pkgPath string) bool {
	if fp.loaded[pkgPath] || fp.root[pkgPath] {
		return true
	}
	if v, ok := fp.memo[pkgPath]; ok {
		return v
	}
	v := false
	for _, m := range fp.nested {
		if pkgPath == m || strings.HasPrefix(pkgPath, m+"/") {
			v = true
			break
		}
	}
	fp.memo[pkgPath] = v
	return v
}

// inThisLoad reports whether pkgPath is a package this module's load
// produced (its declaring files are this module's own).
func (fp *typedFirstParty) inThisLoad(pkgPath string) bool { return fp.loaded[pkgPath] }

// outsideRepo reports whether a repoRelSlash result names a path outside
// the repository.
func outsideRepo(rel string) bool {
	return rel == ".." || strings.HasPrefix(rel, "../") || filepath.IsAbs(rel)
}
