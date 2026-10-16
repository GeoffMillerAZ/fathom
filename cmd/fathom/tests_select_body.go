package main

// tests_select_body.go — which symbols of a changed file `fathom tests
// select` counts as changed (TQ-29, wave hon1, O1001). A Go symbol is
// changed only when its own text differs between --since and the working
// tree the tests run against (what `git diff --name-only <since>` already
// compares): a declaration whose lines merely moved, because something was
// inserted above it, is not. A symbol new since --since, or one the working
// tree no longer declares, stays changed. Anything this cannot read
// exactly keeps the old file-level reading, every symbol of the file
// changed: a file that is not Go, a file absent at --since or from the
// working tree, a revision git does not know, a parse error, a header
// (build constraints, package clause, imports, a cgo preamble) that
// changed, or a name declared twice in one file.
//
// A symbol's text is its declaration from the `func`, type name or value
// name to its end, plus any //go: directive in its doc comment (//go:embed
// and //go:linkname change what a declaration means). A const in a
// parenthesized group that uses iota or an implicit repeat reads the whole
// group, because inserting a constant above it changes its value without
// changing its line. The old side is read through internal/gitro's
// read-only blob read, never a checkout.

import (
	"bytes"
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/gitro"
)

// testsSelectBlobCap bounds the --since side read of one changed file.
const testsSelectBlobCap = 16 << 20

// testsSelectOldBlob reads rel as committed at rev (injected in tests).
var testsSelectOldBlob = func(ctx context.Context, root, rev, rel string) ([]byte, error) {
	data, _, err := gitro.BlobAt(ctx, root, rev, rel, testsSelectBlobCap)
	return data, err
}

// bodyStats counts what bodyChangedSymbols decided, for the stderr note.
type bodyStats struct {
	Unchanged int // symbols of changed Go files whose own text did not change
	FileLevel int // changed files read at file level (every symbol changed)
}

// bodyChangedSymbols returns the canonical keys of the non-test symbols
// defined in changedFiles that changed since rev, under the rule above.
// root is the directory the indexed paths are relative to. Order follows
// nodes; the caller sorts.
func bodyChangedSymbols(ctx context.Context, root, rev string, nodes []core.SymbolNode, changedFiles []string) ([]string, bodyStats) {
	changed := make(map[string]bool, len(changedFiles))
	for _, f := range changedFiles {
		changed[f] = true
	}
	byFile := map[string][]core.SymbolNode{}
	for _, n := range nodes {
		if changed[n.FilePath] && !strings.HasPrefix(n.Name, "Test") {
			byFile[n.FilePath] = append(byFile[n.FilePath], n)
		}
	}
	var keys []string
	var st bodyStats
	for _, f := range changedFiles {
		syms := byFile[f]
		if len(syms) == 0 {
			continue
		}
		delete(byFile, f) // a path listed twice is read once
		oldUnits, newUnits, ok := goUnitsBothSides(ctx, root, rev, f)
		if !ok {
			st.FileLevel++
			for _, n := range syms {
				keys = append(keys, n.Key())
			}
			continue
		}
		for _, n := range syms {
			o, inOld := oldUnits[n.Name]
			w, inNew := newUnits[n.Name]
			if inOld && inNew && o != nil && w != nil && bytes.Equal(o, w) {
				st.Unchanged++
				continue
			}
			keys = append(keys, n.Key())
		}
	}
	return keys, st
}

// goUnitsBothSides parses rel at rev and in the working tree and returns
// each side's declaration texts by symbol name (a nil text marks a name
// declared more than once). ok is false when the file must be read at file
// level.
func goUnitsBothSides(ctx context.Context, root, rev, rel string) (oldUnits, newUnits map[string][]byte, ok bool) {
	if !strings.HasSuffix(rel, ".go") || rev == "" {
		return nil, nil, false
	}
	oldSrc, err := testsSelectOldBlob(ctx, root, rev, rel)
	if err != nil {
		return nil, nil, false
	}
	newSrc, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return nil, nil, false
	}
	oldDecl, oerr := goDeclUnits(oldSrc)
	newDecl, nerr := goDeclUnits(newSrc)
	if oerr != nil || nerr != nil || !bytes.Equal(oldDecl.header, newDecl.header) || importsRebound(oldDecl.imports, newDecl.imports) {
		return nil, nil, false
	}
	return oldDecl.units, newDecl.units, true
}

// importsRebound reports whether the named imports changed in a way that
// can rebind a name an unchanged body uses: one import went and another
// came (a version bump keeps the local name and changes the package). An
// import only added is used by changed code; one only removed was used
// only by code that changed.
func importsRebound(old, cur map[string]bool) bool {
	removed, added := false, false
	for k := range old {
		removed = removed || !cur[k]
	}
	for k := range cur {
		added = added || !old[k]
	}
	return removed && added
}

var errGoUnitsSyntax = errors.New("go source does not parse")

