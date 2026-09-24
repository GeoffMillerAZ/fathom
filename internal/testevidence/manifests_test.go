package testevidence

import (
	"os"
	"path/filepath"
	"reflect"
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

// TestLoadManifestsFromContents_MatchesLoadManifests pins the factored-out
// entry point ADR-044 §1.2 needs: a caller that has tests/*.cue CONTENT
// from somewhere other than root's own live directory (a `git show
// <base>:tests/cart.cue` read, for internal/conserve's integrity plane
// judging guardedness "at base") gets the identical validation and
// decoding LoadManifests itself applies to files it reads off disk.
func TestLoadManifestsFromContents_MatchesLoadManifests(t *testing.T) {
	files := map[string][]byte{
		"tests/cart.cue":   []byte(validManifest),
		"tests/broken.cue": []byte(missingRunnerManifest),
	}
	manifests, warnings, err := LoadManifestsFromContents(files)
	if err != nil {
		t.Fatalf("LoadManifestsFromContents: %v", err)
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
	if len(warnings) != 1 || !strings.Contains(warnings[0], "broken.cue") {
		t.Fatalf("warnings = %v, want exactly one naming broken.cue", warnings)
	}
}

// TestLoadManifestsFromContents_EmptyIsNilNilNil mirrors LoadManifests'
// own "no manifests" contract for an empty content map.
func TestLoadManifestsFromContents_EmptyIsNilNilNil(t *testing.T) {
	manifests, warnings, err := LoadManifestsFromContents(nil)
	if err != nil || manifests != nil || warnings != nil {
		t.Fatalf("got manifests=%v warnings=%v err=%v, want nil,nil,nil", manifests, warnings, err)
	}
}

// TestLoadManifestsFromContents_DecodesCertifies proves the certifies
// decoding path (TC-15/17) survives the factor-out unchanged — this is
// the exact fact internal/conserve's integrity plane reads to build
// GuardedInput.ManifestCertifies.
func TestLoadManifestsFromContents_DecodesCertifies(t *testing.T) {
	files := map[string][]byte{
		"tests/cart.cue": []byte(`
component: "cart"
suites: unit: {
	class:  "unit"
	runner: "go-test"
	inputs: paths: ["."]
	needs: []
	runtime: {}
	flake: {}
	tier: "blocking"
	certifies: [
		{kind: "invariant", id: "I5", tests: ["TestGuardedByManifest"]},
	]
}
`),
	}
	manifests, warnings, err := LoadManifestsFromContents(files)
	if err != nil {
		t.Fatalf("LoadManifestsFromContents: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("want no warnings, got %v", warnings)
	}
	if len(manifests) != 1 || len(manifests[0].Suites) != 1 {
		t.Fatalf("want 1 manifest with 1 suite, got %+v", manifests)
	}
	refs := manifests[0].Suites[0].Certifies
	if len(refs) != 1 || len(refs[0].Tests) != 1 || refs[0].Tests[0] != "TestGuardedByManifest" {
		t.Fatalf("Certifies = %+v, want one ref naming TestGuardedByManifest", refs)
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

// TestLoadManifests_DecodesSuiteCertifies pins TC-15/TC-17: now that
// test-kit's #Suite.certifies has landed upstream (schema/testkit/manifest.cue
// re-mirrored), decodeManifest must actually populate Suite.Certifies from a
// manifest's declared `certifies` block — not leave it perpetually empty
// (types.go's old comment: "always empty this round"). internal/certify's
// CompileCertifications already reads Suite.Certifies through
// DeclaredCertifiesJSON; a decoder that drops the field silently makes every
// suite-level certifies declaration inert.
func TestLoadManifests_DecodesSuiteCertifies(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "cart.cue", `
component: "cart"
suites: unit: {
	class:  "unit"
	runner: "go-test"
	inputs: paths: ["."]
	needs: []
	runtime: {}
	flake: {}
	tier: "blocking"
	certifies: [
		{kind: "invariant", id: "I5", tests: ["TestQuery_EmptyConcernIDReturnsError"]},
		{kind: "incident", id: "INC-21", strength: "smoke"},
	]
}
`)
	manifests, warnings, err := LoadManifests(root)
	if err != nil {
		t.Fatalf("LoadManifests: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("want no warnings, got %v", warnings)
	}
	if len(manifests) != 1 || len(manifests[0].Suites) != 1 {
		t.Fatalf("want 1 manifest with 1 suite, got %+v", manifests)
	}
	refs := manifests[0].Suites[0].Certifies
	if len(refs) != 2 {
		t.Fatalf("Certifies = %+v, want 2 entries", refs)
	}
	want0 := CertifyRef{Kind: "invariant", ID: "I5", Strength: "asserts", Tests: []string{"TestQuery_EmptyConcernIDReturnsError"}}
	if !reflect.DeepEqual(refs[0], want0) {
		t.Errorf("refs[0] = %+v, want %+v", refs[0], want0)
	}
	want1 := CertifyRef{Kind: "incident", ID: "INC-21", Strength: "smoke"}
	if !reflect.DeepEqual(refs[1], want1) {
		t.Errorf("refs[1] = %+v, want %+v", refs[1], want1)
	}
}
