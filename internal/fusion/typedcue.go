// CUE typed edge enrichment (ADR-041 decision 5, docs/plans/substrate.md
// SU-10). This is the second producer of the shared typed_edges table (the
// first is internal/fusion/typedcalls.go's Go pass) — package-granular,
// additive, never blocking ingest.
//
// Unlike Go, no SCIP exporter exists for CUE, so cue/load's own package
// resolution (cue.mod-aware import paths, multi-file packages) is the
// oracle: EnrichCUEEdges loads every CUE package under repoPath with
// cue/load (which already understands cue.mod, so it walks module
// boundaries the way go/packages walks go.mod ones — a nested cue.mod, such
// as a testdata fixture module, is its own instance tree and is not folded
// into the parent's), then for every TOP-LEVEL field (a "#Name" definition
// or a plain field) walks its value expression for CUE reference shapes:
//
//   - unification: `#Kit: #Block & {...}` — an edge to #Block.
//   - field type references: `profile: #Profile` — the value IS the ref.
//   - list element types: `uses: [...#Block]` — the Ellipsis element type.
//   - cross-package selectors: `blocks.#Block`, where "blocks" is bound by
//     an import — resolved via build.Instance.LookupImport, never a
//     string-matched guess.
//
// A same-package (same or different file) definition resolves to its
// SymbolKey and becomes core.EdgeDirect; a reference this pass cannot
// settle (an unbound identifier, an import that failed to load, a name the
// target package does not define) becomes core.EdgeUnresolved — never
// silently dropped, mirroring the Go pass's honesty discipline.
//
// Scope, deliberately narrower than the Go pass (documented, not a gap
// pretending otherwise): only TOP-LEVEL fields are walked (matching
// cueextract.go's own localDefs scope — a nested field's dotted key is not
// a stable enough handle to re-derive here); connector-scope filtering
// (allowed) is applied per PACKAGE (a package with zero allowed files is
// skipped) rather than per file within a partially-allowed package; no
// content-hash cache — cue/load's own parse is cheap enough on the
// calibration subjects (this repo's own schema/, .fathom/) that a
// per-package fingerprint cache was not worth the complexity this round.
//
// Store keys: a CUE package's typed-edge rows are stored under
// "cue:"+<id>, never the bare id a Go pkg.ID or a future producer (SCIP
// import, the TS/JS/Python file resolver) might also use — the shared
// typed_edges table is keyed by (serviceID, pkgPath) and PutTypedEdges
// REPLACES that key's rows wholesale, so two producers sharing a directory
// (a package with both .go and .cue sources, or a nested cue.mod matching
// its parent's Go import path) must never collide. The reconcile pass below
// purges only "cue:"-prefixed rows this run did not produce, leaving every
// other producer's rows untouched.
package fusion

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/literal"
	"cuelang.org/go/cue/load"
	"cuelang.org/go/cue/token"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// cuePackageKeyPrefix namespaces every CUE-produced typed-edge package row
// (see the package doc's Store keys note).
const cuePackageKeyPrefix = "cue:"

// hasCUESource reports whether refs contains at least one ".cue" file.
func hasCUESource(refs []core.SourceRef) bool {
	for _, r := range refs {
		if strings.HasSuffix(r.Path, ".cue") {
			return true
		}
	}
	return false
}

// hasCUEModule reports whether repoPath has a cue.mod/module.cue — cue/load
// requires a module to resolve import paths, exactly as go/packages
// requires go.mod.
func hasCUEModule(repoPath string) bool {
	if repoPath == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(repoPath, "cue.mod", "module.cue"))
	return err == nil
}

