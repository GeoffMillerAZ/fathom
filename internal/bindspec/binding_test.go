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

// TestLoadBindings_OptionalFieldSyntaxRejected is critic finding 1's
// regression test (phase-O merge, BLOCKING): a trailing '?' on a field
// name in an INSTANCE (not the #Binding schema) silently declares that
// field ABSENT rather than setting it — a `scopes?: [...]` typo would
// otherwise widen the binding to every file instead of the author's
// intended restriction, the exact inverse of design §3.2's "intersects,
// never widens" doctrine. LoadBindings must refuse this loudly, naming
// the offending field.
func TestLoadBindings_OptionalFieldSyntaxRejected(t *testing.T) {
	cases := []struct {
		name      string
		cue       string
		wantField string
	}{
		{"scopes written optional", `bindings: [{module: "platform-standards", scopes?: ["infra/**"]}]`, "scopes"},
		{"max_rules written optional", `bindings: [{module: "platform-standards", max_rules?: 3}]`, "max_rules"},
		{"min_tier written optional", `bindings: [{module: "platform-standards", min_tier?: "doc-claimed"}]`, "min_tier"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeBindings(t, tc.cue)
			_, err := LoadBindings(path)
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
// directly: uncommenting ExampleTemplate()'s worked example (stripping the
// leading "// " a human would delete) produces a binding LoadBindings
// accepts with the concrete values actually set — not one that silently
// loads with scopes/max_rules/min_tier all absent.
func TestExampleTemplate_UncommentedRoundTrips(t *testing.T) {
	uncommented := uncommentCUEExample(t, ExampleTemplate())
	path := writeBindings(t, uncommented)
	set, err := LoadBindings(path)
	if err != nil {
		t.Fatalf("LoadBindings(uncommented scaffold): %v\n---\n%s", err, uncommented)
	}
	if len(set.Bindings) != 1 {
		t.Fatalf("Bindings = %v", set.Bindings)
	}
	b := set.Bindings[0]
	if b.Module != "platform-standards" {
		t.Errorf("Module = %q", b.Module)
	}
	if len(b.Scopes) != 2 {
		t.Errorf("Scopes = %v, want the scaffold's two globs actually set (not absent)", b.Scopes)
	}
	if b.MaxRules != 2 {
		t.Errorf("MaxRules = %d, want 2 actually set", b.MaxRules)
	}
	if b.MinTier != "doc-claimed" {
		t.Errorf("MinTier = %q, want \"doc-claimed\" actually set", b.MinTier)
	}
}

// uncommentCUEExample extracts the "// bindings: [{ ... }]" block from a
// commented scaffold and strips each line's leading "// " (or "//" on a
// blank comment line) — mirroring exactly what a human does by hand.
func uncommentCUEExample(t *testing.T, scaffold string) string {
	t.Helper()
	var out strings.Builder
	inBlock := false
	for _, line := range strings.Split(scaffold, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "// bindings: [{") {
			inBlock = true
		}
		if !inBlock {
			continue
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
		t.Fatal("uncommentCUEExample found no \"bindings: [{\" block in the scaffold")
	}
	return out.String()
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

// TestExplicitEmptyBindings pins IO-7's one-vocabulary rule: `bindings: []`
// is the sanctioned inert literal (the activate skill and the kit packs
// write it), so BOTH the loader and the strict authoring gate accept it —
// rejecting it made doctor FAIL a file fathom's own tooling prescribes.
func TestExplicitEmptyBindings(t *testing.T) {
	path := writeBindings(t, `bindings: []`)
	set, err := LoadBindings(path)
	if err != nil {
		t.Fatalf("LoadBindings(bindings: []) = %v, want valid inert config", err)
	}
	if len(set.Bindings) != 0 {
		t.Errorf("want zero bindings, got %d", len(set.Bindings))
	}
	if err := ValidateStrict(path); err != nil {
		t.Fatalf("ValidateStrict(bindings: []) = %v, want accept", err)
	}
}

// TestExampleTemplate_IsCommentedNoOp pins O22 (docs/dogfood-observations.md):
// the ACTUAL bytes ExampleTemplate() renders — the exact scaffold `fathom
// init` writes to .fathom/modules.cue — must load CLEAN as an empty,
// scaffolded config, not fail with "bindings: required, MUST be present".
// Before the O22 fix, every fresh `fathom init` failed `doctor`'s
// binding_file check on a file fathom itself had just written; the
// fully-commented scaffold has no top-level "bindings" field at all
// (LookupPath sees it as absent, exactly like an entirely blank file),
// which is EMPTY, not invalid — only a field that IS present can be
// malformed. The design's "no active config by default" deliverable is
// still enforced structurally: LoadBindings reports zero bindings.
func TestExampleTemplate_IsCommentedNoOp(t *testing.T) {
	path := writeBindings(t, ExampleTemplate())
	set, err := LoadBindings(path)
	if err != nil {
		t.Fatalf("LoadBindings(ExampleTemplate()) = %v, want a clean load (O22: an all-commented scaffold is EMPTY, not invalid)", err)
	}
	if len(set.Bindings) != 0 {
		t.Errorf("Bindings = %v, want none — the scaffold is fully commented", set.Bindings)
	}
}

// TestLoadBindings_TopLevelFieldAbsent_IsEmptyNotError is the narrower unit
// test behind TestExampleTemplate_IsCommentedNoOp: a file with no
// "bindings" field at all loads as an empty BindingSet, never an error.
// This is the ONLY way LoadBindings returns a zero-binding result without
// an error: an explicit `bindings: []` (present but declaring nothing via
// an empty list) is unchanged and still rejected (TestLoadBindings_Negatives'
// "empty bindings list" case), so len(set.Bindings)==0 + err==nil
// unambiguously means "field omitted" — no separate Scaffolded/Empty flag
// on BindingSet is needed.
func TestLoadBindings_TopLevelFieldAbsent_IsEmptyNotError(t *testing.T) {
	for _, content := range []string{
		"",
		"// nothing but a comment\n",
	} {
		path := writeBindings(t, content)
		set, err := LoadBindings(path)
		if err != nil {
			t.Fatalf("LoadBindings(%q) = %v, want a clean empty load", content, err)
		}
		if len(set.Bindings) != 0 {
			t.Errorf("LoadBindings(%q) Bindings = %v, want none", content, set.Bindings)
		}
	}
}
