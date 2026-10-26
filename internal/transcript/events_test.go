package transcript

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func fixture(t *testing.T, name string) Result {
	t.Helper()
	r := DecodeTranscriptEvents(filepath.Join("testdata", name), "fixture-salt", 0)
	if len(r.Issues) != 0 {
		t.Fatalf("fixture issues: %+v", r.Issues)
	}
	return r
}

func ofKind(r Result, kind string) []Event {
	var events []Event
	for _, e := range r.Events {
		if e.Kind == kind {
			events = append(events, e)
		}
	}
	return events
}

func TestTranscriptEventsAndOrder(t *testing.T) {
	r := fixture(t, "events.jsonl")
	var kinds []string
	for _, e := range r.Events {
		kinds = append(kinds, e.Kind)
	}
	want := []string{"line", "line", "tool_use", "tool_use", "skill", "tool_use", "subagent_start",
		"line", "tool_result", "line", "tool_result", "subagent_stop", "line", "hook_additional_context",
		"line", "hook_additional_context", "line", "subagent_start", "line", "subagent_stop"}
	if !reflect.DeepEqual(kinds, want) {
		t.Fatalf("event order: %v, want %v", kinds, want)
	}
	uses := ofKind(r, "tool_use")
	if uses[0].Name != "Bash" || uses[0].ToolUseID != "bash-1" || uses[0].Shape != "object{string}" {
		t.Fatalf("tool use: %+v", uses[0])
	}
	if got := ofKind(r, "skill"); len(got) != 1 || got[0].Name != "intent-design" {
		t.Fatalf("skills: %+v", got)
	}
	results := ofKind(r, "tool_result")
	if !results[0].IsError || results[0].Bytes != 5 || results[1].IsError || results[1].Bytes != 6 {
		t.Fatalf("results: %+v", results)
	}
	for _, e := range ofKind(r, "hook_additional_context") {
		if e.HookEvent != "PostToolUse" {
			t.Fatalf("hook event lost: %+v", e)
		}
	}
	for _, e := range r.Events {
		if e.Cwd != metadataHash("fixture-salt", "fixture-repo") || e.GitBranch != metadataHash("fixture-salt", "fixture-branch") || e.Timestamp != "2026-10-01T12:00:00Z" || e.Line < 1 {
			t.Fatalf("line metadata lost: %+v", e)
		}
	}
}

func TestTranscriptDedupeByToolUseID(t *testing.T) {
	r := fixture(t, "duplicates.jsonl")
	if got := ofKind(r, "tool_use"); len(got) != 2 || got[0].ToolUseID != "a" || got[1].ToolUseID != "b" {
		t.Fatalf("deduplicated uses: %+v", got)
	}
	if got := ofKind(r, "tool_result"); len(got) != 2 || got[0].ToolUseID != "a" || got[1].ToolUseID != "b" {
		t.Fatalf("deduplicated results: %+v", got)
	}
	if r.Duplicates != 2 {
		t.Fatalf("duplicates = %d, want 2", r.Duplicates)
	}
}

func TestTranscriptInputShapeKeepsNoKeysOrValues(t *testing.T) {
	r := fixture(t, "shapes.jsonl")
	var got []string
	for _, e := range ofKind(r, "tool_use") {
		got = append(got, e.Shape)
	}
	want := []string{"object{array[boolean,null,number],object{string},string}", "object{array[boolean,null,number],object{string},string}", "array[]", "string", "number", "boolean", "null"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("shapes: %v, want %v", got, want)
	}
}

func TestTranscriptMalformedLineCountedAndNamed(t *testing.T) {
	r := DecodeTranscriptEvents(filepath.Join("testdata", "malformed.jsonl"), "fixture-salt", 0)
	if r.MalformedLines != 3 || len(r.Issues) != 3 || len(ofKind(r, "tool_use")) != 1 {
		t.Fatalf("malformed reading: %+v", r)
	}
	for i, issue := range r.Issues {
		if issue.File != "session:0" || issue.Line != i+1 || issue.Kind != "malformed_line" {
			t.Fatalf("unnamed bad line: %+v", issue)
		}
	}
}

func TestTranscriptUnreadableFileCountedAndNamed(t *testing.T) {
	// A directory opens but cannot be read as a file, even when tests run as root.
	r := DecodeTranscriptEvents("testdata", "fixture-salt", 0)
	if r.UnreadableFiles != 1 || len(r.Issues) != 1 || len(r.Events) != 0 || r.Issues[0].File != "session:0" || r.Issues[0].Kind != "unreadable_file" {
		t.Fatalf("unreadable reading: %+v", r)
	}
}

func TestTranscriptEventsContainNoText(t *testing.T) {
	r := fixture(t, "events.jsonl")
	b, err := json.Marshal(r.Events)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Events) == 0 {
		t.Fatal("privacy needs a nonempty reading")
	}
	for _, secret := range []string{"sk-fixture-secret-prompt-12345", "private-command", "private-output", "private-context", "private-task"} {
		if strings.Contains(string(b), secret) || strings.Contains(fmt.Sprintf("%#v", r.Events), secret) {
			t.Fatalf("event retained %q", secret)
		}
	}
}

func TestTranscriptResultRepresentations(t *testing.T) {
	r := fixture(t, "results.jsonl")
	var got []int
	for _, e := range ofKind(r, "tool_result") {
		got = append(got, e.Bytes)
	}
	if want := []int{7, 5, 3, 2}; !reflect.DeepEqual(got, want) {
		t.Fatalf("result bytes: %v, want %v", got, want)
	}
}

func TestTranscriptHookAttachments(t *testing.T) {
	r := fixture(t, "attachments.jsonl")
	got := ofKind(r, "hook_additional_context")
	if len(got) != 2 || got[0].HookEvent != "PostToolUse" || got[0].ToolUseID != "a" || got[1].HookEvent != "SessionStart" {
		t.Fatalf("hook attachments: %+v", got)
	}
	if strings.Contains(fmt.Sprintf("%#v", r.Events), "private-attachment") {
		t.Fatal("attachment text retained")
	}
}

func TestTranscriptSubagentFileBoundaries(t *testing.T) {
	r := DecodeTranscriptEvents(filepath.Join("testdata", "subagents", "agent-fixture.jsonl"), "fixture-salt", 0)
	if len(r.Events) != 3 || r.Events[0].Kind != "subagent_start" || r.Events[1].Kind != "line" || r.Events[2].Kind != "subagent_stop" {
		t.Fatalf("subagent boundaries: %+v", r)
	}
	for _, e := range r.Events {
		if e.Cwd != metadataHash("fixture-salt", "fixture-repo") || e.Timestamp != "2026-10-01T12:00:00Z" {
			t.Fatalf("subagent metadata: %+v", e)
		}
	}
}

func TestTranscriptOversizeLineCountedAndContinues(t *testing.T) {
	p := filepath.Join(t.TempDir(), "oversize.jsonl")
	if err := os.WriteFile(p, []byte(strings.Repeat("x", LineMax+1)+"\n{\"type\":\"user\"}"), 0600); err != nil {
		t.Fatal(err)
	}
	r := DecodeTranscriptEvents(p, "fixture-salt", 0)
	if r.OversizeLines != 1 || len(r.Issues) != 1 || r.Issues[0].Line != 1 || r.Issues[0].Kind != "oversize_line" || len(r.Events) != 1 || r.Events[0].Line != 2 {
		t.Fatalf("oversize reading: %+v", r)
	}
}