// enrichCUEEdges is the fusion.Pipeline.Ingest call-site hook, the CUE
// sibling of enrichTypedEdges: best-effort, never aborts or fails the
// surrounding Ingest.
//
// RT-105 (typedgate.go): it runs only when a .cue input is in changed or no
// successful CUE pass is recorded, and under the process-wide heavy
// toolchain slot the typed Go pass holds (typedPassSem), so the two never
// load at once.
func (p *Pipeline) enrichCUEEdges(ctx context.Context, svc core.ServiceRef, refs []core.SourceRef, changed []string) (TypedEdgeReport, error) {
	if p.cfg.DisableTypedEdges || !hasCUESource(refs) || !hasCUEModule(svc.Path) {
		return TypedEdgeReport{}, nil
	}
	if !cuePassNeeded(changed, p.passStateOK(ctx, svc.ID, cuePassStateKey)) {
		return TypedEdgeReport{Skipped: SkippedPassReason}, nil
	}
	release, err := acquireTypedPassSlot(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fusion: CUE typed-edge pass %q: %v while waiting for the process-wide typed-pass slot — syntax-only CUE edges remain the baseline\n", svc.ID, err)
		return TypedEdgeReport{}, haltError(ctx, err)
	}
	defer release()
	atomic.AddInt64(&p.cuePassTotal, 1)
	p.beginPassState(ctx, svc.ID, cuePassStateKey)
	allowed := make(map[string]bool)
	for _, r := range refs {
		if strings.HasSuffix(r.Path, ".cue") {
			allowed[r.Path] = true
		}
	}
	rep, err := EnrichCUEEdges(ctx, p.st, svc.ID, svc.Path, allowed)
	switch {
	case err != nil:
		fmt.Fprintf(os.Stderr, "fusion: CUE typed-edge pass %q failed to load: %v — syntax-only CUE edges remain the baseline\n", svc.ID, err)
		if hErr := haltError(ctx, err); hErr != nil {
			return rep, hErr
		}
	case rep.PackagesFailed > 0:
		fmt.Fprintf(os.Stderr, "fusion: CUE typed-edge pass %q: %d/%d package(s) failed typed loading\n", svc.ID, rep.PackagesFailed, rep.PackagesTotal)
	default:
		p.commitPassState(ctx, svc.ID, cuePassStateKey)
	}
	return rep, nil
}

// CUEPassTotal returns the process-lifetime count of CUE passes this
// Pipeline actually ran (past the RT-105 gate).
func (p *Pipeline) CUEPassTotal() int64 { return atomic.LoadInt64(&p.cuePassTotal) }

// cuePkgInfo is one loaded CUE package's typed-edge working state.
type cuePkgInfo struct {
	inst    *build.Instance
	dir     string // repo-relative, forward-slash
	qualPkg string // core.QualifiedPackage(dir, PkgName)
	// defs maps a top-level "#Name" definition to its SymbolKey — built
	// from every file in the instance, so a cross-FILE same-package
	// reference resolves exactly like a same-file one.
	defs map[string]string
	// allowedHere is false when the connector scope excludes every file in
	// this package (nothing to walk, nothing to keep on reconcile).
	allowedHere bool
}

