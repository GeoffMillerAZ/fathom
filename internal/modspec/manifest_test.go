package modspec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestManifestSchemaMatchesSource is the drift gate internal/semantic's own
// manifest.go doc comment names as the convention: manifest_schema.cue
// (embedded here) must stay byte-identical to schema/module/schema.cue (the
// authoritative source a human edits) — see doc.go / manifest.go's package
// comment.
func TestManifestSchemaMatchesSource(t *testing.T) {
	root := repoRoot(t)
	source, err := os.ReadFile(filepath.Join(root, "schema", "module", "schema.cue"))
	if err != nil {
		t.Fatalf("read schema/module/schema.cue: %v", err)
	}
	if string(source) != string(manifestSchema) {
		t.Fatalf("internal/modspec/manifest_schema.cue has drifted from schema/module/schema.cue — copy the source file verbatim over the embedded mirror")
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(wd, "..", "..")
}

func writeManifest(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "module.cue")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

const validModuleCUE = `
name:        "engineering-standards-module"
version:     "v1"
description: "Engineering standards and governing docs, compiled as claims."
corpus: include: ["CLAUDE.md", "ADR.md", "docs/design/*.md"]
watermark:    "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
tier_profile: "engineering"
eval_evidence: {
	evaled: true
	path:   "schema/module/eval/RESULTS.md"
}
recompile: {
	allowed:         true
	prompt_identity: "standards-module/v1"
}
license: "Apache-2.0"
provenance: {
	compiled_by: "claude-haiku-4-5-20251001"
	compiled_at: "2026-07-28T00:00:00Z"
}
`

func TestLoadManifest_Valid(t *testing.T) {
	path := writeManifest(t, validModuleCUE)
	m, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if m.Name != "engineering-standards-module" {
		t.Errorf("Name = %q", m.Name)
	}
	if m.Version != "v1" {
		t.Errorf("Version = %q", m.Version)
	}
	if len(m.Corpus.Include) != 3 {
		t.Errorf("Corpus.Include = %v", m.Corpus.Include)
	}
	if m.Watermark != "4b825dc642cb6eb9a060e54bf8d69288fbee4904" {
		t.Errorf("Watermark = %q", m.Watermark)
	}
	if !m.EvalEvidence.Evaled || m.EvalEvidence.Path == "" {
		t.Errorf("EvalEvidence = %+v", m.EvalEvidence)
	}
	if !m.Recompile.Allowed || m.Recompile.PromptIdentity == "" {
		t.Errorf("Recompile = %+v", m.Recompile)
	}
	if m.License != "Apache-2.0" {
		t.Errorf("License = %q", m.License)
	}
	if m.Provenance.CompiledBy == "" || m.Provenance.CompiledAt == "" {
		t.Errorf("Provenance = %+v", m.Provenance)
	}
}

// TestLoadManifest_DefaultsAppliedWhenFieldAbsent proves the reader applies
// §3.4/§3.7's schema defaults (tier_profile, license) via presence-checked
// lookups (lookupStringDefault), not plain zero-value decode — a manifest
// that omits tier_profile/license entirely still loads with the documented
// defaults, matching #Module's own `| *"engineering"` / `| *"unspecified"`.
func TestLoadManifest_DefaultsAppliedWhenFieldAbsent(t *testing.T) {
	src := `
name:        "minimal-module"
version:     "v1"
description: "A minimal module leaning on schema defaults."
corpus: include: ["docs/**"]
watermark: "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
eval_evidence: evaled: false
recompile: allowed: false
provenance: {
	compiled_by: "deterministic"
	compiled_at: "2026-07-28T00:00:00Z"
}
`
	path := writeManifest(t, src)
	m, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if m.TierProfile != "engineering" {
		t.Errorf("TierProfile default = %q, want \"engineering\"", m.TierProfile)
	}
	if m.License != "unspecified" {
		t.Errorf("License default = %q, want \"unspecified\"", m.License)
	}
}

// TestLoadManifest_UnknownFieldIgnored is the exact §3 reader contract:
// "a conforming manifest READER MUST ignore unknown fields it encounters."
// A manifest carrying a field no #Module revision this package knows about
// still loads cleanly under LoadManifest, even though the SAME file fails
// ValidateStrict (the authoring gate, which unifies against the closed
// schema and rejects it) — proving the two functions really do apply
// different strengths.
func TestLoadManifest_UnknownFieldIgnored(t *testing.T) {
	src := validModuleCUE + "\nfuture_field: \"something a v0.3 #Module might add\"\n"
	path := writeManifest(t, src)

	if _, err := LoadManifest(path); err != nil {
		t.Fatalf("LoadManifest should ignore the unknown field, got: %v", err)
	}
	if err := ValidateStrict(path); err == nil {
		t.Fatal("ValidateStrict should reject the unknown field (closedness is the authoring gate) but did not")
	}
}

func TestLoadManifest_Negatives(t *testing.T) {
	cases := []struct {
		name      string
		cue       string
		wantField string
	}{
		{"empty corpus include", `
name: "x", version: "v1", description: "x"
corpus: include: []
watermark: "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
eval_evidence: evaled: false
recompile: allowed: false
provenance: {compiled_by: "deterministic", compiled_at: "2026-07-28T00:00:00Z"}
`, "corpus.include"},
		{"missing watermark", `
name: "x", version: "v1", description: "x"
corpus: include: ["docs/**"]
eval_evidence: evaled: false
recompile: allowed: false
provenance: {compiled_by: "deterministic", compiled_at: "2026-07-28T00:00:00Z"}
`, "watermark"},
		{"malformed watermark", `
name: "x", version: "v1", description: "x"
corpus: include: ["docs/**"]
watermark: "not-a-real-watermark"
eval_evidence: evaled: false
recompile: allowed: false
provenance: {compiled_by: "deterministic", compiled_at: "2026-07-28T00:00:00Z"}
`, "watermark"},
		{"evaled:true with no path", `
name: "x", version: "v1", description: "x"
corpus: include: ["docs/**"]
watermark: "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
eval_evidence: evaled: true
recompile: allowed: false
provenance: {compiled_by: "deterministic", compiled_at: "2026-07-28T00:00:00Z"}
`, "eval_evidence.path"},
		{"recompile allowed with no prompt_identity", `
name: "x", version: "v1", description: "x"
corpus: include: ["docs/**"]
watermark: "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
eval_evidence: evaled: false
recompile: allowed: true
provenance: {compiled_by: "deterministic", compiled_at: "2026-07-28T00:00:00Z"}
`, "recompile.prompt_identity"},
		{"malformed compiled_at", `
name: "x", version: "v1", description: "x"
corpus: include: ["docs/**"]
watermark: "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
eval_evidence: evaled: false
recompile: allowed: false
provenance: {compiled_by: "deterministic", compiled_at: "not-a-date"}
`, "provenance.compiled_at"},
		{"bad name shape", `
name: "Not_Valid", version: "v1", description: "x"
corpus: include: ["docs/**"]
watermark: "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
eval_evidence: evaled: false
recompile: allowed: false
provenance: {compiled_by: "deterministic", compiled_at: "2026-07-28T00:00:00Z"}
`, "name"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeManifest(t, tc.cue)
			_, err := LoadManifest(path)
			if err == nil {
				t.Fatal("expected an error, got none")
			}
			le, ok := err.(*LoadError)
			if !ok {
				t.Fatalf("expected *LoadError, got %T: %v", err, err)
			}
			found := false
			for _, f := range le.Fields {
				if f.Field == tc.wantField {
					found = true
				}
			}
			if !found {
				t.Errorf("LoadError.Fields = %v, want a field named %q", le.Fields, tc.wantField)
			}
			if !strings.Contains(err.Error(), tc.wantField) {
				t.Errorf("error text %q does not mention field %q", err.Error(), tc.wantField)
			}
		})
	}
}

