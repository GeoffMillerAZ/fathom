package ask

import (
	"context"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

// Span names one Go function in a module: the file (slash path relative to
// the module root), the function's name as the index stores it ("F", or
// "Recv.Method"; empty matches any) and its line range (Start may be its
// doc comment's first line).
type Span struct {
	Repo  string `json:"repo"`
	File  string `json:"file"`
	Name  string `json:"name"`
	Start int    `json:"start"`
	End   int    `json:"end"`
}

// Why a span was excluded rather than computed. An excluded span is
// counted, never guessed (EXP-2).
const (
	// ExcludedLoad: the span's package failed to load or type-check.
	ExcludedLoad = "load"
	// ExcludedNotLoaded: the file is in no loaded package (a build
	// constraint for this platform, a nested module, a generated main).
	ExcludedNotLoaded = "not_loaded"
	// ExcludedNoDecl: no function declaration in the file matches the span.
	ExcludedNoDecl = "no_decl"
)

// Computed is one span's facts, or the reason it was excluded. Source is
// the function's text from its func keyword to its closing brace (no doc
// comment) and Guard, set when LockedWrite applies, the receiver type's
// declaration: both feed a question and never a reading.
type Computed struct {
	Span     Span
	Line     int // the func keyword's line
	Facts    Facts
	Excluded string
	Source   string
	Guard    string
}

// LoadBatch is how many package directories one go/packages load covers.
const LoadBatch = 16

// Compute computes every span's facts over the module at root, returning
// one Computed per span in input order. It loads the spans' package
// directories LoadBatch at a time (tests included, each package
// type-checked from source) and keeps none of them afterwards. env is the
// go command's environment (nil: this process's, with GOWORK=off and
// GOTOOLCHAIN=local). A load that fails outright excludes its batch's spans
// as ExcludedLoad; err is reserved for ctx's cancellation.
func Compute(ctx context.Context, root string, spans []Span, env []string) ([]Computed, error) {
	if env == nil {
		env = append(os.Environ(), "GOWORK=off", "GOTOOLCHAIN=local")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	realRoot := realPath(abs)
	out := make([]Computed, len(spans))
	byDir := map[string][]int{}
	for i, s := range spans {
		out[i].Span = s
		dir := filepath.ToSlash(filepath.Dir(filepath.FromSlash(s.File)))
		byDir[dir] = append(byDir[dir], i)
	}
	dirs := make([]string, 0, len(byDir))
	for d := range byDir {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	for lo := 0; lo < len(dirs); lo += LoadBatch {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		batch := dirs[lo:min(lo+LoadBatch, len(dirs))]
		idx := loadFiles(ctx, realRoot, batch, env)
		for _, d := range batch {
			for _, i := range byDir[d] {
				out[i] = computeOne(idx, realRoot, spans[i])
			}
		}
	}
	return out, nil
}

// loadedFile is one parsed file and the package variant that owns it.
type loadedFile struct {
	pkg  *packages.Package
	file *ast.File
	bad  bool // the owning package had errors
}

// loadFiles loads dirs (relative to root) and indexes every syntax file by
// its real absolute path. A file in several variants (p and p [p.test])
// maps to the first error-free one in package-ID order.
func loadFiles(ctx context.Context, root string, dirs []string, env []string) map[string]loadedFile {
	patterns := make([]string, len(dirs))
	for i, d := range dirs {
		if d == "." {
			patterns[i] = "."
		} else {
			patterns[i] = "./" + d
		}
	}
	cfg := &packages.Config{
		Context: ctx,
		Dir:     root,
		Env:     env,
		Tests:   true,
		Fset:    token.NewFileSet(),
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedImports |
			packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
	}
	idx := map[string]loadedFile{}
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return idx
	}
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].ID < pkgs[j].ID })
	for _, p := range pkgs {
		bad := len(p.Errors) > 0 || p.IllTyped || p.TypesInfo == nil
		for _, f := range p.Syntax {
			name := realPath(p.Fset.Position(f.Pos()).Filename)
			if prev, ok := idx[name]; ok && !prev.bad {
				continue
			}
			idx[name] = loadedFile{pkg: p, file: f, bad: bad}
		}
		if bad {
			// A file the package lists but did not parse still belongs to
			// a package that failed to load.
			for _, g := range p.GoFiles {
				name := realPath(g)
				if _, ok := idx[name]; !ok {
					idx[name] = loadedFile{pkg: p, bad: true}
				}
			}
		}
	}
	return idx
}

