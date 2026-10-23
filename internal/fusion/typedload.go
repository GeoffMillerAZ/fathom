package fusion

// The typed resolver reads only roots' Syntax and TypesInfo. Asking
// go/packages for NeedDeps together with NeedSyntax/NeedTypesInfo retains
// these for the entire import closure. Instead, load metadata with the same
// go list flags (-compiled=true -test=true -export=false -deps=true), then
// check the graph leaves first, following x/tools v0.52.0's loadPackage.
//
// Roots keep full function bodies, Info maps and shared ASTs. Dependencies
// keep complete types, but no Info or Syntax. Only immutable dependencies
// skip body checking: unlike the old loader, a body type error there no
// longer fails the roots. Declaration errors still fail them. First-party
// and replaced dependencies retain body checking and error propagation.

import (
	"context"
	"fmt"
	"go/ast"
	"go/scanner"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"golang.org/x/tools/go/packages"
)

func typedImmutableDependency(p *packages.Package, repoPath string, moduleMode bool) bool {
	if p.Module == nil {
		return moduleMode // only module mode identifies these as standard library
	}
	m := p.Module
	if m.Main || m.Replace != nil || m.Dir == "" {
		return false
	}
	rel, err := filepath.Rel(repoPath, m.Dir)
	// An unknown location cannot justify skipping body checks.
	return err == nil && !filepath.IsLocal(rel)
}

type typedLoadNode struct {
	p          *packages.Package
	root       bool
	importers  []*typedLoadNode
	unfinished atomic.Int32
}

type typedParseResult struct {
	ready chan struct{}
	file  *ast.File
	err   error
}

type typedSourceLoader struct {
	gov        *typedPassGovernor
	repoPath   string
	moduleMode bool
	fset       *token.FileSet
	cpu        chan struct{}
	rootFiles  map[string]bool
	parseMu    sync.Mutex
	// Per-load, roots only: variants share ASTs without retaining dependency
	// syntax in the parse memo after its type-check completes.
	parsed map[string]*typedParseResult
}

func loadTypedPackages(gov *typedPassGovernor, repoPath, dir string, patterns []string, env []string) ([]*packages.Package, *token.FileSet, error) {
	roots, err := typedPackagesLoad(&packages.Config{
		Context: gov.ctx, Dir: dir, Tests: true, Env: env,
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedDeps | packages.NeedModule | packages.NeedTypesSizes,
	}, patterns...)
	if err != nil {
		return nil, nil, err
	}
	ld := &typedSourceLoader{
		gov: gov, repoPath: repoPath, fset: token.NewFileSet(),
		cpu:       make(chan struct{}, runtime.GOMAXPROCS(0)),
		rootFiles: make(map[string]bool), parsed: make(map[string]*typedParseResult),
	}
	rootSet := make(map[*packages.Package]bool)
	for _, p := range roots {
		rootSet[p] = true
		ld.moduleMode = ld.moduleMode || p.Module != nil
		for _, name := range p.CompiledGoFiles {
			ld.rootFiles[name] = true
		}
	}
	// packages.Load has already removed invalid imports/cycles: Imports is
	// a DAG, including non-root variants introduced by external tests.
	nodes := make(map[*packages.Package]*typedLoadNode)
	var all []*typedLoadNode
	packages.Visit(roots, nil, func(p *packages.Package) {
		n := &typedLoadNode{p: p, root: rootSet[p]}
		nodes[p] = n
		all = append(all, n)
	})
	var leaves []*typedLoadNode
	for _, n := range all {
		for _, imp := range n.p.Imports {
			nodes[imp].importers = append(nodes[imp].importers, n)
			n.unfinished.Add(1)
		}
		if len(n.p.Imports) == 0 {
			leaves = append(leaves, n)
		}
	}
	var wg sync.WaitGroup
	var enqueue func(*typedLoadNode)
	enqueue = func(n *typedLoadNode) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ld.check(n)
			for _, importer := range n.importers {
				if importer.unfinished.Add(-1) == 0 {
					enqueue(importer)
				}
			}
		}()
	}
	for _, n := range leaves {
		enqueue(n)
	}
	wg.Wait()
	if gov.ctx.Err() != nil {
		return nil, nil, context.Cause(gov.ctx)
	}
	appendTypedRuntimeDiagnostics(gov.ctx, dir, env, roots)
	if gov.ctx.Err() != nil {
		return nil, nil, context.Cause(gov.ctx)
	}
	return roots, ld.fset, nil
}

