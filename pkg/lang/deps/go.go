package deps

import (
	"context"
	"errors"
	"go/ast"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"path"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
)

const maxPackageEntries = 4096

type goRead struct {
	ctx  context.Context
	repo repositoryFS
	out  *result
}

type goConfig struct {
	dir       string
	manifest  *modfile.File
	ambiguous bool
}

func readGo(ctx context.Context, repo repositoryFS, src []byte, resolver Resolver, out *result) {
	g := goRead{ctx, repo, out}
	pos := token.NewFileSet()
	parsed, err := parser.ParseFile(pos, out.Path, src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		g.unknown(UnknownParseError, "", 0, DecideToolchain)
		return
	}
	dir := path.Dir(out.Path)
	if path.Ext(out.Path) != ".go" || goConditionalName(path.Base(out.Path)) {
		g.unknown(UnknownUnsupportedConstruct, "", 0, DecideToolchain, DecideConfigFile)
	}
	g.packageSources(dir, parsed.Name.Name, out.Path)
	if parsed.Name.Name == "main" {
		// Native PGO auto mode selects the profile next to the main package.
		g.optionalInput(KindBuildInput, path.Join(dir, "default.pgo"), ResolvedFile)
	}
	cfg := g.config(dir)

	hasEmbed := false
	for _, spec := range parsed.Imports {
		if ctx.Err() != nil {
			return
		}
		line := pos.PositionFor(spec.Path.Pos(), false).Line
		raw, err := strconv.Unquote(spec.Path.Value)
		if err != nil || raw == "C" || module.CheckImportPath(raw) != nil || !validPath(raw) {
			g.unknown(UnknownUnsupportedConstruct, raw, line, DecideToolchain)
			continue
		}
		hasEmbed = hasEmbed || raw == "embed"
		if resolver != nil {
			out.resolveExtension(ctx, resolver, ResolveRequest{
				Repo: repo, From: out.Path, Raw: raw, Kind: KindImport, Line: line,
			})
		} else {
			g.resolveImport(cfg, dir, raw, line)
		}
	}
	g.directives(parsed, pos, dir, hasEmbed)
}

func (g goRead) unknown(code UnknownCode, raw string, line int, sources ...DecidableBy) {
	g.out.addUnknown(Unknown{Code: code, Raw: raw, Line: line, DecidableBy: sources})
}

func (g goRead) edge(kind Kind, raw, to string, res Resolution, version string, line int) {
	g.out.addEdge(Edge{Kind: kind, Raw: raw, To: to, Resolution: res, Version: version, Line: line, Provenance: ProvenanceBuiltin})
}

func (g goRead) config(start string) goConfig {
	cfg := goConfig{}
	for dir := start; ; dir = path.Dir(dir) {
		if g.ctx.Err() != nil {
			return cfg
		}
		name := path.Join(dir, "go.work")
		if g.optionalInput(KindConfigRef, name, ResolvedFile) {
			g.optionalInput(KindConfigRef, path.Join(dir, "go.work.sum"), ResolvedFile)
			cfg.ambiguous = true
			g.unknown(UnknownUnsupportedConstruct, name, 0, DecideToolchain, DecideConfigFile)
		}
		if cfg.dir == "" {
			name := path.Join(dir, "go.mod")
			if g.optionalInput(KindConfigRef, name, ResolvedFile) {
				cfg.dir = dir
				src, code := readSource(g.repo, name)
				if code != "" {
					g.unknown(code, name, 0, DecideConfigFile)
				} else {
					f, err := modfile.Parse(name, src, nil)
					if err != nil || f.Module == nil || module.CheckImportPath(f.Module.Mod.Path) != nil {
						g.unknown(UnknownParseError, name, 0, DecideConfigFile)
					} else {
						cfg.manifest = f
						if len(f.Replace) > 0 {
							cfg.ambiguous = true
						}
						if len(f.Replace)+len(f.Require)+len(f.Exclude)+len(f.Godebug)+len(f.Ignore)+len(f.Tool) > 0 {
							g.unknown(UnknownUnsupportedConstruct, name, 0, DecideToolchain, DecideLockfile, DecideConfigFile)
						}
					}
				}
				g.optionalInput(KindConfigRef, path.Join(dir, "go.sum"), ResolvedFile)
				vendor := path.Join(dir, "vendor")
				if g.optionalInput(KindConfigRef, vendor, ResolvedDirectory) {
					g.optionalInput(KindConfigRef, path.Join(vendor, "modules.txt"), ResolvedFile)
					cfg.ambiguous = true
					g.unknown(UnknownUnsupportedConstruct, vendor, 0, DecideToolchain, DecideConfigFile)
				}
			}
		}
		if dir == "." {
			break
		}
	}
	if cfg.dir == "" {
		g.unknown(UnknownMissingConfig, "", 0, DecideConfigFile)
	}
	return cfg
}

