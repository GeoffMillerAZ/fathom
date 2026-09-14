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
	sessions := []SessionResult{{Index: 0, RawPath: path, FinalAnswer: "register via fusion.New(), golden-test it"}}
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
	sessions := []SessionResult{{RawPath: path, FinalAnswer: "New"}}
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
	sessions := []SessionResult{{RawPath: path, FinalAnswer: "fusion.New, probably"}}
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

// TestGoldSourceTool_ComposesCitationFromGrepLine is O53 defect 6a: a
// gold citation token ("path#Lnn") must be satisfied by a plain
// grep-style "path:nn:" result line, which never spells the "#Lnn" form
// out literally.
func TestGoldSourceTool_ComposesCitationFromGrepLine(t *testing.T) {
	dir := t.TempDir()
	path := writeGoldTranscript(t, dir, "session.jsonl", []goldStep{
		{Name: "Grep", Result: "internal/CLAUDE.md:48:Add new hot-loop tools to the companion allowlist"},
	})
	sessions := []SessionResult{{RawPath: path, FinalAnswer: "Per internal/CLAUDE.md#L48."}}
	task := Task{Oracle: Oracle{AnswerAllOf: []string{"internal/CLAUDE.md#L48"}}}

	tool, evidence := GoldSourceTool(sessions, task)
	if tool != "Grep" {
		t.Errorf("gold source tool = %q, want Grep", tool)
	}
	if evidence != "internal/CLAUDE.md#L48" {
		t.Errorf("evidence = %q, want internal/CLAUDE.md#L48", evidence)
	}
}

// TestGoldSourceTool_ComposesCitationFromReadNumberedLines is O53 defect
// 6a/6b's own proof: a Read result whose output is Claude Code's own
// numbered-line form (no file path repeated in the output — it was
// already the tool_use's own input) must compose against that input path
// and be credited — the FIRST result to actually carry line 48 — rather
// than a LATER, unrelated result that happens to spell the citation out
// as plain text (the old defect: labeling Bash where Read produced the
// content).
func TestGoldSourceTool_ComposesCitationFromReadNumberedLines(t *testing.T) {
	dir := t.TempDir()
	path := writeGoldTranscript(t, dir, "session.jsonl", []goldStep{
		{Name: "Read", Input: `{"file_path":"/repo/internal/CLAUDE.md"}`, Result: "    47\tsome context\n    48\tAdd new hot-loop tools to the companion allowlist\n"},
		{Name: "Bash", Result: "note: the citation is internal/CLAUDE.md#L48 in the docs"},
	})
	sessions := []SessionResult{{RawPath: path, FinalAnswer: "Per internal/CLAUDE.md#L48."}}
	task := Task{Oracle: Oracle{AnswerAllOf: []string{"internal/CLAUDE.md#L48"}}}

	tool, evidence := GoldSourceTool(sessions, task)
	if tool != "Read" {
		t.Errorf("gold source tool = %q, want Read — the FIRST result to carry line 48, via a composed citation, not the LATER Bash result that merely spells it out", tool)
	}
	if evidence != "internal/CLAUDE.md#L48" {
		t.Errorf("evidence = %q, want internal/CLAUDE.md#L48", evidence)
	}
}

// TestGoldSourceTool_CitationNeverComposedIsNone: neither grep nor
// numbered-line shapes appear anywhere, so a citation-shaped gold token
// must not match by accident.
func TestGoldSourceTool_CitationNeverComposedIsNone(t *testing.T) {
	dir := t.TempDir()
	path := writeGoldTranscript(t, dir, "session.jsonl", []goldStep{
		{Name: "Bash", Result: "internal/CLAUDE.md has the rule but no line number here"},
	})
	sessions := []SessionResult{{RawPath: path, FinalAnswer: "internal/CLAUDE.md#L48"}}
	task := Task{Oracle: Oracle{AnswerAllOf: []string{"internal/CLAUDE.md#L48"}}}

	tool, evidence := GoldSourceTool(sessions, task)
	if tool != GoldSourceNone || evidence != "" {
		t.Errorf("got (%q,%q), want (%q,\"\")", tool, evidence, GoldSourceNone)
	}
}

