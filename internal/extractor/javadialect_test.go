package extractor

// javadialect_test.go — goldens for the java dialect (LB-Q-java, design
// §13.2 steps 10/12). No generic "TestDialectGoldens" runner exists for a
// tags-engine dialect beyond TestTagsDialectParity's five migrated
// languages (see that file's own comment); this mirrors
// cdialect_test.go/terraformdialect_test.go's per-dialect pattern instead,
// reusing the generic extractGolden/marshalGolden helpers
// tsdialect_parity_test.go already defines for a tags dialect.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var javaFixtures = []string{"shapes.java", "imports.java", "broken.java", "ctors.java"}

// TestJavaDialectGoldens runs every named dialects/java/testdata/*.java
// fixture through NewTSExtractor("java") and compares symbols against its
// hand-written *.golden.json (written by reading each fixture and reasoning
// through tsextract.go's qualification/signature/sort rules, cross-checked
// against indexbench lang-probe's raw capture output — LB-Q-java's own
// PROVENANCE.md has the detail).
func TestJavaDialectGoldens(t *testing.T) {
	e, err := NewTSExtractor("java")
	if err != nil {
		t.Fatalf("NewTSExtractor(java): %v", err)
	}
	defer e.Close()

	dir := filepath.Join("dialects", "java", "testdata")
	for _, name := range javaFixtures {
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

// TestJavaDialectConstructorCalls (O268, LB-F230): a constructor invocation
// is a call to its class by its simple name, plain, generic, qualified,
// `outer.new Inner()` and an anonymous class alike; the anonymous class's
// own calls stay with its method, and array creation is no call.
func TestJavaDialectConstructorCalls(t *testing.T) {
	e, err := NewTSExtractor("java")
	if err != nil {
		t.Fatalf("NewTSExtractor(java): %v", err)
	}
	defer e.Close()
	got := extractGolden(t, e, filepath.Join("dialects", "java", "testdata", "ctors.java"))
	calls := map[string]string{}
	for _, s := range got.Symbols {
		calls[s.Name] = strings.Join(s.Calls, ",")
	}
	for name, want := range map[string]string{
		"Factory.make":      "ArrayList,add",
		"Factory.nested":    "Inner,Member",
		"Factory.qualified": "HashMap",
		"Factory.anon":      "Runnable",
		"run":               "Widget,helper",
		"Factory.arrays":    "",
	} {
		if calls[name] != want {
			t.Errorf("%s calls %q, want %q", name, calls[name], want)
		}
	}
}
