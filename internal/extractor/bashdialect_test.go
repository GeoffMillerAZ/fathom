package extractor

// bashdialect_test.go — goldens for the bash dialect (LB-Q-bash, design
// §13.2 steps 10/12). Mirrors cdialect_test.go's per-dialect pattern
// (no generic "TestDialectGoldens" runner exists beyond
// TestTagsDialectParity's five migrated languages), reusing the generic
// extractGolden/marshalGolden helpers tsdialect_parity_test.go defines.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

var bashFixtures = []string{"functions.sh", "multiline.sh", "broken.sh"}

// TestBashDialectGoldens runs every named dialects/bash/testdata/*.sh
// fixture through NewTSExtractor("bash") and compares symbols against its
// hand-written *.golden.json (written from indexbench lang-probe's output,
// reviewed before this test first went green).
func TestBashDialectGoldens(t *testing.T) {
	e, err := NewTSExtractor("bash")
	if err != nil {
		t.Fatalf("NewTSExtractor(bash): %v", err)
	}
	defer e.Close()

	dir := filepath.Join("dialects", "bash", "testdata")
	for _, name := range bashFixtures {
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