// TestGoldSourceTool_EmptyFinalAnswerIsNone is O53 defect 6c: a lookup
// trial that never produced a final answer must not credit any tool,
// however plainly a tool result carried the gold token along the way.
func TestGoldSourceTool_EmptyFinalAnswerIsNone(t *testing.T) {
	dir := t.TempDir()
	path := writeGoldTranscript(t, dir, "session.jsonl", []goldStep{
		{Name: "Grep", Result: "register the extractor via fusion.New(), golden-test it"},
	})
	sessions := []SessionResult{{RawPath: path, FinalAnswer: ""}}
	task := Task{Oracle: Oracle{AnswerAllOf: []string{"fusion.New"}}}

	tool, evidence := GoldSourceTool(sessions, task)
	if tool != GoldSourceNone || evidence != "" {
		t.Errorf("got (%q,%q), want (%q,\"\") — a trial with no final answer must not credit a tool", tool, evidence, GoldSourceNone)
	}
}

// TestGoldSourceTool_ChangeTaskCreditsToolEvenWithNoFinalAnswer proves the
// empty-answer gate is scoped to lookup mode only: a change task's
// success is decided by tests, not by FinalAnswer text, so its gold
// source must still be credited when FinalAnswer is empty (an agent that
// just made the right edits and said nothing in particular).
func TestGoldSourceTool_ChangeTaskCreditsToolEvenWithNoFinalAnswer(t *testing.T) {
	dir := t.TempDir()
	path := writeGoldTranscript(t, dir, "session.jsonl", []goldStep{
		{Name: "Read", Input: `{"file_path":"/repo/internal/extractor/goextract.go"}`, Result: "package extractor"},
	})
	sessions := []SessionResult{{RawPath: path, FinalAnswer: ""}}
	task := Task{Gold: Gold{Files: []string{"internal/extractor/goextract.go"}}}

	tool, evidence := GoldSourceTool(sessions, task)
	if tool != "Read" || evidence != "internal/extractor/goextract.go" {
		t.Errorf("got (%q,%q), want (Read, internal/extractor/goextract.go) — change-mode gold source must not require a final answer", tool, evidence)
	}
}

// TestGoldSourceTool_ExcludedReadNeverCounts is O53 defect 6d: a Read of
// an excluded (answer-key) path must never be credited, even when its
// content plainly carries the gold token — the LATER, legitimate result
// must win instead.
func TestGoldSourceTool_ExcludedReadNeverCounts(t *testing.T) {
	dir := t.TempDir()
	path := writeGoldTranscript(t, dir, "session.jsonl", []goldStep{
		{Name: "Read", Input: `{"file_path":"/repo/testdata/contextbench/tasks-home.json"}`, Result: "the ground truth is fusion.New"},
		{Name: "mcp__fathom__obligations", Result: "register the extractor via fusion.New(), golden-test it"},
	})
	sessions := []SessionResult{{RawPath: path, FinalAnswer: "fusion.New"}}
	task := Task{
		Oracle:  Oracle{AnswerAllOf: []string{"fusion.New"}},
		Subject: Subject{ExcludePaths: []string{"testdata/contextbench/**"}},
	}

	tool, evidence := GoldSourceTool(sessions, task)
	if tool != "mcp__fathom__obligations" {
		t.Errorf("gold source tool = %q, want the SECOND (legitimate) result — the excluded answer-key read must never be credited", tool)
	}
	if evidence != "fusion.New" {
		t.Errorf("evidence = %q, want fusion.New", evidence)
	}
}

