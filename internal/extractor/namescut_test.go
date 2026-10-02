package extractor

import "testing"

// TestNameCutAt: names.cut_at keeps the captured text before its first occurrence
// of the marker, after strip_prefix and trim_quotes (LB-F124). A Conan
// reference is `name/version[@user/channel][#revision]`; the symbol it uses is
// the name, so a recipe's `self.requires("zlib/[>=1.2 <2]")` is a use of zlib.
// A text without the marker, one that starts with it, and a manifest that
// does not set it are all left as written.
func TestNameCutAt(t *testing.T) {
	r := &tsDialectRules{trimQuotes: true, cutAt: "/"}
	f := r.nameNormalizer()
	if f == nil {
		t.Fatal("cut_at must give a normalizer")
	}
	for in, want := range map[string]string{
		"zlib/1.3.1":                "zlib",
		`"zlib/1.3.1"`:              "zlib",
		"openssl/[>=3 <4]":          "openssl",
		"pkg/1.0@user/channel#rev":  "pkg",
		"gnu-config":                "gnu-config",
		"/leading":                  "/leading",
		"":                          "",
		`"unterminated/1.0`:         `"unterminated`,
		"zlib/1.3.1%1700000000.123": "zlib",
		"a/b/c":                     "a",
	} {
		if got := f(in); got != want {
			t.Errorf("cut_at(%q) = %q, want %q", in, got, want)
		}
	}
	if f := (&tsDialectRules{trimQuotes: true}).nameNormalizer(); f == nil || f("a/b") != "a/b" {
		t.Error("without cut_at the text is left whole")
	}
}