func (ld *typedSourceLoader) acquireCPU() bool {
	select {
	case ld.cpu <- struct{}{}:
		if ld.gov.ctx.Err() != nil {
			<-ld.cpu
			return false
		}
		return true
	case <-ld.gov.ctx.Done():
		return false
	}
}

func (ld *typedSourceLoader) parse(name string) (*ast.File, error) {
	v := &typedParseResult{ready: make(chan struct{})}
	if ld.rootFiles[name] {
		ld.parseMu.Lock()
		if hit := ld.parsed[name]; hit != nil {
			ld.parseMu.Unlock()
			select {
			case <-hit.ready:
				return hit.file, hit.err
			case <-ld.gov.ctx.Done():
				return nil, context.Cause(ld.gov.ctx)
			}
		}
		ld.parsed[name] = v
		ld.parseMu.Unlock()
	}
	defer close(v.ready)
	if !ld.acquireCPU() {
		v.err = context.Cause(ld.gov.ctx)
		return nil, v.err
	}
	defer func() { <-ld.cpu }()
	// Bound reads as well as parsing: queued parsers must not hold all of
	// the import closure's source bytes while waiting for a CPU token.
	src, err := os.ReadFile(name)
	if err != nil {
		v.err = err
	} else {
		v.file, v.err = ld.gov.parseFile(ld.fset, name, src)
	}
	return v.file, v.err
}

func appendTypedLoadError(p *packages.Package, err error) {
	switch err := err.(type) {
	case packages.Error:
		p.Errors = append(p.Errors, err)
	case *os.PathError:
		p.Errors = append(p.Errors, packages.Error{Pos: err.Path + ":1", Msg: err.Err.Error(), Kind: packages.ParseError})
	case scanner.ErrorList:
		for _, e := range err {
			p.Errors = append(p.Errors, packages.Error{Pos: e.Pos.String(), Msg: e.Msg, Kind: packages.ParseError})
		}
	case types.Error:
		p.TypeErrors = append(p.TypeErrors, err)
		p.Errors = append(p.Errors, packages.Error{Pos: err.Fset.Position(err.Pos).String(), Msg: err.Msg, Kind: packages.TypeError})
	default:
		p.Errors = append(p.Errors, packages.Error{Pos: "-", Msg: err.Error(), Kind: packages.UnknownError})
	}
}

