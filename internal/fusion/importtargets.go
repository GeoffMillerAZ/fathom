package fusion

// importtargets.go — LB-F40 (docs/design/language-breadth.md §1.2,
// import_resolution): the file resolver's module resolution on its own.
// ResolveImportSpecs parses every file's imports and resolves each spec to
// files by the language's manifest-selected strategy (resolvestrategy.go),
// over the same universe and repo-manifest reader ResolveFileEdges uses, and
// reports the file(s) the spec binds — no call is matched, no edge is drawn
// and nothing is stored. The language harness (cmd/indexbench lang-oracle)
// scores the result against an oracle's import references.

import (
	"context"
	"fmt"
	"sort"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// ResolvedImport is one import spec of one file and the files of the
// universe its strategy resolves it to.
type ResolvedImport struct {
	File string
	Spec extractor.ImportSpec
	// StartLine and EndLine span the statement that binds the spec
	// (extractor.ImportSpan), 1-based and inclusive.
	StartLine, EndLine int
	// Targets are sorted, never the importing file, and empty when the
	// spec resolves to no file of the universe (an import from outside the
	// repository, or one the strategy cannot follow).
	Targets []string
	// External marks a spec whose module path is rooted at a crate outside
	// the universe (std, core, a registry dependency; crate-mod only,
	// LB-F197): no file of the repository could be its target, so a scorer
	// leaves it out of scope instead of counting it unresolved.
	External bool
}

// ResolveImportSpecs resolves every import spec of files, in file order
// and, within a file, in extraction order. lang is the TSExtractor registry
// key ResolveFileEdges takes; files is the resolution universe; symbols are
// those files' extracted symbols (a named import binds the file that
// defines its name at top level, and a namespace-dir wildcard the files
// declaring its namespace — ResolveFileEdges' own rules).
//
// What a spec resolves to follows the bindings resolveImports makes:
//   - a named import: the file that defines the name at the top level of
//     one of the module's files, or that one of them re-exports it from
//     (LB-F75: with the module's files, which the statement also names); else, where the strategy has a submodule form, the one file
//     `Module<sep>Name` names (`from pkg import mod`); else the module's
//     files; else (namespace-dir, LB-F78) the file of the package or class
//     the module path less its last segment names that defines the name,
//     or all of that package's files;
//   - a whole-module import: the files declaring the type an alias names
//     (namespace-dir: C#'s `using A = N.T;`), else the module's files, else
//     the files declaring the namespace an alias names (`using A = N;`);
//   - a wildcard: the files declaring the namespace (namespace-dir), else
//     the files declaring the type (C#'s `using static N.T;`), else the
//     module's files.
//
// ctx is accepted for parity with ResolveFileEdges; nothing blocks on it.
// Unlike ResolveFileEdges it runs whether or not the repository carries a
// SCIP index: it measures the strategy and writes nothing.
func ResolveImportSpecs(ctx context.Context, lang, repoPath string, files []core.SourceFile, symbols []core.SymbolNode) ([]ResolvedImport, FileResolveReport) {
	_ = ctx
	ext, err := extractor.NewFileSpecExtractor(lang)
	if err != nil {
		return nil, FileResolveReport{Skipped: true, SkipReason: err.Error()}
	}
	defer ext.Close() // RT-107: its grammar modules are released, never leaked
	paths := make([]string, 0, len(files))
	for _, f := range files {
		paths = append(paths, f.Path)
	}
	r, ok := newImportResolver(lang, repoPath, paths, diskManifestReader(repoPath))
	if !ok {
		return nil, FileResolveReport{Skipped: true, SkipReason: fmt.Sprintf("dialect %q names no import strategy", lang)}
	}
	fileSet := make(map[string]bool, len(paths))
	for _, p := range paths {
		fileSet[p] = true
	}
	idx := newResolveIndex(r, symbols, fileSet)

	var report FileResolveReport
	var faults []string
	var out []ResolvedImport
	spansOf := make(map[string][]extractor.ImportSpan, len(files))
	specs := make(map[string]extractor.FileSpecs, len(files))
	for _, f := range files {
		spans, ierr := ext.ExtractImportSpans(f.Path, f.Content)
		if ierr != nil {
			faults = append(faults, f.Path) // counted and named, never a silent skip
			continue
		}
		spansOf[f.Path] = spans
		// The file's re-exports (LB-F75): a name imported through a barrel
		// or a package's __init__ binds the file defining it.
		if fs, err := ext.ExtractFileSpecs(f.Path, f.Content); err == nil {
			specs[f.Path] = fs
		}
	}
	x := newExportIndex(r, specs, idx.topLevel)
	for _, f := range files {
		spans, ok := spansOf[f.Path]
		if !ok {
			continue
		}
		report.FilesScanned++
		report.ImportsParsed += len(spans)
		for _, sp := range spans {
			ri := ResolvedImport{
				File: f.Path, Spec: sp.ImportSpec, StartLine: sp.StartLine, EndLine: sp.EndLine,
				Targets: r.specTargets(f.Path, sp.ImportSpec, idx, x),
			}
			ri.External = len(ri.Targets) == 0 && r.strategy == "crate-mod" && r.env.rustExternal(f.Path, sp.ImportSpec.Module)
			out = append(out, ri)
		}
	}
	report.Faults = len(faults)
	report.FaultFiles = firstN(faults, 5)
	report.AncestorIncludes = r.env.ancestorIncludes()
	report.CompileDBs = len(r.env.compDatabase().names)
	return out, report
}

// specTargets is the files spec binds in importer (ResolveImportSpecs'
// rules), sorted, without the importer itself.
func (r *importResolver) specTargets(importer string, spec extractor.ImportSpec, idx *resolveIndex, x *exportIndex) []string {
	keyFile, declared, types := idx.keyFile, idx.declared, idx.types
	var targets []string
	switch spec.Kind {
	case extractor.ImportNamed:
		module := r.resolveImport(importer, spec, idx)
		if key, ok := lookupExported(module, spec.ExportedName, x); ok {
			// A re-exported name binds its defining file and still names
			// the module it is imported from (LB-F75): both are what the
			// statement reaches.
			targets = append([]string{keyFile[key]}, module...)
		} else if sep := r.submoduleSep(); sep != "" && spec.ExportedName != "" {
			if sub := r.resolve(importer, spec.Module+sep+spec.ExportedName); len(sub) == 1 {
				targets = sub
			}
		}
		if len(targets) == 0 {
			targets = module
		}
		if len(targets) == 0 {
			// LB-F78: the package or class the path less its last
			// segment names, the file defining the name when one does.
			if key, ok := idx.memberImport(r, importer, spec); ok {
				targets = []string{keyFile[key]}
			} else {
				targets = idx.memberImportFiles(r, importer, spec)
			}
		}
	case extractor.ImportWhole:
		switch keys := r.aliasedType(types, spec); {
		case r.wholeIsWildcard():
			targets = r.resolveImport(importer, spec, idx)
		case len(keys) > 0:
			targets = filesOf(keys, keyFile)
		default:
			targets = r.wholeTargets(importer, spec.Module, declared)
		}
	case extractor.ImportWildcard:
		if fs := r.declaredFiles(declared, spec.Module); len(fs) > 0 {
			targets = fs
		} else if keys := r.typeKeys(types, spec.Module); len(keys) > 0 {
			targets = filesOf(keys, keyFile)
		} else {
			targets = r.resolveImport(importer, spec, idx)
		}
	}
	seen := map[string]bool{importer: true}
	var out []string
	for _, t := range targets {
		if t != "" && !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	sort.Strings(out)
	return out
}
