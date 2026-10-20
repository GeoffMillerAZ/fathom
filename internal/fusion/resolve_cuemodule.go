package fusion

// resolve_cuemodule.go — the cue-module import strategy (LB-F583, design §6's
// closed set; the CUE dialect, LB-Q-cue). A CUE import path names the files of
// the package the cue binary loads for it (cue/load's rules, v0.16.1):
//
//   - The importing file belongs to the nearest module at or above it, a
//     directory holding cue.mod/module.cue (a file under cue.mod/ to the module
//     holding that cue.mod); the module's path is the file's `module` field,
//     a major version suffix (`@v0`) dropped. A file in no module imports
//     nothing.
//   - A path is `path[@version][:name]`; the package name is the qualifier,
//     else the path's last element. A path whose first element has no dot is
//     the standard library and names no file.
//   - A path inside the module (the module path, or below it) names the
//     directory at that path under the module root: its files of the package
//     and, when it has one, those of each ancestor directory up to the module
//     root (CUE adds an ancestor's files of the same package to the instance).
//   - Any other path is looked up in the module's cue.mod/gen, cue.mod/pkg and
//     cue.mod/usr trees, which CUE reads as one, each walked up to its own
//     root. A registry dependency served from the module cache is outside the
//     repository: it names no file, and the cache is never read or fetched.
//   - A file whose name starts with "_" or ".", a _test.cue and a _tool.cue
//     file are no part of an imported package (cue/load leaves them out
//     outside test and cmd mode), and a file with no package clause none.
//
// ".:name" is the package name instantiated at the importing file's own
// directory (the cue binary's own spelling, never valid in an import
// declaration): what the dialect's reader writes as each file's re-export,
// so a reference through an import of a package split across files reaches
// the file defining the name.
//
// The universe's packages are read from its symbols (each file's package
// entity, extractor.CUEPackageOf), once per pass, and its modules from the
// module files' `module` entities (signed with the path), else through the
// env's reader, once per directory. The doc-xref pass's cue-import and
// cue-package strategies (cuexref.go) read the same index, so the two passes
// load the same files for one import.

