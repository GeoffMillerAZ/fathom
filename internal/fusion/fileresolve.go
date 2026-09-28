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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
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
	// is present (SU-9 is authoritative), lang has no registered
	// tree-sitter extractor, or (RT-108) no TS/JS/Python file changed since
	// its last successful pass. SkipReason names why; never a silent empty.
	Skipped    bool
	SkipReason string
	// RT-108 (design §7.2): Reused counts files whose imports came from
	// file_imports (unchanged fingerprint: not read, not parsed); Faults
	// counts files whose import parse faulted (this run or recorded for
	// the same bytes), FaultFiles names up to five; Gated counts files the
	// size and binary gates kept out of the parse.
	Reused     int
	Faults     int
	FaultFiles []string
	Gated      int
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
	if hasScipIndex(repoPath) {
		return nil, FileResolveReport{Skipped: true, SkipReason: "a SCIP index is present for this repo; the typed importer (SU-9) is authoritative"}
	}
	ext, err := extractor.NewTSExtractor(lang)
	if err != nil {
		return nil, FileResolveReport{Skipped: true, SkipReason: err.Error()}
	}
	defer ext.Close() // RT-107: its grammar modules are released, never leaked
	return resolveFileEdgesWith(ctx, ext, lang, repoPath, files, symbols)
}

// resolveFileEdgesWith is ResolveFileEdges over an extractor the caller
// owns — the pipeline passes its own (RT-107), so an ingest instantiates no
// new grammar module for the pass.
func resolveFileEdgesWith(ctx context.Context, ext *extractor.TSExtractor, lang, repoPath string, files []core.SourceFile, symbols []core.SymbolNode) ([]core.Edge, FileResolveReport) {
	_ = ctx
	paths := make([]string, 0, len(files))
	specs := make(map[string][]extractor.ImportSpec, len(files))
	var faults []string
	for _, f := range files {
		paths = append(paths, f.Path)
		sp, ierr := ext.ExtractImports(f.Path, f.Content)
		if ierr != nil {
			faults = append(faults, f.Path) // counted and named, never a silent skip
			continue
		}
		specs[f.Path] = sp
	}
	edges, report := resolveImports(lang, paths, specs, symbols)
	report.Faults = len(faults)
	report.FaultFiles = firstN(faults, 5)
	return edges, report
}

func firstN(xs []string, n int) []string {
	if len(xs) > n {
		return append([]string(nil), xs[:n]...)
	}
	return xs
}

