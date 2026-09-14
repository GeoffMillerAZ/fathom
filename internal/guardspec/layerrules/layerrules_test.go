package layerrules

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/guardspec"
)

// tmpModule builds a two-package throwaway Go module ("a" imports "b")
// under t.TempDir() and returns its root.
func tmpModule(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write := func(rel, content string) {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module tmpmod\n\ngo 1.24\n")
	write("a/a.go", "package a\n\nimport \"tmpmod/b\"\n\nfunc F() { b.G() }\n")
	write("b/b.go", "package b\n\nfunc G() {}\n")
	return dir
}

func TestBuildIndex_SimpleImport(t *testing.T) {
	dir := tmpModule(t)
	idx, err := BuildIndex(dir)
	if err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}
	if idx.PackageEdges == nil {
		t.Fatal("PackageEdges is nil")
	}
	deps := idx.PackageEdges.Deps("a")
	if len(deps) != 1 || deps[0] != "b" {
		t.Errorf("Deps(a) = %v, want [b]", deps)
	}
	if deps := idx.PackageEdges.Deps("b"); len(deps) != 0 {
		t.Errorf("Deps(b) = %v, want none", deps)
	}
}

func TestBuildIndex_NoGoMod(t *testing.T) {
	if _, err := BuildIndex(t.TempDir()); err == nil {
		t.Fatal("expected an error for a directory with no go.mod")
	}
}

func writeRuleFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "guard.cue")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCheck_SeededViolation(t *testing.T) {
	dir := tmpModule(t)
	ruleFile := writeRuleFile(t, `
guards: []
layer_rules: [{id: "a-not-b", kind: "import_direction", sentence: "a must not import b", from: ["a"], to: ["b"]}]
`)
	violations, skipped, err := Check(dir, ruleFile)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(skipped) != 0 {
		t.Errorf("skipped = %v, want none", skipped)
	}
	if len(violations) != 1 {
		t.Fatalf("violations = %v, want 1", violations)
	}
	if violations[0].RuleID != "a-not-b" {
		t.Errorf("RuleID = %q", violations[0].RuleID)
	}
}

func TestCheck_SatisfiedRule(t *testing.T) {
	dir := tmpModule(t)
	ruleFile := writeRuleFile(t, `
guards: []
layer_rules: [{id: "b-not-a", kind: "import_direction", sentence: "b must not import a", from: ["b"], to: ["a"]}]
`)
	violations, _, err := Check(dir, ruleFile)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("violations = %v, want none", violations)
	}
}

func TestCheck_WarnSeverityNeverReported(t *testing.T) {
	dir := tmpModule(t)
	ruleFile := writeRuleFile(t, `
guards: []
layer_rules: [{id: "a-not-b-warn", kind: "import_direction", sentence: "s", severity: "warn", from: ["a"], to: ["b"]}]
`)
	violations, _, err := Check(dir, ruleFile)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("violations = %v, want none (warn severity never fails)", violations)
	}
}

func TestCheck_CrossServiceDirectionSkipped(t *testing.T) {
	dir := tmpModule(t)
	ruleFile := writeRuleFile(t, `
guards: []
layer_rules: [{id: "fleet-rule", kind: "cross_service_direction", sentence: "s", from_service: "x", to_service: "y"}]
`)
	violations, skipped, err := Check(dir, ruleFile)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(violations) != 0 {
		t.Errorf("violations = %v, want none", violations)
	}
	if len(skipped) != 1 || skipped[0] != "fleet-rule" {
		t.Errorf("skipped = %v, want [fleet-rule]", skipped)
	}
}

// sanity: guardspec.LayerRuleIndex zero value never panics EvaluateLayerRule.
func TestZeroIndex(t *testing.T) {
	var idx guardspec.LayerRuleIndex
	rule := guardspec.LayerRule{ID: "x", Kind: guardspec.ImportDirection, From: []string{"a"}, To: []string{"b"}}
	if got := guardspec.EvaluateLayerRule(rule, idx); len(got) != 0 {
		t.Errorf("got %v, want none", got)
	}
}
