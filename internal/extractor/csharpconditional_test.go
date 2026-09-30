package extractor

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// csharpTestdata reads one dialects/csharp/testdata fixture.
func csharpTestdata(t *testing.T, name string) []byte {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("dialects", "csharp", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return src
}

// csharpSymbols indexes nodes by name.
func csharpSymbols(nodes []core.SymbolNode) map[string]core.SymbolNode {
	out := map[string]core.SymbolNode{}
	for _, n := range nodes {
		out[n.Name] = n
	}
	return out
}

func csharpNames(m map[string]core.SymbolNode) []string {
	out := make([]string, 0, len(m))
	for n := range m {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// csharpBothExtractors runs src through the csharp dialect and through the
// CGO extractor ingest dispatches .cs to (fusion's pipeline).
func csharpBothExtractors(t *testing.T, path string, src []byte) map[string]map[string]core.SymbolNode {
	t.Helper()
	e, err := NewTSExtractor("csharp")
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	tsNodes, err := e.Extract("svc", path, src)
	if err != nil {
		t.Fatal(err)
	}
	cgoNodes, err := NewCSharpExtractor().Extract("svc", path, src)
	if err != nil {
		t.Fatal(err)
	}
	return map[string]map[string]core.SymbolNode{"dialect": csharpSymbols(tsNodes), "cgo": csharpSymbols(cgoNodes)}
}

// conditional.cs is O148's shape (Newtonsoft.Json 4e13299d,
// Src/Newtonsoft.Json.Tests/TestFixtureBase.cs L279-L284, reduced from
// that file with its trigger kept): two method headers in the branches of
// one #if/#else share one body, and the grammar, reading the group as a
// node whose branches must each hold whole declarations, parses no
// namespace_declaration for the file. OnlyElse is declared only in an #else
// branch.

// TestCSharpDialect_IfElseHeadersKeepNamespace (O148, LB-F53): the csharp
// dialect reads a file that parses neither as written nor with its #if
// directives blanked as each group's first branch, so the namespace parses
// and every type declared in it carries it as its package; a member
// declared only in an #else branch of such a file is not read (the
// measured trade).
func TestCSharpDialect_IfElseHeadersKeepNamespace(t *testing.T) {
	got := csharpBothExtractors(t, "src/Tests/Conditional.cs", csharpTestdata(t, "conditional.cs"))["dialect"]
	ns, ok := got["Fixture.Conditional"]
	if !ok || ns.Kind != core.KindCSharpNamespace {
		t.Fatalf("no CSharpNamespace Fixture.Conditional; symbols %v", csharpNames(got))
	}
	for _, name := range []string{"Utils", "Utils.Name", "Widget", "Widget.Before", "Widget.Check", "Widget.Widget", "Widget.OnlyFeature", "Checks", "Checks.Equal", "Checks.Normalize", "Checks.ThrowsAsync"} {
		n, ok := got[name]
		if !ok {
			t.Errorf("missing %s; symbols %v", name, csharpNames(got))
			continue
		}
		if n.Package != "Fixture.Conditional" {
			t.Errorf("%s: package %q, want Fixture.Conditional", name, n.Package)
		}
	}
	for _, name := range []string{"Widget.Setup", "Widget.OnlyElse"} {
		if _, ok := got[name]; ok {
			t.Errorf("%s: an #else branch of a file read as first branches is read", name)
		}
	}
	if w := got["Widget"]; w.Line != 12 || w.EndLine != 43 {
		t.Errorf("Widget: lines %d-%d, want 12-43 (offsets kept)", w.Line, w.EndLine)
	}
}

// TestCSharpExtractor_IfElseHeadersKeepNamespace is the same file through
// the CGO extractor: the namespace is emitted and spans every type declared
// in it, whose members qualify under their type. Its Package stays
// pkgFromPath's (O185).
func TestCSharpExtractor_IfElseHeadersKeepNamespace(t *testing.T) {
	got := csharpBothExtractors(t, "src/Tests/Conditional.cs", csharpTestdata(t, "conditional.cs"))["cgo"]
	ns, ok := got["Fixture.Conditional"]
	if !ok || ns.Kind != core.KindCSharpNamespace {
		t.Fatalf("no CSharpNamespace Fixture.Conditional; symbols %v", csharpNames(got))
	}
	if ns.Line != 3 || ns.EndLine != 82 {
		t.Errorf("namespace: lines %d-%d, want 3-82", ns.Line, ns.EndLine)
	}
	for _, name := range []string{"Utils", "Utils.Name", "Widget", "Widget.Before", "Widget.Check", "Widget.Widget", "Widget.OnlyFeature", "Checks", "Checks.Equal", "Checks.Normalize", "Checks.ThrowsAsync"} {
		n, ok := got[name]
		if !ok {
			t.Errorf("missing %s; symbols %v", name, csharpNames(got))
			continue
		}
		if n.Line < ns.Line || n.EndLine > ns.EndLine {
			t.Errorf("%s: lines %d-%d outside the namespace's %d-%d", name, n.Line, n.EndLine, ns.Line, ns.EndLine)
		}
	}
	for _, name := range []string{"Widget.Setup", "Widget.OnlyElse"} {
		if _, ok := got[name]; ok {
			t.Errorf("%s: an #else branch of a file read as first branches is read", name)
		}
	}
}

// TestCSharp_DirectivesBlankedKeepEveryBranch: a file that does not parse as
// written but parses with its #if directive lines blanked (branches.cs: a
// group inside an argument list) is read that way by both extractors, every
// branch kept, so Slow, declared only in an #else branch, is read.
func TestCSharp_DirectivesBlankedKeepEveryBranch(t *testing.T) {
	for label, got := range csharpBothExtractors(t, "src/Reader.cs", csharpTestdata(t, "branches.cs")) {
		for _, name := range []string{"Fixture.Branches", "Reader", "Reader.Read", "Reader.Fast", "Reader.Slow"} {
			if _, ok := got[name]; !ok {
				t.Errorf("%s: missing %s; symbols %v", label, name, csharpNames(got))
			}
		}
		if calls := got["Reader.Read"].Calls; !slices.Contains(calls, "Sum") {
			t.Errorf("%s: Reader.Read calls %v, want Sum", label, calls)
		}
	}
}

// TestCSharp_IfElseThatParsesReadsEveryBranch: a file that parses as
// written is read as written, both branches of each #if group included, by
// either extractor.
func TestCSharp_IfElseThatParsesReadsEveryBranch(t *testing.T) {
	src := []byte(`namespace N
{
    public class T
    {
#if FEATURE
        public void A() {}
#else
        public void B() {}
#endif
    }
}
`)
	for label, got := range csharpBothExtractors(t, "N/T.cs", src) {
		for _, name := range []string{"N", "T", "T.A", "T.B"} {
			if _, ok := got[name]; !ok {
				t.Errorf("%s: missing %s; symbols %v", label, name, csharpNames(got))
			}
		}
	}
}

// TestCSharpConditionalTexts pins the two readings: directive lines
// blanked, then non-first branches too; offsets and line breaks unchanged;
// a directive found only where the compiler finds one.
func TestCSharpConditionalTexts(t *testing.T) {
	// blank is s with every byte but a line break a space.
	blank := func(s string) string {
		return strings.Map(func(r rune) rune {
			if r == '\n' || r == '\r' {
				return r
			}
			return ' '
		}, s)
	}
	lines := func(ls ...string) string { return strings.Join(ls, "\n") + "\n" }

	t.Run("no group", func(t *testing.T) {
		src := lines("#region R", "class C {}", "#endregion")
		if got := csharpConditionalTexts([]byte(src)); got != nil {
			t.Fatalf("got %q, want nil", got)
		}
	})

	t.Run("groups nested and chained", func(t *testing.T) {
		src := lines(
			"class C {",
			"#if A",
			"  void X() {}",
			"# if B // nested",
			"  void Y() {}",
			"#endif",
			"#elif C",
			"  void Z() {}",
			"#else",
			"  #region R",
			"  void W() {}",
			"  #endregion",
			"#endif",
			"}",
		)
		wantAll := lines(
			"class C {",
			blank("#if A"),
			"  void X() {}",
			blank("# if B // nested"),
			"  void Y() {}",
			blank("#endif"),
			blank("#elif C"),
			"  void Z() {}",
			blank("#else"),
			"  #region R",
			"  void W() {}",
			"  #endregion",
			blank("#endif"),
			"}",
		)
		wantFirst := lines(
			"class C {",
			blank("#if A"),
			"  void X() {}",
			blank("# if B // nested"),
			"  void Y() {}",
			blank("#endif"),
			blank("#elif C"),
			blank("  void Z() {}"),
			blank("#else"),
			blank("  #region R"),
			blank("  void W() {}"),
			blank("  #endregion"),
			blank("#endif"),
			"}",
		)
		got := csharpConditionalTexts([]byte(src))
		if len(got) != 2 {
			t.Fatalf("got %d texts, want 2", len(got))
		}
		if string(got[0]) != wantAll {
			t.Errorf("directives blanked:\n%s\nwant:\n%s", got[0], wantAll)
		}
		if string(got[1]) != wantFirst {
			t.Errorf("first branches:\n%s\nwant:\n%s", got[1], wantFirst)
		}
		if bytes.Equal(got[0], []byte(src)) {
			t.Error("src aliased or unchanged")
		}
	})

	t.Run("no directive inside a comment or a multi-line string", func(t *testing.T) {
		src := lines(
			"/* a comment",
			"#if NOT_A_DIRECTIVE",
			"*/ var s = @\"verbatim",
			"#else",
			"\"; var r = \"\"\"",
			"#endif",
			"\"\"\";",
			"var c = '\"'; var q = \"/*\";",
			"#if A",
			"int a;",
			"#else",
			"int b;",
			"#endif",
		)
		want := lines(
			"/* a comment",
			"#if NOT_A_DIRECTIVE",
			"*/ var s = @\"verbatim",
			"#else",
			"\"; var r = \"\"\"",
			"#endif",
			"\"\"\";",
			"var c = '\"'; var q = \"/*\";",
			blank("#if A"),
			"int a;",
			blank("#else"),
			blank("int b;"),
			blank("#endif"),
		)
		got := csharpConditionalTexts([]byte(src))
		if len(got) != 2 || string(got[1]) != want {
			t.Fatalf("got:\n%s\nwant:\n%s", got, want)
		}
	})

	t.Run("CRLF and multi-byte text keep their offsets", func(t *testing.T) {
		src := "#if A\r\nint a; // é\r\n#else\r\nint b; // ü\r\n#endif\r\n"
		for _, text := range csharpConditionalTexts([]byte(src)) {
			if len(text) != len(src) {
				t.Fatalf("length %d, want %d", len(text), len(src))
			}
			for i := range src {
				if (src[i] == '\n' || src[i] == '\r') != (text[i] == '\n' || text[i] == '\r') {
					t.Fatalf("line break moved at byte %d: %q", i, text)
				}
			}
		}
	})
}