// TestGoldSourceTool_AllHitsExcludedIsNone: when the ONLY hit is inside
// an excluded path, the trial reads "none" rather than crediting it.
func TestGoldSourceTool_AllHitsExcludedIsNone(t *testing.T) {
	dir := t.TempDir()
	path := writeGoldTranscript(t, dir, "session.jsonl", []goldStep{
		{Name: "Read", Input: `{"file_path":"/repo/testdata/contextbench/tasks-home.json"}`, Result: "the ground truth is fusion.New"},
	})
	sessions := []SessionResult{{RawPath: path, FinalAnswer: "fusion.New"}}
	task := Task{
		Oracle:  Oracle{AnswerAllOf: []string{"fusion.New"}},
		Subject: Subject{ExcludePaths: []string{"testdata/contextbench/**"}},
	}

	tool, evidence := GoldSourceTool(sessions, task)
	if tool != GoldSourceNone || evidence != "" {
		t.Errorf("got (%q,%q), want (%q,\"\") — the only hit is inside an excluded path", tool, evidence, GoldSourceNone)
	}
}

// TestGoldSourceTool_ExcludedGrepLineFilteredMixedResultStillCounts
// covers the mixed-result case: a single Grep call whose own body spans
// several files — the line from the excluded path must not count, but a
// legitimate line elsewhere in the SAME result still does.
func TestGoldSourceTool_ExcludedGrepLineFilteredMixedResultStillCounts(t *testing.T) {
	dir := t.TempDir()
	path := writeGoldTranscript(t, dir, "session.jsonl", []goldStep{
		{Name: "Grep", Result: "testdata/contextbench/tasks-home.json:3:ground_truth: fusion.New\ninternal/extractor/goextract.go:12:register via fusion.New()"},
	})
	sessions := []SessionResult{{RawPath: path, FinalAnswer: "fusion.New"}}
	task := Task{
		Oracle:  Oracle{AnswerAllOf: []string{"fusion.New"}},
		Subject: Subject{ExcludePaths: []string{"testdata/contextbench/**"}},
	}

	tool, evidence := GoldSourceTool(sessions, task)
	if tool != "Grep" || evidence != "fusion.New" {
		t.Errorf("got (%q,%q), want (Grep, fusion.New) — the non-excluded line in the same result must still count", tool, evidence)
	}
}

// TestGoldSourceTool_ExcludedGrepLineAloneIsNone: the converse — every
// line in the result is from an excluded path, so nothing counts.
func TestGoldSourceTool_ExcludedGrepLineAloneIsNone(t *testing.T) {
	dir := t.TempDir()
	path := writeGoldTranscript(t, dir, "session.jsonl", []goldStep{
		{Name: "Grep", Result: "testdata/contextbench/tasks-home.json:3:ground_truth: fusion.New"},
	})
	sessions := []SessionResult{{RawPath: path, FinalAnswer: "fusion.New"}}
	task := Task{
		Oracle:  Oracle{AnswerAllOf: []string{"fusion.New"}},
		Subject: Subject{ExcludePaths: []string{"testdata/contextbench/**"}},
	}

	tool, evidence := GoldSourceTool(sessions, task)
	if tool != GoldSourceNone || evidence != "" {
		t.Errorf("got (%q,%q), want (%q,\"\")", tool, evidence, GoldSourceNone)
	}
}

// TestGoldSourceTool_AnswerAnyOfGroupMembersAreGoldSignals proves
// goldSignal flattens answer_any_of groups into the lookup mode's own
// token list (kept alongside defect 6's other fixes — a task using ONLY
// answer_any_of, introduced by O53 defect 5, must not silently read
// goldModeNone).
func TestGoldSourceTool_AnswerAnyOfGroupMembersAreGoldSignals(t *testing.T) {
	dir := t.TempDir()
	path := writeGoldTranscript(t, dir, "session.jsonl", []goldStep{
		{Name: "mcp__fathom__obligations", Result: "carried on core.Edge, never mutating Calls"},
	})
	sessions := []SessionResult{{RawPath: path, FinalAnswer: "carried on the Edge struct"}}
	task := Task{Oracle: Oracle{AnswerAnyOf: [][]string{{"core.Edge", "Edge struct"}}}}

	tool, evidence := GoldSourceTool(sessions, task)
	if tool != "mcp__fathom__obligations" || evidence != "core.Edge" {
		t.Errorf("got (%q,%q), want (mcp__fathom__obligations, core.Edge)", tool, evidence)
	}
}
