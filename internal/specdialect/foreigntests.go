package specdialect

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

var (
	foreignTestNameRe = regexp.MustCompile(`\bTest[A-Z0-9_][A-Za-z0-9_]*\b`)
	foreignTestCodeRe = regexp.MustCompile("`([^`]+)`")
	foreignTestRunRe  = regexp.MustCompile(`(?:^|\s)-run(?:=|\s+)(?:"([^"]+)"|'([^']+)'|([^\s` + "`" + `]+))`)
)

const foreignTestReadLimit = 1 << 20

// mapForeignTestRefs reads explicit Go test identifiers from a foreign task
// text. Names must be in backticks or a -run argument; Spec Kit also accepts
// bare identifiers and test files. When resolvePaths is true,
// read only those files under root, including through symlinks. Unreadable,
// invalid, missing or escaping files contribute nothing. Nothing is executed.
func mapForeignTestRefs(root, text string, resolvePaths bool) []string {
	words := foreignTestWords(text)
	var sources []string
	if resolvePaths {
		sources = append(sources, text)
	} else {
		for _, m := range foreignTestCodeRe.FindAllStringSubmatch(text, -1) {
			sources = append(sources, m[1])
		}
		// Scan commands both in prose and inside a code span.
		for _, source := range append([]string{text}, sources...) {
			for _, m := range foreignTestRunRe.FindAllStringSubmatch(source, -1) {
				for _, arg := range m[1:] {
					if arg != "" {
						sources = append(sources, arg)
					}
				}
			}
		}
	}
	var names []string
	for _, source := range sources {
		for _, word := range foreignTestWords(source) {
			if strings.Contains(word, ".go") || strings.Contains(word, `\`) {
				continue
			}
			// Certificates belong to top-level tests, including subtest references.
			word = strings.TrimRight(strings.TrimPrefix(word, "^"), "$.")
			name, _, _ := strings.Cut(word, "/")
			name = strings.TrimSuffix(name, "$")
			if foreignTestNameRe.FindString(name) == name && name != "" {
				names = append(names, name)
			}
		}
	}
	if resolvePaths {
		if tree, err := os.OpenRoot(root); err == nil {
			defer tree.Close()
			for _, path := range words {
				path = strings.TrimRight(path, ".")
				if !strings.HasSuffix(path, "_test.go") || !filepath.IsLocal(path) {
					continue
				}
				// Check before opening so a FIFO cannot block waiting for a writer.
				info, err := tree.Stat(path)
				if err != nil || !info.Mode().IsRegular() {
					continue
				}
				f, err := tree.Open(path)
				if err != nil {
					continue
				}
				info, err = f.Stat()
				if err != nil || !info.Mode().IsRegular() {
					_ = f.Close()
					continue
				}
				body, err := io.ReadAll(io.LimitReader(f, foreignTestReadLimit+1))
				_ = f.Close()
				if err != nil || len(body) > foreignTestReadLimit {
					continue
				}
				names = append(names, foreignGoTestNames(body)...)
			}
		}
	}
	return sortedForeignTests(names)
}

func foreignTestWords(text string) []string {
	return strings.FieldsFunc(text, func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune("`\"'()[]{}<>,;:!?=|", r)
	})
}

// foreignGoTestNames accepts top-level Go test declarations, not comments,
// examples, methods or helpers whose names merely start with Test.
func foreignGoTestNames(body []byte) []string {
	f, err := parser.ParseFile(token.NewFileSet(), "", body, 0)
	if err != nil {
		return nil
	}
	imports := map[string]bool{}
	for _, imp := range f.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil || path != "testing" {
			continue
		}
		name := "testing"
		if imp.Name != nil {
			name = imp.Name.Name
		}
		imports[name] = true
	}
	var out []string
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || fn.Body == nil || !foreignTestNameRe.MatchString(fn.Name.Name) ||
			foreignTestNameRe.FindString(fn.Name.Name) != fn.Name.Name || fn.Type.TypeParams != nil ||
			fn.Type.Results.NumFields() != 0 || fn.Type.Params.NumFields() != 1 {
			continue
		}
		param := fn.Type.Params.List[0]
		ptr, ok := param.Type.(*ast.StarExpr)
		if !ok {
			continue
		}
		if sel, ok := ptr.X.(*ast.SelectorExpr); ok {
			pkg, ok := sel.X.(*ast.Ident)
			if ok && imports[pkg.Name] && sel.Sel.Name == "T" {
				out = append(out, fn.Name.Name)
			}
		} else if name, ok := ptr.X.(*ast.Ident); ok && name.Name == "T" && imports["."] {
			out = append(out, fn.Name.Name)
		}
	}
	return out
}

func sortedForeignTests(names []string) []string {
	if len(names) == 0 {
		return nil
	}
	names = append([]string(nil), names...)
	sort.Strings(names)
	out := names[:0]
	for _, name := range names {
		if len(out) == 0 || out[len(out)-1] != name {
			out = append(out, name)
		}
	}
	return out
}
