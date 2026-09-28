package docindex

// depdocs_lang.go — the per-ecosystem doc-comment readers behind
// depDocUnits (KR-11): Go doc comments (go/parser), `.d.ts` JSDoc, Python
// docstrings (`.pyi` stubs before modules) and Rust `///`. Each is a pure
// function of one file's bytes: public surface only (an unexported name, a
// method on an unexported type, a `private` member, a `pub(crate)` item, a
// test or an internal package is never a unit), a symbol with no doc text
// is not a unit (the API indexer, SG-37, owns signatures; this owns words),
// and output is in file order — deterministic.
//
// Go is read with go/parser (ADR-004, ADR-013: Go keeps the stdlib parser);
// the other three are line-level readers, not the tree-sitter bindings of
// ADR-013: a doc unit needs a comment and the name that follows it, not a
// syntax tree, and the docs layer stays free of the CGO those bindings bring
// (docindex builds under CGO_ENABLED=0 like the store it writes to). Symbol
// and signature extraction, where a syntax tree earns its keep, is the
// dependency API indexer's job (SG-37).

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"regexp"
	"strings"
	"unicode"
)

// ---- Go ------------------------------------------------------------------

func skipGoDepDir(name string) bool {
	switch name {
	case "internal", "testdata", "vendor", "node_modules":
		return true
	}
	return strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
}

func wantsGoFile(_, name string) bool {
	return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
}

// extractGoDocs reads one Go file's doc comments: the package clause's, and
// every exported documented declaration (functions, methods on exported
// types, types, consts and vars). A `package main` file yields only its
// package doc — a command's other declarations are not importable API.
func extractGoDocs(rel string, src []byte) ([]rawDepUnit, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, rel, src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	for _, cg := range f.Comments {
		if cg.End() >= f.Package {
			break
		}
		for _, c := range cg.List {
			if strings.HasPrefix(c.Text, "//go:build") && strings.Contains(c.Text, "ignore") {
				return nil, nil // a file the build excludes is not the package's docs
			}
		}
	}

	line := func(p token.Pos) int { return fset.Position(p).Line }
	var out []rawDepUnit
	emit := func(heading, signature string, doc *ast.CommentGroup, end token.Pos) {
		text := strings.TrimSpace(doc.Text())
		if text == "" {
			return
		}
		body := text
		if signature != "" {
			body = signature + "\n\n" + text
		}
		out = append(out, rawDepUnit{Heading: heading, Kind: KindDepDoc, Body: body, LineStart: line(doc.Pos()), LineEnd: line(end)})
	}

	if f.Doc != nil {
		emit("package "+f.Name.Name, "", f.Doc, f.Doc.End())
	}
	if f.Name.Name == "main" {
		return out, nil
	}

	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if !d.Name.IsExported() || d.Doc == nil {
				continue
			}
			heading := d.Name.Name
			if d.Recv != nil && len(d.Recv.List) > 0 {
				recv := goRecvTypeName(d.Recv.List[0].Type)
				if recv == "" || !ast.IsExported(recv) {
					continue // a method on an unexported type is not public API
				}
				heading = recv + "." + d.Name.Name
			}
			emit(heading, goFuncSignature(fset, d), d.Doc, d.Type.End())
		case *ast.GenDecl:
			if d.Tok != token.TYPE && d.Tok != token.CONST && d.Tok != token.VAR {
				continue
			}
			groupDocUsed := false
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					if !s.Name.IsExported() {
						continue
					}
					doc := s.Doc
					if doc == nil && !groupDocUsed && d.Doc != nil {
						doc, groupDocUsed = d.Doc, true
					}
					if doc != nil {
						emit(s.Name.Name, goTypeLabel(fset, s), doc, s.End())
					}
				case *ast.ValueSpec:
					var first *ast.Ident
					for _, n := range s.Names {
						if n.IsExported() {
							first = n
							break
						}
					}
					if first == nil {
						continue
					}
					doc := s.Doc
					if doc == nil && !groupDocUsed && d.Doc != nil {
						doc, groupDocUsed = d.Doc, true
					}
					if doc != nil {
						emit(first.Name, d.Tok.String()+" "+first.Name, doc, s.End())
					}
				}
			}
		}
	}
	return out, nil
}

// goRecvTypeName unwraps a receiver's type expression to its bare type name
// (T, *T, T[P], *T[P, Q]).
func goRecvTypeName(e ast.Expr) string {
	for {
		switch t := e.(type) {
		case *ast.StarExpr:
			e = t.X
		case *ast.IndexExpr:
			e = t.X
		case *ast.IndexListExpr:
			e = t.X
		case *ast.ParenExpr:
			e = t.X
		case *ast.Ident:
			return t.Name
		default:
			return ""
		}
	}
}

