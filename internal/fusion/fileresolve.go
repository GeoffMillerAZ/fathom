// Per-file import/module-scope resolver (ADR-041 decision 5,
// docs/plans/substrate.md SU-11; generalized by LB-F11 to every tags dialect
// whose manifest selects an import strategy — resolvestrategy.go — of which
// TypeScript/JavaScript and Python are today's) — the
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
// Since LB-F74 the same holds for every name a file references outside a
// call's own text (a type, `new X`, a superclass, a value): the dialect's
// references query (extractor.FileSpecs, refresolve.go), drawn from the
// innermost definition enclosing it.
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
//   - A destructured method off a locally-constructed object and a call
//     through a class instance's method are left unresolved — each would
//     need real type information this pass does not have. A name a module
//     re-exports (`export * from`, a Dart barrel, a Python package's own
//     imports) binds the file that defines it (LB-F75, exports.go).
//   - Default/namespace/whole-module bindings resolve a BARE use through
//     them (`def()`) to the target's default export (`export default X`,
//     `module.exports = X`), else its top-level definition of the bound
//     name, else its sole top-level symbol — never a guess among several.
//   - Python absolute imports resolve from repoPath's own root (no
//     PYTHONPATH/src-layout modeling); relative imports (`from .` / `from
//     ..pkg`) resolve against the importing file's own directory.
package fusion

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"

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
	// LB-Q-c-rest (include-path, C and C++): CompileDBs is the compile
	// databases read; AncestorIncludes the distinct quoted includes that
	// resolved only rooted at an ancestor directory of the including file;
	// the Proto counters the calls (distinct caller file and name) the
	// header-prototype binding settled: ProtoBound to the one defining file,
	// ProtoAmbiguous left unbound for two definers, ProtoNoDefiner for none
	// in the universe, ProtoNotBuilt for definers the build does not compile,
	// and ProtoBuildFiltered those the build narrowed to the one it compiles
	// (counted again in ProtoBound).
	CompileDBs         int
	AncestorIncludes   int
	ProtoBound         int
	ProtoAmbiguous     int
	ProtoNoDefiner     int
	ProtoNotBuilt      int
	ProtoBuildFiltered int
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

// diskManifestReader reads a repo manifest (a strategy's composer.json,
// Cargo.toml, …) from repoPath, bounded by maxManifestBytes.
func diskManifestReader(repoPath string) func(string) ([]byte, bool) {
	return func(rel string) ([]byte, bool) {
		if repoPath == "" {
			return nil, false
		}
		f, err := os.Open(filepath.Join(repoPath, filepath.FromSlash(rel)))
		if err != nil {
			return nil, false
		}
		defer f.Close()
		b, err := io.ReadAll(io.LimitReader(f, maxManifestBytes+1))
		if err != nil || len(b) > maxManifestBytes {
			return nil, false
		}
		return b, true
	}
}

// newImportResolver is lang's manifest-selected strategy (LB-F11) over the
// universe paths; ok=false when lang's manifest names no strategy.
func newImportResolver(lang, root string, paths []string, read func(string) ([]byte, bool)) (*importResolver, bool) {
	langs, err := extractor.FileResolveLangs()
	if err != nil {
		return nil, false
	}
	for _, l := range langs {
		if l.Lang == lang {
			r := newLangResolver(l)
			r.env = newResolveEnv(root, paths, read)
			return r, true
		}
	}
	return nil, false
}

// newLangResolver is the resolver one file-resolve language's manifest
// selects, without its universe (env).
func newLangResolver(l extractor.FileResolveLang) *importResolver {
	return &importResolver{strategy: l.Strategy, params: l.Params, pkgScope: l.Strategy == "namespace-dir" && extractor.PackageScoped(l.Lang)}
}

// importResolver resolves one language's module specs by its strategy.
type importResolver struct {
	strategy string
	params   extractor.ImportParams
	env      *resolveEnv
	// pkgScope: the language's code sees its own package's (C#: and every
	// enclosing namespace's) names unqualified (LB-F74).
	pkgScope bool
}

func (r *importResolver) resolve(importer, module string) []string {
	return resolveByStrategy(r.strategy, r.params, r.env, importer, module)
}

// wholeIsWildcard: under these strategies a whole-module import (#include,
// require, source, Swift's import) brings every top-level name of its
// target into scope unqualified, rather than binding a prefix.
func (r *importResolver) wholeIsWildcard() bool {
	switch r.strategy {
	case "include-path", "file-symbol", "swift-module":
		return true
	}
	return false
}

// submoduleSep is the separator a named import that names a submodule
// (Python's `from pkg import mod`) is retried with, "" when the strategy
// has no such form.
func (r *importResolver) submoduleSep() string {
	switch r.strategy {
	case "dotted-module", "namespace-dir":
		if r.params.Separator != "" {
			return r.params.Separator
		}
		return "."
	}
	return ""
}

