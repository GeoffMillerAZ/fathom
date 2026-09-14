package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func repoRootForRulesGen(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(file)
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("could not find repo root (no go.mod within 6 levels)")
	return ""
}

func TestPackageNameFor(t *testing.T) {
	cases := []struct{ dir, want string }{
		{"rules", "rules"},
		{"./rules", "rules"},
		{"layer-rules", "layer_rules"},
		{"Rules", "rules"},
		{"123rules", "rules"}, // leading digit is not a valid Go identifier start
		{"", "rules"},
		{".", "rules"},
	}
	for _, tc := range cases {
		if got := packageNameFor(tc.dir); got != tc.want {
			t.Errorf("packageNameFor(%q) = %q, want %q", tc.dir, got, tc.want)
		}
	}
}

func writeFixtureGuardCue(t *testing.T, root, content string) {
	t.Helper()
	dir := filepath.Join(root, ".fathom")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "guard.cue"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCmdRulesGenTest_NoGuardCue(t *testing.T) {
	root := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := cmdRulesGenTest([]string{"--root", root, "--out", filepath.Join(root, "rules")}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "guard.cue") {
		t.Errorf("stderr = %q, want a mention of guard.cue", stderr.String())
	}
}

func TestCmdRulesGenTest_ZeroRules(t *testing.T) {
	root := t.TempDir()
	writeFixtureGuardCue(t, root, "guards: []\n")
	out := filepath.Join(root, "rules")
	var stdout, stderr bytes.Buffer
	code := cmdRulesGenTest([]string{"--root", root, "--out", out}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "nothing to generate") {
		t.Errorf("stdout = %q, want a mention of nothing to generate", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(out, "layer_rules_test.go")); err == nil {
		t.Error("layer_rules_test.go should not be written when there are zero layer_rules")
	}
}

func TestCmdRulesGenTest_WritesDeterministicFile(t *testing.T) {
	root := t.TempDir()
	writeFixtureGuardCue(t, root, `
guards: []
layer_rules: [
	{id: "b-rule", kind: "no_cycles", sentence: "s", scope: ["internal/**"]},
	{id: "a-rule", kind: "import_direction", sentence: "s", from: ["x"], to: ["y"]},
]
`)
	out := filepath.Join(root, "rules")

	var stdout1, stderr1 bytes.Buffer
	if code := cmdRulesGenTest([]string{"--root", root, "--out", out}, &stdout1, &stderr1); code != 0 {
		t.Fatalf("first run: exit %d, stderr=%s", code, stderr1.String())
	}
	if !strings.Contains(stdout1.String(), "a-rule") || !strings.Contains(stdout1.String(), "b-rule") {
		t.Errorf("stdout = %q, want both rule ids named", stdout1.String())
	}
	first, err := os.ReadFile(filepath.Join(out, "layer_rules_test.go"))
	if err != nil {
		t.Fatal(err)
	}

	var stdout2, stderr2 bytes.Buffer
	if code := cmdRulesGenTest([]string{"--root", root, "--out", out}, &stdout2, &stderr2); code != 0 {
		t.Fatalf("second run: exit %d, stderr=%s", code, stderr2.String())
	}
	second, err := os.ReadFile(filepath.Join(out, "layer_rules_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Errorf("regenerating from the same guard.cue produced different bytes:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
	if !strings.Contains(string(first), "package rules") {
		t.Errorf("generated file does not declare package rules:\n%s", first)
	}
	if !strings.Contains(string(first), `"github.com/geoffmilleraz/fathom/pkg/layerrules"`) {
		t.Errorf("generated file must import the PUBLIC pkg/layerrules facade (internal/... is unimportable from an adopting repo):\n%s", first)
	}
}

// TestCmdRulesGenTest_SeededViolation is VB-14's required proof: build the
// generated test against a temp module fixture that violates one rule, and
// run `go test` on it — proving the SAME rule text fails a real `go test`
// invocation, not just guardspec.EvaluateLayerRule in-process. The temp
// module is a genuinely SEPARATE module (its own go.mod, replaced onto
// this checkout) so this also proves the generated file is importable
// from outside fathom's own module tree (pkg/layerrules, never
// internal/...) — the "stranger" premise VB-15 documents.
//
// Skipped under -short: it shells out to `go test`, which resolves
// fathom's transitive dependencies through the replace directive.
func TestCmdRulesGenTest_SeededViolation(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to `go test` against a temp module; skipped under -short")
	}
	repoRoot := repoRootForRulesGen(t)
	mod := t.TempDir()
	write := func(rel, content string) {
		full := filepath.Join(mod, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module seededviolation\n\ngo 1.24\n\nrequire github.com/geoffmilleraz/fathom v0.0.0\n\nreplace github.com/geoffmilleraz/fathom => "+repoRoot+"\n")
	// "outer" calls into "inner" — the rule below forbids exactly this.
	write("inner/inner.go", "package inner\n\nfunc F() {}\n")
	write("outer/outer.go", "package outer\n\nimport \"seededviolation/inner\"\n\nfunc G() { inner.F() }\n")
	writeFixtureGuardCue(t, mod, `
guards: []
layer_rules: [
	{id: "outer-not-inner", kind: "import_direction", sentence: "outer must not import inner", from: ["outer"], to: ["inner"]},
]
`)

	out := filepath.Join(mod, "rules")
	var stdout, stderr bytes.Buffer
	if code := cmdRulesGenTest([]string{"--root", mod, "--out", out}, &stdout, &stderr); code != 0 {
		t.Fatalf("cmdRulesGenTest: exit %d, stderr=%s", code, stderr.String())
	}

	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = mod
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("generated test PASSED against a module that violates outer-not-inner (want a failure); output:\n%s", output)
	}
	if !strings.Contains(string(output), "outer-not-inner") {
		t.Errorf("go test output does not name the violated rule id 'outer-not-inner':\n%s", output)
	}
	if !strings.Contains(string(output), "outer must not import inner") {
		t.Errorf("go test output does not carry the rule's sentence:\n%s", output)
	}
}
