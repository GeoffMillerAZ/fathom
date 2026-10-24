package extractor

// jinja2dialect_test.go — goldens for the jinja2 dialect (LB-Q-jinja2, design
// §13.2 steps 10/12). Mirrors bashdialect_test.go, reusing the generic
// extractGolden/marshalGolden
// helpers tsdialect_parity_test.go defines.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

var jinja2Fixtures = []string{"base.html.j2", "page.html.j2", "broken.jinja"}

// TestJinja2DialectGoldens runs every named dialects/jinja2/testdata/*
// fixture through NewTSExtractor("jinja2") and compares symbols against its
// hand-written *.golden.json (written from indexbench lang-probe's output,
// reviewed before this test first went green).
func TestJinja2DialectGoldens(t *testing.T) {
	e, err := NewTSExtractor("jinja2")
	if err != nil {
		t.Fatalf("NewTSExtractor(jinja2): %v", err)
	}
	defer e.Close()

	dir := filepath.Join("dialects", "jinja2", "testdata")
	for _, name := range jinja2Fixtures {
		fx := filepath.Join(dir, name)
		t.Run(name, func(t *testing.T) {
			got := extractGolden(t, e, fx)
			gp := goldenPathFor(fx)
			if *updateGoldens {
				if err := os.WriteFile(gp, marshalGolden(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			raw, err := os.ReadFile(gp)
			if err != nil {
				t.Fatalf("read golden: %v (run with -update to create)", err)
			}
			var want goldenFile
			if err := json.Unmarshal(raw, &want); err != nil {
				t.Fatalf("%s: %v", gp, err)
			}
			gb, wb := marshalGolden(got), marshalGolden(want)
			if !bytes.Equal(gb, wb) {
				t.Errorf("%s: mismatch\n--- got\n%s\n--- want\n%s", fx, gb, wb)
			}
		})
	}
}
