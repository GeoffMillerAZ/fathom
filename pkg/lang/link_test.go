package lang_test

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/pkg/lang"
)

// This test binary does not import pkg/lang/cue, so the family is not
// registered: asking for it is an error that says what to import, and a
// Reader without it reads no CUE and lists none.
func TestUnlinkedCUE(t *testing.T) {
	if _, err := lang.New(lang.Link("cue")); err == nil || !strings.Contains(err.Error(), "pkg/lang/cue") {
		t.Fatalf("Link(cue) without the package: %v", err)
	}
	if _, err := lang.New(lang.Link("nope")); err == nil || !strings.Contains(err.Error(), "unknown family") {
		t.Fatalf("Link(nope): %v", err)
	}
	r := newReader(t)
	if res := extract(t, r, "a.cue", "package a\n\nx: 1\n"); res.Skipped != lang.SkipNotLinked {
		t.Fatalf("skipped %q, want %q", res.Skipped, lang.SkipNotLinked)
	}
	for _, d := range r.Dialects() {
		switch d.ID {
		case "cue", "timoni", "fathom-concerns", "fathom-layer-packs":
			t.Errorf("Dialects lists %s, a CUE-family dialect this Reader cannot read", d.ID)
		}
	}
}
