package extractor

// csharpdialect_test.go — goldens for the csharp dialect (LB-Q-csharp,
// design §13.2 steps 10/12). Mirrors cdialect_test.go's pattern (no generic
// "TestDialectGoldens" runner exists for a tags-engine dialect beyond
// TestTagsDialectParity's five migrated languages).

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

var csharpFixtures = []string{"shapes.cs", "namespaces.cs", "imports.cs", "operators.cs", "broken.cs", "conditional.cs", "branches.cs"}

// TestCSharpDialectGoldens runs every named dialects/csharp/testdata/*.cs
// fixture through NewTSExtractor("csharp") and compares symbols + imports
// against its hand-written *.golden.json (written from indexbench
// lang-probe's own capture and tree output, reviewed against the fixtures
// before this test first went green — design §13.2 step 10).
func TestCSharpDialectGoldens(t *testing.T) {
	e, err := NewTSExtractor("csharp")
	if err != nil {
		t.Fatalf("NewTSExtractor(csharp): %v", err)
	}
	defer e.Close()

	dir := filepath.Join("dialects", "csharp", "testdata")
	for _, name := range csharpFixtures {
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
