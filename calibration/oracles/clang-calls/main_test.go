package main

import (
	"bufio"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestCalledName: the callee of a call-shaped expression is its last name
// outside a trailing template argument list, and only when `(` follows.
func TestCalledName(t *testing.T) {
	for _, c := range []struct {
		src, want string
		ok        bool
	}{
		{"buf.size()", "size", true},
		{"::fmt::detail::copy<Char>(a)", "copy", true},
		{"p->template get<0, T>  (x)", "get", true},
		{"f", "", false},             // no call follows
		{"x.~T()", "", false},        // a destructor
		{"operator()(1)", "", false}, // an operator
	} {
		src := []byte(c.src)
		end := strings.IndexByte(c.src, '(')
		if end < 0 {
			end = len(src)
		}
		for end > 0 && src[end-1] == ' ' {
			end--
		}
		got, _, ok := calledName(src, 0, end)
		if got != c.want || ok != c.ok {
			t.Errorf("calledName(%q) = %q, %v; want %q, %v", c.src, got, ok, c.want, c.ok)
		}
	}
}

// TestConstructedType: a construct expression's site is the type written before
// `(` or `{`, its last name outside template arguments.
func TestConstructedType(t *testing.T) {
	for _, c := range []struct {
		src, want string
		at        int
		ok        bool
	}{
		{"string_view(p, n)", "string_view", 0, true},
		{"std::basic_string<char>{}", "basic_string", 5, true},
		{"::fmt::v12::format_error(\"x\")", "format_error", 12, true},
		{"f(1)", "f", 0, true},
		{"x = 1", "", 0, false},
	} {
		got, at, ok := constructedType([]byte(c.src), 0, len(c.src))
		if got != c.want || ok != c.ok || (ok && at != c.at) {
			t.Errorf("constructedType(%q) = %q@%d, %v; want %q@%d, %v", c.src, got, at, ok, c.want, c.at, c.ok)
		}
	}
	if got := lastIdent("const fmt::basic_string_view<char>"); got != "basic_string_view" {
		t.Errorf("lastIdent = %q", got)
	}
}

// TestASTParserSites: a dump's location file is written only when it changes
// (an includedFrom names the includer and changes nothing), a macro argument's
// call is at its spelling location, a site in a template pattern stays
// dependent though an instantiation resolves it, a function's declaration
// names it, and a file outside the subject writes nothing.
func TestASTParserSites(t *testing.T) {
	root := t.TempDir()
	src := "template <class B> int f(B& b) { return b.size(); }\n" + // 1
		"int g(int);\n" + // 2
		"int h() { return CHECK(g(1)) + int(2); }\n" // 3
	if err := os.WriteFile(filepath.Join(root, "a.cc"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	a := filepath.Join(root, "a.cc")
	off := func(line, col int) int { // 1-based to byte offset
		ls := strings.SplitAfter(src, "\n")
		n := 0
		for i := 0; i < line-1; i++ {
			n += len(ls[i])
		}
		return n + col - 1
	}
	loc := func(o, tok int) string { return `{"offset":` + itoa(o) + `,"col":1,"tokLen":` + itoa(tok) + `}` }
	sizeAt := off(1, 43)
	dump := `{"id":"0x1","kind":"TranslationUnitDecl","loc":{},"range":{"begin":{},"end":{}},"inner":[
 {"id":"0x2","kind":"TypedefDecl","loc":{"offset":1,"file":"/usr/include/x.h","line":1,"col":1,"tokLen":1,"includedFrom":{"file":"` + a + `"}},"range":{"begin":{"offset":1,"col":1,"tokLen":1},"end":{"offset":1,"col":1,"tokLen":1}},
  "inner":[{"id":"0x3","kind":"DeclRefExpr","range":{"begin":{"offset":0,"col":1,"tokLen":1},"end":{"offset":0,"col":1,"tokLen":1}},"referencedDecl":{"id":"0x9","kind":"FunctionDecl","name":"x"}}]},
 {"id":"0x4","kind":"FunctionTemplateDecl","loc":{"offset":24,"file":"` + a + `","line":1,"col":25,"tokLen":1},"range":{"begin":` + loc(0, 8) + `,"end":` + loc(52, 1) + `},"inner":[
  {"id":"0x5","kind":"CXXDependentScopeMemberExpr","range":{"begin":` + loc(sizeAt-2, 1) + `,"end":` + loc(sizeAt, 4) + `},"type":{"qualType":"<dependent type>"},"member":"size"},
  {"id":"0x6","kind":"MemberExpr","range":{"begin":` + loc(sizeAt-2, 1) + `,"end":` + loc(sizeAt, 4) + `},"type":{"qualType":"<bound member function type>"},"name":"size"}]},
 {"id":"0x7","kind":"DeclRefExpr","range":{"begin":{"spellingLoc":` + loc(off(3, 24), 1) + `,"expansionLoc":{"offset":` + itoa(off(3, 18)) + `,"file":"` + a + `","col":18,"tokLen":5,"isMacroArgExpansion":true}},"end":{"spellingLoc":` + loc(off(3, 24), 1) + `,"expansionLoc":` + loc(off(3, 18), 5) + `}},"referencedDecl":{"id":"0xa","kind":"FunctionDecl","name":"g"}},
 {"id":"0xb","kind":"FunctionDecl","loc":` + loc(off(2, 5), 1) + `,"range":{"begin":` + loc(off(2, 1), 3) + `,"end":` + loc(off(2, 10), 1) + `},"name":"g"},
 {"id":"0x8","kind":"CXXFunctionalCastExpr","range":{"begin":` + loc(off(3, 32), 3) + `,"end":` + loc(off(3, 37), 1) + `},"type":{"qualType":"int"},"castKind":"NoOp"}
]}`
	var got []site
	p := &astParser{r: bufio.NewReader(strings.NewReader(dump)), dir: root, files: newFileCache(root), add: func(s site) { got = append(got, s) }}
	if err := p.top(); err != nil {
		t.Fatal(err)
	}
	merged := map[siteKey]string{}
	for _, s := range got {
		k := siteKey{s.Path, s.Line, s.Col, s.Name}
		if prev, ok := merged[k]; !ok || classRank[s.Class] < classRank[prev] {
			merged[k] = s.Class
		}
	}
	want := map[siteKey]string{
		{"a.cc", 1, 43, "size"}: "dependent",
		{"a.cc", 3, 24, "g"}:    "resolved",
		{"a.cc", 3, 32, "int"}:  "cast",
		{"a.cc", 2, 5, "g"}:     "declaration",
	}
	if !reflect.DeepEqual(merged, want) {
		t.Errorf("sites = %v, want %v", merged, want)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}
