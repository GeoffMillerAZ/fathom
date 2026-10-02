package fusion

// rustcrate.go — LB-F197 (O287, O288, O289): Rust's crate roots, a `use` path's
// root and its inline modules. Behaviour described from the Rust reference and
// Cargo's target layout (the module tree a crate root declares, `crate` /
// `self` / `super` in a path, a workspace member named by its package or
// [lib] name); nothing is copied from rust-analyzer's hir-def.
//
//   - A crate root is a file Cargo names: a [lib] or [[bin]] `path`, the
//     defaults src/lib.rs, src/main.rs, src/bin/*.rs and src/bin/*/main.rs,
//     and each integration test, example and bench (tests/*.rs, tests/*/main.rs,
//     [[test]] path, ...). A root's child modules sit beside it, not in a
//     directory named for it, and `crate::` from any file of the crate is
//     that root.
//   - A `use` path's first segment is `crate`, `self`, `super`, a child module
//     in scope (uniform paths), or a workspace member's crate name. A bare
//     name in a named or wildcard import (`use a::{S}`, `use super::*`) is
//     therefore a path root, where a bare `mod x;` is a child module file.
//   - `super` inside an inline `mod tests { }` names the file's own module.
//   - A path rooted at none of those is an external crate (std, core, alloc,
//     a registry dependency): out of scope, never an unresolved import.
//
// Nothing here is stored; the root index is built once per resolve.