// goFuncSignature prints a function's declaration line without its body or
// doc, collapsed to one line.
func goFuncSignature(fset *token.FileSet, d *ast.FuncDecl) string {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, &ast.FuncDecl{Recv: d.Recv, Name: d.Name, Type: d.Type}); err != nil {
		return "func " + d.Name.Name
	}
	return strings.Join(strings.Fields(buf.String()), " ")
}

// goTypeLabel is the short "type Name struct|interface|<expr>" line a type's
// unit leads with (the full field list would only bloat the index).
func goTypeLabel(fset *token.FileSet, s *ast.TypeSpec) string {
	label := "type " + s.Name.Name
	if s.Assign.IsValid() {
		label += " ="
	}
	switch t := s.Type.(type) {
	case *ast.StructType:
		return label + " struct"
	case *ast.InterfaceType:
		return label + " interface"
	default:
		var buf bytes.Buffer
		if printer.Fprint(&buf, fset, t) == nil {
			if e := strings.Join(strings.Fields(buf.String()), " "); len(e) <= 80 {
				return label + " " + e
			}
		}
		return label
	}
}

// ---- npm: .d.ts JSDoc ----------------------------------------------------

func skipNPMDepDir(name string) bool {
	switch name {
	case "node_modules", "test", "tests", "__tests__", "__mocks__":
		return true
	}
	return strings.HasPrefix(name, ".")
}

func wantsDTS(_, name string) bool {
	return strings.HasSuffix(name, ".d.ts") || strings.HasSuffix(name, ".d.mts") || strings.HasSuffix(name, ".d.cts")
}

var (
	dtsDeclRE   = regexp.MustCompile(`^\s*(?:export\s+)?(?:declare\s+)?(?:default\s+)?(?:abstract\s+)?(?:async\s+)?(function|class|interface|type|const|let|var|enum|namespace|module)\s+([A-Za-z_$][\w$]*)`)
	dtsMemberRE = regexp.MustCompile(`^\s*(?:(?:public|static|readonly|abstract|declare|async|override)\s+)*(?:get\s+|set\s+)?([A-Za-z_$][\w$]*)\??\s*[(:<=;,]`)
	dtsHiddenRE = regexp.MustCompile(`^\s*(?:private|protected)\b|@(?:internal|private)\b`)
)

// extractDTSDocs reads a `.d.ts` file's JSDoc blocks, each keyed to the
// declaration that follows it. A `private`/`protected` member and a block
// tagged `@internal` are not public surface.
func extractDTSDocs(rel string, src []byte) ([]rawDepUnit, error) {
	lines := strings.Split(strings.ReplaceAll(string(src), "\r\n", "\n"), "\n")
	var out []rawDepUnit
	for i := 0; i < len(lines); i++ {
		trim := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(trim, "/**") {
			continue
		}
		blockStart, end := i, i
		for end < len(lines) && !strings.Contains(lines[end], "*/") {
			end++
		}
		if end >= len(lines) {
			break
		}
		var text []string
		for k := blockStart; k <= end; k++ {
			l := strings.TrimSpace(lines[k])
			l = strings.TrimPrefix(l, "/**")
			l = strings.TrimSuffix(l, "*/")
			l = strings.TrimSpace(l)
			l = strings.TrimSpace(strings.TrimPrefix(l, "*"))
			text = append(text, l)
		}
		doc := strings.TrimSpace(strings.Join(text, "\n"))
		// The declaration: the first line after the block that is not blank,
		// a plain comment or a decorator.
		j := end + 1
		for j < len(lines) {
			t := strings.TrimSpace(lines[j])
			if t == "" || strings.HasPrefix(t, "//") || strings.HasPrefix(t, "@") {
				j++
				continue
			}
			break
		}
		i = end
		if doc == "" || j >= len(lines) || dtsHiddenRE.MatchString(doc) || dtsHiddenRE.MatchString(lines[j]) {
			continue
		}
		decl := lines[j]
		var name string
		if m := dtsDeclRE.FindStringSubmatch(decl); m != nil {
			name = m[2]
		} else if m := dtsMemberRE.FindStringSubmatch(decl); m != nil {
			name = m[1]
		}
		if name == "" || strings.HasPrefix(name, "_") {
			continue
		}
		out = append(out, rawDepUnit{
			Heading:   name,
			Kind:      KindDepDoc,
			Body:      doc + "\n\n" + strings.TrimSpace(decl),
			LineStart: blockStart + 1,
			LineEnd:   j + 1,
		})
	}
	return out, nil
}

// ---- PyPI: docstrings ----------------------------------------------------

