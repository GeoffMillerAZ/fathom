package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/scenariobench"
)

// writeParityCorpus writes a one-task corpus whose arms are the given raw
// JSON objects by name (control is the criterion's control). Parity never
// touches the subject, so its repo and commit are placeholders.
func writeParityCorpus(t *testing.T, dir string, arms map[string]string) string {
	t.Helper()
	names := make([]string, 0, len(arms))
	for name := range arms {
		names = append(names, name)
	}
	sort.Strings(names)
	var armsJSON, predJSON []string
	for _, name := range names {
		armsJSON = append(armsJSON, `"`+name+`": `+arms[name])
		predJSON = append(predJSON, `"`+name+`": "fixture"`)
	}
	corpus := `{
  "arms": {` + strings.Join(armsJSON, ",") + `},
  "tasks": [{
    "id": "T1",
    "category": "bug_fix",
    "prompt": "fix it",
    "derivation": "synthetic fixture for scenariobench's own tests",
    "subject": {"repo_url": "https://example.invalid/subject.git", "base_commit": "deadbeef"},
    "sessions": {"mode": "single", "count": 1, "max_turns": 5},
    "oracle": {"fail_to_pass": ["TestPass"]},
    "predictions": {` + strings.Join(predJSON, ",") + `}
  }],
  "criterion": {"id": "C1", "treatment": "full", "control": "control", "min_trials": 1}
}`
	path := filepath.Join(dir, "corpus.json")
	if err := os.WriteFile(path, []byte(corpus), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

const (
	plainArm    = `{"wired": false, "instructions": "plain"}`
	toolsArm    = `{"wired": false, "instructions": "plain", "allowed_tools": ["Read"]}`
	declaredArm = `{"wired": false, "instructions": "plain", "allowed_tools": ["Read"], "declares": ["tools"]}`
	wiredArm    = `{"wired": true, "instructions": "stanza", "surface": "fathom"}`
)

func runParity(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	stderr = captureStderr(t, func() {
		stdout = captureStdout(t, func() {
			code = run(append([]string{"parity"}, args...))
		})
	})
	return code, stdout, stderr
}

func TestCmdParityAllPass(t *testing.T) {
	path := writeParityCorpus(t, t.TempDir(), map[string]string{"control": plainArm, "full": plainArm})
	code, out, errOut := runParity(t, "-corpus", path)
	if code != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, out, errOut)
	}
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("stdout = %q, want a header and one line per arm", out)
	}
	if !regexp.MustCompile(`^parity: baseline control, 1 tasks, tree sha256 [0-9a-f]{64}$`).MatchString(lines[0]) {
		t.Fatalf("header = %q", lines[0])
	}
	if lines[1] != "control: PASS" || lines[2] != "full: PASS" {
		t.Fatalf("arm lines = %q", lines[1:])
	}
	if errOut != "" {
		t.Fatalf("stderr = %q, want empty", errOut)
	}
}

func TestCmdParityUndeclaredFails(t *testing.T) {
	path := writeParityCorpus(t, t.TempDir(), map[string]string{"control": plainArm, "full": toolsArm})
	code, out, _ := runParity(t, "-corpus", path)
	if code != 1 {
		t.Fatalf("exit=%d, want 1; stdout=%q", code, out)
	}
	if !strings.Contains(out, "\nfull: FAIL tools\n") {
		t.Fatalf("stdout = %q, want a `full: FAIL tools` line", out)
	}
}

func TestCmdParityDeclaredPasses(t *testing.T) {
	path := writeParityCorpus(t, t.TempDir(), map[string]string{"control": plainArm, "full": declaredArm})
	code, out, _ := runParity(t, "-corpus", path)
	if code != 0 || !strings.Contains(out, "\nfull: PASS\n") {
		t.Fatalf("exit=%d stdout=%q", code, out)
	}
}

func TestCmdParityFailListsDeclared(t *testing.T) {
	arm := `{"wired": false, "instructions": "plain", "allowed_tools": ["Read"], "notes_file": true, "env": {"X": "1"}, "declares": ["settings"]}`
	path := writeParityCorpus(t, t.TempDir(), map[string]string{"control": plainArm, "full": arm})
	code, out, _ := runParity(t, "-corpus", path)
	if code != 1 || !strings.Contains(out, "\nfull: FAIL notes_file, tools (declared: settings)\n") {
		t.Fatalf("exit=%d stdout=%q", code, out)
	}
}

func TestCmdParityWiredArmIsIncomplete(t *testing.T) {
	path := writeParityCorpus(t, t.TempDir(), map[string]string{"control": plainArm, "full": wiredArm})
	code, out, _ := runParity(t, "-corpus", path)
	if code != 2 {
		t.Fatalf("exit=%d, want 2 (INCOMPLETE and no FAIL); stdout=%q", code, out)
	}
	if !strings.Contains(out, "\nfull: INCOMPLETE notes_file, prompt, settings, tools: ") || !strings.Contains(out, "settings.json") {
		t.Fatalf("stdout = %q, want the INCOMPLETE line with unrendered fields and the render error", out)
	}
	if !strings.Contains(out, "\ncontrol: PASS\n") {
		t.Fatalf("stdout = %q, want the baseline row PASS", out)
	}
}

