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
	repo fs.FS
	out  *result
}

type goConfig struct {
	dir       string
	manifest  *modfile.File
	ambiguous bool
}

func readGo(ctx context.Context, repo fs.FS, src []byte, resolver Resolver, out *result) {
	g := goRead{ctx, repo, out}
	pos := token.NewFileSet()
	parsed, err := parser.ParseFile(pos, out.Path, src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		g.unknown(ParseError, "", 1, Toolchain)
		return
	}
	dir := path.Dir(out.Path)
	if path.Ext(out.Path) != ".go" || goConditionalName(path.Base(out.Path)) {
		g.unknown(UnsupportedConstruct, "", 1, Toolchain, ConfigFile)
	}
	g.packageSources(dir, parsed.Name.Name, out.Path)
	cfg := g.config(dir)

	hasEmbed := false
	for _, spec := range parsed.Imports {
		if ctx.Err() != nil {
			return
		}
		line := pos.PositionFor(spec.Path.Pos(), false).Line
		raw, err := strconv.Unquote(spec.Path.Value)
		if err != nil || raw == "C" || module.CheckImportPath(raw) != nil || !validPath(raw) {
			g.unknown(UnsupportedConstruct, raw, line, Toolchain)
			continue
		}
		hasEmbed = hasEmbed || raw == "embed"
		if resolver != nil {
			out.resolveExtension(repo, resolver, raw, line)
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
	g.out.addEdge(Edge{Kind: kind, Raw: raw, To: to, Resolution: res, Version: version, Line: line, Provenance: Builtin})
}

func (g goRead) config(start string) goConfig {
	cfg := goConfig{}
	for dir := start; ; dir = path.Dir(dir) {
		if g.ctx.Err() != nil {
			return cfg
		}
		name := path.Join(dir, "go.work")
		if g.configInput(name, File) {
			g.configInput(path.Join(dir, "go.work.sum"), File)
			cfg.ambiguous = true
			g.unknown(UnsupportedConstruct, name, 1, Toolchain, ConfigFile)
		}
		if cfg.dir == "" {
			name := path.Join(dir, "go.mod")
			if g.configInput(name, File) {
				cfg.dir = dir
				src, code := readSource(g.repo, name)
				if code != "" {
					g.unknown(code, name, 1, ConfigFile)
				} else {
					f, err := modfile.Parse(name, src, nil)
					if err != nil || f.Module == nil || module.CheckImportPath(f.Module.Mod.Path) != nil {
						g.unknown(ParseError, name, 1, ConfigFile)
					} else {
						cfg.manifest = f
						if len(f.Replace) > 0 {
							cfg.ambiguous = true
						}
						if len(f.Replace)+len(f.Require)+len(f.Exclude)+len(f.Godebug)+len(f.Ignore)+len(f.Tool) > 0 {
							g.unknown(UnsupportedConstruct, name, 1, Toolchain, Lockfile, ConfigFile)
						}
					}
				}
				g.configInput(path.Join(dir, "go.sum"), File)
				vendor := path.Join(dir, "vendor")
				if g.configInput(vendor, Directory) {
					g.configInput(path.Join(vendor, "modules.txt"), File)
					cfg.ambiguous = true
					g.unknown(UnsupportedConstruct, vendor, 1, Toolchain, ConfigFile)
				}
			}
		}
		if dir == "." {
			break
		}
	}
	if cfg.dir == "" {
		g.unknown(MissingConfig, "", 1, ConfigFile)
	}
	return cfg
}

// A present but unreadable or irregular file still changes resolution policy.
func (g goRead) configInput(name string, resolution Resolution) bool {
	info, err := fs.Lstat(g.repo, name)
	if errors.Is(err, fs.ErrNotExist) {
		return false
	}
	if err != nil || (resolution == File && !info.Mode().IsRegular()) ||
		(resolution == Directory && !info.IsDir()) {
		g.unknown(MissingConfig, name, 1, ConfigFile)
		return true
	}
	g.edge(ConfigRef, name, name, resolution, "", 1)
	return true
}

func (g goRead) resolveImport(cfg goConfig, fromDir, raw string, line int) {
	if cfg.ambiguous || cfg.manifest == nil {
		g.unknown(UnresolvedAlias, raw, line, Toolchain, ConfigFile)
		return
	}
	if suffix, local := importSuffix(raw, cfg.manifest.Module.Mod.Path); local {
		for _, req := range cfg.manifest.Require {
			if _, overlap := importSuffix(raw, req.Mod.Path); overlap {
				g.unknown(UnresolvedAlias, raw, line, Toolchain, ConfigFile)
				return
			}
		}
		target := path.Join(cfg.dir, suffix)
		if target != fromDir && g.withinPackage(cfg.dir, target) && g.packageSources(target, "", "") {
			g.edge(Import, raw, target, Directory, "", line)
		} else {
			g.unknown(UnresolvedAlias, raw, line, Toolchain, ConfigFile)
		}
		return
	}
	// A go.mod requirement gives a minimum and go.sum records integrity.
	// Neither proves which version provides this package under module selection.
	g.edge(Import, raw, "go:"+raw, External, "", line)
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
		info, err := fs.Lstat(g.repo, dir)
		if err != nil || !info.IsDir() {
			return false
		}
		_, err = fs.Lstat(g.repo, path.Join(dir, "go.mod"))
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
		g.unknown(code, dir, 1, Observation)
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
				g.unknown(UnsupportedConstruct, full, 1, Toolchain)
			}
			continue
		}
		if !validPath(full) || goConditionalName(name) || !e.Type().IsRegular() {
			g.unknown(UnsupportedConstruct, full, 1, Toolchain, ConfigFile)
			continue
		}
		src, code := readSource(g.repo, full)
		if code != "" {
			g.unknown(code, full, 1, Observation)
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), full, src, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			g.unknown(ParseError, full, 1, Toolchain)
			continue
		}
		if wantPackage == "" {
			wantPackage = f.Name.Name
		}
		if hasBuildConstraint(f) || hasCgoImport(f) || f.Name.Name != wantPackage || (current == "" && f.Name.Name == "main") {
			g.unknown(UnsupportedConstruct, full, 1, Toolchain, ConfigFile)
			continue
		}
		found = true
		if current != "" {
			g.edge(BuildInput, full, full, File, "", 1)
		}
	}
	return found
}

