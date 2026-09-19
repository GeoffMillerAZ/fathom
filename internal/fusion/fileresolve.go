// Per-file import/module-scope resolver for TypeScript/JavaScript and
// Python (ADR-041 decision 5, docs/plans/substrate.md SU-11) — the
// stack-graphs-style fallback for languages with no SCIP exporter wired
// yet (SU-9, another wave): where no `index.scip`/`.fathom/scip/*.scip`
// exists for the service, ResolveFileEdges parses each file's import
// statements (internal/extractor.TSExtractor.ExtractImports, itself driven
// by the "import.*" captures added to javascript.scm/typescript.scm/
// python.scm) and, for every already-extracted call site (SymbolNode.Calls
// — the naive same-file text the tree-sitter queries already collect)
// whose callee is bound by an import resolving to a file in the corpus, or
// is defined at module scope in the SAME file, emits a core.Edge of kind
// core.EdgeFileResolved from the enclosing symbol to the resolved target.
//
// This is weaker evidence than a typed pass by construction: no type
// checker, no overload resolution, just import-graph plus name-in-scope
// matching. core.EdgeConfidenceOf special-cases EdgeFileResolved as
// ALWAYS heuristic (never resolved) — see internal/core/edge_confidence.go
// — so find_callers/resolved_nav/blast_radius never present it as typed;
// it is additive alongside the syntax-only Calls-based graph, never a
// replacement for it.
//
// Scope, deliberately narrower than a real typed pass (documented, not a
// gap pretending otherwise):
//   - Only the FIRST dotted segment of a call site's callee text is
//     consulted for member access (`ns.foo.bar()` resolves via "ns" + the
//     "foo" member only, mirroring the depth every tree-sitter call
//     capture already collapses to — CollectCalls's own contract).
//   - A named import bound to another named import's re-export, a
//     destructured method off a locally-constructed object, and a call
//     through a class instance's method are all left unresolved — each
//     would need real type information this pass does not have.
//   - Default/namespace/whole-module bindings resolve a BARE call through
//     them (`def()`) only when the target file has EXACTLY ONE top-level
//     symbol — an unambiguous "sole export" heuristic, not a real
//     default-export graph.
//   - Python absolute imports resolve from repoPath's own root (no
//     PYTHONPATH/src-layout modeling); relative imports (`from .` / `from
//     ..pkg`) resolve against the importing file's own directory.
package fusion

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// FileResolveReport summarises one ResolveFileEdges run.
type FileResolveReport struct {
	// FilesScanned is the number of files whose imports were parsed.
	FilesScanned int
	// ImportsParsed is the total import specifiers found across those files.
	ImportsParsed int
	// EdgesResolved is the number of core.EdgeFileResolved edges emitted.
	EdgesResolved int
	// Skipped is true when the resolver did not run at all — a SCIP index
	// is present (SU-9 is authoritative) or lang has no registered
	// tree-sitter extractor. SkipReason names why; never a silent empty.
	Skipped    bool
	SkipReason string
}

// hasScipIndex reports whether repoPath already carries a SCIP index this
// service's ingest would import instead (SU-9, another wave — not
// implemented here, only detected so this resolver defers to it). Checked:
// an `index.scip` file at the repo root, or any `*.scip` file directly
// under `.fathom/scip/`. ResolveFileEdges must never run alongside a real
// typed SCIP import: a file-resolved edge is strictly weaker evidence, and
// the shared typed_edges table replaces a package's rows wholesale, so
// running both would let this pass silently downgrade a SCIP-backed row.
func hasScipIndex(repoPath string) bool {
	if repoPath == "" {
		return false
	}
	if _, err := os.Stat(filepath.Join(repoPath, "index.scip")); err == nil {
		return true
	}
	entries, err := os.ReadDir(filepath.Join(repoPath, ".fathom", "scip"))
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".scip") {
			return true
		}
	}
	return false
}