func TestCmdParityFailBeatsIncomplete(t *testing.T) {
	arms := map[string]string{
		"control": plainArm,
		"full":    wiredArm,
		"zlast":   toolsArm,
	}
	path := writeParityCorpus(t, t.TempDir(), arms)
	code, out, _ := runParity(t, "-corpus", path)
	if code != 1 {
		t.Fatalf("exit=%d, want 1; stdout=%q", code, out)
	}
	if !strings.Contains(out, "\nzlast: FAIL tools\n") || !strings.Contains(out, "\nfull: INCOMPLETE ") {
		t.Fatalf("stdout = %q", out)
	}
}

func TestCmdParityUsageAndLoadErrors(t *testing.T) {
	dir := t.TempDir()
	path := writeParityCorpus(t, dir, map[string]string{"control": plainArm, "full": plainArm})
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing corpus", args: nil, want: "scenariobench parity: -corpus is required"},
		{name: "unknown baseline", args: []string{"-corpus", path, "-baseline", "nope"}, want: "control, full"},
		{name: "unknown task", args: []string{"-corpus", path, "-task", "T9"}, want: `"T9"`},
		{name: "absent corpus", args: []string{"-corpus", filepath.Join(dir, "absent.json")}, want: "scenariobench parity: "},
		{name: "absent tree", args: []string{"-corpus", path, "-tree", filepath.Join(dir, "absent")}, want: "scenariobench parity: "},
		{name: "stray argument", args: []string{"-corpus", path, "extra"}, want: "scenariobench parity: "},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code, out, errOut := runParity(t, tc.args...)
			if code != 2 {
				t.Fatalf("exit=%d, want 2; stdout=%q stderr=%q", code, out, errOut)
			}
			if out != "" {
				t.Fatalf("stdout = %q, want nothing on a refusal", out)
			}
			if !strings.Contains(errOut, tc.want) || !strings.Contains(errOut, "scenariobench parity: ") {
				t.Fatalf("stderr = %q, want %q", errOut, tc.want)
			}
		})
	}
	if code, _, _ := runParity(t, "-nonesuch"); code != 2 {
		t.Fatalf("unknown flag exit=%d, want 2", code)
	}
}

func TestCmdParityJSON(t *testing.T) {
	path := writeParityCorpus(t, t.TempDir(), map[string]string{"control": plainArm, "full": toolsArm})
	code, out, _ := runParity(t, "-corpus", path, "-json")
	if code != 1 {
		t.Fatalf("exit=%d, want 1; stdout=%q", code, out)
	}
	var rep scenariobench.ParityReport
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("stdout is not a ParityReport: %v\n%s", err, out)
	}
	if rep.Baseline != "control" || len(rep.Tasks) != 1 || len(rep.Arms) != 2 || rep.TreeSHA256 == "" {
		t.Fatalf("report = %+v", rep)
	}
	if rep.Arms[1].Arm != "full" || rep.Arms[1].Verdict != scenariobench.ParityFail || len(rep.Arms[1].Undeclared) != 1 || rep.Arms[1].Undeclared[0] != "tools" {
		t.Fatalf("full row = %+v", rep.Arms[1])
	}
	if !strings.Contains(out, "\n  ") {
		t.Fatalf("-json output is not indented: %q", out)
	}
}

func TestCmdParitySkippedTreeEntriesGoToStderrInOneLine(t *testing.T) {
	tree := t.TempDir()
	if err := os.WriteFile(filepath.Join(tree, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(tree, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("a.txt", filepath.Join(tree, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	path := writeParityCorpus(t, t.TempDir(), map[string]string{"control": plainArm, "full": plainArm})
	code, out, errOut := runParity(t, "-corpus", path, "-tree", tree)
	if code != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, out, errOut)
	}
	if strings.Contains(out, "skipped") {
		t.Fatalf("stdout carries the skipped entries: %q", out)
	}
	line := strings.TrimSuffix(errOut, "\n")
	if strings.Contains(line, "\n") || !strings.Contains(line, "2") || !strings.Contains(line, ".git") || !strings.Contains(line, "link") {
		t.Fatalf("stderr = %q, want ONE line naming the count and each skipped entry", errOut)
	}
}

func TestCmdParityHelpAndUsage(t *testing.T) {
	help := captureStderr(t, func() { run([]string{"parity", "-help"}) })
	for _, want := range []string{"-corpus", "-baseline", "-task", "-tree", "-json"} {
		if !strings.Contains(help, want) {
			t.Fatalf("help missing %q: %s", want, help)
		}
	}
	usage := captureStderr(t, func() { run(nil) })
	if !strings.Contains(usage, "parity") {
		t.Fatalf("parity not discoverable in usage: %s", usage)
	}
}
