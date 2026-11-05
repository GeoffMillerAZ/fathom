package cuelang_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/extractor/cuelang"
)

var cueFamily = []string{"cue", "timoni", "fathom-concerns", "fathom-layer-packs"}

// TestRegistrationPrecedesFirstUse: this test makes no call to RegisterCUE
// before its first lookup, so every dialect it finds was registered by the
// package's init, which runs before any test or main.
func TestRegistrationPrecedesFirstUse(t *testing.T) {
	for _, id := range cueFamily {
		e, ok := extractor.NativeDialectExtractor(id)
		if !ok || e == nil {
			t.Fatalf("NativeDialectExtractor(%q) = %v, %v; want the CUE family registered at init", id, e, ok)
		}
	}
	fs, err := extractor.NewFileSpecExtractor("cue")
	if err != nil {
		t.Fatal(err)
	}
	if fs.Name() != "cue" {
		t.Errorf("NewFileSpecExtractor(cue).Name() = %q, want the native CUE reader", fs.Name())
	}
	langs, err := extractor.FileResolveLangs()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, l := range langs {
		if l.Lang == "cue" {
			found = true
		}
	}
	if !found {
		t.Error("FileResolveLangs omits cue after registration")
	}
}

func TestRegisterCUEIsIdempotent(t *testing.T) {
	cuelang.RegisterCUE()
	cuelang.RegisterCUE()
	for _, id := range cueFamily {
		if _, ok := extractor.NativeDialectExtractor(id); !ok {
			t.Errorf("%q lost after a second RegisterCUE", id)
		}
	}
}
