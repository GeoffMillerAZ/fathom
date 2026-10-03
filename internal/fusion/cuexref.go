package fusion

// cuexref.go — the CUE dialect's cross-file strategies for the doc-xref pass
// (LB-Q-cue, docxref.go): cue-import resolves an import path to the files of
// the package it loads, and an entity named below it to its declarations
// there; cue-package resolves a name a file does not declare to its
// declarations in the other files of the same package. The extractor
// (internal/extractor/cuedoc.go) records each reference as written; this file
// resolves it over the set the pass reads, by CUE's own module rules:
//
//   - A module is a directory holding cue.mod/module.cue; its path is that
//     file's `module` field (the extractor signs the field with it), the major
//     version suffix (`@v0`) dropped. A file belongs to the nearest module at
//     or above it; a file under cue.mod/ to the module that holds that cue.mod.
//   - An import path inside the module (the module path, or below it) names
//     the directory at that path under the module root; any other path is
//     looked up in the module's vendored trees, cue.mod/pkg, cue.mod/gen and
//     cue.mod/usr, which CUE reads as one. A path with no module (the standard
//     library) or one served only by a registry resolves to nothing.
//   - A package is the files of that directory, and of its ancestors up to the
//     module root, whose package clause names it: the import path's last
//     element, or its `:name` qualifier.
//
// Nothing here reads a file: the package of a file is its root entity
// ("package/<name>"), the module path the signature of the module file's
// `module` entity.

import (
	"path"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// cueXrefIndex is the set's CUE files by module, directory and package.
type cueXrefIndex struct {
	modules map[string]string          // module root directory -> module path ("." for the repository root)
	pkgOf   map[string]string          // .cue file -> its package name
	byDir   map[string][]string        // directory -> its .cue files with a package clause, sorted
	names   map[string]map[string]bool // file -> its entity names
}

const cueModuleFile = "cue.mod/module.cue"

// newCUEXrefIndex indexes the CUE files of a doc-xref set; nil when the set
// holds none.
func newCUEXrefIndex(files []DocFile) *cueXrefIndex {
	var ix *cueXrefIndex
	for i := range files {
		f := &files[i]
		if !strings.HasSuffix(f.Path, ".cue") {
			continue
		}
		if ix == nil {
			ix = &cueXrefIndex{modules: map[string]string{}, pkgOf: map[string]string{}, byDir: map[string][]string{}, names: map[string]map[string]bool{}}
		}
		names := map[string]bool{}
		for _, n := range f.Result.Nodes {
			names[n.Name] = true
		}
		ix.names[f.Path] = names
		if f.Path == cueModuleFile || strings.HasSuffix(f.Path, "/"+cueModuleFile) {
			for _, n := range f.Result.Nodes {
				if n.Name == "module" && n.Signature != "" {
					mp := n.Signature
					if at := strings.LastIndexByte(mp, '@'); at > 0 {
						mp = mp[:at]
					}
					ix.modules[path.Dir(path.Dir(f.Path))] = mp
				}
			}
			continue
		}
		for _, r := range f.Result.Roots {
			if name, ok := strings.CutPrefix(r, "package/"); ok {
				ix.pkgOf[f.Path] = name
				d := path.Dir(f.Path)
				ix.byDir[d] = append(ix.byDir[d], f.Path)
				break
			}
		}
	}
	if ix != nil {
		for d := range ix.byDir {
			sort.Strings(ix.byDir[d])
		}
	}
	return ix
}

// moduleOf is the module root (and path) a file belongs to.
func (ix *cueXrefIndex) moduleOf(file string) (root, modPath string, ok bool) {
	dir := path.Dir(file)
	if i := strings.Index("/"+dir+"/", "/cue.mod/"); i >= 0 {
		dir = strings.TrimSuffix(("/" + dir + "/")[1:i+1], "/")
		if dir == "" {
			dir = "."
		}
	}
	for {
		if mp, found := ix.modules[dir]; found {
			return dir, mp, true
		}
		if dir == "." || dir == "/" || dir == "" {
			return "", "", false
		}
		dir = path.Dir(dir)
	}
}

// packageFiles is the files of package pkg instantiated at dir: dir's and its
// ancestors' (up to stop, the module root) files of that package.
func (ix *cueXrefIndex) packageFiles(dir, stop, pkg string) []string {
	var out []string
	for {
		for _, f := range ix.byDir[dir] {
			if ix.pkgOf[f] == pkg {
				out = append(out, f)
			}
		}
		if dir == stop || dir == "." || dir == "/" || dir == "" {
			return out
		}
		dir = path.Dir(dir)
	}
}

// importFiles is the files of the package an import path loads, from file.
func (ix *cueXrefIndex) importFiles(file, importPath string) []string {
	root, modPath, ok := ix.moduleOf(file)
	if !ok {
		return nil
	}
	p, pkg, qualified := strings.Cut(importPath, ":")
	if !qualified {
		pkg = path.Base(p)
	}
	if p == "" || pkg == "" {
		return nil
	}
	var dirs []string
	if rest, inModule := strings.CutPrefix(p, modPath); modPath != "" && inModule && (rest == "" || strings.HasPrefix(rest, "/")) {
		dirs = []string{path.Join(root, strings.TrimPrefix(rest, "/"))}
	} else {
		for _, tree := range []string{"cue.mod/pkg", "cue.mod/gen", "cue.mod/usr"} {
			dirs = append(dirs, path.Join(root, tree, p))
		}
	}
	var out []string
	for _, d := range dirs {
		stop := root
		if strings.Contains("/"+d+"/", "/cue.mod/") {
			stop = d // a vendored package is its own directory's
		}
		out = append(out, ix.packageFiles(d, stop, pkg)...)
	}
	sort.Strings(out)
	return out
}

// targets resolves one CUE cross-file reference of file to (file, entity)
// pairs: the package entity of each file an import loads, or each other
// file's declaration of a name.
func (ix *cueXrefIndex) targets(file string, c extractor.DocCrossFileRef) [][2]string {
	if ix == nil {
		return nil
	}
	var files []string
	name := c.Raw
	switch c.Strategy {
	case extractor.CUEImportStrategy:
		if c.Target == "" { // the import entity: the package's files
			for _, f := range ix.importFiles(file, c.Raw) {
				if pkg := ix.pkgOf[f]; pkg != "" && f != file {
					files = append(files, f)
				}
			}
			var out [][2]string
			for _, f := range files {
				out = append(out, [2]string{f, "package/" + ix.pkgOf[f]})
			}
			return out
		}
		files = ix.importFiles(file, c.Target)
	case extractor.CUEPackageStrategy:
		pkg := ix.pkgOf[file]
		if pkg == "" {
			return nil
		}
		stop := path.Dir(file)
		if root, _, ok := ix.moduleOf(file); ok {
			stop = root
		}
		files = ix.packageFiles(path.Dir(file), stop, pkg)
	default:
		return nil
	}
	var out [][2]string
	for _, f := range files {
		if f != file && ix.names[f][name] {
			out = append(out, [2]string{f, name})
		}
	}
	return out
}