import (
	"path"
	"regexp"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// rustRoots is the crate-root index of a universe.
type rustRoots struct {
	root     map[string]string   // root file -> its manifest's directory
	libRoot  map[string]string   // manifest directory -> its library root file
	byDir    map[string][]string // manifest directory -> its root files, sorted
	libNames map[string]string   // [lib] name (underscored) -> manifest directory
}

var (
	cargoTargetRe = regexp.MustCompile(`^\[\[?\s*(lib|bin|test|example|bench)\s*\]\]?$`)
	cargoPathRe   = regexp.MustCompile(`^\s*path\s*=\s*"([^"]+)"`)
)

// nearestManifest is the nearest ancestor directory of file holding a
// Cargo.toml, ok false when none does.
func (e *resolveEnv) nearestManifest(file string) (string, bool) {
	for _, a := range ancestors(path.Dir(file)) {
		if e.manifest(joinRel(a, "Cargo.toml")) != nil {
			return a, true
		}
	}
	return "", false
}

// rustRootIndex builds, once, every crate root the manifests of the universe
// name or imply.
func (e *resolveEnv) rustRootIndex() *rustRoots {
	if e.rust != nil {
		return e.rust
	}
	rr := &rustRoots{root: map[string]string{}, libRoot: map[string]string{}, byDir: map[string][]string{}, libNames: map[string]string{}}
	e.rust = rr
	add := func(dir, file string) {
		if file == "" || !e.set[file] {
			return
		}
		if _, dup := rr.root[file]; dup {
			return
		}
		rr.root[file] = dir
		rr.byDir[dir] = append(rr.byDir[dir], file)
	}
	for _, m := range e.manifestsNamed("Cargo.toml") {
		dir := relDir(m)
		kind, name, p := "", "", ""
		flush := func() {
			switch kind {
			case "lib":
				if p == "" {
					p = "src/lib.rs"
				}
				if f := joinRel(dir, p); e.set[f] {
					rr.libRoot[dir] = f
					add(dir, f)
				}
				if name != "" {
					rr.libNames[strings.ReplaceAll(name, "-", "_")] = dir
				}
			case "bin", "test", "example", "bench":
				if p != "" {
					add(dir, joinRel(dir, p))
				} else if name != "" {
					switch kind {
					case "bin":
						add(dir, joinRel(dir, "src/bin", name+".rs"))
						add(dir, joinRel(dir, "src/bin", name, "main.rs"))
					default:
						add(dir, joinRel(dir, kind+"s", name+".rs"))
						add(dir, joinRel(dir, kind+"s", name, "main.rs"))
					}
				}
			}
			kind, name, p = "", "", ""
		}
		for _, line := range strings.Split(string(e.manifest(m)), "\n") {
			t := strings.TrimSpace(line)
			if strings.HasPrefix(t, "[") {
				flush()
				if sm := cargoTargetRe.FindStringSubmatch(t); sm != nil {
					kind = sm[1]
				}
				continue
			}
			if kind == "" {
				continue
			}
			if sm := cargoPathRe.FindStringSubmatch(t); sm != nil {
				p = sm[1]
			} else if sm := cargoNameRe.FindStringSubmatch(t); sm != nil {
				name = sm[1]
			}
		}
		flush()
		// Cargo's default layout.
		add(dir, joinRel(dir, "src/lib.rs"))
		if f := joinRel(dir, "src/lib.rs"); e.set[f] {
			rr.libRoot[dir] = f
		}
		add(dir, joinRel(dir, "src/main.rs"))
	}
	for _, p := range e.paths {
		if !strings.HasSuffix(p, ".rs") {
			continue
		}
		dir, ok := e.nearestManifest(p)
		if !ok {
			continue
		}
		rel := strings.TrimPrefix(p, dir+"/")
		if dir == "" {
			rel = p
		}
		parts := strings.Split(rel, "/")
		switch {
		case len(parts) == 3 && parts[0] == "src" && parts[1] == "bin":
			add(dir, p)
		case len(parts) == 4 && parts[0] == "src" && parts[1] == "bin" && parts[3] == "main.rs":
			add(dir, p)
		case len(parts) == 2 && (parts[0] == "tests" || parts[0] == "examples" || parts[0] == "benches"):
			add(dir, p)
		case len(parts) == 3 && (parts[0] == "tests" || parts[0] == "examples" || parts[0] == "benches") && parts[2] == "main.rs":
			add(dir, p)
		}
	}
	for d := range rr.byDir {
		sortStrings(rr.byDir[d])
	}
	return rr
}

func sortStrings(xs []string) {
	for i := 1; i < len(xs); i++ {
		for j := i; j > 0 && xs[j] < xs[j-1]; j-- {
			xs[j], xs[j-1] = xs[j-1], xs[j]
		}
	}
}

// rustCrateRoot is the root file of the crate holding file ("" when no
// manifest names one): the file itself when it is a root, else the root of
// its manifest nearest above it, the library before a binary, and among
// sibling integration tests the one named for its directory.
func (e *resolveEnv) rustCrateRoot(file string) string {
	rr := e.rustRootIndex()
	if _, ok := rr.root[file]; ok {
		return file
	}
	dir, ok := e.nearestManifest(file)
	if !ok {
		return ""
	}
	fileDir := relDir(file)
	best, bestDepth := "", -1
	for _, c := range rr.byDir[dir] {
		cd := relDir(c)
		if cd != fileDir && !strings.HasPrefix(fileDir+"/", cd+"/") && cd != "" {
			continue
		}
		depth := len(strings.Split(cd, "/"))
		if cd == "" {
			depth = 0
		}
		switch {
		case depth > bestDepth:
			best, bestDepth = c, depth
		case depth == bestDepth && (rr.libRoot[dir] == c || path.Base(c) == path.Base(cd)+".rs") && rr.libRoot[dir] != best:
			best = c
		}
	}
	return best
}

// rustModDirOf is the directory a Rust file's child modules live in: beside a
// crate root, a mod.rs, lib.rs or main.rs; in a directory named for any other
// file.
func (e *resolveEnv) rustModDirOf(file string) string {
	if _, ok := e.rustRootIndex().root[file]; ok {
		return relDir(file)
	}
	return rustModDir(file)
}

// rustLibRootOf is the library root file and source directory of the workspace
// member named name ([package] name or [lib] name), ok false for any other
// name.
func (e *resolveEnv) rustLibRootOf(name string) (file, dir string, ok bool) {
	crateDir, found := e.crateByName()[name]
	if !found {
		crateDir, found = e.rustRootIndex().libNames[name]
	}
	if !found {
		return "", "", false
	}
	if f := e.rustRootIndex().libRoot[crateDir]; f != "" {
		return f, relDir(f), true
	}
	return "", rustSrcDir(crateDir), true
}

// inlineModDepth is how many inline `mod x { }` blocks of file hold line.
func (idx *resolveIndex) inlineModDepth(file string, line int) int {
	n := 0
	for _, s := range idx.byFile[file] {
		if s.Kind == core.KindRustMod && s.Line < line && line <= s.EndLine {
			n++
		}
	}
	return n
}

// resolveRustPath resolves a Rust module path from importer. use marks the
// path of a `use` (a bare first segment is then a path root or a module in
// scope, not the `mod x;` of the declaring file); depth is the inline modules
// around the statement. Segments after the module the path names may be items
// inside it: the deepest file the path names is the answer.
func resolveRustPath(env *resolveEnv, importer, spec string, use bool, depth int) []string {
	if !use && !strings.Contains(spec, "::") {
		return firstFile(env, []string{joinRel(env.rustModDirOf(importer), spec)}, []string{".rs"}, []string{"mod.rs"})
	}
	segs := strings.Split(strings.TrimPrefix(spec, "::"), "::")
	for i := range segs {
		segs[i] = strings.TrimSpace(segs[i])
	}
	var base, rootFile string
	switch segs[0] {
	case "crate":
		rootFile = env.rustCrateRoot(importer)
		if rootFile != "" {
			base = relDir(rootFile)
		} else {
			base = env.crateSrcDir(importer)
		}
		segs = segs[1:]
	case "self":
		base, segs = env.rustModDirOf(importer), segs[1:]
	case "super":
		n := 0
		for n < len(segs) && segs[n] == "super" {
			n++
		}
		segs = segs[n:]
		base = env.rustModDirOf(importer)
		for pops := max(n-depth, 0); pops > 0; pops-- {
			base = relDir(base)
		}
	default:
		if f := firstFile(env, []string{joinRel(env.rustModDirOf(importer), segs[0])}, []string{".rs"}, []string{"mod.rs"}); use && f != nil {
			// a child module in scope: `use util::{X}` after `mod util;`
			base = env.rustModDirOf(importer)
			break
		}
		lib, dir, ok := env.rustLibRootOf(segs[0])
		if !ok {
			return nil // std, core, or an external crate
		}
		base, rootFile, segs = dir, lib, segs[1:]
	}
	if base == "\x00" {
		return nil
	}
	for i := len(segs); i >= 1; i-- {
		if f := firstFile(env, []string{joinRel(append([]string{base}, segs[:i]...)...)}, []string{".rs"}, []string{"mod.rs"}); f != nil {
			return f
		}
	}
	if rootFile != "" {
		return []string{rootFile}
	}
	return rustModuleFile(env, base)
}

// rustExternal reports whether a use of spec from importer names an external
// crate: its path root is no crate/self/super, no child module and no
// workspace member.
func (e *resolveEnv) rustExternal(importer, spec string) bool {
	root := strings.TrimSpace(strings.SplitN(strings.TrimPrefix(spec, "::"), "::", 2)[0])
	switch root {
	case "", "crate", "self", "super":
		return false
	}
	if firstFile(e, []string{joinRel(e.rustModDirOf(importer), root)}, []string{".rs"}, []string{"mod.rs"}) != nil {
		return false
	}
	_, _, ok := e.rustLibRootOf(root)
	return !ok
}

// resolveImport resolves one import spec of importer. Under crate-mod a
// named or wildcard import's bare module (`use a::{S}`, `use super::*`) is a
// path root (resolveRustPath); every other strategy resolves the module text.
func (r *importResolver) resolveImport(importer string, spec extractor.ImportSpec, idx *resolveIndex) []string {
	if r.strategy != "crate-mod" {
		return r.resolve(importer, spec.Module)
	}
	use := spec.Kind != extractor.ImportWhole
	depth := 0
	if idx != nil {
		depth = idx.inlineModDepth(importer, spec.Line)
	}
	return r.resolveRust(importer, spec.Module, use, depth)
}

// resolveRust is resolveRustPath through the resolver's trimming and sorting.
func (r *importResolver) resolveRust(importer, module string, use bool, depth int) []string {
	module = strings.TrimSpace(module)
	if module == "" {
		return nil
	}
	out := resolveRustPath(r.env, importer, module, use, depth)
	if len(out) == 0 {
		return nil
	}
	return dedupSorted(append([]string(nil), out...))
}

// resolveUse resolves the module of a re-export (`pub use inner::T;`): a path
// of a `use`, so under crate-mod its bare first segment is a path root.
func (r *importResolver) resolveUse(importer, module string) []string {
	if r.strategy != "crate-mod" {
		return r.resolve(importer, module)
	}
	return r.resolveRust(importer, module, true, 0)
}
