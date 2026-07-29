package bindspec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBindingSchemaMatchesSource(t *testing.T) {
	root := repoRoot(t)
	source, err := os.ReadFile(filepath.Join(root, "schema", "binding", "schema.cue"))
	if err != nil {
		t.Fatalf("read schema/binding/schema.cue: %v", err)
	}
	if string(source) != string(bindingSchema) {
		t.Fatalf("internal/bindspec/binding_schema.cue has drifted from schema/binding/schema.cue — copy the source file verbatim over the embedded mirror")
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

func writeBindings(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "modules.cue")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

const validBindingsCUE = `
bindings: [{
	module:    "platform-standards"
	scopes:    ["infra/**", "**/*.go"]
	max_rules: 3
	min_tier:  "doc-claimed"
}]
`

func TestLoadBindings_Valid(t *testing.T) {
	path := writeBindings(t, validBindingsCUE)
	set, err := LoadBindings(path)
	if err != nil {
		t.Fatalf("LoadBindings: %v", err)
	}
	if len(set.Bindings) != 1 {
		t.Fatalf("Bindings = %v", set.Bindings)
	}
	b := set.Bindings[0]
	if b.Module != "platform-standards" {
		t.Errorf("Module = %q", b.Module)
	}
	if len(b.Scopes) != 2 {
		t.Errorf("Scopes = %v", b.Scopes)
	}
	if b.MaxRules != 3 {
		t.Errorf("MaxRules = %d", b.MaxRules)
	}
	if b.MinTier != "doc-claimed" {
		t.Errorf("MinTier = %q", b.MinTier)
	}
}

func TestLoadBindings_DefaultsAppliedWhenFieldAbsent(t *testing.T) {
	path := writeBindings(t, `bindings: [{module: "platform-standards"}]`)
	set, err := LoadBindings(path)
	if err != nil {
		t.Fatalf("LoadBindings: %v", err)
	}
	b := set.Bindings[0]
	if b.MaxRules != DefaultMaxRules {
		t.Errorf("MaxRules default = %d, want %d", b.MaxRules, DefaultMaxRules)
	}
	if len(b.Scopes) != 0 {
		t.Errorf("Scopes = %v, want none (module's own scopes apply)", b.Scopes)
	}
	if b.MinTier != "" {
		t.Errorf("MinTier = %q, want empty (no floor)", b.MinTier)
	}
}

func TestLoadBindings_MultipleBindings(t *testing.T) {
	path := writeBindings(t, `
bindings: [
	{module: "platform-standards"},
	{module: "security-baseline", scopes: ["**/*.tf"], max_rules: 1},
]
`)
	set, err := LoadBindings(path)
	if err != nil {
		t.Fatalf("LoadBindings: %v", err)
	}
	if len(set.Bindings) != 2 {
		t.Fatalf("Bindings = %v", set.Bindings)
	}
	if set.Bindings[1].Module != "security-baseline" || set.Bindings[1].MaxRules != 1 {
		t.Errorf("Bindings[1] = %+v", set.Bindings[1])
	}
}

// TestLoadBindings_UnknownFieldIgnored is the exact reader contract
// mirrored from internal/modspec: a binding file carrying a field no
// #Bindings revision this package knows about still loads cleanly under
// LoadBindings, even though the SAME file fails ValidateStrict.
func TestLoadBindings_UnknownFieldIgnored(t *testing.T) {
	src := `
bindings: [{
	module: "platform-standards"
	future_field: "something a future #Binding might add"
}]
`
	path := writeBindings(t, src)
	if _, err := LoadBindings(path); err != nil {
		t.Fatalf("LoadBindings should ignore the unknown field, got: %v", err)
	}
	if err := ValidateStrict(path); err == nil {
		t.Fatal("ValidateStrict should reject the unknown field but did not")
	}
}

func TestLoadBindings_Negatives(t *testing.T) {
	cases := []struct {
		name      string
		cue       string
		wantField string
	}{
		{"empty bindings list", `bindings: []`, "bindings"},
		{"bad module name", `bindings: [{module: "Not_Valid"}]`, "module"},
		{"max_rules zero", `bindings: [{module: "platform-standards", max_rules: 0}]`, "max_rules"},
		{"bad min_tier", `bindings: [{module: "platform-standards", min_tier: "gold"}]`, "min_tier"},
		{"empty scopes list", `bindings: [{module: "platform-standards", scopes: []}]`, "scopes"},
		{"duplicate module", `bindings: [{module: "platform-standards"}, {module: "platform-standards"}]`, "module"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeBindings(t, tc.cue)
			_, err := LoadBindings(path)
			if err == nil {
				t.Fatal("expected an error, got none")
			}
			le, ok := err.(*LoadError)
			if !ok {
				t.Fatalf("expected *LoadError, got %T: %v", err, err)
			}
			found := false
			for _, f := range le.Fields {
				if strings.Contains(f.Field, tc.wantField) {
					found = true
				}
			}
			if !found {
				t.Errorf("LoadError.Fields = %v, want a field naming %q", le.Fields, tc.wantField)
			}
		})
	}
}

func TestLoadBindings_MissingFile(t *testing.T) {
	if _, err := LoadBindings(filepath.Join(t.TempDir(), "nope.cue")); err == nil {
		t.Fatal("expected an error for a missing file")
	}
}

func TestValidateStrict_ValidPasses(t *testing.T) {
	path := writeBindings(t, validBindingsCUE)
	if err := ValidateStrict(path); err != nil {
		t.Fatalf("ValidateStrict: %v", err)
	}
}

func TestValidateStrict_EmptyBindingsFails(t *testing.T) {
	path := writeBindings(t, `bindings: []`)
	if err := ValidateStrict(path); err == nil {
		t.Fatal("expected ValidateStrict to reject an empty bindings list")
	}
}

// TestExampleTemplate_IsCommentedNoOp proves the fathom-init scaffold is
// valid, empty CUE (nothing to bind) — the design's "no active config by
// default" deliverable enforced structurally, not just by convention.
func TestExampleTemplate_IsCommentedNoOp(t *testing.T) {
	path := writeBindings(t, ExampleTemplate())
	if _, err := LoadBindings(path); err == nil {
		t.Fatal("expected an error: a commented-only template has no `bindings` field, which is required")
	}
	// But it must still be syntactically valid CUE (cue vet must accept an
	// empty file) — LoadBindings failing on the REQUIRED bindings field is
	// the expected, honest failure mode, not a parse error.
	le, ok := func() (*LoadError, bool) {
		_, err := LoadBindings(path)
		e, ok := err.(*LoadError)
		return e, ok
	}()
	if !ok {
		t.Fatal("expected a *LoadError (missing required field), not a parse failure")
	}
	if len(le.Fields) != 1 || le.Fields[0].Field != "bindings" {
		t.Errorf("LoadError.Fields = %v, want exactly one \"bindings\" field error", le.Fields)
	}
}