// resolveFileEdgesWith is ResolveFileEdges over an extractor the caller
// owns — the pipeline passes its own (RT-107), so an ingest instantiates no
// new grammar module for the pass.
func resolveFileEdgesWith(ctx context.Context, ext *extractor.TSExtractor, lang, repoPath string, files []core.SourceFile, symbols []core.SymbolNode) ([]core.Edge, FileResolveReport) {
	_ = ctx
	paths := make([]string, 0, len(files))
	specs := make(map[string]extractor.FileSpecs, len(files))
	var faults []string
	for _, f := range files {
		paths = append(paths, f.Path)
		sp, ierr := ext.ExtractFileSpecs(f.Path, f.Content)
		if ierr != nil {
			faults = append(faults, f.Path) // counted and named, never a silent skip
			continue
		}
		specs[f.Path] = sp
	}
	r, ok := newImportResolver(lang, repoPath, paths, diskManifestReader(repoPath))
	if !ok {
		return nil, FileResolveReport{Skipped: true, SkipReason: fmt.Sprintf("dialect %q names no import strategy", lang)}
	}
	if r.strategy == "include-path" {
		// Headers the router sent to the other language (cproto.go).
		extra := foreignHeaders(repoPath, r.env.set)
		for _, p := range extra {
			b, err := os.ReadFile(filepath.Join(repoPath, filepath.FromSlash(p)))
			if err != nil || len(b) > MaxExtractSize {
				continue
			}
			if sp, ierr := ext.ExtractFileSpecs(p, b); ierr == nil {
				specs[p] = sp
				r.env.addForeign([]string{p})
			}
		}
	}
	edges, report := resolveImports(r, paths, specs, symbols)
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

// resolveImports resolves every file's call sites and references through
// its parsed import specifiers. paths is the resolution universe, in order;
// a file absent from specs (a parse fault, or a gated file — counted by the
// caller) resolves nothing and is not counted as scanned.
func resolveImports(r *importResolver, paths []string, specs map[string]extractor.FileSpecs, symbols []core.SymbolNode) ([]core.Edge, FileResolveReport) {
	var report FileResolveReport
	fileSet := make(map[string]bool, len(paths))
	for _, p := range paths {
		fileSet[p] = true
	}
	idx := newResolveIndex(r, symbols, fileSet)
	idx.specs = specs
	x := newExportIndex(r, specs, idx.topLevel)

	var edges []core.Edge
	for _, path := range paths {
		fs, parsed := specs[path]
		if !parsed {
			continue // a fault or a gated file: counted by the caller
		}
		report.FilesScanned++
		report.ImportsParsed += len(fs.Imports)
		sc := idx.scopeOf(r, x, path, fs.Imports)
		syms := idx.byFile[path]
		edges = append(edges, idx.callEdges(r, sc, path, syms)...)
		edges = append(edges, idx.refEdges(r, sc, path, syms, fs.Refs)...)
	}
	sortEdges(edges)
	edges = slices.CompactFunc(edges, func(a, b core.Edge) bool {
		return a.FromFile == b.FromFile && a.FromSymbol == b.FromSymbol && a.ToSymbol == b.ToSymbol && a.Kind == b.Kind
	})
	report.EdgesResolved = len(edges)
	report.AncestorIncludes = r.env.ancestorIncludes()
	report.CompileDBs = len(r.env.compDatabase().names)
	report.ProtoBound = len(idx.proto.bound)
	report.ProtoAmbiguous = len(idx.proto.ambiguous)
	report.ProtoNoDefiner = len(idx.proto.noDefiner)
	report.ProtoNotBuilt = len(idx.proto.notBuilt)
	report.ProtoBuildFiltered = len(idx.proto.buildFiltered)
	return edges, report
}

// resolveIndex is the universe's symbols indexed for binding.
type resolveIndex struct {
	// topLevel[file][name] = that symbol's full SymbolKey — the only
	// source a resolved edge's To is ever drawn from. "Top level" means an
	// unqualified Name (no "."): TSExtractor's qualifyTSDefs only dots a
	// callable's name when its immediate parent is a container, so a
	// module-scope function/class/variable always has a bare Name.
	topLevel map[string]map[string]string
	// qualified[file][name] is every symbol by its full Name (a member
	// "Y.Z" included) and keyFile its file — a dotted call through a
	// wildcard-imported type resolves to the member when the target
	// defines it (D7). declared[pkg] is the files declaring a package or
	// namespace, for namespace-dir languages whose namespaces need not
	// mirror directories (C#); types the namespace-dir qualified type names
	// (importResolver.declaredTypes).
	qualified map[string]map[string]string
	keyFile   map[string]string
	// keyFiles is every file a key is declared in: two files of one
	// directory declaring one name (a C# partial class, a Ruby class
	// reopened) share a SymbolKey, and keyFile keeps only the last.
	keyFiles map[string][]string
	symName  map[string]string
	// isType is every key whose canonical kind is a type (typeContainers):
	// the only names a package scope binds (matchPackage).
	isType   map[string]bool
	declared map[string][]string
	types    map[string][]string
	byFile   map[string][]core.SymbolNode

	// include-path (cproto.go): each file's parsed imports, its own includes
	// resolved, the function names a header declares, the external definers
	// of each name, and the prototype binding's counts.
	specs     map[string]extractor.FileSpecs
	incl      map[string][]string
	protoDecl map[string]map[string]bool
	definers  map[string][]string
	proto     *protoCounts
}

func newResolveIndex(r *importResolver, symbols []core.SymbolNode, fileSet map[string]bool) *resolveIndex {
	members := memberKeys(symbols)
	nameable := symbols
	if len(members) > 0 {
		nameable = make([]core.SymbolNode, 0, len(symbols))
		for _, s := range symbols {
			if !members[s.Key()] {
				nameable = append(nameable, s)
			}
		}
	}
	idx := &resolveIndex{
		topLevel:  map[string]map[string]string{},
		qualified: map[string]map[string]string{},
		keyFile:   make(map[string]string, len(symbols)),
		keyFiles:  make(map[string][]string, len(symbols)),
		isType:    map[string]bool{},
		symName:   make(map[string]string, len(symbols)),
		declared:  map[string][]string{},
		types:     r.declaredTypes(nameable, fileSet),
		byFile:    map[string][]core.SymbolNode{},
		proto:     newProtoCounts(),
	}
	for _, s := range symbols {
		idx.symName[s.Key()] = s.Name
		if c, ok := extractor.CanonicalKind(string(s.Kind)); ok && typeContainers[c] {
			idx.isType[s.Key()] = true
		}
		idx.byFile[s.FilePath] = append(idx.byFile[s.FilePath], s)
		q := idx.qualified[s.FilePath]
		if q == nil {
			q = map[string]string{}
			idx.qualified[s.FilePath] = q
		}
		if _, dup := q[s.Name]; !dup {
			q[s.Name] = s.Key()
		}
		idx.keyFile[s.Key()] = s.FilePath
		if fs := idx.keyFiles[s.Key()]; !slices.Contains(fs, s.FilePath) {
			idx.keyFiles[s.Key()] = append(fs, s.FilePath)
		}
		if s.Package != "" && fileSet[s.FilePath] {
			if fs := idx.declared[s.Package]; len(fs) == 0 || fs[len(fs)-1] != s.FilePath {
				idx.declared[s.Package] = append(fs, s.FilePath)
			}
		}
		if strings.Contains(s.Name, ".") || members[s.Key()] {
			continue
		}
		m := idx.topLevel[s.FilePath]
		if m == nil {
			m = map[string]string{}
			idx.topLevel[s.FilePath] = m
		}
		m[s.Name] = s.Key()
	}
	return idx
}

// fileScope is the names one file's imports bring into scope.
type fileScope struct {
	named       map[string]string   // LocalName -> resolved target SymbolKey
	whole       map[string]string   // LocalName -> resolved target FILE path
	typeAlias   map[string][]string // LocalName -> the keys of the type it aliases
	wild        map[string]string   // a wildcard-imported name -> its key
	staticNames map[string]bool     // the names a static import of a type added to wild
	wildFiles   []string            // the files a wildcard import brought (sorted, distinct)
	x           *exportIndex
}

// scopeOf binds path's imports (LB-F11; LB-F75: a name re-exported by the
// module it is imported from binds the definition it re-exports; LB-F78: a
// named import a namespace-dir module path does not name as a file binds
// the top-level name or the member of the package or class its path less
// the last segment names).
func (idx *resolveIndex) scopeOf(r *importResolver, x *exportIndex, path string, imports []extractor.ImportSpec) fileScope {
	sc := fileScope{named: map[string]string{}, whole: map[string]string{}, typeAlias: map[string][]string{}, x: x}
	var wildFiles []string   // files every top-level name of which is in scope
	var staticTypes []string // keys of the types whose members are in scope
	for _, spec := range imports {
		if spec.LocalName == "" {
			continue // side-effect-only import: no bound name to attribute a call to
		}
		switch spec.Kind {
		case extractor.ImportNamed:
			if key, ok := lookupExported(r.resolve(path, spec.Module), spec.ExportedName, x); ok {
				sc.named[spec.LocalName] = key
				continue
			}
			// "from X import Y" where Y is itself a submodule (X/Y.py or
			// X/Y/__init__.py), not a symbol X defines — retry as a
			// whole-module binding one level deeper.
			if sep := r.submoduleSep(); sep != "" {
				if t2 := r.resolve(path, spec.Module+sep+spec.ExportedName); len(t2) == 1 {
					sc.whole[spec.LocalName] = t2[0]
					continue
				}
			}
			if key, ok := idx.memberImport(r, path, spec); ok {
				sc.named[spec.LocalName] = key
			}
		case extractor.ImportWhole:
			if r.wholeIsWildcard() {
				wildFiles = append(wildFiles, r.resolve(path, spec.Module)...)
			} else if keys := r.aliasedType(idx.types, spec); len(keys) > 0 {
				sc.typeAlias[spec.LocalName] = keys
			} else if targets := r.wholeTargets(path, spec.Module, idx.declared); len(targets) == 1 {
				sc.whole[spec.LocalName] = targets[0]
			}
		case extractor.ImportWildcard:
			if fs := r.declaredFiles(idx.declared, spec.Module); len(fs) > 0 {
				wildFiles = append(wildFiles, fs...)
			} else if keys := r.typeKeys(idx.types, spec.Module); len(keys) > 0 && r.params.TypeWildcard == "members" {
				staticTypes = append(staticTypes, keys...) // C#'s using static N.T: T's members
			} else if len(keys) > 0 {
				wildFiles = append(wildFiles, filesOf(keys, idx.keyFile)...)
			} else {
				wildFiles = append(wildFiles, r.resolve(path, spec.Module)...)
			}
		}
	}
	if r.strategy == "include-path" {
		wildFiles = idx.includeClosure(r, wildFiles) // a header's own includes are in scope too
	}
	sc.wild, sc.staticNames = withStaticMembers(wildcardNames(wildFiles, x), staticTypes, idx.qualified, idx.keyFile, idx.symName)
	sort.Strings(wildFiles)
	sc.wildFiles = slices.Compact(wildFiles)
	return sc
}

// lookupExported is the first target file, in sorted order, that defines
// name at the top level or re-exports it (LB-F75).
func lookupExported(targets []string, name string, x *exportIndex) (string, bool) {
	for _, t := range targets {
		if key, ok := x.lookup(t, name); ok {
			return key, true
		}
	}
	return "", false
}

// memberImport is LB-F78's package/member split, for namespace-dir: a named
// import `a.b.n` whose module path names no file of its own binds n as the
// top-level name a file of package a.b defines (Kotlin's top-level function
// or property; a class declared in a file of another name), else as the
// member n of the class a.b names (Java's `import static a.b.Foo.bar`, a
// nested class).
func (idx *resolveIndex) memberImport(r *importResolver, importer string, spec extractor.ImportSpec) (string, bool) {
	files := idx.memberImportFiles(r, importer, spec)
	if key, ok := lookupNamed(files, spec.ExportedName, idx.topLevel); ok {
		return key, true
	}
	parent := memberImportParent(r, spec)
	last := parent
	if i := strings.LastIndex(parent, r.submoduleSep()); i >= 0 {
		last = parent[i+len(r.submoduleSep()):]
	}
	for _, k := range r.typeKeys(idx.types, parent) {
		if mk, ok := idx.qualified[idx.keyFile[k]][idx.symName[k]+"."+spec.ExportedName]; ok {
			return mk, true
		}
	}
	for _, f := range files {
		if mk, ok := idx.qualified[f][last+"."+spec.ExportedName]; ok {
			return mk, true
		}
	}
	return "", false
}

// memberImportFiles is the files a named import's module path less its last
// segment names (namespace-dir only): the strategy's, else the files
// declaring it as their package, else the files declaring it as a type.
func (idx *resolveIndex) memberImportFiles(r *importResolver, importer string, spec extractor.ImportSpec) []string {
	parent := memberImportParent(r, spec)
	if parent == "" {
		return nil
	}
	if fs := r.resolve(importer, parent); len(fs) > 0 {
		return fs
	}
	if fs := r.declaredFiles(idx.declared, parent); len(fs) > 0 {
		return fs
	}
	return filesOf(r.typeKeys(idx.types, parent), idx.keyFile)
}

// memberImportParent is a namespace-dir named import's module path less its
// last segment ("" when the strategy is another or the path has one).
func memberImportParent(r *importResolver, spec extractor.ImportSpec) string {
	if r.strategy != "namespace-dir" || spec.ExportedName == "" {
		return ""
	}
	sep := r.submoduleSep()
	parent, ok := strings.CutSuffix(spec.Module, sep+spec.ExportedName)
	if !ok {
		i := strings.LastIndex(spec.Module, sep)
		if i <= 0 {
			return ""
		}
		parent = spec.Module[:i]
	}
	return strings.Trim(parent, sep)
}

// callEdges resolves every call site of path's symbols (SU-11, LB-F11,
// LB-F67), then, for a call no import binds, the language's own scope
// rules: a constant looked up lexically (imports.constants "lexical",
// LB-F76) and a name of the caller's own package (LB-F74).
func (idx *resolveIndex) callEdges(r *importResolver, sc fileScope, path string, syms []core.SymbolNode) []core.Edge {
	var edges []core.Edge
	localTop := idx.topLevel[path]
	for _, s := range syms {
		if len(s.Calls) == 0 {
			continue
		}
		fromKey := s.Key()
		seen := map[string]bool{}
		for _, call := range s.Calls {
			call = strings.ReplaceAll(call, "::", ".") // PHP's Foo::bar (LB-F65) reads like Foo.bar
			if sc.staticNames[call] && enclosingTypeDeclares(s.Name, call, idx.qualified[path]) {
				continue // C#: the caller's own type is searched before any using static
			}
			toKey, ok, self := matchSelfMember(call, s.Name, idx.qualified[path])
			if self && !ok {
				continue // self.x binds to the enclosing class or to nothing, never a top-level x
			}
			if !ok && r.params.Constants == "lexical" && isConstantName(call) {
				toKey, ok = idx.matchLexicalConstant(call, s.Name, localTop, sc.wild)
			}
			if !ok {
				toKey, ok = matchFileResolvedCall(call, sc.named, sc.whole, sc.x, localTop, sc.wild)
			}
			if !ok {
				toKey, ok = matchTypeAlias(call, sc.typeAlias, idx.qualified, idx.keyFile, idx.symName)
			}
			if !ok {
				toKey, ok = matchWildMember(call, sc.wild, idx.qualified, idx.keyFile)
			}
			if !ok {
				toKey, ok = matchNamedMember(call, sc.named, idx.qualified, idx.keyFile, idx.symName)
			}
			if !ok && r.pkgScope && !enclosingTypeDeclares(s.Name, call, idx.qualified[path]) {
				toKey, ok = idx.matchPackage(r, call, s.Package)
			}
			if !ok {
				toKey, ok = idx.matchPrototype(r, sc, path, call)
			}
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
	return edges
}

// matchFileResolvedCall resolves one call-site text (e.g. "foo", "ns.foo",
// "pkg.util.other" — only the segment immediately past a whole-module
// binding's own prefix is consulted, see the package doc) against, in
// order: a named import binding (always a single bare identifier), a
// whole-module import binding (LocalName may itself be dotted — Python
// `import pkg.mod` — so matched by longest-prefix, not a fixed split;
// member access — a name the target defines or re-exports, LB-F75 — or a
// bare call through it when the target file has exactly one top-level
// symbol), a same-file module-scope sibling (localTop; nil for a
// reference, which never binds within its own file), and finally a name a
// wildcard import brought into scope (LB-F11).
func matchFileResolvedCall(call string, named, whole map[string]string, x *exportIndex, localTop, wild map[string]string) (string, bool) {
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
			// A bare use of a whole-module binding: the module's default
			// export (LB-F75), else its top-level definition of the bound
			// name (`import settle from './settle.js'`), else its sole one.
			if key, ok := x.lookup(target, "default"); ok {
				return key, true
			}
			syms := x.topLevelOnly(target)
			if key, ok := syms[call]; ok {
				return key, true
			}
			if len(syms) == 1 {
				for _, key := range syms {
					return key, true
				}
			}
		} else if key, ok2 := x.lookup(target, member); ok2 {
			return key, true
		}
	}

	if !hasDot {
		if key, ok := localTop[bareBase]; ok {
			return key, true
		}
		if key, ok := wild[bareBase]; ok {
			return key, true
		}
	}
	return "", false
}

// selfQualifiers are the receivers that name the caller's own class:
// Python's self and cls, PHP's $this, self:: and static:: (stored as this,
// self, static under calls.qualify, LB-F65) and TypeScript's this.
var selfQualifiers = map[string]bool{"self": true, "cls": true, "this": true, "static": true}

// matchSelfMember resolves a call "self.x…" (any selfQualifiers head) made
// in a method to the enclosing class's member x — the caller's name less
// its last segment, joined to the rest — in the caller's own file (LB-F65).
// self reports a self-qualified call, bound or not: one the class does not
// define (inherited, or a field) binds to nothing, never to a top-level x.
func matchSelfMember(call, caller string, local map[string]string) (key string, ok, self bool) {
	head, rest, dotted := strings.Cut(call, ".")
	if !dotted || !selfQualifiers[head] {
		return "", false, false
	}
	i := strings.LastIndexByte(caller, '.')
	if i < 0 {
		return "", false, true
	}
	key, ok = local[caller[:i]+"."+rest]
	return key, ok, true
}

// matchNamedMember resolves a dotted call "Y.Z…" whose head Y is a name a
// named import binds (Python's `from m import Repo` then `Repo.create()`,
// LB-F65): to the member "Y.Z" when the imported symbol's file defines it,
// else to Y itself — matchWildMember's rule for a named binding.
func matchNamedMember(call string, named map[string]string, qualified map[string]map[string]string, keyFile, symName map[string]string) (string, bool) {
	head, rest, dotted := strings.Cut(call, ".")
	if !dotted {
		return "", false
	}
	key, ok := named[head]
	if !ok {
		return "", false
	}
	member, _, _ := strings.Cut(rest, ".")
	if mk, ok := qualified[keyFile[key]][symName[key]+"."+member]; ok {
		return mk, true
	}
	return key, true
}

// matchWildMember resolves a dotted call "Y.Z…" whose head Y is a name a
// wildcard import brought into scope (C#'s `using X;` then `Y.Z()`, Java's
// `import a.*;` then `Y.z()`, owner decision D7): to the member "Y.Z" when
// Y's file defines it, else to Y itself — the same one-segment depth the
// whole-module path keeps.
func matchWildMember(call string, wild map[string]string, qualified map[string]map[string]string, keyFile map[string]string) (string, bool) {
	head, rest, dotted := strings.Cut(call, ".")
	if !dotted {
		return "", false
	}
	key, ok := wild[head]
	if !ok {
		return "", false
	}
	member, _, _ := strings.Cut(rest, ".")
	if mk, ok := qualified[keyFile[key]][head+"."+member]; ok {
		return mk, true
	}
	return key, true
}

// matchTypeAlias resolves a dotted call "A.M…" whose head A is a using
// alias of a type (C#'s `using A = N.T;`, O145): to T's member M when a
// file declaring T defines it, else to T itself.
func matchTypeAlias(call string, typeAlias map[string][]string, qualified map[string]map[string]string, keyFile, symName map[string]string) (string, bool) {
	head, rest, dotted := strings.Cut(call, ".")
	keys, ok := typeAlias[head]
	if !ok || !dotted {
		return "", false
	}
	member, _, _ := strings.Cut(rest, ".")
	for _, k := range keys {
		if mk, ok := qualified[keyFile[k]][symName[k]+"."+member]; ok {
			return mk, true
		}
	}
	return keys[0], true
}

// withStaticMembers adds to wild the members of the types a static import
// brought into scope (C#'s `using static N.T;`, O145), each bare member
// name mapped to its symbol, and returns the names it added; a name two of
// them define, or one wild already binds to another symbol, is ambiguous
// and left out.
func withStaticMembers(wild map[string]string, typeKeys []string, qualified map[string]map[string]string, keyFile, symName map[string]string) (map[string]string, map[string]bool) {
	if len(typeKeys) == 0 {
		return wild, nil
	}
	members := map[string]string{}
	ambiguous := map[string]bool{}
	for _, k := range typeKeys {
		prefix := symName[k] + "."
		for name, mk := range qualified[keyFile[k]] {
			m, ok := strings.CutPrefix(name, prefix)
			if !ok || m == "" || strings.Contains(m, ".") {
				continue
			}
			if prev, seen := members[m]; seen && prev != mk {
				ambiguous[m] = true
			}
			members[m] = mk
		}
	}
	if wild == nil {
		wild = map[string]string{}
	}
	added := map[string]bool{}
	for m, mk := range members {
		if prev, seen := wild[m]; ambiguous[m] || (seen && prev != mk) {
			delete(wild, m)
			continue
		}
		wild[m] = mk
		added[m] = true
	}
	return wild, added
}

// enclosingTypeDeclares reports whether a type enclosing caller (a member
// name "T.M", "Outer.Inner.M") declares member in the caller's file: C#'s
// simple-name lookup searches the enclosing types before any using directive,
// so a `using static` member of that name is not what the call means.
func enclosingTypeDeclares(caller, member string, qualified map[string]string) bool {
	for c := caller; ; {
		i := strings.LastIndex(c, ".")
		if i < 0 {
			return false
		}
		c = c[:i]
		if _, ok := qualified[c+"."+member]; ok {
			return true
		}
	}
}

// declaredTypes indexes, for namespace-dir only, every symbol of the
// universe declared in a package or namespace by its qualified name
// (package, separator, name: MyApp.Util.Helper, a nested type
// MyApp.Util.Outer.Inner) to its keys, sorted — so a spec naming a type
// resolves to the files declaring it, wherever they sit (a C# type need
// not have a file of its own name). nil for any other strategy.
func (r *importResolver) declaredTypes(symbols []core.SymbolNode, fileSet map[string]bool) map[string][]string {
	if r.strategy != "namespace-dir" {
		return nil
	}
	sep := r.submoduleSep()
	out := map[string][]string{}
	for _, s := range symbols {
		if s.Package == "" || s.Name == "" || !fileSet[s.FilePath] {
			continue
		}
		q := s.Package + sep + s.Name
		out[q] = append(out[q], s.Key())
	}
	for _, ks := range out {
		sort.Strings(ks)
	}
	return out
}

// typeKeys is the keys of the type module names (declaredTypes), a
// leading global:: dropped.
func (r *importResolver) typeKeys(types map[string][]string, module string) []string {
	if types == nil {
		return nil
	}
	module = strings.TrimPrefix(strings.TrimSpace(module), "global::")
	return types[strings.Trim(module, r.submoduleSep())]
}

// aliasedType is the keys of the type an aliased whole import names (C#'s
// `using A = N.T;`); nil when the import has no alias of its own or the
// module is no declared type.
func (r *importResolver) aliasedType(types map[string][]string, spec extractor.ImportSpec) []string {
	if spec.LocalName == "" || spec.LocalName == spec.Module {
		return nil
	}
	return r.typeKeys(types, spec.Module)
}

// wholeTargets is the files a whole-module import resolves to: the
// strategy's, else (namespace-dir) the files declaring module as their
// namespace, which need not mirror a directory (C#'s `using A = N;`).
func (r *importResolver) wholeTargets(importer, module string, declared map[string][]string) []string {
	if targets := r.resolve(importer, module); len(targets) > 0 {
		return targets
	}
	return r.declaredFiles(declared, module)
}

// filesOf is the sorted, distinct files of keys.
func filesOf(keys []string, keyFile map[string]string) []string {
	var out []string
	for _, k := range keys {
		if f := keyFile[k]; f != "" {
			out = append(out, f)
		}
	}
	sort.Strings(out)
	return slices.Compact(out)
}

// declaredFiles is, for namespace-dir only, the files whose symbols
// declare module as their package or namespace (sorted), so a namespace
// that does not mirror a directory still resolves (D7); nil otherwise.
func (r *importResolver) declaredFiles(declared map[string][]string, module string) []string {
	if r.strategy != "namespace-dir" {
		return nil
	}
	fs := append([]string(nil), declared[module]...)
	sort.Strings(fs)
	return fs
}

// lookupNamed is the first target file, in sorted order, defining name at
// the top level.
func lookupNamed(targets []string, name string, topLevel map[string]map[string]string) (string, bool) {
	for _, t := range targets {
		if key, ok := topLevel[t][name]; ok {
			return key, true
		}
	}
	return "", false
}

// wildcardNames is every name the wildcard-imported files define or
// re-export (LB-F75), each mapped to its symbol — a name two different
// files define (or two re-exports reach in different files) is ambiguous
// and left out, never guessed.
func wildcardNames(files []string, x *exportIndex) map[string]string {
	if len(files) == 0 {
		return nil
	}
	out := map[string]exportHit{}
	ambiguous := map[string]bool{}
	for _, f := range files {
		for name, h := range x.visibleNames(f) {
			if prev, ok := out[name]; ok && prev != h {
				ambiguous[name] = true
			}
			out[name] = h
		}
	}
	keys := make(map[string]string, len(out))
	for name, h := range out {
		if !ambiguous[name] {
			keys[name] = h.key
		}
	}
	return keys
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
	var headers []core.SourceRef // .h files: the language is a sniff of the text (CT-3)
	for _, r := range refs {
		if lang := fileResolveLangFor(r.Path); lang != "" {
			langFiles[lang] = append(langFiles[lang], r)
		} else if isHeaderPath(r.Path) {
			headers = append(headers, r)
		}
	}
	// LB-Q-c-rest (step 4): a .h goes to the language the header router
	// sends it to — the C++ sniff says yes: C++; no: C — so the file
	// resolver reads the files the extractor did, as the measurement
	// harness does. A header the router sent to the other language is still
	// read by an include-path language for its includes and prototypes.
	foreign := map[string][]core.SourceRef{}
	if cLang, cppLang := headerLangs(); cLang != "" || cppLang != "" {
		for _, r := range headers {
			b, err := p.conn.Fetch(ctx, r)
			if err != nil || len(b) > MaxExtractSize || connector.IsBinary(b) {
				continue
			}
			to, other := cLang, cppLang
			if extractor.LooksLikeCppHeader(b) {
				to, other = cppLang, cLang
			}
			if to != "" {
				langFiles[to] = append(langFiles[to], r)
			}
			if other != "" {
				foreign[other] = append(foreign[other], r)
			}
		}
	}
	if len(langFiles) == 0 {
		return total, nil
	}
	// RT-108: the resolution depends only on the TS/JS/Python files — when
	// none changed (new, changed, deleted) since the last successful pass,
	// the stored edges are the answer.
	if !passNeeded(changed, p.passStateOK(ctx, svc.ID, fileResolveStateKey), fileResolveInput) {
		total.Skipped = true
		total.SkipReason = SkippedPassReason
		return total, nil
	}
	p.beginPassState(ctx, svc.ID, fileResolveStateKey)

	// Repo manifests a strategy consults (Cargo.toml, composer.json, …) are
	// read through the connector, whatever their language.
	byPath := make(map[string]core.SourceRef, len(refs))
	for _, r := range refs {
		byPath[r.Path] = r
	}
	read := func(rel string) ([]byte, bool) {
		r, ok := byPath[rel]
		if !ok {
			return nil, false
		}
		b, err := p.conn.Fetch(ctx, r)
		return b, err == nil
	}
	langs, _ := extractor.FileResolveLangs()
	for _, fl := range langs { // sorted by Lang: deterministic order
		lang := fl.Lang
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
		r := newLangResolver(fl)
		edges, rep, err := p.resolveLang(ctx, svc, r, read, ext, lrefs, foreign[lang])
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
		total.CompileDBs += rep.CompileDBs
		total.AncestorIncludes += rep.AncestorIncludes
		total.ProtoBound += rep.ProtoBound
		total.ProtoAmbiguous += rep.ProtoAmbiguous
		total.ProtoNoDefiner += rep.ProtoNoDefiner
		total.ProtoNotBuilt += rep.ProtoNotBuilt
		total.ProtoBuildFiltered += rep.ProtoBuildFiltered
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

// extractImports parses one file's imports, references and re-exports in
// one parse (a var so a test can fault it).
var extractImports = func(ext *extractor.TSExtractor, path string, src []byte) (extractor.FileSpecs, error) {
	return ext.ExtractFileSpecs(path, src)
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
func (p *Pipeline) resolveLang(ctx context.Context, svc core.ServiceRef, r *importResolver, read func(string) ([]byte, bool), ext *extractor.TSExtractor, lrefs, foreign []core.SourceRef) ([]core.Edge, FileResolveReport, error) {
	var rep FileResolveReport
	lang := ext.Name()
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
	specs := make(map[string]extractor.FileSpecs, len(lrefs))
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
				// A row is the file's extractor.FileSpecs (LB-F74); one an
				// older binary wrote (a bare import list, under an older
				// ExtractorEpoch's fingerprint) does not decode and is
				// parsed again.
				var sp extractor.FileSpecs
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
	r.env = newResolveEnv(svc.Path, paths, read)
	if r.strategy == "include-path" {
		// Headers the router sent to the other language (cproto.go): read
		// for their includes and prototypes, never persisted, never bound.
		for _, fr := range foreign {
			b, ferr := p.conn.Fetch(ctx, fr)
			if ferr != nil || len(b) > MaxExtractSize {
				continue
			}
			if sp, ierr := extractImports(ext, fr.Path, b); ierr == nil {
				specs[fr.Path] = sp
				r.env.addForeign([]string{fr.Path})
			}
		}
	}
	edges, res := resolveImports(r, paths, specs, symbols)
	res.Reused, res.Gated = rep.Reused, rep.Gated
	res.Faults, res.FaultFiles = len(faults), firstN(faults, 5)
	return edges, res, nil
}

// fileResolveExtractor returns the pipeline's tree-sitter extractor for
// lang and a no-op release; when a test replaced it with another kind, a
// fresh one released by the returned func.
func (p *Pipeline) fileResolveExtractor(lang string) (*extractor.TSExtractor, func(), error) {
	for _, ext := range fileResolveExts()[lang] {
		if e, ok := p.exts[ext].(*extractor.TSExtractor); ok && e.Name() == lang {
			return e, func() {}, nil
		}
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
// TSExtractor registry key this pass resolves it under (a dialect whose
// manifest selects an import strategy, LB-F11), "" for anything else (the
// pipeline hook simply skips such files).
func fileResolveLangFor(relPath string) string {
	return fileResolveLangByExt()[strings.ToLower(path.Ext(relPath))]
}

// isHeaderPath reports whether relPath is a .h, the extension no dialect
// claims (the router sniffs it, CT-3).
func isHeaderPath(relPath string) bool { return strings.EqualFold(path.Ext(relPath), ".h") }

// headerLangs is the file-resolve languages a .h can be routed to: "c" and
// "cpp", each "" when no dialect of that name resolves files.
func headerLangs() (c, cpp string) {
	langs, err := extractor.FileResolveLangs()
	if err != nil {
		return "", ""
	}
	for _, l := range langs {
		switch l.Lang {
		case "c":
			c = "c"
		case "cpp":
			cpp = "cpp"
		}
	}
	return c, cpp
}

// fileResolveInput reports whether path is an input of the file resolver: a
// file of a file-resolve language, or a .h the header router may send to one.
func fileResolveInput(path string) bool {
	if fileResolveLangFor(path) != "" {
		return true
	}
	if isHeaderPath(path) {
		c, cpp := headerLangs()
		return c != "" || cpp != ""
	}
	return false
}

var (
	fileResolveOnce  sync.Once
	fileResolveByExt map[string]string
	fileResolveExtsM map[string][]string
)

func fileResolveIndex() {
	fileResolveOnce.Do(func() {
		fileResolveByExt, fileResolveExtsM = map[string]string{}, map[string][]string{}
		langs, err := extractor.FileResolveLangs()
		if err != nil {
			return // the dialect registry failed to load: nothing resolves
		}
		for _, l := range langs {
			for _, e := range l.Extensions {
				fileResolveByExt[e] = l.Lang
			}
			fileResolveExtsM[l.Lang] = l.Extensions
		}
	})
}

func fileResolveLangByExt() map[string]string { fileResolveIndex(); return fileResolveByExt }

// fileResolveExts is each file-resolve language's extensions, sorted.
func fileResolveExts() map[string][]string { fileResolveIndex(); return fileResolveExtsM }

// fileResolvedPackageKey names the typed_edges store key ResolveFileEdges'
// pipeline hook replaces wholesale each run for one language — namespaced
// so it can never collide with a Go pkg.ID or a CUE "cue:"-prefixed key
// sharing the same service.
// fileResolvedKeyPrefix namespaces the file resolver's typed-edge rows.
const fileResolvedKeyPrefix = "file-resolved:"

func fileResolvedPackageKey(lang string) string { return fileResolvedKeyPrefix + lang }
