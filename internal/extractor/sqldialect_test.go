package extractor

// sqldialect_test.go — goldens for the sql dialect (LB-Q-sql, design §13.2
// steps 10/12), mirroring cdialect_test.go's per-dialect pattern (no
// generic "TestDialectGoldens" runner exists for a tags-engine dialect
// beyond TestTagsDialectParity's five migrated languages).

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

var sqlFixtures = []string{"shapes.sql", "quoted.sql", "broken.sql"}

// TestSQLDialectGoldens runs every named dialects/sql/testdata/*.sql
// fixture through NewTSExtractor("sql") and compares symbols against its
// hand-written *.golden.json (written from indexbench lang-probe's output,
// reviewed against each fixture before this test first went green).
func TestSQLDialectGoldens(t *testing.T) {
	e, err := NewTSExtractor("sql")
	if err != nil {
		t.Fatalf("NewTSExtractor(sql): %v", err)
	}
	defer e.Close()

	dir := filepath.Join("dialects", "sql", "testdata")
	for _, name := range sqlFixtures {
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
