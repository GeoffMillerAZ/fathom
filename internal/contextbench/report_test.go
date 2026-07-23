package contextbench

import (
	"strings"
	"testing"
)

func TestArmResultFromClaudeJSON(t *testing.T) {
	// A trimmed real "claude -p --output-format json" envelope (field names
	// verified against a live probe; see docs/plans/falsifiability.md B1-2).
	raw := []byte(`{
		"is_error": false,
		"num_turns": 3,
		"result": "cmd/fathom/servehttp.go",
		"duration_ms": 9094,
		"usage": {
			"input_tokens": 6,
			"cache_creation_input_tokens": 19314,
			"cache_read_input_tokens": 82547,
			"output_tokens": 186
		}
	}`)
	task := Task{ID: "B1-T02", GroundTruth: []string{"cmd/fathom/servehttp.go"}, ScoringRule: ExactMatch}

	ar, err := ArmResultFromClaudeJSON(task, "fathom", raw)
	if err != nil {
		t.Fatalf("ArmResultFromClaudeJSON: %v", err)
	}
	if ar.TaskID != "B1-T02" || ar.Arm != "fathom" {
		t.Errorf("identity fields wrong: %+v", ar)
	}
	if !ar.Success {
		t.Errorf("expected success, got failure: %+v", ar)
	}
	if ar.IsError {
		t.Errorf("expected is_error=false")
	}
	wantTotal := int64(6 + 19314 + 82547 + 186)
	if ar.TotalTokens != wantTotal {
		t.Errorf("TotalTokens = %d, want %d", ar.TotalTokens, wantTotal)
	}
	if ar.WallTimeMS != 9094 {
		t.Errorf("WallTimeMS = %d, want 9094", ar.WallTimeMS)
	}
	if ar.NumTurns != 3 {
		t.Errorf("NumTurns = %d, want 3", ar.NumTurns)
	}
}

func TestArmResultFromClaudeJSON_ClaudeReportedError(t *testing.T) {
	raw := []byte(`{"is_error": true, "result": "Not logged in", "usage": {}}`)
	task := Task{ID: "T1", GroundTruth: []string{"x"}, ScoringRule: ExactMatch}

	ar, err := ArmResultFromClaudeJSON(task, "grep", raw)
	if err != nil {
		t.Fatalf("ArmResultFromClaudeJSON: %v", err)
	}
	if !ar.IsError {
		t.Errorf("expected IsError=true")
	}
	if ar.Success {
		t.Errorf("a claude-reported error must never score as success")
	}
}

func TestArmResultFromClaudeJSON_MalformedInput(t *testing.T) {
	task := Task{ID: "T1", GroundTruth: []string{"x"}, ScoringRule: ExactMatch}
	if _, err := ArmResultFromClaudeJSON(task, "fathom", []byte("not json")); err == nil {
		t.Fatal("expected an error for malformed claude -p JSON")
	}
}

func TestSummarize(t *testing.T) {
	results := []ArmResult{
		{TaskID: "T1", Arm: "fathom", Success: true, TotalTokens: 100},
		{TaskID: "T2", Arm: "fathom", Success: false, TotalTokens: 200},
		{TaskID: "T1", Arm: "grep", Success: true, TotalTokens: 1000},
		{TaskID: "T2", Arm: "grep", Success: true, TotalTokens: 3000},
	}
	sum := Summarize(results)

	fathom := sum["fathom"]
	if fathom.TaskCount != 2 || fathom.SuccessCount != 1 {
		t.Errorf("fathom aggregate = %+v", fathom)
	}
	if fathom.TotalTokens != 300 {
		t.Errorf("fathom TotalTokens = %d, want 300", fathom.TotalTokens)
	}

	grep := sum["grep"]
	if grep.TaskCount != 2 || grep.SuccessCount != 2 {
		t.Errorf("grep aggregate = %+v", grep)
	}
	if grep.TotalTokens != 4000 {
		t.Errorf("grep TotalTokens = %d, want 4000", grep.TotalTokens)
	}
}

func TestRenderMarkdown_DeterministicAndContainsKeyFacts(t *testing.T) {
	results := []ArmResult{
		{TaskID: "B1-T01", Arm: "fathom", Success: true, TotalTokens: 100, WallTimeMS: 5000},
		{TaskID: "B1-T01", Arm: "grep", Success: true, TotalTokens: 900, WallTimeMS: 9000},
	}
	md1 := RenderMarkdown(results)
	md2 := RenderMarkdown(results)
	if md1 != md2 {
		t.Fatal("RenderMarkdown is not deterministic across identical input")
	}
	for _, want := range []string{"B1-T01", "fathom", "grep", "100", "900"} {
		if !strings.Contains(md1, want) {
			t.Errorf("summary markdown missing %q:\n%s", want, md1)
		}
	}
}