// EnrichCUEEdges loads every CUE package under repoPath with cue/load and
// persists typed core.Edge rows through st.PutTypedEdges, one call per CUE
// package. See the package doc for the full design and its documented
// scope limits.
//
// allowed is the connector-scope filter (repo-relative forward-slash .cue
// paths, as SourceRef.Path records them): a package with zero allowed files
// is skipped entirely. nil disables filtering (whole-repo scope) — used by
// the pipeline's own tests and by a caller operating on a self-contained
// fixture module.
//
// The returned error is non-nil only when repoPath carries no loadable CUE
// module at all; a per-package failure (inst.Err) is counted in the report,
// never returned as an error, matching EnrichTypedEdges' contract.
func EnrichCUEEdges(ctx context.Context, st store.Store, serviceID, repoPath string, allowed map[string]bool) (TypedEdgeReport, error) {
	start := time.Now()
	var report TypedEdgeReport

	if abs, absErr := filepath.Abs(repoPath); absErr == nil {
		repoPath = abs
	}
	if !hasCUEModule(repoPath) {
		report.Duration = time.Since(start)
		return report, fmt.Errorf("fusion: CUE typed-edge load %q: no cue.mod/module.cue", repoPath)
	}

	cfg := &load.Config{
		Dir:     repoPath,
		Package: "*", // every package; files with no package clause -> "_"
		// Hermetic: this pass never needs a registry lookup (first-party
		// intra-module imports and stdlib-only builtin packages are
		// resolved locally); an inline-empty registry keeps a broken or
		// slow $CUE_REGISTRY in the caller's real environment from ever
		// reaching this load, mirroring TypedEdgeEnv's GOWORK=off posture.
		Env: []string{"CUE_REGISTRY=inline:{}"},
	}
	insts := load.Instances([]string{"./..."}, cfg)
	report.PackagesTotal = len(insts)
	if len(insts) == 0 {
		report.Duration = time.Since(start)
		return report, nil
	}

	// Pass 1: build every package's def table BEFORE resolving any edges —
	// cross-package references need every table populated first, and a
	// same-package cross-file reference needs its own package's table
	// complete before that package's edges are walked.
	// infos is keyed by build.Instance.Dir (an absolute path), NOT by
	// *build.Instance pointer: cue/load hands a DEPENDENCY reference
	// (build.Instance.Imports, what LookupImport returns) a freshly built
	// Instance distinct from the one load.Instances' own top-level slice
	// returns for that same package directory — same Dir, same Files,
	// different pointer. Keying by Dir makes resolveImported's lookup
	// below find the top-level instance's (already fully built) def table
	// regardless of which pointer identity cue/load handed back.
	infos := make(map[string]*cuePkgInfo, len(insts))
	for _, inst := range insts {
		info := &cuePkgInfo{inst: inst}
		info.dir = repoRelSlash(repoPath, inst.Dir)
		info.qualPkg = cueQualifiedPackage(info.dir, inst.PkgName)
		info.allowedHere = allowed == nil || anyCUEFileAllowed(repoPath, allowed, inst.Files)
		info.defs = map[string]string{}
		for _, f := range inst.Files {
			for _, d := range f.Decls {
				field, ok := d.(*ast.Field)
				if !ok {
					continue
				}
				name, _, lerr := ast.LabelName(field.Label)
				if lerr != nil || !strings.HasPrefix(name, "#") {
					continue
				}
				info.defs[name] = core.SymbolKey(serviceID, info.qualPkg, name)
			}
		}
		infos[inst.Dir] = info
	}

	keep := make(map[string]bool, len(insts))
	for _, inst := range insts {
		info := infos[inst.Dir]
		if !info.allowedHere {
			report.PackagesSkipped++
			continue
		}
		pkgKey := cuePackageKeyPrefix + cuePkgID(inst)
		keep[pkgKey] = true

		if inst.Err != nil {
			report.PackagesFailed++
			continue
		}

		var edges []core.Edge
		for _, f := range inst.Files {
			fromFile := repoRelSlash(repoPath, f.Filename)
			imports := cueFileImportPaths(f)
			for _, d := range f.Decls {
				field, ok := d.(*ast.Field)
				if !ok {
					continue
				}
				name, _, lerr := ast.LabelName(field.Label)
				if lerr != nil || name == "" {
					continue
				}
				fromKey := core.SymbolKey(serviceID, info.qualPkg, name)
				line := field.Pos().Line()
				edges = append(edges, resolveCUEFieldRefs(serviceID, fromKey, fromFile, line, field.Value, info, infos, imports)...)
			}
		}
		sortEdges(edges)

		if putErr := st.PutTypedEdges(ctx, serviceID, pkgKey, edges); putErr != nil {
			report.PackagesFailed++
			continue
		}
		report.PackagesLoaded++
		for _, e := range edges {
			switch e.Kind {
			case core.EdgeDirect:
				report.EdgesDirect++
			case core.EdgeUnresolved:
				report.EdgesUnresolved++
			}
		}
	}

	// Reconcile: purge stored "cue:" rows this run did not produce (deleted
	// packages, or packages the connector now ignores). Skipped on
	// wholesale failure, mirroring EnrichTypedEdges.
	wholesaleFailure := report.PackagesFailed > 0 && report.PackagesLoaded == 0
	if !wholesaleFailure {
		if stored, listErr := st.ListTypedEdgePackages(ctx, serviceID); listErr == nil {
			for _, id := range stored {
				if !strings.HasPrefix(id, cuePackageKeyPrefix) || keep[id] {
					continue
				}
				if delErr := st.DeleteTypedEdgePackage(ctx, serviceID, id); delErr != nil {
					report.PackagesFailed++
				}
			}
		}
	}

	report.Duration = time.Since(start)
	return report, nil
}

