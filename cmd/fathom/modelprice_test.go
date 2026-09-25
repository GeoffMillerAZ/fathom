package main

// modelprice_test.go — refactor-conservation gap-closing wave 3, item 2
// (docs/plans/refactor-conservation.md): rateForModel is the small,
// explicit table a dry-run compile consults so it never prints a
// confident "est. 0.00-0.00" for a model it simply has no price data for.

import "testing"

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
