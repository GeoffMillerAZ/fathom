package extractor

import (
	"strings"
	"testing"
)

// blankOut is s with each listed word (or exact span) replaced by spaces of
// its own length, the rewrite cppMacroTexts promises.
func blankOut(s string, spans ...string) string {
	for _, sp := range spans {
		s = strings.Replace(s, sp, strings.Repeat(" ", len(sp)), 1)
	}
	return s
}

// TestCppMacroTextsRewritesByShape (O455): each decorator, try/catch,
// lone-line, access-label and template-constraint macro is rewritten where
// it is written, at equal byte length; what the shapes do not name is kept.
func TestCppMacroTextsRewritesByShape(t *testing.T) {
	cases := []struct {
		name, src, want string
	}{
		{"class decorator", "class LIB_API logger final : public sink {};\n",
			blankOut("class LIB_API logger final : public sink {};\n", "LIB_API")},
		{"class decorator with arguments", "struct LIB_VISIBLE(\"default\") error {};\n",
			blankOut("struct LIB_VISIBLE(\"default\") error {};\n", `LIB_VISIBLE("default")`)},
		{"leading and trailing", "LIB_INLINE int f(int x) LIB_NOEXCEPT { return x; }\n",
			blankOut("LIB_INLINE int f(int x) LIB_NOEXCEPT { return x; }\n", "LIB_INLINE", "LIB_NOEXCEPT")},
		{"two leading", "template <typename T> LIB_CONSTEXPR LIB_INLINE auto g(T v) -> T { return v; }\n",
			blankOut("template <typename T> LIB_CONSTEXPR LIB_INLINE auto g(T v) -> T { return v; }\n", "LIB_CONSTEXPR", "LIB_INLINE")},
		{"after the return type", "std::string LIB_INLINE name(int l) { return n(l); }\n",
			blankOut("std::string LIB_INLINE name(int l) { return n(l); }\n", "LIB_INLINE")},
		{"before a destructor", "struct s {\n  LIB_API ~s();\n};\n",
			blankOut("struct s {\n  LIB_API ~s();\n};\n", "LIB_API")},
		{"after an attribute", "[[noreturn]] LIB_API void fail();\n",
			blankOut("[[noreturn]] LIB_API void fail();\n", "LIB_API")},
		{"try and catch", "void f() {\n  LIB_TRY { g(); }\n  LIB_CATCH(...) { h(); }\n}\n",
			"void f() {\n  try     { g(); }\n  catch    (...) { h(); }\n}\n"},
		{"a catch macro that is the whole handler", "void f() {\n  LIB_TRY { g(); }\n  LIB_LOGGER_CATCH(where)\n}\n",
			blankOut("void f() {\n  LIB_TRY { g(); }\n  LIB_LOGGER_CATCH(where)\n}\n", "LIB_TRY", "LIB_LOGGER_CATCH(where)")},
		{"lone namespace macros", "LIB_BEGIN_NAMESPACE\nint f();\nLIB_END_NAMESPACE\n",
			blankOut("LIB_BEGIN_NAMESPACE\nint f();\nLIB_END_NAMESPACE\n", "LIB_BEGIN_NAMESPACE", "LIB_END_NAMESPACE")},
		{"lone function-like macro", "LIB_PRAGMA(push)\nint f();\n",
			blankOut("LIB_PRAGMA(push)\nint f();\n", "LIB_PRAGMA(push)")},
		{"if constexpr macro", "void f() { if LIB_CONSTEXPR (x) { g(); } }\n",
			blankOut("void f() { if LIB_CONSTEXPR (x) { g(); } }\n", "LIB_CONSTEXPR")},
		{"access label macro", "class c {\nLIB_PRIVATE_UNLESS_TESTED:\n  int x;\n};\n",
			"class c {\nprivate                  :\n  int x;\n};\n"},
		{"template constraint", "template <typename T, LIB_ENABLE_IF(sizeof(T) > 1)> T w(T v);\n",
			blankOut("template <typename T, LIB_ENABLE_IF(sizeof(T) > 1)> T w(T v);\n", ", LIB_ENABLE_IF(sizeof(T) > 1)")},
		{"conditional directives", "template <typename C>\n#if A\nusing s = x<C>;\n#else\nusing s = y;\n#endif\n",
			blankOut("template <typename C>\n#if A\nusing s = x<C>;\n#else\nusing s = y;\n#endif\n", "#if A", "#else", "#endif")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			texts := cppMacroTexts([]byte(c.src))
			if len(texts) != 1 {
				t.Fatalf("cppMacroTexts: %d texts, want 1", len(texts))
			}
			if got := string(texts[0]); got != c.want {
				t.Errorf("rewrite\n got %q\nwant %q", got, c.want)
			}
			if len(texts[0]) != len(c.src) {
				t.Errorf("length %d, want %d: every offset must stay", len(texts[0]), len(c.src))
			}
		})
	}
}

// TestCppMacroTextsKeepsWhatIsNoDecorator: a macro that is a type, a call or
// a test-case head; a capitals word in a comment, a string, a raw string or a
// directive; and a capitals type parameter are left as written.
func TestCppMacroTextsKeepsWhatIsNoDecorator(t *testing.T) {
	for _, src := range []string{
		"HANDLE h{nullptr};\nDWORD n = 0;\n",                         // a type: its variable follows
		"void f() { ASSERT_EQ(a, b); }\n",                            // a call
		"TEST(suite, name) { run(); }\n",                             // a function-like macro head
		"// class LIB_API logger\nconst char *s = \"LIB_API x\";\n",  // comment and string
		"auto r = R\"(class LIB_API logger {)\";\n",                  // raw string
		"#define LIB_API __attribute__((visibility(\"default\")))\n", // a directive
		"template <typename T> T id(T v) { return v; }\n",            // nothing to do
		"int x = MAX_SIZE + 1;\n",                                    // an expression
		"struct MY_STRUCT var;\n",                                    // a type named in capitals
	} {
		if texts := cppMacroTexts([]byte(src)); texts != nil {
			t.Errorf("cppMacroTexts(%q) = %q, want nil", src, texts[0])
		}
	}
}
