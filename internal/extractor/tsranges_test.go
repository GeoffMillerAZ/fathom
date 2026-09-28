package extractor

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// TestTSExtractRangesAndSignatures (LB-F3, design §3.3): EndLine is the
// definition node's last line; a callable's Signature runs from the
// definition's start to its @body capture (the first line when there is no
// body), whitespace collapsed, capped at signature.max_chars with "…".
func TestTSExtractRangesAndSignatures(t *testing.T) {
	type want struct {
		line, end int
		sig       string
	}
	cases := []struct {
		dialect, path, src string
		want               map[string]want
	}{
		{"python", "m.py", `class Base:
    @staticmethod
    def build(
        first,
        second,
    ):
        return first

def short(a): return a
`, map[string]want{
			"Base":       {1, 7, ""},
			"Base.build": {3, 7, "def build( first, second, ):"},
			"short":      {9, 9, "def short(a):"},
		}},
		{"javascript", "m.js", `class Widget {
  render(a) {
    return a;
  }
}
const arrow = (a, b) =>
  a + b;
function* gen() { yield 1; }
`, map[string]want{
			"Widget":        {1, 5, ""},
			"Widget.render": {2, 4, "render(a)"},
			"arrow":         {6, 7, "arrow = (a, b) =>"},
			"gen":           {8, 8, "function* gen()"},
		}},
		{"typescript", "m.ts", `export interface Shape {
  area(): number;
}
class Base {
  protected describe(
    prefix: string,
  ): string {
    return prefix;
  }
}
`, map[string]want{
			"Shape":         {1, 3, ""},
			"Base.describe": {5, 9, "protected describe( prefix: string, ): string"},
		}},
		{"ruby", "m.rb", `module M
  class C
    def m(a,
          b)
      a
    end
    def self.s; end
  end
end
`, map[string]want{
			"M":      {1, 9, ""},
			"M::C":   {2, 8, ""},
			"M::C.m": {3, 6, "def m(a, b)"},
			"M::C.s": {7, 7, "def self.s; end"},
		}},
		{"kotlin", "m.kt", `interface S {
    fun area(): Double
}
class C {
    fun deep() = compute()
    fun f(
        a: Int,
    ): Int {
        return a
    }
}
`, map[string]want{
			"S":      {1, 3, ""},
			"S.area": {2, 2, "fun area(): Double"},
			"C.deep": {5, 5, "fun deep()"},
			"C.f":    {6, 10, "fun f( a: Int, ): Int"},
		}},
	}
	for _, c := range cases {
		t.Run(c.dialect, func(t *testing.T) {
			e, err := NewTSExtractor(c.dialect)
			if err != nil {
				t.Fatal(err)
			}
			defer e.Close()
			nodes, err := e.Extract("svc", c.path, []byte(c.src))
			if err != nil {
				t.Fatal(err)
			}
			seen := map[string]bool{}
			for _, n := range nodes {
				w, ok := c.want[n.Name]
				if !ok {
					continue
				}
				seen[n.Name] = true
				if n.Line != w.line || n.EndLine != w.end || n.Signature != w.sig {
					t.Errorf("%s: line %d end %d sig %q; want %d %d %q", n.Name, n.Line, n.EndLine, n.Signature, w.line, w.end, w.sig)
				}
			}
			for name := range c.want {
				if !seen[name] {
					t.Errorf("%s not extracted (got %v)", name, symStrings(nodes))
				}
			}
		})
	}

	t.Run("capped", func(t *testing.T) {
		long := "def f(" + strings.Repeat("argument_name, ", 30) + "last):\n    pass\n"
		e, _ := NewTSExtractor("python")
		defer e.Close()
		nodes, err := e.Extract("svc", "m.py", []byte(long))
		if err != nil || len(nodes) != 1 {
			t.Fatalf("nodes %v err %v", nodes, err)
		}
		sig := nodes[0].Signature
		if utf8.RuneCountInString(sig) != 200 || !strings.HasSuffix(sig, "…") || !strings.HasPrefix(sig, "def f(argument_name, ") {
			t.Errorf("capped signature = %q (%d runes)", sig, utf8.RuneCountInString(sig))
		}
	})
}
