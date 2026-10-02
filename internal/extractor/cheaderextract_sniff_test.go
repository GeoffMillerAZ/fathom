package extractor

import "testing"

// TestLooksLikeCppIgnoresCommentsAndStrings (LB-F195, O365): a C++ signal word
// in a comment or a string literal is prose, not C++; the sniff reads code only.
func TestLooksLikeCppIgnoresCommentsAndStrings(t *testing.T) {
	c := map[string]string{
		"line comment":  "/* no */ // a class of handles\nint f(void);\n",
		"block comment": "/*\n * the namespace of ids; a template for X::Y\n */\nint f(void);\n",
		"string":        "static const char *s = \"public: virtual class\";\nint f(void);\n",
		"char literal":  "int c = ':'; /* constexpr */\n",
		"error text":    "#error don't use this class here\nint f(void);\n",
		"extern C":      "#ifdef __cplusplus\nextern \"C\" {\n#endif\nint f(void);\n",
	}
	for name, src := range c {
		if looksLikeCpp([]byte(src)) {
			t.Errorf("%s: routed to C++, want C", name)
		}
	}
	cpp := map[string]string{
		"class after comment": "/* c */\nclass A {};\n",
		"scope operator":      "int x = std::max(1, 2);\n",
		"namespace":           "// note\nnamespace n { }\n",
		"digit separator":     "int big = 1'000; template <class T> T f();\n",
	}
	for name, src := range cpp {
		if !looksLikeCpp([]byte(src)) {
			t.Errorf("%s: routed to C, want C++", name)
		}
	}
}