func packageEntries(repo fs.FS, name string) ([]fs.DirEntry, UnknownCode) {
	info, err := fs.Lstat(repo, name)
	if err != nil || !info.IsDir() {
		return nil, MissingConfig
	}
	f, err := repo.Open(name)
	if err != nil {
		return nil, MissingConfig
	}
	defer f.Close()
	d, ok := f.(fs.ReadDirFile)
	if !ok {
		return nil, MissingConfig
	}
	var entries []fs.DirEntry
	for len(entries) <= maxPackageEntries {
		more, err := d.ReadDir(maxPackageEntries + 1 - len(entries))
		entries = append(entries, more...)
		if len(entries) > maxPackageEntries {
			return nil, ReadLimit
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil || len(more) == 0 {
			return nil, MissingConfig
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
	// Embed belongs to a standalone var or a var spec. Flag directives elsewhere
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
					g.unknown(UnsupportedConstruct, "", line, Toolchain)
					continue
				}
				for _, arg := range args {
					g.resolveEmbed(dir, arg.Arg, pos.PositionFor(arg.Pos, false).Line)
				}
			case strings.HasPrefix(comment.Text, "//go:") || constraint.IsPlusBuild(comment.Text):
				g.unknown(UnsupportedConstruct, "", line, Toolchain, ConfigFile)
			}
		}
	}
}

func (g goRead) resolveEmbed(dir, raw string, line int) {
	name := strings.TrimPrefix(raw, "all:")
	if strings.ContainsAny(name, "*?[\\") {
		g.unknown(UnboundedGlob, raw, line, Toolchain)
		return
	}
	if !validPath(name) || module.CheckFilePath(name) != nil {
		g.unknown(UnsupportedConstruct, raw, line, Toolchain)
		return
	}
	for _, part := range strings.Split(name, "/") {
		switch part {
		case ".git", ".hg", ".svn", ".bzr", "vendor":
			g.unknown(UnsupportedConstruct, raw, line, Toolchain)
			return
		}
	}
	target := path.Join(dir, name)
	info, err := fs.Lstat(g.repo, target)
	switch {
	case err != nil:
		g.unknown(MissingConfig, raw, line, Observation)
	case info.IsDir():
		g.unknown(UnboundedGlob, raw, line, Toolchain)
	case !info.Mode().IsRegular() || !g.withinPackage(dir, path.Dir(target)):
		g.unknown(UnsupportedConstruct, raw, line, Toolchain)
	default:
		g.edge(Embed, raw, target, File, "", line)
	}
}