func realPath(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}

func computeOne(idx map[string]loadedFile, root string, s Span) Computed {
	c := Computed{Span: s}
	lf, ok := idx[realPath(filepath.Join(root, filepath.FromSlash(s.File)))]
	switch {
	case !ok:
		c.Excluded = ExcludedNotLoaded
		return c
	case lf.bad || lf.file == nil:
		c.Excluded = ExcludedLoad
		return c
	}
	fd := findFunc(lf.pkg.Fset, lf.file, s)
	if fd == nil {
		c.Excluded = ExcludedNoDecl
		return c
	}
	c.Facts = spanFacts(lf.pkg, lf.file, fd)
	src, err := os.ReadFile(lf.pkg.Fset.Position(lf.file.Pos()).Filename)
	if err != nil {
		c.Excluded = ExcludedLoad
		return c
	}
	c.Source = sourceText(lf.pkg.Fset, src, fd.Pos(), fd.End())
	c.Line = lf.pkg.Fset.Position(fd.Pos()).Line
	if c.Facts.LockedWrite.Applies {
		c.Guard = guardDecl(lf.pkg, lf.pkg.TypesInfo.Defs[fd.Recv.List[0].Names[0]])
	}
	return c
}

func sourceText(fset *token.FileSet, src []byte, from, to token.Pos) string {
	a, b := fset.Position(from).Offset, fset.Position(to).Offset
	if a < 0 || b > len(src) || a > b {
		return ""
	}
	return string(src[a:b])
}

// funcName is the index's name for fd: "F", or "Recv.Method" with the
// receiver's pointer and type parameters stripped.
func funcName(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return fd.Name.Name
	}
	t := fd.Recv.List[0].Type
	for {
		switch x := t.(type) {
		case *ast.StarExpr:
			t = x.X
			continue
		case *ast.IndexExpr:
			t = x.X
			continue
		case *ast.IndexListExpr:
			t = x.X
			continue
		case *ast.ParenExpr:
			t = x.X
			continue
		}
		break
	}
	if id, ok := t.(*ast.Ident); ok {
		return id.Name + "." + fd.Name.Name
	}
	return fd.Name.Name
}

// findFunc is the declaration in f named like s whose lines (doc comment
// included) contain s.Start.
func findFunc(fset *token.FileSet, f *ast.File, s Span) *ast.FuncDecl {
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if s.Name != "" && s.Name != funcName(fd) && s.Name != fd.Name.Name {
			continue
		}
		from := fd.Pos()
		if fd.Doc != nil {
			from = fd.Doc.Pos()
		}
		if fset.Position(from).Line <= s.Start && s.Start <= fset.Position(fd.End()).Line {
			return fd
		}
	}
	return nil
}

// guardDecl is the receiver's named type declaration, as "type T struct{...}".
func guardDecl(pkg *packages.Package, recv types.Object) string {
	if recv == nil {
		return ""
	}
	n, ok := types.Unalias(deref(recv.Type())).(*types.Named)
	if !ok {
		return ""
	}
	pos := n.Obj().Pos()
	for _, f := range pkg.Syntax {
		if f.FileStart > pos || pos > f.FileEnd {
			continue
		}
		var spec *ast.TypeSpec
		ast.Inspect(f, func(x ast.Node) bool {
			if ts, ok := x.(*ast.TypeSpec); ok && ts.Name.Pos() == pos {
				spec = ts
			}
			return spec == nil
		})
		if spec == nil {
			return ""
		}
		src, err := os.ReadFile(pkg.Fset.Position(f.Pos()).Filename)
		if err != nil {
			return ""
		}
		return "type " + strings.TrimSpace(sourceText(pkg.Fset, src, spec.Pos(), spec.End()))
	}
	return ""
}