func skipPyDepDir(name string) bool {
	switch name {
	case "tests", "test", "__pycache__", "node_modules", "_vendor", "vendored":
		return true
	}
	return strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".dist-info") ||
		strings.HasSuffix(name, ".egg-info") || strings.HasSuffix(name, ".data")
}

func wantsPyFile(_, name string) bool {
	if !strings.HasSuffix(name, ".py") && !strings.HasSuffix(name, ".pyi") {
		return false
	}
	base := strings.TrimSuffix(strings.TrimSuffix(name, ".pyi"), ".py")
	return base != "setup" && base != "conftest" && !strings.HasPrefix(base, "test_") && !strings.HasSuffix(base, "_test")
}

var pyDefRE = regexp.MustCompile(`^(\s*)(?:async\s+)?(def|class)\s+([A-Za-z_]\w*)`)

func pyPublicName(name string) bool {
	if !strings.HasPrefix(name, "_") {
		return true
	}
	return len(name) > 4 && strings.HasPrefix(name, "__") && strings.HasSuffix(name, "__")
}

type pyFrame struct {
	indent  int
	name    string
	isClass bool
	public  bool
}

// extractPyDocs reads a module's docstring and the docstrings of its public
// top-level functions and classes and their public methods.
func extractPyDocs(rel string, src []byte) ([]rawDepUnit, error) {
	lines := strings.Split(strings.ReplaceAll(string(src), "\r\n", "\n"), "\n")
	var out []rawDepUnit
	if k := pyFirstStatement(lines); k >= 0 {
		if doc, s, e, ok := pyDocstringAt(lines, k); ok {
			out = append(out, rawDepUnit{Heading: "module " + rel, Kind: KindDepDoc, Body: doc, LineStart: s, LineEnd: e})
		}
	}
	var stack []pyFrame
	for i := 0; i < len(lines); i++ {
		m := pyDefRE.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}
		indent := len(m[1])
		for len(stack) > 0 && stack[len(stack)-1].indent >= indent {
			stack = stack[:len(stack)-1]
		}
		name, isClass := m[3], m[2] == "class"
		public := pyPublicName(name)
		var path []string
		for _, fr := range stack {
			if !fr.isClass || !fr.public {
				public = false // a member of a private class, or a nested function
			}
			path = append(path, fr.name)
		}
		stack = append(stack, pyFrame{indent: indent, name: name, isClass: isClass, public: public})
		if !public {
			continue
		}
		sigEnd := pySignatureEnd(lines, i)
		doc, _, e, ok := pyDocstringAt(lines, sigEnd+1)
		if !ok {
			continue
		}
		out = append(out, rawDepUnit{
			Heading: strings.Join(append(path, name), "."), Kind: KindDepDoc,
			Body:      strings.TrimSpace(lines[i]) + "\n\n" + doc,
			LineStart: i + 1, LineEnd: e,
		})
		i = e - 1 // the docstring's own lines are not definitions
	}
	return out, nil
}

// pyFirstStatement returns the index of the first non-blank, non-comment
// line (the only place a module docstring can be), or -1.
func pyFirstStatement(lines []string) int {
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		return i
	}
	return -1
}

// pySignatureEnd returns the index of the line that ends a def/class
// header: the first line at which brackets have closed and a top-level
// colon has been seen (so a multi-line parameter list and a return
// annotation both resolve). It looks at most 40 lines ahead.
func pySignatureEnd(lines []string, i int) int {
	depth, sawColon := 0, false
	for j := i; j < len(lines) && j < i+40; j++ {
		l := lines[j]
		if h := strings.IndexByte(l, '#'); h >= 0 {
			l = l[:h]
		}
		for _, r := range l {
			switch r {
			case '(', '[', '{':
				depth++
			case ')', ']', '}':
				depth--
			case ':':
				if depth <= 0 {
					sawColon = true
				}
			}
		}
		if depth <= 0 && sawColon {
			return j
		}
	}
	return i
}