// ResolveFileEdges is the per-file resolver described in the package doc.
// lang is "javascript" or "python" (internal/extractor's TSExtractor
// registry key — "javascript" also covers .jsx/.ts/.tsx/.mjs/.cjs/.mts/
// .cts, matching the extractor's own variant table). files is the
// resolution universe: every file an import may resolve against, and the
// set ExtractImports parses; symbols is every already-extracted
// core.SymbolNode for those same files — both the callers whose Calls
// entries are walked and the sole source of a resolved edge's To (never
// invented: a target is only ever a key drawn from this slice).
//
// ctx is accepted for signature parity with the other Enrich* passes and
// future cancellation; nothing here currently blocks on it.
func ResolveFileEdges(ctx context.Context, lang, repoPath string, files []core.SourceFile, symbols []core.SymbolNode) ([]core.Edge, FileResolveReport) {
	_ = ctx
	var report FileResolveReport

	if hasScipIndex(repoPath) {
		report.Skipped = true
		report.SkipReason = "a SCIP index is present for this repo; the typed importer (SU-9) is authoritative"
		return nil, report
	}

	ext, err := extractor.NewTSExtractor(lang)
	if err != nil {
		report.Skipped = true
		report.SkipReason = err.Error()
		return nil, report
	}

	fileSet := make(map[string]bool, len(files))
	for _, f := range files {
		fileSet[f.Path] = true
	}

	// topLevel[file][name] = that symbol's full SymbolKey — the only
	// source a resolved edge's To is ever drawn from. "Top level" means an
	// unqualified Name (no "."): TSExtractor's qualifyTSDefs only dots a
	// callable's name when its immediate parent is a container, so a
	// module-scope function/class/variable always has a bare Name.
	topLevel := make(map[string]map[string]string, len(files))
	for _, s := range symbols {
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

	var edges []core.Edge
	for _, f := range files {
		specs, ierr := ext.ExtractImports(f.Path, f.Content)
		if ierr != nil {
			continue // best-effort, matching the Go/CUE typed passes' per-file honesty
		}
		report.FilesScanned++
		report.ImportsParsed += len(specs)

		named := map[string]string{} // LocalName -> resolved target SymbolKey
		whole := map[string]string{} // LocalName -> resolved target FILE path
		for _, spec := range specs {
			if spec.LocalName == "" {
				continue // side-effect-only import: no bound name to attribute a call to
			}
			switch spec.Kind {
			case extractor.ImportNamed:
				target, ok := resolveModule(lang, f.Path, spec.Module, fileSet)
				if ok {
					if key, ok2 := topLevel[target][spec.ExportedName]; ok2 {
						named[spec.LocalName] = key
						continue
					}
				}
				// Python "from X import Y" where Y is itself a submodule
				// (X/Y.py or X/Y/__init__.py), not a symbol X defines —
				// retry as a whole-module binding one level deeper.
				if lang == "python" {
					if t2, ok2 := resolveModule(lang, f.Path, spec.Module+"."+spec.ExportedName, fileSet); ok2 {
						whole[spec.LocalName] = t2
					}
				}
			case extractor.ImportWhole:
				if target, ok := resolveModule(lang, f.Path, spec.Module, fileSet); ok {
					whole[spec.LocalName] = target
				}
			}
		}
		if len(named) == 0 && len(whole) == 0 && len(topLevel[f.Path]) == 0 {
			continue
		}

		for _, s := range symbols {
			if s.FilePath != f.Path || len(s.Calls) == 0 {
				continue
			}
			fromKey := s.Key()
			seen := map[string]bool{}
			for _, call := range s.Calls {
				toKey, ok := matchFileResolvedCall(call, named, whole, topLevel, topLevel[f.Path])
				if !ok || toKey == fromKey || seen[toKey] {
					continue
				}
				seen[toKey] = true
				edges = append(edges, core.Edge{
					FromFile: f.Path, FromSymbol: fromKey, ToSymbol: toKey,
					Kind: core.EdgeFileResolved,
				})
			}
		}
	}
	sortEdges(edges)
	report.EdgesResolved = len(edges)
	return edges, report
}

// matchFileResolvedCall resolves one call-site text (e.g. "foo", "ns.foo",
// "pkg.util.other" — only the segment immediately past a whole-module
// binding's own prefix is consulted, see the package doc) against, in
// order: a named import binding (always a single bare identifier), a
// whole-module import binding (LocalName may itself be dotted — Python
// `import pkg.mod` — so matched by longest-prefix, not a fixed split;
// member access, or a bare call through it when the target file has
// exactly one top-level symbol), and finally a same-file module-scope
// sibling.
func matchFileResolvedCall(call string, named, whole map[string]string, topLevel map[string]map[string]string, localTop map[string]string) (string, bool) {
	bareBase := call
	hasDot := false
	if i := strings.IndexByte(call, '.'); i >= 0 {
		bareBase, hasDot = call[:i], true
	}

	if !hasDot {
		if key, ok := named[bareBase]; ok {
			return key, true
		}
	}

	if target, member, ok := longestWholePrefix(call, whole); ok {
		if member == "" {
			if syms := topLevel[target]; len(syms) == 1 {
				for _, key := range syms {
					return key, true
				}
			}
		} else if key, ok2 := topLevel[target][member]; ok2 {
			return key, true
		}
	}

	if !hasDot {
		if key, ok := localTop[bareBase]; ok {
			return key, true
		}
	}
	return "", false
}

// longestWholePrefix finds the longest key in whole that either equals
// call exactly (a bare call through the whole-module binding itself,
// member "") or is a dotted prefix of it (member is the single segment
// immediately following that prefix — deeper chains are not consulted,
// matching the package doc's documented depth limit). Longest-prefix
// matters because a whole-module binding may itself be dotted (Python
// `import pkg.mod` binds "pkg.mod", not "pkg") while another binding in
// the same file could bind a shorter prefix of the same text.
//
// Iterates the whole map, but the result never depends on iteration
// order: two DIFFERENT keys of the SAME length can never both match call
// (a match's length pins it to one unique substring of call, and map keys
// are themselves unique), so "the longest match" has exactly one possible
// winner regardless of enumeration order.
func longestWholePrefix(call string, whole map[string]string) (target, member string, ok bool) {
	best := -1
	for prefix, t := range whole {
		if call == prefix {
			if len(prefix) > best {
				best, target, member, ok = len(prefix), t, "", true
			}
			continue
		}
		if strings.HasPrefix(call, prefix+".") {
			rem := call[len(prefix)+1:]
			if j := strings.IndexByte(rem, '.'); j >= 0 {
				rem = rem[:j]
			}
			if len(prefix) > best {
				best, target, member, ok = len(prefix), t, rem, true
			}
		}
	}
	return
}

// tsResolveExts is the extension search order for a resolved TypeScript/
// JavaScript relative import with no extension of its own (SU-11: "TS:
// .ts/.tsx/.js/index.*").
var tsResolveExts = []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"}

// resolveModule resolves an ImportSpec's raw Module text to a file path in
// fileSet, relative to importerPath (the importing file's own repo-relative
// path). Returns ok=false for anything this pass cannot settle: a bare
// package specifier (node_modules/PyPI, outside the corpus by
// construction), a path that resolves to no file in fileSet.
func resolveModule(lang, importerPath, module string, fileSet map[string]bool) (string, bool) {
	if lang == "python" {
		return resolvePyModule(importerPath, module, fileSet)
	}
	return resolveTSModule(importerPath, module, fileSet)
}

// resolveTSModule handles TypeScript/JavaScript's relative-only resolution
// (a bare specifier is a package import — never in the corpus, correctly
// left unresolved).
func resolveTSModule(importerPath, spec string, fileSet map[string]bool) (string, bool) {
	if !strings.HasPrefix(spec, "./") && !strings.HasPrefix(spec, "../") {
		return "", false
	}
	candidate := path.Clean(path.Join(path.Dir(importerPath), spec))
	if fileSet[candidate] {
		return candidate, true
	}
	for _, ext := range tsResolveExts {
		if p := candidate + ext; fileSet[p] {
			return p, true
		}
	}
	for _, ext := range tsResolveExts {
		if p := candidate + "/index" + ext; fileSet[p] {
			return p, true
		}
	}
	return "", false
}

// enrichFileResolvedEdges is the fusion.Pipeline.Ingest call-site hook for
// ResolveFileEdges, the SU-11 sibling of enrichCUEEdges: best-effort, never
// aborts or fails the surrounding Ingest. Unlike enrichCUEEdges it has no
// per-package content-hash cache (see the package doc) — it re-fetches and
// re-parses the service's ENTIRE TypeScript/JavaScript/Python corpus every
// ingest via the connector, independent of this run's own hash-gate, the
// same "whole tree, every run" shape the Go/CUE typed passes use.
//
// A read failure fetching one file here is best-effort skipped (already
// surfaced, if fatal, by the main ingest pass above) rather than aborting
// the whole language's resolution.
func (p *Pipeline) enrichFileResolvedEdges(ctx context.Context, svc core.ServiceRef, refs []core.SourceRef) FileResolveReport {
	var total FileResolveReport
	if p.cfg.DisableTypedEdges {
		return total
	}
	if hasScipIndex(svc.Path) {
		total.Skipped = true
		total.SkipReason = "a SCIP index is present for this repo; the typed importer (SU-9) is authoritative"
		return total
	}

	langFiles := map[string][]core.SourceRef{}
	for _, r := range refs {
		if lang := fileResolveLangFor(r.Path); lang != "" {
			langFiles[lang] = append(langFiles[lang], r)
		}
	}
	if len(langFiles) == 0 {
		return total
	}

	for _, lang := range []string{"javascript", "python"} { // deterministic order
		lrefs, ok := langFiles[lang]
		if !ok {
			continue
		}
		ext, err := extractor.NewTSExtractor(lang)
		if err != nil {
			fmt.Fprintf(os.Stderr, "fusion: file-resolved edge pass %q (%s): %v\n", svc.ID, lang, err)
			continue
		}
		files := make([]core.SourceFile, 0, len(lrefs))
		var symbols []core.SymbolNode
		for _, r := range lrefs {
			content, ferr := p.conn.Fetch(ctx, r)
			if ferr != nil {
				continue
			}
			files = append(files, core.SourceFile{Path: r.Path, Content: content, Lang: lang})
			nodes, eerr := ext.Extract(svc.ID, r.Path, content)
			if eerr == nil {
				symbols = append(symbols, nodes...)
			}
		}

		edges, rep := ResolveFileEdges(ctx, lang, svc.Path, files, symbols)
		total.FilesScanned += rep.FilesScanned
		total.ImportsParsed += rep.ImportsParsed
		total.EdgesResolved += len(edges)
		if putErr := p.st.PutTypedEdges(ctx, svc.ID, fileResolvedPackageKey(lang), edges); putErr != nil {
			fmt.Fprintf(os.Stderr, "fusion: file-resolved edge pass %q (%s): store: %v\n", svc.ID, lang, putErr)
		}
	}
	return total
}

// fileResolveLangFor maps a repo-relative path's extension to the
// TSExtractor registry key this pass resolves it under, "" for anything
// else (the pipeline hook simply skips such files).
func fileResolveLangFor(relPath string) string {
	switch {
	case strings.HasSuffix(relPath, ".py"):
		return "python"
	case strings.HasSuffix(relPath, ".js"), strings.HasSuffix(relPath, ".jsx"),
		strings.HasSuffix(relPath, ".ts"), strings.HasSuffix(relPath, ".tsx"),
		strings.HasSuffix(relPath, ".mjs"), strings.HasSuffix(relPath, ".cjs"),
		strings.HasSuffix(relPath, ".mts"), strings.HasSuffix(relPath, ".cts"):
		return "javascript"
	}
	return ""
}

// fileResolvedPackageKey names the typed_edges store key ResolveFileEdges'
// pipeline hook replaces wholesale each run for one language — namespaced
// so it can never collide with a Go pkg.ID or a CUE "cue:"-prefixed key
// sharing the same service.
func fileResolvedPackageKey(lang string) string { return "file-resolved:" + lang }

// resolvePyModule handles Python's dotted-path resolution: an absolute
// `pkg.mod` resolves from repoPath's own root (importerPath's root, since
// fileSet is already repo-relative); a relative `.mod`/`..pkg.mod`/`.`
// resolves from importerPath's own directory, walking up one additional
// parent per dot beyond the first (`.` = the importer's own package).
func resolvePyModule(importerPath, spec string, fileSet map[string]bool) (string, bool) {
	dir := ""
	dotted := spec
	if strings.HasPrefix(spec, ".") {
		dots := 0
		for dots < len(spec) && spec[dots] == '.' {
			dots++
		}
		dotted = spec[dots:]
		dir = path.Dir(importerPath)
		for n := 1; n < dots; n++ {
			dir = path.Dir(dir)
		}
	}
	rel := strings.ReplaceAll(dotted, ".", "/")
	candidate := rel
	switch {
	case dir != "" && dir != "." && rel != "":
		candidate = dir + "/" + rel
	case dir != "" && dir != "." && rel == "":
		candidate = dir
	}
	candidate = path.Clean(candidate)
	if p := candidate + ".py"; fileSet[p] {
		return p, true
	}
	if p := candidate + "/__init__.py"; fileSet[p] {
		return p, true
	}
	return "", false
}
