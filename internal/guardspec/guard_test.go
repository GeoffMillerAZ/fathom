package guardspec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGuardSchemaMatchesSource(t *testing.T) {
	root := repoRoot(t)
	source, err := os.ReadFile(filepath.Join(root, "schema", "guard", "schema.cue"))
	if err != nil {
		t.Fatalf("read schema/guard/schema.cue: %v", err)
	}
	if string(source) != string(guardSchema) {
		t.Fatalf("internal/guardspec/guard_schema.cue has drifted from schema/guard/schema.cue — copy the source file verbatim over the embedded mirror")
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

func writeGuards(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "guard.cue")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

const validGuardsCUE = `
guards: [{
	scopes:          ["internal/store/**", "**/migrations/**"]
	note:            "Store schema is spec-governed; read docs/spec first."
	max_rules:       4
	include_modules: true
}]
`

func TestLoadGuards_Valid(t *testing.T) {
	path := writeGuards(t, validGuardsCUE)
	set, err := LoadGuards(path)
	if err != nil {
		t.Fatalf("LoadGuards: %v", err)
	}
	if len(set.Guards) != 1 {
		t.Fatalf("Guards = %v", set.Guards)
	}
	g := set.Guards[0]
	if len(g.Scopes) != 2 {
		t.Errorf("Scopes = %v", g.Scopes)
	}
	if g.Note == "" {
		t.Error("Note should be set")
	}
	if g.MaxRules != 4 {
		t.Errorf("MaxRules = %d", g.MaxRules)
	}
	if !g.IncludeModules {
		t.Error("IncludeModules should be true")
	}
}

func TestLoadGuards_DefaultsAppliedWhenFieldAbsent(t *testing.T) {
	path := writeGuards(t, `guards: [{scopes: ["**/*.go"]}]`)
	set, err := LoadGuards(path)
	if err != nil {
		t.Fatalf("LoadGuards: %v", err)
	}
	g := set.Guards[0]
	if g.MaxRules != DefaultMaxRules {
		t.Errorf("MaxRules default = %d, want %d", g.MaxRules, DefaultMaxRules)
	}
	if g.IncludeModules {
		t.Error("IncludeModules default should be false")
	}
}

func TestLoadGuards_CommandScopes(t *testing.T) {
	path := writeGuards(t, `
guards: [{
	command_scopes: ["terraform apply*", "kubectl *", "*/migrate *"]
	note: "Infra commands are guarded; check the runbook first."
}]
`)
	set, err := LoadGuards(path)
	if err != nil {
		t.Fatalf("LoadGuards: %v", err)
	}
	if len(set.Guards[0].CommandScopes) != 3 {
		t.Errorf("CommandScopes = %v", set.Guards[0].CommandScopes)
	}
}

func TestLoadGuards_UnknownFieldIgnored(t *testing.T) {
	src := `
guards: [{
	scopes: ["**/*.go"]
	future_field: "something a future #Guard might add"
}]
`
	path := writeGuards(t, src)
	if _, err := LoadGuards(path); err != nil {
		t.Fatalf("LoadGuards should ignore the unknown field, got: %v", err)
	}
	if err := ValidateStrict(path); err == nil {
		t.Fatal("ValidateStrict should reject the unknown field but did not")
	}
}

func TestLoadGuards_Negatives(t *testing.T) {
	cases := []struct {
		name      string
		cue       string
		wantField string
	}{
		{"empty guards list", `guards: []`, "guards"},
		{"max_rules zero", `guards: [{scopes: ["**/*.go"], max_rules: 0}]`, "max_rules"},
		{"empty scopes list", `guards: [{scopes: []}]`, "scopes"},
		{"empty command_scopes list", `guards: [{command_scopes: []}]`, "command_scopes"},
		{"neither scopes nor command_scopes", `guards: [{note: "no scope at all"}]`, "guards[0]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeGuards(t, tc.cue)
			_, err := LoadGuards(path)
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

// TestLoadGuards_OptionalFieldSyntaxRejected is critic finding 1's
// regression test (phase-O merge, BLOCKING) — see the identical bindspec
// test's doc comment for the full reasoning; mirrored here for guard.cue's
// note/max_rules/include_modules fields.
func TestLoadGuards_OptionalFieldSyntaxRejected(t *testing.T) {
	cases := []struct {
		name      string
		cue       string
		wantField string
	}{
		{"note written optional", `guards: [{scopes: ["**/*.go"], note?: "spec-governed"}]`, "note"},
		{"max_rules written optional", `guards: [{scopes: ["**/*.go"], max_rules?: 4}]`, "max_rules"},
		{"include_modules written optional", `guards: [{scopes: ["**/*.go"], include_modules?: true}]`, "include_modules"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeGuards(t, tc.cue)
			_, err := LoadGuards(path)
			if err == nil {
				t.Fatal("expected an error for a field written with a trailing '?', got none")
			}
			le, ok := err.(*LoadError)
			if !ok {
				t.Fatalf("expected *LoadError, got %T: %v", err, err)
			}
			found := false
			for _, f := range le.Fields {
				if strings.Contains(f.Field, tc.wantField) && strings.Contains(f.Err.Error(), "'?'") {
					found = true
				}
			}
			if !found {
				t.Errorf("LoadError.Fields = %v, want a field naming %q and mentioning '?'", le.Fields, tc.wantField)
			}
		})
	}
}

// TestExampleTemplate_UncommentedRoundTrips proves the scaffold's fix
// directly, mirroring bindspec's identical test.
func TestExampleTemplate_UncommentedRoundTrips(t *testing.T) {
	uncommented := uncommentGuardCUEExample(t, ExampleTemplate())
	path := writeGuards(t, uncommented)
	set, err := LoadGuards(path)
	if err != nil {
		t.Fatalf("LoadGuards(uncommented scaffold): %v\n---\n%s", err, uncommented)
	}
	if len(set.Guards) != 1 {
		t.Fatalf("Guards = %v", set.Guards)
	}
	g := set.Guards[0]
	if len(g.Scopes) != 2 {
		t.Errorf("Scopes = %v, want the scaffold's two globs actually set", g.Scopes)
	}
	if g.Note == "" {
		t.Error("Note = \"\", want the scaffold's note actually set (not silently dropped)")
	}
	if g.MaxRules != 4 {
		t.Errorf("MaxRules = %d, want 4 actually set", g.MaxRules)
	}
	if !g.IncludeModules {
		t.Error("IncludeModules = false, want true actually set")
	}
}

// uncommentGuardCUEExample extracts the FIRST "// guards: [{ ... }]" block
// from a commented scaffold and strips each line's leading "// " — mirrors
// what a human does by hand uncommenting one example.
func uncommentGuardCUEExample(t *testing.T, scaffold string) string {
	t.Helper()
	var out strings.Builder
	inBlock := false
	for _, line := range strings.Split(scaffold, "\n") {
		trimmed := strings.TrimSpace(line)
		if !inBlock {
			if strings.HasPrefix(trimmed, "// guards: [{") {
				inBlock = true
			} else {
				continue
			}
		}
		content := strings.TrimPrefix(trimmed, "//")
		content = strings.TrimPrefix(content, " ")
		out.WriteString(content)
		out.WriteString("\n")
		if trimmed == "// }]" {
			break
		}
	}
	if out.Len() == 0 {
		t.Fatal("uncommentGuardCUEExample found no \"guards: [{\" block in the scaffold")
	}
	return out.String()
}

func TestLoadGuards_MissingFile(t *testing.T) {
	if _, err := LoadGuards(filepath.Join(t.TempDir(), "nope.cue")); err == nil {
		t.Fatal("expected an error for a missing file")
	}
}

func TestValidateStrict_ValidPasses(t *testing.T) {
	path := writeGuards(t, validGuardsCUE)
	if err := ValidateStrict(path); err != nil {
		t.Fatalf("ValidateStrict: %v", err)
	}
}

func TestValidateStrict_EmptyGuardsFails(t *testing.T) {
	path := writeGuards(t, `guards: []`)
	if err := ValidateStrict(path); err == nil {
		t.Fatal("expected ValidateStrict to reject an empty guards list")
	}
}

func TestExampleTemplate_IsCommentedNoOp(t *testing.T) {
	path := writeGuards(t, ExampleTemplate())
	_, err := LoadGuards(path)
	le, ok := err.(*LoadError)
	if !ok {
		t.Fatalf("expected a *LoadError (missing required field), not a parse failure; got %T: %v", err, err)
	}
	if len(le.Fields) != 1 || le.Fields[0].Field != "guards" {
		t.Errorf("LoadError.Fields = %v, want exactly one \"guards\" field error", le.Fields)
	}
}
