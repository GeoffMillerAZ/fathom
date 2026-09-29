package connector_test

// unsupported_language_test.go — coverage for
// connector.UnsupportedLanguageForPath (O69, wave P1). The Long Code Arena
// bench run found bumble-tech/appyx — a real Kotlin repository — reporting
// only ~165 symbols with its detected language misread, find_symbol
// returning zero Kotlin candidates, and outline coming back empty: Kotlin
// had no extractor (no tree-sitter grammar wired, no dedicated extractor
// file), so every .kt/.kts file fell through fusion's dispatch to the
// "no extractor registered" branch and was fingerprinted-and-skipped with NO
// counter anywhere naming what was lost. UnsupportedLanguageForPath is the
// disclosure primitive: it names a KNOWN programming language for an
// extension that has no registered extractor, so ingest/list_services/
// doctor can say "<lang>: 412 files, no extractor" instead of staying
// silent. It must never overlap connector.RegisteredExtensions() — a path
// with a real extractor is never "unsupported". Kotlin closed this gap in
// wave P2 (O81, extractor.NewTSExtractor("kotlin")) and was removed from
// unsupportedLangExtMap — see language_test.go for its now-registered
// coverage.

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
)

func TestUnsupportedLanguageForPath_KnownLanguages(t *testing.T) {
	cases := map[string]string{
		"lib.ex":       "elixir",
		"lib_test.exs": "elixir",
		"core.clj":     "clojure",
		"core.cljs":    "clojure",
		"core.cljc":    "clojure",
		"Main.hs":      "haskell",
		"lib.ml":       "ocaml",
		"lib.mli":      "ocaml",
		// "Program.fs" is not listed since LB-F25: .fs is F# or a GLSL
		// fragment shader by sniff (design §5), so it is never disclosed.
		"script.fsx":   "fsharp",
		"mod.erl":      "erlang",
		"mod.hrl":      "erlang",
		"main.zig":     "zig",
		"main.nim":     "nim",
		"script.jl":    "julia",
		"Main.elm":     "elm",
		"main.cr":      "crystal",
		"Build.groovy": "groovy",
	}
	for path, want := range cases {
		got, ok := connector.UnsupportedLanguageForPath(path)
		if !ok {
			t.Errorf("UnsupportedLanguageForPath(%q) ok = false, want true (%s)", path, want)
			continue
		}
		if got != want {
			t.Errorf("UnsupportedLanguageForPath(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestUnsupportedLanguageForPath_RegisteredExtensionsNeverMatch(t *testing.T) {
	// A path with a real, registered extractor is never "unsupported" — the
	// two tables must stay disjoint or a supported language would get
	// double-counted as both extracted and unsupported.
	for _, ext := range connector.RegisteredExtensions() {
		path := "x" + ext
		if ext[0] != '.' {
			path = ext // full basename key, e.g. "Dockerfile", "go.mod"
		}
		if lang, ok := connector.UnsupportedLanguageForPath(path); ok {
			t.Errorf("UnsupportedLanguageForPath(%q) = (%q, true), want ok=false — %q has a registered extractor", path, lang, ext)
		}
	}
}

func TestUnsupportedLanguageForPath_NonCodeNeverMatches(t *testing.T) {
	for _, path := range []string{
		"README.rst",
		"data.xyz",
		"go.sum",
		"Gemfile.lock",
		"Cargo.lock",
		"requirements.txt",
		"noext",
		"image.png",
		"Program.fs", // ambiguous: F# or GLSL (design §5)
		"lib.m",      // ambiguous: Objective-C or MATLAB
		"top.v",      // ambiguous: Verilog, Coq or V
		"tool.pl",    // ambiguous: Perl or Prolog
	} {
		if lang, ok := connector.UnsupportedLanguageForPath(path); ok {
			t.Errorf("UnsupportedLanguageForPath(%q) = (%q, true), want ok=false", path, lang)
		}
	}
}