func TestLoadManifest_MissingFile(t *testing.T) {
	if _, err := LoadManifest(filepath.Join(t.TempDir(), "nope.cue")); err == nil {
		t.Fatal("expected an error for a missing file")
	}
}

func TestValidateStrict_ValidPasses(t *testing.T) {
	path := writeManifest(t, validModuleCUE)
	if err := ValidateStrict(path); err != nil {
		t.Fatalf("ValidateStrict: %v", err)
	}
}

func TestValidateStrict_MissingWatermarkFails(t *testing.T) {
	src := `
name: "x", version: "v1", description: "x"
corpus: include: ["docs/**"]
eval_evidence: evaled: false
recompile: allowed: false
provenance: {compiled_by: "deterministic", compiled_at: "2026-07-28T00:00:00Z"}
`
	path := writeManifest(t, src)
	if err := ValidateStrict(path); err == nil {
		t.Fatal("expected ValidateStrict to reject a manifest with no watermark")
	}
}

// TestTemplate_RoundTripsThroughLoadAndValidateStrict proves `fathom module
// init`'s scaffold is not just syntactically valid CUE but a fully
// conforming #Module instance: both the lenient reader and the strict
// authoring gate accept it unmodified.
func TestTemplate_RoundTripsThroughLoadAndValidateStrict(t *testing.T) {
	src := Template(TemplateInput{
		Name:        "my-module",
		Description: "A scaffolded module.",
		Include:     []string{"docs/**", "README.md"},
		Watermark:   "4b825dc642cb6eb9a060e54bf8d69288fbee4904",
		CompiledAt:  "2026-07-28T00:00:00Z",
	})
	path := writeManifest(t, src)

	if err := ValidateStrict(path); err != nil {
		t.Fatalf("ValidateStrict(Template output): %v\n---\n%s", err, src)
	}
	m, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest(Template output): %v\n---\n%s", err, src)
	}
	if m.Name != "my-module" {
		t.Errorf("Name = %q", m.Name)
	}
	if m.TierProfile != "engineering" {
		t.Errorf("TierProfile = %q", m.TierProfile)
	}
	if m.License != "unspecified" {
		t.Errorf("License = %q", m.License)
	}
	// Phase J critic finding 2: "deterministic" is store-format-v0 §7's
	// oracle-class, HIGH-TRUST provenance value — a freshly scaffolded
	// module that has never been compiled must not claim it by default,
	// the same honest-default discipline License already follows.
	if m.Provenance.CompiledBy != "unspecified" {
		t.Errorf("Provenance.CompiledBy = %q, want the honest default \"unspecified\"", m.Provenance.CompiledBy)
	}
	if m.Recompile.Allowed {
		t.Error("Template must scaffold recompile.allowed: false — a freshly scaffolded module has no prompt identity to name yet")
	}
	if m.EvalEvidence.Evaled {
		t.Error("Template must scaffold eval_evidence.evaled: false — a freshly scaffolded module has no eval evidence yet")
	}
}

func TestTemplate_WithExcludeAndOverrides(t *testing.T) {
	src := Template(TemplateInput{
		Name:        "another-module",
		Description: "Overrides tier_profile/license explicitly.",
		Include:     []string{"docs/**"},
		Exclude:     []string{"docs/design/**"},
		Watermark:   "manifest:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		TierProfile: "custom",
		License:     "MIT",
		CompiledAt:  "2026-07-28T00:00:00Z",
	})
	path := writeManifest(t, src)
	m, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest: %v\n---\n%s", err, src)
	}
	if m.TierProfile != "custom" {
		t.Errorf("TierProfile = %q", m.TierProfile)
	}
	if m.License != "MIT" {
		t.Errorf("License = %q", m.License)
	}
	if len(m.Corpus.Exclude) != 1 || m.Corpus.Exclude[0] != "docs/design/**" {
		t.Errorf("Corpus.Exclude = %v", m.Corpus.Exclude)
	}
}
