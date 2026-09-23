package testevidence

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestTestKitManifestSchemaMatchesSource is the drift gate: the embedded
// testkit_manifest.cue must be byte-identical to schema/testkit/manifest.cue
// (internal/config's TestConfigSchemaMatchesSource precedent).
func TestTestKitManifestSchemaMatchesSource(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	src := filepath.Join(filepath.Dir(file), "..", "..", "schema", "testkit", "manifest.cue")
	want, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read source schema %s: %v", src, err)
	}
	if string(want) != string(testKitManifestSchema) {
		t.Fatalf(
			"internal/testevidence/testkit_manifest.cue has drifted from %s — "+
				"copy the source file verbatim (schema/testkit/manifest.cue is authoritative)",
			src,
		)
	}
}

const validManifest = `
component: "cart"
suites: unit: {
	class:  "unit"
	runner: "go-test"
	inputs: paths: ["."]
	needs: []
	runtime: {}
	flake: {}
	tier: "blocking"
}
`

const missingRunnerManifest = `
component: "broken"
suites: unit: {
	class: "unit"
	inputs: paths: ["."]
	needs: []
	runtime: {}
	flake: {}
	tier: "blocking"
}
`

func writeManifest(t *testing.T, root, name, content string) {
	t.Helper()
	dir := filepath.Join(root, "tests")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadManifests_ValidAndMalformed(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "cart.cue", validManifest)
	writeManifest(t, root, "broken.cue", missingRunnerManifest)

	manifests, warnings, err := LoadManifests(root)
	if err != nil {
		t.Fatalf("LoadManifests: %v", err)
	}
	if len(manifests) != 1 {
		t.Fatalf("want 1 valid manifest, got %d (%+v)", len(manifests), manifests)
	}
	m := manifests[0]
	if m.Component != "cart" {
		t.Errorf("component = %q, want cart", m.Component)
	}
	if len(m.Suites) != 1 || m.Suites[0].Name != "unit" || m.Suites[0].Class != "unit" || m.Suites[0].Tier != "blocking" {
		t.Errorf("suites = %+v, want one unit/unit/blocking suite", m.Suites)
	}

	if len(warnings) != 1 {
		t.Fatalf("want 1 warning for the malformed manifest, got %d: %v", len(warnings), warnings)
	}
	if !strings.Contains(warnings[0], "broken.cue") {
		t.Errorf("warning %q does not name the malformed file", warnings[0])
	}
}

func TestLoadManifests_NoTestsDir(t *testing.T) {
	root := t.TempDir()
	manifests, warnings, err := LoadManifests(root)
	if err != nil {
		t.Fatalf("LoadManifests: %v", err)
	}
	if len(manifests) != 0 || len(warnings) != 0 {
		t.Fatalf("want empty result for a root with no tests/, got manifests=%v warnings=%v", manifests, warnings)
	}
}

func TestLoadManifests_ConformanceRequiresPort(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "adapter.cue", `
component: "adapter"
suites: conf: {
	class:  "conformance"
	runner: "go-test"
	inputs: paths: ["."]
	needs: []
	runtime: {}
	flake: {}
	tier: "blocking"
}
`)
	_, warnings, err := LoadManifests(root)
	if err != nil {
		t.Fatalf("LoadManifests: %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("want a warning for a conformance suite missing port, got %v", warnings)
	}
}
