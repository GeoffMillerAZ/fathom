package readout

import (
	"os/exec"
	"strings"
	"testing"
)

// TestReadoutDoesNotImportFusion pins LL-6 (ADR-092): the outline builds a
// langroute.Router (newOutlineRouter), not a fusion.Pipeline, so the readout's
// closure holds neither the store nor the LLM client through the pipeline. It
// fails if the package, or anything it imports, reaches internal/fusion.
func TestReadoutDoesNotImportFusion(t *testing.T) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("no go toolchain on PATH")
	}
	out, err := exec.Command(goBin, "list", "-deps", "-f", "{{.ImportPath}}", ".").CombinedOutput()
	if err != nil {
		t.Skipf("go list failed (offline or no module cache): %v\n%s", err, out)
	}
	for _, p := range strings.Fields(string(out)) {
		if strings.HasSuffix(p, "/internal/fusion") {
			t.Fatalf("internal/readout reaches %s; the outline must build a langroute.Router (newOutlineRouter)", p)
		}
	}
}

// TestNewOutlineRouter: the outline's router builds, and reads a Go file.
func TestNewOutlineRouter(t *testing.T) {
	r, err := newOutlineRouter()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if _, ok := r.Exts()[".go"]; !ok {
		t.Error("the outline router has no .go extractor")
	}
	if _, ok := r.Exts()[".cue"]; !ok {
		t.Error("the outline router lost the CUE extractor it had through the pipeline")
	}
}