// A present or uninspectable input can affect the build.
func (g goRead) optionalInput(kind Kind, name string, resolution Resolution) bool {
	info, err := g.repo.Stat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return false
	}
	if err != nil || (resolution == ResolvedFile && !info.Mode().IsRegular()) ||
		(resolution == ResolvedDirectory && !info.IsDir()) {
		g.unknown(UnknownMissingConfig, name, 0, DecideConfigFile)
		return true
	}
	g.edge(kind, name, name, resolution, "", 0)
	return true
}

func (g goRead) resolveImport(cfg goConfig, fromDir, raw string, line int) {
	if cfg.ambiguous || cfg.manifest == nil {
		g.unknown(UnknownUnresolvedAlias, raw, line, DecideToolchain, DecideConfigFile)
		return
	}
	if suffix, local := importSuffix(raw, cfg.manifest.Module.Mod.Path); local {
		for _, req := range cfg.manifest.Require {
			if _, overlap := importSuffix(raw, req.Mod.Path); overlap {
				g.unknown(UnknownUnresolvedAlias, raw, line, DecideToolchain, DecideConfigFile)
				return
			}
		}
		target := path.Join(cfg.dir, suffix)
		if target != fromDir && g.withinPackage(cfg.dir, target) && g.packageSources(target, "", "") {
			g.edge(KindImport, raw, target, ResolvedDirectory, "", line)
		} else {
			g.unknown(UnknownUnresolvedAlias, raw, line, DecideToolchain, DecideConfigFile)
		}
		return
	}
	// A go.mod requirement gives a minimum and go.sum records integrity.
	// Neither proves which version provides this package under module selection.
	g.edge(KindImport, raw, "go:"+raw, ResolvedExternal, "", line)
}

func importSuffix(operand, prefix string) (string, bool) {
	if operand == prefix {
		return "", true
	}
	return strings.CutPrefix(operand, prefix+"/")
}

// withinPackage rules out symlinks and module boundaries below base, including
// the target directory. Paths have already been checked as logical identities.
func (g goRead) withinPackage(base, dir string) bool {
	for dir != base {
		info, err := g.repo.Stat(dir)
		if err != nil || !info.IsDir() {
			return false
		}
		_, err = g.repo.Stat(path.Join(dir, "go.mod"))
		if !errors.Is(err, fs.ErrNotExist) {
			return false
		}
		dir = path.Dir(dir)
	}
	return true
}

// packageSources inspects a single package directory. When current is set it
// records sibling build inputs. An import target needs at least one ordinary
// non-main source file and gives its directory identity.
func (g goRead) packageSources(dir, wantPackage, current string) bool {
	entries, code := packageEntries(g.repo, dir)
	if code != "" {
		g.unknown(code, dir, 0, DecideObservation)
		return false
	}
	found := false
	for _, e := range entries {
		if g.ctx.Err() != nil {
			return false
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
			continue
		}
		full := path.Join(dir, name)
		if full == current || e.IsDir() {
			continue
		}
		if !strings.HasSuffix(name, ".go") {
			if nativeCompanion(name) {
				g.unknown(UnknownUnsupportedConstruct, full, 0, DecideToolchain)
			}
			continue
		}
		if !validPath(full) || goConditionalName(name) || !e.Type().IsRegular() {
			g.unknown(UnknownUnsupportedConstruct, full, 0, DecideToolchain, DecideConfigFile)
			continue
		}
		src, code := readSource(g.repo, full)
		if code != "" {
			g.unknown(code, full, 0, DecideObservation)
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), full, src, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			g.unknown(UnknownParseError, full, 0, DecideToolchain)
			continue
		}
		if wantPackage == "" {
			wantPackage = f.Name.Name
		}
		if hasBuildConstraint(f) || hasCgoImport(f) || f.Name.Name != wantPackage || (current == "" && f.Name.Name == "main") {
			g.unknown(UnknownUnsupportedConstruct, full, 0, DecideToolchain, DecideConfigFile)
			continue
		}
		found = true
		if current != "" {
			g.edge(KindBuildInput, full, full, ResolvedFile, "", 0)
		}
	}
	return found
}

