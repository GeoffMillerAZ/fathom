package main

// langoracle_go.go — LB-F58: Go's L3. Go is native (goextract.go): it has no
// tags dialect and no file resolver (extractor.FileResolveLangs skips the
// go-package strategy), so fusion.ResolveFileEdges cannot run for it. What
// ingest stores for Go across files is the typed go/packages pass's edges
// (fusion.EnrichTypedEdges): the harness runs that pass in process into an
// in-memory store and scores its edges against the SCIP index's cross-file
// edges, and resolves each import spec with Go's own loader to the measured
// files of the package it names, scored against the index's references on
// the import lines (import_resolution).

import (
	"context"
	"fmt"
	"go/parser"
	"go/token"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/langbench"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// goDialect is the native Go dialect's id.
const goDialect = "go"

// noteGoTypedFileEdges discloses what Go's L3 metrics measure.
const noteGoTypedFileEdges = "go: file_edge_precision and file_edge_recall score the typed go/packages pass's edges (fusion.EnrichTypedEdges, the cross-file edges ingest stores for Go, run in process into an in-memory store), and import_resolution each import spec resolved by Go's own loader (go/packages, the typed pass's environment) to the measured files of the package it names; Go has no tags dialect and no file resolver (LB-F58)"

// goTypedEdges runs the typed pass over root, scoped to the measured files
// (the connector-scope filter ingest passes), and returns the edges it
// stored.
func goTypedEdges(ctx context.Context, service, root string, inScope map[string]bool) ([]core.Edge, fusion.TypedEdgeReport, error) {
	st := store.NewMemoryStore()
	defer st.Close()
	rep, err := fusion.EnrichTypedEdges(ctx, st, service, root, inScope)
	if err != nil {
		return nil, rep, err
	}
	edges, err := st.TypedEdges(ctx, service)
	return edges, rep, err
}

// goImportSites is Go's imports: every import spec of the measured .go
// files, at the lines of the spec itself, resolved by go/packages (loaded
// from root under fusion.TypedEdgeEnv, as the typed pass loads) to the
// measured files of the package its path names; a path no package of the
// module has (the standard library, a dependency) resolves to nothing.
// faults names the files whose imports could not be parsed.
func goImportSites(ctx context.Context, root string, files []core.SourceFile, inScope map[string]bool) (sites []langbench.ImportSite, faults []string, err error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, nil, err
	}
	pkgs, err := packages.Load(&packages.Config{Context: ctx, Dir: abs, Env: fusion.TypedEdgeEnv(), Mode: packages.NeedName | packages.NeedFiles}, "./...")
	if err != nil {
		return nil, nil, fmt.Errorf("go/packages load: %w", err)
	}
	rel := goRelPath(abs)
	filesOf := map[string][]string{} // import path -> the package's measured files
	for _, p := range pkgs {
		for _, f := range p.GoFiles {
			if r, ok := rel(f); ok && inScope[r] {
				filesOf[p.PkgPath] = append(filesOf[p.PkgPath], r)
			}
		}
	}
	for _, fs := range filesOf {
		sort.Strings(fs)
	}
	for _, f := range files {
		fset := token.NewFileSet()
		af, perr := parser.ParseFile(fset, f.Path, f.Content, parser.ImportsOnly)
		if perr != nil && af == nil {
			faults = append(faults, f.Path)
			continue
		}
		for _, spec := range af.Imports {
			ip, uerr := strconv.Unquote(spec.Path.Value)
			if uerr != nil {
				continue
			}
			local := path.Base(ip)
			if spec.Name != nil {
				local = spec.Name.Name
			}
			var targets []string
			for _, t := range filesOf[ip] {
				if t != f.Path {
					targets = append(targets, t)
				}
			}
			sites = append(sites, langbench.ImportSite{Path: f.Path, StartLine: fset.Position(spec.Pos()).Line,
				EndLine: fset.Position(spec.End()).Line, Local: local, Targets: targets})
		}
	}
	return sites, faults, nil
}

// goRelPath maps an absolute file go/packages reports to its path relative
// to root (forward slashes), trying root as given and with its symlinks
// resolved (go list may report either).
func goRelPath(root string) func(string) (string, bool) {
	roots := []string{root}
	if r, err := filepath.EvalSymlinks(root); err == nil && r != root {
		roots = append(roots, r)
	}
	return func(f string) (string, bool) {
		for _, r := range roots {
			if rel, err := filepath.Rel(r, f); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return filepath.ToSlash(rel), true
			}
		}
		return "", false
	}
}