// pyDocstringAt reads a docstring that starts on the first non-blank line at
// or after idx: a triple-quoted (possibly multi-line) or single-quoted
// literal, with its optional r/u/b/f prefix. It returns the cleaned text and
// the 1-based first and last line, or ok=false when that line is not a
// string literal.
func pyDocstringAt(lines []string, idx int) (text string, start, end int, ok bool) {
	k := idx
	for k < len(lines) && strings.TrimSpace(lines[k]) == "" {
		k++
	}
	if k >= len(lines) {
		return "", 0, 0, false
	}
	t := strings.TrimSpace(lines[k])
	for n := 0; n < 2 && len(t) > 0 && strings.ContainsRune("rRuUbBfF", rune(t[0])); n++ {
		t = t[1:]
	}
	var delim string
	switch {
	case strings.HasPrefix(t, `"""`):
		delim = `"""`
	case strings.HasPrefix(t, `'''`):
		delim = `'''`
	case strings.HasPrefix(t, `"`):
		delim = `"`
	case strings.HasPrefix(t, `'`):
		delim = `'`
	default:
		return "", 0, 0, false
	}
	rest := t[len(delim):]
	if c := strings.Index(rest, delim); c >= 0 {
		return strings.TrimSpace(rest[:c]), k + 1, k + 1, true
	}
	if len(delim) == 1 {
		return "", 0, 0, false // an unterminated one-line string is not a docstring
	}
	body := []string{rest}
	for e := k + 1; e < len(lines); e++ {
		if c := strings.Index(lines[e], delim); c >= 0 {
			body = append(body, lines[e][:c])
			return pyCleanDoc(body), k + 1, e + 1, true
		}
		body = append(body, lines[e])
	}
	return "", 0, 0, false
}

// pyCleanDoc trims a docstring the way inspect.cleandoc does: the first
// line as is, the rest dedented by their common indentation.
func pyCleanDoc(lines []string) string {
	margin := -1
	for _, l := range lines[1:] {
		if strings.TrimSpace(l) == "" {
			continue
		}
		n := len(l) - len(strings.TrimLeftFunc(l, unicode.IsSpace))
		if margin < 0 || n < margin {
			margin = n
		}
	}
	out := []string{strings.TrimSpace(lines[0])}
	for _, l := range lines[1:] {
		if margin > 0 && len(l) >= margin {
			l = l[margin:]
		}
		out = append(out, strings.TrimRightFunc(l, unicode.IsSpace))
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

// ---- Cargo: Rust /// -----------------------------------------------------

func skipRustDepDir(name string) bool {
	switch name {
	case "target", "tests", "benches", "examples", "node_modules", "vendor":
		return true
	}
	return strings.HasPrefix(name, ".")
}

// wantsRustFile reads only a crate's src/ tree.
func wantsRustFile(dir, name string) bool {
	return strings.HasSuffix(name, ".rs") && (dir == "src" || strings.HasPrefix(dir, "src/"))
}

var rustItemRE = regexp.MustCompile(`^\s*pub\s+(?:(?:unsafe|async|const|extern(?:\s+"[^"]*")?)\s+)*(fn|struct|enum|trait|type|const|static|mod|union)\s+([A-Za-z_]\w*)`)

// extractRustDocs reads a Rust file's `//!` module/crate docs and the `///`
// runs above `pub` items (a `pub(crate)` item is not public).
func extractRustDocs(rel string, src []byte) ([]rawDepUnit, error) {
	lines := strings.Split(strings.ReplaceAll(string(src), "\r\n", "\n"), "\n")
	var out []rawDepUnit

	// Inner docs: the first contiguous //! run.
	for i := 0; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if strings.HasPrefix(t, "//!") {
			start := i
			var text []string
			for i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), "//!") {
				text = append(text, rustDocLine(strings.TrimSpace(lines[i])[3:]))
				i++
			}
			heading := "module " + rel
			if rel == "src/lib.rs" || rel == "src/main.rs" {
				heading = "crate"
			}
			if body := strings.TrimSpace(strings.Join(text, "\n")); body != "" {
				out = append(out, rawDepUnit{Heading: heading, Kind: KindDepDoc, Body: body, LineStart: start + 1, LineEnd: i})
			}
			break
		}
		if t != "" && !strings.HasPrefix(t, "//") && !strings.HasPrefix(t, "#!") {
			break // an item came first: no inner docs
		}
	}

	for i := 0; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(t, "///") || strings.HasPrefix(t, "////") {
			continue
		}
		start := i
		var text []string
		for i < len(lines) {
			u := strings.TrimSpace(lines[i])
			if !strings.HasPrefix(u, "///") || strings.HasPrefix(u, "////") {
				break
			}
			text = append(text, rustDocLine(u[3:]))
			i++
		}
		j := i
		for j < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[j]), "#[") {
			j++
		}
		i-- // the loop's own i++ resumes after the run
		if j >= len(lines) {
			continue
		}
		m := rustItemRE.FindStringSubmatch(lines[j])
		body := strings.TrimSpace(strings.Join(text, "\n"))
		if m == nil || body == "" {
			continue
		}
		item := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(lines[j]), "{"))
		out = append(out, rawDepUnit{Heading: m[2], Kind: KindDepDoc, Body: body + "\n\n" + item, LineStart: start + 1, LineEnd: j + 1})
	}
	return out, nil
}

// rustDocLine strips the single conventional space after a doc marker.
func rustDocLine(s string) string {
	return strings.TrimRightFunc(strings.TrimPrefix(s, " "), unicode.IsSpace)
}
