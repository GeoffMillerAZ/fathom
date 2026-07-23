package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/contextbench"
)

const testTasksJSON = `{
  "version": 1,
  "tasks": [
    {"id": "T1", "prompt": "p1", "ground_truth": ["cmd/fathom/servehttp.go"], "scoring_rule": "exact-match"},
    {"id": "T2", "prompt": "p2", "ground_truth": ["a", "b"], "scoring_rule": "all-of"}
  ]
}`

func writeTasksFixture(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "tasks.json")
	if err := os.WriteFile(path, []byte(testTasksJSON), 0o644); err != nil {
		t.Fatalf("writing tasks fixture: %v", err)
	}
	return path
}

func TestCmdScore_PassAndFail(t *testing.T) {
	dir := t.TempDir()
	tasksPath := writeTasksFixture(t, dir)

	claudeJSON := filepath.Join(dir, "claude-out.json")
	if err := os.WriteFile(claudeJSON, []byte(`{"is_error":false,"num_turns":2,"result":"cmd/fathom/servehttp.go","duration_ms":1234,"usage":{"input_tokens":1,"output_tokens":2}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(dir, "T1-fathom.json")

	rc := run([]string{"score", "-tasks", tasksPath, "-task-id", "T1", "-arm", "fathom", "-claude-json", claudeJSON, "-out", outPath})
	if rc != 0 {
		t.Fatalf("score exited %d", rc)
	}

	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}
	var ar contextbench.ArmResult
	if err := json.Unmarshal(raw, &ar); err != nil {
		t.Fatalf("parsing output: %v", err)
	}
	if !ar.Success {
		t.Errorf("expected success=true, got %+v", ar)
	}
	if ar.TaskID != "T1" || ar.Arm != "fathom" {
		t.Errorf("identity fields wrong: %+v", ar)
	}
}

func TestCmdScore_UnknownTaskID(t *testing.T) {
	dir := t.TempDir()
	tasksPath := writeTasksFixture(t, dir)
	claudeJSON := filepath.Join(dir, "claude-out.json")
	os.WriteFile(claudeJSON, []byte(`{"result":"x","usage":{}}`), 0o644)

	rc := run([]string{"score", "-tasks", tasksPath, "-task-id", "does-not-exist", "-arm", "fathom", "-claude-json", claudeJSON, "-out", filepath.Join(dir, "out.json")})
	if rc == 0 {
		t.Fatal("expected non-zero exit for an unknown task id")
	}
}

func TestCmdScore_MissingFlags(t *testing.T) {
	rc := run([]string{"score"})
	if rc != 2 {
		t.Fatalf("expected exit 2 for missing flags, got %d", rc)
	}
}

func TestCmdReport_AggregatesAndRenders(t *testing.T) {
	dir := t.TempDir()
	tasksPath := writeTasksFixture(t, dir)
	resultsDir := filepath.Join(dir, "results")
	if err := os.Mkdir(resultsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	ar1 := contextbench.ArmResult{TaskID: "T1", Arm: "fathom", Success: true, TotalTokens: 100}
	ar2 := contextbench.ArmResult{TaskID: "T1", Arm: "grep", Success: true, TotalTokens: 900}
	for name, ar := range map[string]contextbench.ArmResult{"t1-fathom.json": ar1, "t1-grep.json": ar2} {
		b, _ := json.Marshal(ar)
		if err := os.WriteFile(filepath.Join(resultsDir, name), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	outJSON := filepath.Join(dir, "results.json")
	outMD := filepath.Join(dir, "summary.md")
	rc := run([]string{"report", "-tasks", tasksPath, "-results", resultsDir, "-out-json", outJSON, "-out-md", outMD})
	if rc != 0 {
		t.Fatalf("report exited %d", rc)
	}

	rawJSON, err := os.ReadFile(outJSON)
	if err != nil {
		t.Fatalf("reading results json: %v", err)
	}
	var rr contextbench.RunResults
	if err := json.Unmarshal(rawJSON, &rr); err != nil {
		t.Fatalf("parsing results json: %v", err)
	}
	if len(rr.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(rr.Results))
	}
	if rr.Summary["fathom"].TotalTokens != 100 || rr.Summary["grep"].TotalTokens != 900 {
		t.Errorf("summary wrong: %+v", rr.Summary)
	}

	mdBytes, err := os.ReadFile(outMD)
	if err != nil {
		t.Fatalf("reading summary md: %v", err)
	}
	if len(mdBytes) == 0 {
		t.Error("summary markdown is empty")
	}
}

func TestCmdReport_EmptyResultsDirFails(t *testing.T) {
	dir := t.TempDir()
	resultsDir := filepath.Join(dir, "results")
	os.Mkdir(resultsDir, 0o755)

	rc := run([]string{"report", "-results", resultsDir, "-out-json", filepath.Join(dir, "r.json"), "-out-md", filepath.Join(dir, "r.md")})
	if rc == 0 {
		t.Fatal("expected non-zero exit for an empty results dir")
	}
}

func TestRun_UnknownSubcommand(t *testing.T) {
	if rc := run([]string{"bogus"}); rc != 2 {
		t.Fatalf("expected exit 2, got %d", rc)
	}
}

func TestRun_NoArgs(t *testing.T) {
	if rc := run(nil); rc != 2 {
		t.Fatalf("expected exit 2, got %d", rc)
	}
}
