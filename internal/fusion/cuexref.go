package fusion

// cuexref.go — the CUE dialect's cross-file strategies for the doc-xref pass
// (LB-Q-cue, docxref.go): cue-import resolves an import path to the files of
// the package it loads, and an entity named below it to its declarations
// there; cue-package resolves a name a file does not declare to its
// declarations in the other files of the same package. The extractor
// (internal/extractor/cuedoc.go) records each reference as written; this file
// resolves it over the set the pass reads, by CUE's own module rules:
//
//   - An import path names the files of the package the cue binary loads for
//     it: the cue-module import strategy's rules (resolve_cuemodule.go,
//     LB-F583), over the same index, so the doc-xref pass and the file
//     resolver load the same files for one import.
//   - A name a file does not declare is looked up in the files of its own
//     package: its directory's, and its ancestors' up to the module root (its
//     directory's alone outside a module), whatever their names.
//
// Nothing here reads a file: the package of a file is its root entity
// ("package/<name>"), the module path the signature of the module file's
// `module` entity.

import (
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// cueXrefIndex is the set's CUE files by module, directory and package (the
// cue-module strategy's index), and each file's entity names.
type cueXrefIndex struct {
	env   *resolveEnv
	cue   *cuePackageIndex
	names map[string]map[string]bool // file -> its entity names
}

// newCUEXrefIndex indexes the CUE files of a doc-xref set; nil when the set
// holds none.
func newCUEXrefIndex(files []DocFile) *cueXrefIndex {
	var paths []string
	var nodes []core.SymbolNode
	names := map[string]map[string]bool{}
	for i := range files {
		f := &files[i]
		if !strings.HasSuffix(f.Path, ".cue") {
			continue
		}
		paths = append(paths, f.Path)
		nodes = append(nodes, f.Result.Nodes...)
		ns := map[string]bool{}
		for _, n := range f.Result.Nodes {
			ns[n.Name] = true
		}
		names[f.Path] = ns
	}
	if len(paths) == 0 {
		return nil
	}
	env := newResolveEnv("", paths, nil)
	env.indexCUEPackages(nodes)
	return &cueXrefIndex{env: env, cue: env.cuePkgs, names: names}
}

// importFiles is the files of the package an import path loads, from file.
func (ix *cueXrefIndex) importFiles(file, importPath string) []string {
	return resolveCUEModule(ix.env, file, importPath)
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
				if pkg := ix.cue.pkgOf[f]; pkg != "" && f != file {
					files = append(files, f)
				}
			}
			var out [][2]string
			for _, f := range files {
				out = append(out, [2]string{f, "package/" + ix.cue.pkgOf[f]})
			}
			return out
		}
		files = ix.importFiles(file, c.Target)
	case extractor.CUEPackageStrategy:
		pkg := ix.cue.pkgOf[file]
		if pkg == "" {
			return nil
		}
		stop := relDir(file)
		if root, _, ok := ix.cue.moduleOf(ix.env, file); ok {
			stop = root
		}
		files = ix.cue.packageFiles(relDir(file), stop, pkg, true)
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