func packageEntries(repo repositoryFS, name string) ([]fs.DirEntry, UnknownCode) {
	info, err := repo.Stat(name)
	if err != nil || !info.IsDir() {
		return nil, UnknownMissingConfig
	}
	f, err := repo.Open(name)
	if err != nil {
		return nil, UnknownMissingConfig
	}
	defer f.Close()
	d, ok := f.(fs.ReadDirFile)
	if !ok {
		return nil, UnknownMissingConfig
	}
	var entries []fs.DirEntry
	for len(entries) <= maxPackageEntries {
		more, err := d.ReadDir(maxPackageEntries + 1 - len(entries))
		entries = append(entries, more...)
		if len(entries) > maxPackageEntries {
			return nil, UnknownReadLimit
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil || len(more) == 0 {
			return nil, UnknownMissingConfig
		}
	}
	slices.SortFunc(entries, func(a, b fs.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
	return entries, ""
}

func hasBuildConstraint(file *ast.File) bool {
	for _, group := range file.Comments {
		for _, c := range group.List {
			if constraint.IsGoBuild(c.Text) || constraint.IsPlusBuild(c.Text) {
				return true
			}
		}
	}
	return false
}

func hasCgoImport(file *ast.File) bool {
	for _, spec := range file.Imports {
		if raw, _ := strconv.Unquote(spec.Path.Value); raw == "C" {
			return true
		}
	}
	return false
}

// Filename suffixes used by Go's build matching, including historical targets.
var goTargets = strings.Fields("aix android darwin dragonfly freebsd hurd illumos ios js linux nacl netbsd openbsd plan9 solaris wasip1 windows zos " +
	"386 amd64 amd64p32 arm armbe arm64 arm64be loong64 mips mipsle mips64 mips64le mips64p32 mips64p32le ppc ppc64 ppc64le riscv riscv64 s390 s390x sparc sparc64 wasm")

func goConditionalName(name string) bool {
	base := strings.TrimSuffix(name, ".go")
	i := strings.LastIndexByte(base, '_')
	return strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") ||
		(i >= 0 && (base[i+1:] == "test" || slices.Contains(goTargets, base[i+1:])))
}

func nativeCompanion(name string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".c", ".cc", ".cpp", ".cxx", ".m", ".h", ".hh", ".hpp", ".hxx",
		".f", ".for", ".f90", ".s", ".syso", ".swig", ".swigcxx":
		return true
	}
	return false
}

func (g goRead) directives(f *ast.File, pos *token.FileSet, dir string, hasEmbed bool) {
	// KindEmbed belongs to a standalone var or a var spec. Flag directives elsewhere
	// explicitly, even when the parser accepts the surrounding Go syntax.
	embeds := make(map[*ast.CommentGroup]bool)
	for _, decl := range f.Decls {
		v, ok := decl.(*ast.GenDecl)
		if !ok || v.Tok != token.VAR {
			continue
		}
		for _, s := range v.Specs {
			spec := s.(*ast.ValueSpec)
			if len(spec.Names) == 1 && len(spec.Values) == 0 && spec.Type != nil {
				embeds[spec.Doc] = true
				if !v.Lparen.IsValid() {
					embeds[v.Doc] = true
				}
			}
		}
	}
	for _, group := range f.Comments {
		for _, comment := range group.List {
			if g.ctx.Err() != nil {
				return
			}
			line := pos.PositionFor(comment.Pos(), false).Line
			d, ok := ast.ParseDirective(comment.Pos(), comment.Text)
			switch {
			case ok && d.Tool == "go" && d.Name == "embed":
				args, err := d.ParseArgs()
				if !hasEmbed || !embeds[group] || err != nil || len(args) == 0 {
					g.unknown(UnknownUnsupportedConstruct, "", line, DecideToolchain)
					continue
				}
				for _, arg := range args {
					g.resolveEmbed(dir, arg.Arg, pos.PositionFor(arg.Pos, false).Line)
				}
			case strings.HasPrefix(comment.Text, "//go:") || constraint.IsPlusBuild(comment.Text):
				g.unknown(UnknownUnsupportedConstruct, "", line, DecideToolchain, DecideConfigFile)
			}
		}
	}
}

func (g goRead) resolveEmbed(dir, raw string, line int) {
	name := strings.TrimPrefix(raw, "all:")
	if strings.ContainsAny(name, "*?[\\") {
		g.unknown(UnknownUnboundedGlob, raw, line, DecideToolchain)
		return
	}
	if !validPath(name) || module.CheckFilePath(name) != nil {
		g.unknown(UnknownUnsupportedConstruct, raw, line, DecideToolchain)
		return
	}
	for _, part := range strings.Split(name, "/") {
		switch part {
		case ".git", ".hg", ".svn", ".bzr", "vendor":
			g.unknown(UnknownUnsupportedConstruct, raw, line, DecideToolchain)
			return
		}
	}
	target := path.Join(dir, name)
	info, err := g.repo.Stat(target)
	switch {
	case err != nil:
		g.unknown(UnknownMissingConfig, raw, line, DecideObservation)
	case info.IsDir():
		g.unknown(UnknownUnboundedGlob, raw, line, DecideToolchain)
	case !info.Mode().IsRegular() || !g.withinPackage(dir, path.Dir(target)):
		g.unknown(UnknownUnsupportedConstruct, raw, line, DecideToolchain)
	default:
		g.edge(KindEmbed, raw, target, ResolvedFile, "", line)
	}
}
