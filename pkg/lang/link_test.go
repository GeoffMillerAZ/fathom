package lang_test

import (
	"os"
	"os/exec"
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

// The door's closure holds no CUE: importing pkg/lang must not link
// cuelang.org/go, which is what pkg/lang/cue exists to keep out (LL-16).
func TestDoorClosureHoldsNoCUE(t *testing.T) {
	cmd := exec.Command("go", "list", "-deps", "-f", "{{.ImportPath}}", ".")
	cmd.Env = append(os.Environ(), "GOWORK=off")
	out, err := cmd.Output()
	if err != nil {
		t.Skipf("go list: %v", err)
	}
	for _, p := range strings.Fields(string(out)) {
		if strings.HasPrefix(p, "cuelang.org/") || strings.Contains(p, "extractor/cuelang") {
			t.Errorf("pkg/lang links %s", p)
		}
	}
}