import (
	"path"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// cueModTrees are the directories under cue.mod a dependency is read from,
// in cue/load's order.
var cueModTrees = [...]string{"gen", "pkg", "usr"}

// cuePackageIndex is the universe's CUE files by directory and package.
type cuePackageIndex struct {
	pkgOf   map[string]string   // file -> its package name
	byDir   map[string][]string // directory ("" the root) -> its files of a package, sorted
	built   map[string]bool     // the files CUE builds into an imported package (cueBuildFile)
	modules map[string]cueModule
}

// cueModule is one directory's module file, read once.
type cueModule struct {
	path  string // the module path, "" when the file declares none
	found bool   // the directory holds cue.mod/module.cue
}

// cueModuleFile is a module's file, below its root directory.
const cueModuleFile = "cue.mod/module.cue"

// indexCUEPackages indexes the pass's CUE package entities and module files.
func (e *resolveEnv) indexCUEPackages(symbols []core.SymbolNode) {
	x := &cuePackageIndex{pkgOf: map[string]string{}, byDir: map[string][]string{}, built: map[string]bool{}, modules: map[string]cueModule{}}
	for _, s := range symbols {
		if !e.set[s.FilePath] {
			continue
		}
		if s.FilePath == cueModuleFile || strings.HasSuffix(s.FilePath, "/"+cueModuleFile) {
			if s.Name == "module" && s.Signature != "" {
				mp := s.Signature
				if at := strings.LastIndexByte(mp, '@'); at > 0 {
					mp = mp[:at]
				}
				x.modules[relDir(relDir(s.FilePath))] = cueModule{path: mp, found: true}
			}
			continue
		}
		pkg, ok := extractor.CUEPackageOf(s)
		if !ok {
			continue
		}
		if _, dup := x.pkgOf[s.FilePath]; dup {
			continue
		}
		x.pkgOf[s.FilePath] = pkg
		x.built[s.FilePath] = cueBuildFile(s.FilePath)
		d := relDir(s.FilePath)
		x.byDir[d] = append(x.byDir[d], s.FilePath)
	}
	for _, fs := range x.byDir {
		sort.Strings(fs)
	}
	e.cuePkgs = x
}

// cueBuildFile reports whether CUE builds file into a package it imports.
func cueBuildFile(file string) bool {
	base := path.Base(file)
	return !strings.HasPrefix(base, "_") && !strings.HasPrefix(base, ".") &&
		!strings.HasSuffix(base, "_test.cue") && !strings.HasSuffix(base, "_tool.cue")
}

// resolveCUEModule resolves one CUE import path written in importer to the
// files of the package it loads (see the file comment).
func resolveCUEModule(env *resolveEnv, importer, module string) []string {
	ix := env.cuePkgs
	if ix == nil {
		return nil
	}
	p, pkg := cueImportParts(module)
	if p == "" || pkg == "" {
		return nil
	}
	root, modPath, inModule := ix.moduleOf(env, importer)
	if p == "." {
		dir := relDir(importer)
		if inModule {
			if rel, ok := cueVendored(root, dir); ok {
				return ix.treeFiles(root, rel, pkg)
			}
			return ix.packageFiles(dir, root, pkg, false)
		}
		return ix.packageFiles(dir, dir, pkg, false)
	}
	first, _, _ := strings.Cut(p, "/")
	if !strings.Contains(first, ".") || !inModule {
		return nil // the standard library; or no module to resolve against
	}
	if rest, ok := strings.CutPrefix(p, modPath); modPath != "" && ok && (rest == "" || strings.HasPrefix(rest, "/")) {
		return ix.packageFiles(joinRel(root, strings.TrimPrefix(rest, "/")), root, pkg, false)
	}
	return ix.treeFiles(root, p, pkg)
}

// cueImportParts splits an import path into its directory path (a version
// dropped) and its package name: the `:name` qualifier, else the path's last
// element when that is an identifier ("" otherwise: nothing to load).
func cueImportParts(module string) (dir, pkg string) {
	p := module
	explicit := false
	if i := strings.LastIndexAny(p, "/:"); i >= 0 && p[i] == ':' {
		p, pkg, explicit = p[:i], p[i+1:], true
		if p == "" {
			p = "."
		}
	}
	if at := strings.LastIndexByte(p, '@'); at > 0 && at < len(p)-1 && !strings.Contains(p[at:], "/") {
		p = p[:at]
	}
	if !explicit {
		pkg = path.Base(p)
		if p == "." || strings.HasPrefix(pkg, "#") || !cueIdent(pkg) {
			pkg = ""
		}
	}
	if pkg == "_" || pkg == "*" {
		pkg = ""
	}
	return p, pkg
}

// cueIdent reports whether s is a CUE identifier (a package name).
func cueIdent(s string) bool {
	if s == "" || s == "_" {
		return false
	}
	for i, r := range s {
		switch {
		case r == '_' || r == '$' || r == '#' && i == 0:
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9' && i > 0:
		default:
			return false
		}
	}
	return true
}

// moduleOf is the module importer belongs to: its root directory and path.
func (ix *cuePackageIndex) moduleOf(env *resolveEnv, importer string) (root, modPath string, ok bool) {
	dir := relDir(importer)
	if i := strings.Index("/"+dir+"/", "/cue.mod/"); i >= 0 {
		dir = cleanRel(dir[:max(0, i-1)])
	}
	for {
		m, seen := ix.modules[dir]
		if !seen {
			mf := joinRel(dir, cueModuleFile)
			b := env.manifest(mf)
			m = cueModule{found: b != nil || env.set[mf]}
			if b != nil {
				m.path = extractor.CUEModulePath(b)
			}
			ix.modules[dir] = m
		}
		if m.found {
			return dir, m.path, true
		}
		if dir == "" {
			return "", "", false
		}
		dir = relDir(dir)
	}
}

// cueVendored reports whether dir is under one of root's cue.mod trees, and
// the import path below the tree it is at.
func cueVendored(root, dir string) (rel string, ok bool) {
	mod := joinRel(root, "cue.mod")
	for _, t := range cueModTrees {
		base := joinRel(mod, t)
		if dir == base {
			return "", true
		}
		if r, found := strings.CutPrefix(dir, base+"/"); found {
			return r, true
		}
	}
	return "", false
}

// treeFiles is the files of package pkg at importPath in root's cue.mod
// trees, read as one.
func (ix *cuePackageIndex) treeFiles(root, importPath, pkg string) []string {
	var out []string
	for _, t := range cueModTrees {
		base := joinRel(root, "cue.mod", t)
		out = append(out, ix.packageFiles(joinRel(base, importPath), base, pkg, false)...)
	}
	return out
}

// packageFiles is the files of package pkg instantiated at dir: dir's own,
// then, when dir has any, each ancestor's up to stop; only the files CUE
// builds into an imported package unless all.
func (ix *cuePackageIndex) packageFiles(dir, stop, pkg string, all bool) []string {
	var out []string
	for first := true; ; first = false {
		n := len(out)
		for _, f := range ix.byDir[dir] {
			if ix.pkgOf[f] == pkg && (all || ix.built[f]) {
				out = append(out, f)
			}
		}
		if first && len(out) == n {
			return nil // no file of the package here: no ancestor is walked
		}
		if dir == stop || dir == "" || !strings.HasPrefix(dir, stop) {
			return out
		}
		dir = relDir(dir)
	}
}

// wholeFile is the file a whole import binds under cue-module: one of the
// package directory's own (the deepest directory among targets), whose
// re-export of its package (".:name") is the whole instance.
func (r *importResolver) wholeFile(targets []string) string {
	if r.strategy != "cue-module" {
		return targets[0]
	}
	best := targets[0]
	for _, t := range targets[1:] {
		if strings.Count(t, "/") > strings.Count(best, "/") {
			best = t
		}
	}
	return best
}

// appendPackageEdges appends, under cue-module, an edge from from (the import
// entity) to the package entity of each file the import loads, the importing
// file itself left out: the import is what reaches every file of the package.
func (idx *resolveIndex) appendPackageEdges(edges []core.Edge, file string, from *core.SymbolNode, targets []string) []core.Edge {
	for _, t := range targets {
		if t == file {
			continue
		}
		for _, s := range idx.byFile[t] {
			if extractor.IsCUEPackageEntity(s) && s.Key() != from.Key() {
				edges = append(edges, core.Edge{FromFile: file, FromSymbol: from.Key(), ToSymbol: s.Key(), Kind: core.EdgeFileResolved})
				break
			}
		}
	}
	return edges
}

// docXrefSupersedes reports whether lang's doc-xref pass stores every edge
// ingest's file-resolved pass would draw for it, so that pass leaves the
// language out and no edge is stored twice (both store EdgeFileResolved).
// CUE (LB-F583): the doc-xref pass's cue-import strategy reads this file's
// index, draws an import's edge to each loaded file's package entity as
// appendPackageEdges does, and binds a name below an import to every
// declaration of it where the file resolver binds a unique top-level one;
// LB-Q-cue's subjects measured it (the reading's note: no true reference
// only the file resolver finds). The harness still grades CUE's L3 through
// ResolveFileEdges and ResolveImportSpecs.
func docXrefSupersedes(lang string) bool { return lang == "cue" }
