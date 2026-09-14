package scenariobench

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// goldStep is one assistant tool_use + user tool_result pair.
type goldStep struct {
	Name, Input, Result string
}

// writeGoldTranscript writes steps as a `claude -p --output-format
// stream-json` transcript would carry them, one tool_use/tool_result pair
// per step, in order.
func writeGoldTranscript(t *testing.T, dir, name string, steps []goldStep) string {
	t.Helper()
	var out []byte
	for i, s := range steps {
		id := fmt.Sprintf("tu%d", i)
		input := s.Input
		if input == "" {
			input = "{}"
		}
		assistant := map[string]any{
			"type": "assistant",
			"message": map[string]any{"content": []map[string]any{
				{"type": "tool_use", "id": id, "name": s.Name, "input": json.RawMessage(input)},
			}},
		}
		resultJSON, err := json.Marshal(s.Result)
		if err != nil {
			t.Fatal(err)
		}
		user := map[string]any{
			"type": "user",
			"message": map[string]any{"content": []map[string]any{
				{"type": "tool_result", "tool_use_id": id, "content": json.RawMessage(resultJSON)},
			}},
		}
		for _, ev := range []any{assistant, user} {
			line, err := json.Marshal(ev)
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, line...)
			out = append(out, '\n')
		}
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestGoldSourceTool_SecondResultHoldsToken is VB-4's required fixture: two
// tool results, and only the SECOND carries the gold token — the first
// (Grep, a miss) must not be credited.
func TestGoldSourceTool_SecondResultHoldsToken(t *testing.T) {
	dir := t.TempDir()
	path := writeGoldTranscript(t, dir, "session.jsonl", []goldStep{
		{Name: "Grep", Result: "no matches here"},
		{Name: "mcp__fathom__obligations", Result: "register the extractor via fusion.New(), golden-test it"},
	})
	sessions := []SessionResult{{Index: 0, RawPath: path}}
	task := Task{Oracle: Oracle{AnswerAllOf: []string{"fusion.New"}}}

	tool, evidence := GoldSourceTool(sessions, task)
	if tool != "mcp__fathom__obligations" {
		t.Errorf("gold source tool = %q, want the SECOND result's tool", tool)
	}
	if evidence != "fusion.New" {
		t.Errorf("evidence = %q, want the matched token", evidence)
	}
}

// TestGoldSourceTool_WholeTokenNotSubstring: a lookup token must not match
// merely as a substring of a longer word in the result body.
func TestGoldSourceTool_WholeTokenNotSubstring(t *testing.T) {
	dir := t.TempDir()
	path := writeGoldTranscript(t, dir, "session.jsonl", []goldStep{
		{Name: "Bash", Result: "membership renewal notice"},
	})
	sessions := []SessionResult{{RawPath: path}}
	task := Task{Oracle: Oracle{AnswerAllOf: []string{"New"}}}

	tool, evidence := GoldSourceTool(sessions, task)
	if tool != GoldSourceNone || evidence != "" {
		t.Errorf("got (%q,%q), want (%q,\"\") — %q must not match inside %q", tool, evidence, GoldSourceNone, "New", "renewal")
	}
}

// TestGoldSourceTool_ChangeTaskFileNamedByToolUse: a change task's gold
// source is the tool whose OWN call targeted a Gold.Files path.
func TestGoldSourceTool_ChangeTaskFileNamedByToolUse(t *testing.T) {
	dir := t.TempDir()
	path := writeGoldTranscript(t, dir, "session.jsonl", []goldStep{
		{Name: "Read", Input: `{"file_path":"/repo/internal/extractor/goextract.go"}`, Result: "package extractor"},
	})
	sessions := []SessionResult{{RawPath: path}}
	task := Task{Gold: Gold{Files: []string{"internal/extractor/goextract.go"}}}

	tool, evidence := GoldSourceTool(sessions, task)
	if tool != "Read" || evidence != "internal/extractor/goextract.go" {
		t.Errorf("got (%q,%q), want (Read, internal/extractor/goextract.go)", tool, evidence)
	}
}

// TestGoldSourceTool_ChangeTaskFileNamedByResultBody: a Grep-shaped result
// that mentions a gold file in its own body also counts, even when the
// call itself (a search) did not target that file directly.
func TestGoldSourceTool_ChangeTaskFileNamedByResultBody(t *testing.T) {
	dir := t.TempDir()
	path := writeGoldTranscript(t, dir, "session.jsonl", []goldStep{
		{Name: "Grep", Result: "internal/extractor/goextract.go:12:func New() {"},
	})
	sessions := []SessionResult{{RawPath: path}}
	task := Task{Gold: Gold{Files: []string{"internal/extractor/goextract.go"}}}

	tool, evidence := GoldSourceTool(sessions, task)
	if tool != "Grep" || evidence != "internal/extractor/goextract.go" {
		t.Errorf("got (%q,%q), want (Grep, internal/extractor/goextract.go)", tool, evidence)
	}
}

// TestGoldSourceTool_GuardsTaskRuleSourceInResult: a guards task's gold
// source is the first result whose body carries the rule's own source path.
func TestGoldSourceTool_GuardsTaskRuleSourceInResult(t *testing.T) {
	dir := t.TempDir()
	path := writeGoldTranscript(t, dir, "session.jsonl", []goldStep{
		{Name: "mcp__fathom__obligations", Result: "rule from internal/CLAUDE.md: extractors register in fusion.New"},
	})
	sessions := []SessionResult{{RawPath: path}}
	task := Task{Rule: &Rule{ID: "R1", Source: "internal/CLAUDE.md"}}

	tool, evidence := GoldSourceTool(sessions, task)
	if tool != "mcp__fathom__obligations" || evidence != "internal/CLAUDE.md" {
		t.Errorf("got (%q,%q), want (mcp__fathom__obligations, internal/CLAUDE.md)", tool, evidence)
	}
}

// TestGoldSourceTool_NoneWhenNoToolResultHoldsIt: the agent knew or
// guessed — no tool result ever carried the signal.
func TestGoldSourceTool_NoneWhenNoToolResultHoldsIt(t *testing.T) {
	dir := t.TempDir()
	path := writeGoldTranscript(t, dir, "session.jsonl", []goldStep{
		{Name: "Bash", Result: "nothing useful"},
	})
	sessions := []SessionResult{{RawPath: path}}
	task := Task{Oracle: Oracle{AnswerAllOf: []string{"fusion.New"}}}

	tool, evidence := GoldSourceTool(sessions, task)
	if tool != GoldSourceNone || evidence != "" {
		t.Errorf("got (%q,%q), want (%q,\"\")", tool, evidence, GoldSourceNone)
	}
}

// TestGoldSourceTool_NoGoldSignalOnTask: a task carrying none of the three
// gold signals (no answer_all_of, no gold files, no rule) always reads
// "none" — never a panic or a guessed shape.
func TestGoldSourceTool_NoGoldSignalOnTask(t *testing.T) {
	tool, evidence := GoldSourceTool(nil, Task{})
	if tool != GoldSourceNone || evidence != "" {
		t.Errorf("got (%q,%q), want (%q,\"\")", tool, evidence, GoldSourceNone)
	}
}