// resolveCUEFieldRefs walks one top-level field's value expression for the
// CUE reference shapes documented at the top of this file, resolving each
// to a core.Edge from fromKey. Unlike cueextract.go's cueReferenceEdges
// (which returns bare/short-form callee strings for the generic
// callgraph resolver), every reference here is settled to a full,
// package-load-confirmed target or explicitly marked EdgeUnresolved.
func resolveCUEFieldRefs(serviceID, fromKey, fromFile string, line int, value ast.Expr, self *cuePkgInfo, infos map[string]*cuePkgInfo, imports map[string]string) []core.Edge {
	if value == nil {
		return nil
	}
	var edges []core.Edge
	unresolved := func() {
		edges = append(edges, core.Edge{
			FromService: serviceID,
			FromFile:    fromFile,
			FromSymbol:  fromKey,
			ToSymbol:    fmt.Sprintf("<unresolved-cue:%d>", line),
			Kind:        core.EdgeUnresolved,
		})
	}
	resolveLocal := func(name string) {
		if key, ok := self.defs[name]; ok {
			edges = append(edges, core.Edge{FromService: serviceID, FromFile: fromFile, FromSymbol: fromKey, ToSymbol: key, Kind: core.EdgeDirect})
			return
		}
		unresolved()
	}
	resolveImported := func(base, name string) {
		impPath, ok := imports[base]
		if !ok {
			return // not an import identifier at all — a local struct-path segment, not a reference
		}
		target := self.inst.LookupImport(impPath)
		if target == nil {
			unresolved()
			return
		}
		tinfo, ok := infos[target.Dir]
		if !ok {
			unresolved()
			return
		}
		if key, ok := tinfo.defs[name]; ok {
			edges = append(edges, core.Edge{FromService: serviceID, FromFile: fromFile, FromSymbol: fromKey, ToSymbol: key, Kind: core.EdgeDirect})
			return
		}
		unresolved()
	}

	var walk func(e ast.Expr)
	walk = func(e ast.Expr) {
		switch v := e.(type) {
		case *ast.Ident:
			if strings.HasPrefix(v.Name, "#") {
				resolveLocal(v.Name)
			}
		case *ast.SelectorExpr:
			name, _, lerr := ast.LabelName(v.Sel)
			if lerr != nil || !strings.HasPrefix(name, "#") {
				return
			}
			ident, ok := v.X.(*ast.Ident)
			if !ok {
				return
			}
			resolveImported(ident.Name, name)
		case *ast.BinaryExpr:
			if v.Op == token.AND {
				walk(v.X)
				walk(v.Y)
			}
		case *ast.ListLit:
			for _, elt := range v.Elts {
				if ell, ok := elt.(*ast.Ellipsis); ok && ell.Type != nil {
					walk(ell.Type)
				}
			}
		}
	}
	walk(value)
	return edges
}

// cueFileImportPaths maps a file's import identifiers (explicit alias, or
// unaliased — the canonical last segment used at a selector base) to the
// unquoted import PATH itself (not just its last segment — see
// cueextract.go's cueImports, which only needs the segment; this pass needs
// the real path to call build.Instance.LookupImport).
func cueFileImportPaths(f *ast.File) map[string]string {
	out := map[string]string{}
	for imp := range f.ImportSpecs() {
		if imp.Path == nil || imp.Path.Value == "" {
			continue
		}
		p := imp.Path.Value
		if trimmed := strings.TrimLeft(p, "#"); len(trimmed) > 0 && (trimmed[0] == '"' || trimmed[0] == '\'') {
			if unquoted, err := literal.Unquote(p); err == nil {
				p = unquoted
			}
		}
		if p == "" {
			continue
		}
		seg := path.Base(p)
		ident := seg
		if imp.Name != nil && imp.Name.Name != "" && imp.Name.Name != "_" {
			ident = imp.Name.Name
		}
		if ident != "" {
			out[ident] = p
		}
	}
	return out
}

// cueQualifiedPackage mirrors core.QualifiedPackage for a CUE package
// addressed by its instance directory rather than one file's path.
func cueQualifiedPackage(dir, pkg string) string {
	return core.QualifiedPackage(path.Join(dir, "x.cue"), pkg)
}

// cuePkgID names a CUE build.Instance for the typed-edge store's
// package-granularity key, before the "cue:" namespace prefix is applied:
// the instance's own ImportPath when cue/load resolved one (the common
// case under a real module), else a directory+package fallback (a
// module-relative instance with no importable path, e.g. one whose only
// files declare no package).
func cuePkgID(inst *build.Instance) string {
	if inst.ImportPath != "" {
		return inst.ImportPath
	}
	return inst.Dir + ":" + inst.PkgName
}

// anyCUEFileAllowed reports whether at least one of files maps to a
// repo-relative path in the allowed set.
func anyCUEFileAllowed(repoPath string, allowed map[string]bool, files []*ast.File) bool {
	for _, f := range files {
		if allowed[repoRelSlash(repoPath, f.Filename)] {
			return true
		}
	}
	return false
}
