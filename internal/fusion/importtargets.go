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
	"strings"

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
//     one of the module's files; else, where the strategy has a submodule
//     form, the one file `Module<sep>Name` names (`from pkg import mod`);
//     else the module's files;
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
	ext, err := extractor.NewTSExtractor(lang)
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
	topLevel, keyFile, declared := importScopes(symbols, fileSet)
	types := r.declaredTypes(symbols, fileSet)

	var report FileResolveReport
	var faults []string
	var out []ResolvedImport
	for _, f := range files {
		spans, ierr := ext.ExtractImportSpans(f.Path, f.Content)
		if ierr != nil {
			faults = append(faults, f.Path) // counted and named, never a silent skip
			continue
		}
		report.FilesScanned++
		report.ImportsParsed += len(spans)
		for _, sp := range spans {
			out = append(out, ResolvedImport{
				File: f.Path, Spec: sp.ImportSpec, StartLine: sp.StartLine, EndLine: sp.EndLine,
				Targets: r.specTargets(f.Path, sp.ImportSpec, topLevel, keyFile, declared, types),
			})
		}
	}
	report.Faults = len(faults)
	report.FaultFiles = firstN(faults, 5)
	return out, report
}

// importScopes indexes symbols as resolveImports does: each file's
// top-level (undotted) names to their keys, each key's file, and the files
// declaring each package or namespace (fileSet's only).
func importScopes(symbols []core.SymbolNode, fileSet map[string]bool) (topLevel map[string]map[string]string, keyFile map[string]string, declared map[string][]string) {
	topLevel = map[string]map[string]string{}
	keyFile = make(map[string]string, len(symbols))
	declared = map[string][]string{}
	for _, s := range symbols {
		keyFile[s.Key()] = s.FilePath
		if s.Package != "" && fileSet[s.FilePath] {
			if fs := declared[s.Package]; len(fs) == 0 || fs[len(fs)-1] != s.FilePath {
				declared[s.Package] = append(fs, s.FilePath)
			}
		}
		if strings.Contains(s.Name, ".") {
			continue
		}
		m := topLevel[s.FilePath]
		if m == nil {
			m = map[string]string{}
			topLevel[s.FilePath] = m
		}
		m[s.Name] = s.Key()
	}
	return topLevel, keyFile, declared
}

// specTargets is the files spec binds in importer (ResolveImportSpecs'
// rules), sorted, without the importer itself.
func (r *importResolver) specTargets(importer string, spec extractor.ImportSpec, topLevel map[string]map[string]string, keyFile map[string]string, declared, types map[string][]string) []string {
	var targets []string
	switch spec.Kind {
	case extractor.ImportNamed:
		module := r.resolve(importer, spec.Module)
		if key, ok := lookupNamed(module, spec.ExportedName, topLevel); ok {
			targets = []string{keyFile[key]}
		} else if sep := r.submoduleSep(); sep != "" && spec.ExportedName != "" {
			if sub := r.resolve(importer, spec.Module+sep+spec.ExportedName); len(sub) == 1 {
				targets = sub
			}
		}
		if len(targets) == 0 {
			targets = module
		}
	case extractor.ImportWhole:
		switch keys := r.aliasedType(types, spec); {
		case r.wholeIsWildcard():
			targets = r.resolve(importer, spec.Module)
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
			targets = r.resolve(importer, spec.Module)
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