func (ld *typedSourceLoader) check(n *typedLoadNode) {
	p := n.p
	p.Fset = ld.fset
	if n.root {
		p.TypesInfo = newTypedLoadInfo()
	}
	if p.PkgPath == "unsafe" {
		p.Types = types.Unsafe
		if n.root {
			p.Syntax = []*ast.File{}
		}
		return
	}
	p.Types = types.NewPackage(p.PkgPath, p.Name)
	if ld.gov.ctx.Err() != nil {
		return
	}
	files := make([]*ast.File, len(p.CompiledGoFiles))
	errs := make([]error, len(p.CompiledGoFiles))
	var wg sync.WaitGroup
	for i, name := range p.CompiledGoFiles {
		wg.Add(1)
		go func() {
			defer wg.Done()
			files[i], errs[i] = ld.parse(name)
		}()
	}
	wg.Wait()
	// Preserve go/packages' CompiledGoFiles order and deterministic errors.
	syntax := make([]*ast.File, 0, len(files))
	for i, f := range files {
		if f != nil {
			syntax = append(syntax, f)
		}
		if errs[i] != nil {
			appendTypedLoadError(p, errs[i])
		}
	}
	if n.root {
		p.Syntax = syntax
	}
	if ld.gov.ctx.Err() != nil {
		return
	}
	tc := &types.Config{
		Importer: typedLoadImporter(func(path string) (*types.Package, error) {
			if path == "unsafe" {
				return types.Unsafe, nil
			}
			imp := p.Imports[path]
			if imp == nil {
				return nil, fmt.Errorf("import %q not loaded: import cycle or missing package (see go list errors)", path)
			}
			if imp.Types != nil && imp.Types.Complete() {
				return imp.Types, nil
			}
			return nil, fmt.Errorf("package %q has incomplete types", path)
		}),
		IgnoreFuncBodies: !n.root && typedImmutableDependency(p, ld.repoPath, ld.moduleMode),
		Error:            func(err error) { appendTypedLoadError(p, err) },
		Sizes:            p.TypesSizes,
	}
	if p.Module != nil && p.Module.GoVersion != "" {
		tc.GoVersion = "go" + p.Module.GoVersion
	}
	if !ld.acquireCPU() {
		return
	}
	typErr := types.NewChecker(tc, ld.fset, p.Types, p.TypesInfo).Files(syntax)
	<-ld.cpu
	// Go 1.21/1.22 returned this without invoking Config.Error (#66525).
	if typErr != nil && len(p.Errors) == 0 && len(syntax) > 0 {
		if msg := typErr.Error(); strings.HasPrefix(msg, "package requires newer Go version") {
			appendTypedLoadError(p, types.Error{Fset: ld.fset, Pos: syntax[0].Package, Msg: msg})
		}
	}
	if typErr != nil && len(p.Errors) == 0 {
		appendTypedLoadError(p, typErr)
	}
	p.IllTyped = len(p.Errors) > 0
	for _, imp := range p.Imports {
		p.IllTyped = p.IllTyped || imp.IllTyped
	}
}

type typedLoadImporter func(string) (*types.Package, error)

func (f typedLoadImporter) Import(path string) (*types.Package, error) { return f(path) }

func newTypedLoadInfo() *types.Info {
	return &types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue), Defs: make(map[*ast.Ident]types.Object),
		Uses: make(map[*ast.Ident]types.Object), Implicits: make(map[ast.Node]types.Object),
		Instances: make(map[*ast.Ident]types.Instance), Scopes: make(map[ast.Node]*types.Scope),
		Selections: make(map[*ast.SelectorExpr]*types.Selection), FileVersions: make(map[*ast.File]string),
	}
}

// Port go/packages' runtime/go-list skew diagnostic. Its response GoVersion
// is not exposed on Package, so repeat its release-tag query only for failed
// loads. An unavailable diagnostic never invents a version or hides errors.
func appendTypedRuntimeDiagnostics(ctx context.Context, dir string, env []string, roots []*packages.Package) {
	var failed []*packages.Package
	packages.Visit(roots, nil, func(p *packages.Package) {
		if p.PkgPath != "unsafe" && len(p.Errors) > 0 {
			failed = append(failed, p)
		}
	})
	var runtimeVersion int
	if len(failed) == 0 {
		return
	}
	if _, err := fmt.Sscanf(runtime.Version(), "go1.%d", &runtimeVersion); err != nil {
		return
	}
	cmd := exec.CommandContext(ctx, "go", "list", "-e", "-f", "{{context.ReleaseTags}}", "--", "unsafe")
	cmd.Dir = dir
	if env == nil {
		env = os.Environ()
	}
	cmd.Env = append(append([]string(nil), env...), "GO111MODULE=off")
	out, err := cmd.Output()
	if err != nil {
		return
	}
	var goVersion int
	for _, tag := range strings.Fields(strings.TrimSpace(strings.Trim(string(out), "[]\n\r "))) {
		var v int
		if _, err := fmt.Sscanf(tag, "go1.%d", &v); err == nil && v > goVersion {
			goVersion = v
		}
	}
	if runtimeVersion < goVersion {
		for _, p := range failed {
			appendTypedLoadError(p, packages.Error{
				Pos: "-", Kind: packages.UnknownError,
				Msg: fmt.Sprintf("This application uses version go1.%d of the source-processing packages but runs version go1.%d of 'go list'. It may fail to process source files that rely on newer language features. If so, rebuild the application using a newer version of Go.", runtimeVersion, goVersion),
			})
		}
	}
}