// goDecls is one side of a Go file: the header that must match exactly
// (build constraints, package clause, blank and dot imports, any cgo
// preamble), the named imports ("alias path" or "path"), and the text of
// each top-level declaration keyed by the name the Go extractor gives the
// symbol: a function's name, "Recv.Method" for a method, a type's, var's or
// const's name (nil: the name is declared more than once).
type goDecls struct {
	header  []byte
	imports map[string]bool
	units   map[string][]byte
}

// goDeclUnits parses src into its goDecls.
func goDeclUnits(src []byte) (goDecls, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", src, parser.ParseComments)
	if err != nil {
		return goDecls{}, errGoUnitsSyntax
	}
	text := func(from, to token.Pos) []byte {
		a, b := fset.Position(from).Offset, fset.Position(to).Offset
		if a < 0 || b > len(src) || a > b {
			return nil
		}
		return src[a:b]
	}

	var header bytes.Buffer
	imports := map[string]bool{}
	for _, cg := range f.Comments {
		if cg.Pos() >= f.Package {
			break
		}
		for _, c := range cg.List {
			if strings.HasPrefix(c.Text, "//go:build") || strings.HasPrefix(c.Text, "// +build") {
				header.WriteString(c.Text + "\n")
			}
		}
	}
	header.WriteString("package " + f.Name.Name + "\n")
	for _, d := range f.Decls {
		gd, isGen := d.(*ast.GenDecl)
		if !isGen || gd.Tok != token.IMPORT {
			continue
		}
		for _, s := range gd.Specs {
			is := s.(*ast.ImportSpec)
			switch {
			case is.Path.Value == `"C"`:
				header.WriteString("import C\n")
				if gd.Doc != nil {
					header.Write(text(gd.Doc.Pos(), gd.Doc.End()))
					header.WriteString("\n")
				}
			case is.Name != nil && (is.Name.Name == "_" || is.Name.Name == "."):
				header.WriteString(is.Name.Name + " " + is.Path.Value + "\n")
			case is.Name != nil:
				imports[is.Name.Name+" "+is.Path.Value] = true
			default:
				imports[is.Path.Value] = true
			}
		}
	}

	units := map[string][]byte{}
	put := func(name string, body []byte) {
		if name == "" || name == "_" {
			return
		}
		if _, dup := units[name]; dup {
			units[name] = nil
			return
		}
		units[name] = body
	}
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			name := d.Name.Name
			if d.Recv != nil && len(d.Recv.List) > 0 {
				name = goReceiverTypeName(d.Recv.List[0].Type) + "." + name
			}
			put(name, joinDirectives(text(d.Pos(), d.End()), d.Doc))
		case *ast.GenDecl:
			if d.Tok == token.IMPORT {
				continue
			}
			whole := d.Tok == token.CONST && d.Lparen.IsValid() && constGroupImplicit(d)
			for _, s := range d.Specs {
				switch s := s.(type) {
				case *ast.TypeSpec:
					put(s.Name.Name, joinDirectives(text(s.Pos(), s.End()), d.Doc, s.Doc))
				case *ast.ValueSpec:
					body := text(s.Pos(), s.End())
					if whole {
						body = text(d.Pos(), d.End())
					}
					body = joinDirectives(body, d.Doc, s.Doc)
					for _, n := range s.Names {
						put(n.Name, body)
					}
				}
			}
		}
	}
	return goDecls{header: header.Bytes(), imports: imports, units: units}, nil
}

// constGroupImplicit reports whether a parenthesized const group gives any
// spec its value implicitly (an omitted value repeats the previous one) or
// through iota: then a spec's value depends on its place in the group.
func constGroupImplicit(d *ast.GenDecl) bool {
	for _, s := range d.Specs {
		vs, ok := s.(*ast.ValueSpec)
		if !ok {
			continue
		}
		if len(vs.Values) == 0 {
			return true
		}
		iota := false
		for _, v := range vs.Values {
			ast.Inspect(v, func(n ast.Node) bool {
				if id, ok := n.(*ast.Ident); ok && id.Name == "iota" {
					iota = true
				}
				return !iota
			})
		}
		if iota {
			return true
		}
	}
	return false
}

// joinDirectives appends the //go: directive lines of docs to body.
func joinDirectives(body []byte, docs ...*ast.CommentGroup) []byte {
	var dirs []string
	for _, cg := range docs {
		if cg == nil {
			continue
		}
		for _, c := range cg.List {
			if strings.HasPrefix(c.Text, "//go:") {
				dirs = append(dirs, c.Text)
			}
		}
	}
	if len(dirs) == 0 {
		return body
	}
	out := append([]byte(nil), body...)
	for _, d := range dirs {
		out = append(out, '\n')
		out = append(out, d...)
	}
	return out
}

// goReceiverTypeName is the receiver's base type name, as the Go extractor
// names a method's Package: the pointer and any type parameters dropped.
func goReceiverTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return goReceiverTypeName(t.X)
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr:
		return goReceiverTypeName(t.X)
	case *ast.IndexListExpr:
		return goReceiverTypeName(t.X)
	}
	return ""
}
