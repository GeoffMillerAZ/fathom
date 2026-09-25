package main

// modelprice_test.go — refactor-conservation gap-closing wave 3, item 2
// (docs/plans/refactor-conservation.md): rateForModel is the small,
// explicit table a dry-run compile consults so it never prints a
// confident "est. 0.00-0.00" for a model it simply has no price data for.

import (
	"os"
	"regexp"
	"testing"
)

func TestRateForModel_KnownDefaultCompileModel(t *testing.T) {
	in, out, ok := rateForModel(defaultCompileModel)
	if !ok {
		t.Fatalf("rateForModel(%q) ok = false, want true (the cheap-tier default is priced)", defaultCompileModel)
	}
	if in <= 0 || out <= 0 {
		t.Errorf("rateForModel(%q) = (%v, %v), want positive rates", defaultCompileModel, in, out)
	}
	if in != knownModelInRatePerKTok || out != knownModelOutRatePerKTok {
		t.Errorf("rateForModel(%q) = (%v, %v), want the named constants (%v, %v)", defaultCompileModel, in, out, knownModelInRatePerKTok, knownModelOutRatePerKTok)
	}
}

func TestRateForModel_UnknownModel(t *testing.T) {
	in, out, ok := rateForModel("some-custom-unpriced-model")
	if ok {
		t.Fatalf("rateForModel(unrecognized) ok = true, want false")
	}
	if in != 0 || out != 0 {
		t.Errorf("rateForModel(unrecognized) = (%v, %v), want (0, 0) on an unknown model", in, out)
	}
}

func TestRateForModel_EmptyModel(t *testing.T) {
	if _, _, ok := rateForModel(""); ok {
		t.Errorf("rateForModel(\"\") ok = true, want false — an empty model name is never a known price")
	}
}

// TestRealRunWithLLMCallSitesShareNamedRateConstants pins this file's own
// doc comment: knownModelInRatePerKTok/knownModelOutRatePerKTok exist "so
// rateForModel and those call sites read the identical number under one
// name instead of a second, silently driftable literal" — naming
// compile.go's and servehttp.go's real (non-dry-run) WithLLM call sites as
// "those call sites". Both build a real LLM client and hit a live
// provider, which no test in this suite does, so this is a source-lexical
// check (`docs/plans/README.md`'s own `oracle: lexical` convention, here
// as a Go regression test rather than a plan row): a caller changing only
// modelprice.go's constants, trusting this doc comment, must have that
// change actually reach both real-run call sites, not silently diverge
// from an untouched 0.001/0.005 literal.
func TestRealRunWithLLMCallSitesShareNamedRateConstants(t *testing.T) {
	sharedRate := regexp.MustCompile(`WithLLM\([^\n]*?,\s*knownModelInRatePerKTok\s*,\s*knownModelOutRatePerKTok\s*\)`)
	literalRate := regexp.MustCompile(`\b0\.001\b|\b0\.005\b`)
	for _, path := range []string{"compile.go", "servehttp.go"} {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if !sharedRate.Match(src) {
			t.Errorf("%s: no WithLLM(..., knownModelInRatePerKTok, knownModelOutRatePerKTok) call found — its real-run rate no longer reads the shared named constant this file documents", path)
		}
		if literalRate.Match(src) {
			t.Errorf("%s: contains a literal 0.001/0.005 rate — reintroduces the second, driftable literal knownModelInRatePerKTok/knownModelOutRatePerKTok exist to replace", path)
		}
	}
}
