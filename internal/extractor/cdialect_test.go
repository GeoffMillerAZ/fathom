package extractor

// cdialect_test.go — goldens for the c dialect (LB-Q-c, design §13.2 steps
// 10/12). No generic "TestDialectGoldens" runner exists for a tags-engine
// dialect beyond TestTagsDialectParity's five migrated languages (see that
// file's own comment); this mirrors terraformdialect_test.go/
// pyprojectdialect_test.go's per-dialect pattern instead, reusing the
// generic extractGolden/marshalGolden helpers tsdialect_parity_test.go
// already defines for a tags dialect. Fixtures are named explicitly (not
// scanned) because testdata/ will eventually want header targets for a
// deeper include-path resolution fixture, which are not their own
// extraction goldens.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

var cFixtures = []string{"shapes.c", "includes.c", "broken.c"}

// TestCDialectGoldens runs every named dialects/c/testdata/*.c fixture
// through NewTSExtractor("c") and compares symbols + imports against its
// hand-written *.golden.json (written from indexbench lang-probe's output,
// reviewed before this test first went green).
func TestCDialectGoldens(t *testing.T) {
	e, err := NewTSExtractor("c")
	if err != nil {
		t.Fatalf("NewTSExtractor(c): %v", err)
	}
	defer e.Close()

	dir := filepath.Join("dialects", "c", "testdata")
	for _, name := range cFixtures {
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