// resolveImports resolves every file's call sites through its parsed import
// specifiers. paths is the resolution universe, in order; a file absent from
// specs (a parse fault, or a gated file — counted by the caller) resolves
// nothing and is not counted as scanned.
func resolveImports(lang string, paths []string, specs map[string][]extractor.ImportSpec, symbols []core.SymbolNode) ([]core.Edge, FileResolveReport) {
	var report FileResolveReport
	fileSet := make(map[string]bool, len(paths))
	for _, p := range paths {
		fileSet[p] = true
	}

	// topLevel[file][name] = that symbol's full SymbolKey — the only
	// source a resolved edge's To is ever drawn from. "Top level" means an
	// unqualified Name (no "."): TSExtractor's qualifyTSDefs only dots a
	// callable's name when its immediate parent is a container, so a
	// module-scope function/class/variable always has a bare Name.
	topLevel := make(map[string]map[string]string, len(paths))
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
	for _, path := range paths {
		fileSpecs, parsed := specs[path]
		if !parsed {
			continue // a fault or a gated file: counted by the caller
		}
		report.FilesScanned++
		report.ImportsParsed += len(fileSpecs)

		named := map[string]string{} // LocalName -> resolved target SymbolKey
		whole := map[string]string{} // LocalName -> resolved target FILE path
		for _, spec := range fileSpecs {
			if spec.LocalName == "" {
				continue // side-effect-only import: no bound name to attribute a call to
			}
			switch spec.Kind {
			case extractor.ImportNamed:
				target, ok := resolveModule(lang, path, spec.Module, fileSet)
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
					if t2, ok2 := resolveModule(lang, path, spec.Module+"."+spec.ExportedName, fileSet); ok2 {
						whole[spec.LocalName] = t2
					}
				}
			case extractor.ImportWhole:
				if target, ok := resolveModule(lang, path, spec.Module, fileSet); ok {
					whole[spec.LocalName] = target
				}
			}
		}
		if len(named) == 0 && len(whole) == 0 && len(topLevel[path]) == 0 {
			continue
		}

		for _, s := range symbols {
			if s.FilePath != path || len(s.Calls) == 0 {
				continue
			}
			fromKey := s.Key()
			seen := map[string]bool{}
			for _, call := range s.Calls {
				toKey, ok := matchFileResolvedCall(call, named, whole, topLevel, topLevel[path])
				if !ok || toKey == fromKey || seen[toKey] {
					continue
				}
				seen[toKey] = true
				edges = append(edges, core.Edge{
					FromFile: path, FromSymbol: fromKey, ToSymbol: toKey,
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
//
// RT-153 (I38): a fetch the producer governor refuses (ErrOverBudget) or a
// cancelled ctx is not a read failure — the pass stops before its store
// write and returns the error, so a halt is a named failure and never a
// wholesale replacement of the language's edges by a shorter set.
func (p *Pipeline) enrichFileResolvedEdges(ctx context.Context, svc core.ServiceRef, refs []core.SourceRef, changed []string) (FileResolveReport, error) {
	var total FileResolveReport
	if p.cfg.DisableTypedEdges {
		return total, nil
	}
	if hasScipIndex(svc.Path) {
		total.Skipped = true
		total.SkipReason = "a SCIP index is present for this repo; the typed importer (SU-9) is authoritative"
		return total, nil
	}

	langFiles := map[string][]core.SourceRef{}
	for _, r := range refs {
		if lang := fileResolveLangFor(r.Path); lang != "" {
			langFiles[lang] = append(langFiles[lang], r)
		}
	}
	if len(langFiles) == 0 {
		return total, nil
	}
	// RT-108: the resolution depends only on the TS/JS/Python files — when
	// none changed (new, changed, deleted) since the last successful pass,
	// the stored edges are the answer.
	if !passNeeded(changed, p.passStateOK(ctx, svc.ID, fileResolveStateKey), func(path string) bool { return fileResolveLangFor(path) != "" }) {
		total.Skipped = true
		total.SkipReason = SkippedPassReason
		return total, nil
	}
	p.beginPassState(ctx, svc.ID, fileResolveStateKey)

	for _, lang := range []string{"javascript", "python"} { // deterministic order
		lrefs, ok := langFiles[lang]
		if !ok {
			continue
		}
		// RT-107: the pipeline's own extractor for the language — never a
		// fresh one per ingest (two per language per ingest, never closed,
		// was the audit's wazero-module-leak-per-ingest hazard).
		ext, done, err := p.fileResolveExtractor(lang)
		if err != nil {
			fmt.Fprintf(os.Stderr, "fusion: file-resolved edge pass %q (%s): %v\n", svc.ID, lang, err)
			continue
		}
		edges, rep, err := p.resolveLang(ctx, svc, lang, ext, lrefs)
		done()
		if err != nil {
			return total, err
		}
		total.FilesScanned += rep.FilesScanned
		total.ImportsParsed += rep.ImportsParsed
		total.EdgesResolved += len(edges)
		total.Reused += rep.Reused
		total.Gated += rep.Gated
		total.Faults += rep.Faults
		total.FaultFiles = firstN(append(total.FaultFiles, rep.FaultFiles...), 5)
		if putErr := p.st.PutTypedEdges(ctx, svc.ID, fileResolvedPackageKey(lang), edges); putErr != nil {
			return total, fmt.Errorf("file-resolved edges (%s): store: %w", lang, putErr)
		}
	}
	if total.Faults > 0 {
		p.diagWarn("fusion: file-resolved edge pass %q: %d file(s) whose imports could not be parsed (%s) — their calls resolve no import edges\n", svc.ID, total.Faults, strings.Join(total.FaultFiles, ", "))
	}
	p.commitPassState(ctx, svc.ID, fileResolveStateKey)
	return total, nil
}

// extractImports parses one file's imports (a var so a test can fault it).
var extractImports = func(ext *extractor.TSExtractor, path string, src []byte) ([]extractor.ImportSpec, error) {
	return ext.ExtractImports(path, src)
}

// fileResolveStateKey is the file resolver's pass-state row (typedgate.go).
const fileResolveStateKey = typedPassStatePrefix + "file-resolved"

// resolveLang resolves one language's files (RT-108, design §7.2): the
// symbols come from the rows this ingest already stored (no re-extraction);
// each file's imports come from file_imports when its fingerprint is
// unchanged (not read, not parsed) and are otherwise read — under the 2 MiB
// cap, the binary sniff and the size gate — parsed once and recorded. A
// parse fault is recorded against the bytes, so it is not retried until
// they change, and is counted and named on every run.
func (p *Pipeline) resolveLang(ctx context.Context, svc core.ServiceRef, lang string, ext *extractor.TSExtractor, lrefs []core.SourceRef) ([]core.Edge, FileResolveReport, error) {
	var rep FileResolveReport
	fis, _ := p.st.(store.FileImportsStore)
	paths := make([]string, 0, len(lrefs))
	for _, r := range lrefs {
		paths = append(paths, r.Path)
	}
	var symbols []core.SymbolNode
	if fis != nil {
		obs, err := fis.SourceObservations(ctx, svc.ID, paths)
		if err != nil {
			if h := haltError(ctx, err); h != nil {
				return nil, rep, h
			}
			return nil, rep, fmt.Errorf("file-resolved edges (%s): symbols: %w", lang, err)
		}
		symbols = make([]core.SymbolNode, 0, len(obs))
		for _, o := range obs {
			symbols = append(symbols, o.ToSymbolNode())
		}
	}
	specs := make(map[string][]extractor.ImportSpec, len(lrefs))
	var faults []string
	for _, r := range lrefs {
		if err := tenancy.ProducerAllowsCtx(ctx, tenancy.ClassIngest); err != nil {
			return nil, rep, err
		}
		fp, fpFound, _ := p.st.Fingerprint(ctx, svc.ID, r.Path)
		if fis != nil && fpFound {
			if row, ok, _ := fis.GetFileImports(ctx, svc.ID, r.Path); ok && row.ContentHash == fp {
				if row.Fault != "" {
					faults = append(faults, r.Path)
					continue
				}
				var sp []extractor.ImportSpec
				if json.Unmarshal([]byte(row.ImportsJSON), &sp) == nil {
					specs[r.Path] = sp
					rep.Reused++
					continue
				}
			}
		}
		content, ferr := p.conn.Fetch(ctx, r)
		if ferr != nil {
			if h := haltError(ctx, ferr); h != nil {
				return nil, rep, h
			}
			rep.Gated++ // over the read cap, or unreadable this run
			continue
		}
		// LB-F10: a generated/minified (or ambiguous) dialect file is
		// gated here too — the extract pass never extracted it.
		if len(content) > MaxExtractSize || connector.IsBinary(content) || gateDialectFile(r.Path, content).excluded != "" {
			rep.Gated++
			continue
		}
		hash := FingerprintHash(content)
		if fis == nil {
			nodes, eerr := ext.Extract(svc.ID, r.Path, content)
			if eerr == nil {
				symbols = append(symbols, nodes...)
			}
		}
		sp, ierr := extractImports(ext, r.Path, content)
		if ierr != nil {
			faults = append(faults, r.Path)
			if fis != nil {
				_ = fis.SetFileImports(ctx, store.FileImports{ServiceID: svc.ID, Path: r.Path, ContentHash: hash, Fault: ierr.Error()})
			}
			continue
		}
		specs[r.Path] = sp
		if fis != nil {
			if b, err := json.Marshal(sp); err == nil {
				_ = fis.SetFileImports(ctx, store.FileImports{ServiceID: svc.ID, Path: r.Path, ContentHash: hash, ImportsJSON: string(b)})
			}
		}
	}
	edges, res := resolveImports(lang, paths, specs, symbols)
	res.Reused, res.Gated = rep.Reused, rep.Gated
	res.Faults, res.FaultFiles = len(faults), firstN(faults, 5)
	return edges, res, nil
}

// fileResolveExtractorKey is the p.exts key whose extractor the file
// resolver shares for each language.
var fileResolveExtractorKey = map[string]string{"javascript": ".js", "python": ".py"}

// fileResolveExtractor returns the pipeline's tree-sitter extractor for
// lang and a no-op release; when a test replaced it with another kind, a
// fresh one released by the returned func.
func (p *Pipeline) fileResolveExtractor(lang string) (*extractor.TSExtractor, func(), error) {
	if e, ok := p.exts[fileResolveExtractorKey[lang]].(*extractor.TSExtractor); ok {
		return e, func() {}, nil
	}
	e, err := extractor.NewTSExtractor(lang)
	if err != nil {
		return nil, func() {}, err
	}
	return e, e.Close, nil
}

// haltError is err when it (or ctx) says the ingest must stop — the
// producer governor's ErrOverBudget or a cancelled/expired ctx — and nil
// for an ordinary per-file read failure a best-effort pass may skip.
func haltError(ctx context.Context, err error) error {
	if errors.Is(err, tenancy.ErrOverBudget) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if cErr := ctx.Err(); cErr != nil {
		return cErr
	}
	return nil
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
// fileResolvedKeyPrefix namespaces the file resolver's typed-edge rows.
const fileResolvedKeyPrefix = "file-resolved:"

func fileResolvedPackageKey(lang string) string { return fileResolvedKeyPrefix + lang }

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
