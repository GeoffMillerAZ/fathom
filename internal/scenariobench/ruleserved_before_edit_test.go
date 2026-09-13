package scenariobench

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// writeRawLines writes each of lines as one line of a stream-json
// transcript, in order — used here to interleave real tool_use/tool_result
// events with a raw hook-rendered line (a plain string, exactly how a
// PreToolUse/PostToolUse hook's stdout or a SessionStart stanza line reads
// in a captured transcript: not itself a tool_use/tool_result event).
func writeRawLines(t *testing.T, dir, name string, lines []string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	content := ""
	for _, l := range lines {
		content += l + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// editLine renders one assistant Edit tool_use of file, as a stream-json
// line.
func editLine(id, file string) string {
	b, _ := json.Marshal(map[string]any{
		"type": "assistant",
		"message": map[string]any{
			"content": []map[string]any{
				{"type": "tool_use", "id": id, "name": "Edit", "input": map[string]string{"file_path": file, "old_string": "a", "new_string": "b"}},
			},
		},
	})
	return string(b)
}

// rawHookLine wraps text (a hook's rendered marker text) as one plain
// system-shaped stream-json line — the marker search only needs the
// substring to appear on the line, so a minimal envelope suffices.
func rawHookLine(text string) string {
	b, _ := json.Marshal(map[string]any{"type": "system", "text": text})
	return string(b)
}

func TestRuleServedBeforeEdit_ServedBeforeFirstEdit(t *testing.T) {
	dir := t.TempDir()
	path := writeRawLines(t, dir, "session.jsonl", []string{
		rawHookLine("⚖️ fathom guard — internal/store/db.go: I2: store stays pure Go"),
		editLine("tu1", "internal/store/db.go"),
	})
	session := SessionResult{RawPath: path}
	got := RuleServedBeforeEdit(session, []string{"internal/store/db.go"})
	if got == nil || !*got {
		t.Fatalf("got %v, want true (guard rendered before the edit)", got)
	}
}

func TestRuleServedBeforeEdit_EditBeforeServed(t *testing.T) {
	dir := t.TempDir()
	path := writeRawLines(t, dir, "session.jsonl", []string{
		editLine("tu1", "internal/store/db.go"),
		rawHookLine("⚖️ fathom obligations — internal/store/db.go: I2: store stays pure Go"),
	})
	session := SessionResult{RawPath: path}
	got := RuleServedBeforeEdit(session, []string{"internal/store/db.go"})
	if got == nil || *got {
		t.Fatalf("got %v, want false (post-hook rendered AFTER the edit it answers)", got)
	}
}

func TestRuleServedBeforeEdit_SessionStartStanzaCounts(t *testing.T) {
	dir := t.TempDir()
	path := writeRawLines(t, dir, "session.jsonl", []string{
		rawHookLine("Obligations — governing rules for the file you are about to touch — are injected automatically."),
		editLine("tu1", "internal/store/db.go"),
	})
	session := SessionResult{RawPath: path}
	got := RuleServedBeforeEdit(session, []string{"internal/store/db.go"})
	if got == nil || !*got {
		t.Fatalf("got %v, want true (the SessionStart stanza counts as served)", got)
	}
}

func TestRuleServedBeforeEdit_NeverServed(t *testing.T) {
	dir := t.TempDir()
	path := writeRawLines(t, dir, "session.jsonl", []string{
		editLine("tu1", "internal/store/db.go"),
	})
	session := SessionResult{RawPath: path}
	got := RuleServedBeforeEdit(session, []string{"internal/store/db.go"})
	if got == nil || *got {
		t.Fatalf("got %v, want false (no served-rule text ever appeared)", got)
	}
}

func TestRuleServedBeforeEdit_NoEditOfGovernedPathIsNotMeasurable(t *testing.T) {
	dir := t.TempDir()
	path := writeRawLines(t, dir, "session.jsonl", []string{
		rawHookLine("⚖️ fathom guard — other.go: some rule"),
		editLine("tu1", "other.go"), // not a governed path
	})
	session := SessionResult{RawPath: path}
	got := RuleServedBeforeEdit(session, []string{"internal/store/db.go"})
	if got != nil {
		t.Fatalf("got %v, want nil (session never edited a governed path)", got)
	}
}

func TestRuleServedBeforeEdit_NoRawPathIsNotMeasurable(t *testing.T) {
	got := RuleServedBeforeEdit(SessionResult{}, []string{"a.go"})
	if got != nil {
		t.Fatalf("got %v, want nil (no raw transcript)", got)
	}
}

func TestRuleServedBeforeEdit_NoGovernedPathsIsNotMeasurable(t *testing.T) {
	got := RuleServedBeforeEdit(SessionResult{RawPath: "irrelevant"}, nil)
	if got != nil {
		t.Fatalf("got %v, want nil (no governed paths given)", got)
	}
}

func TestRuleServedBeforeEdit_SameFileAbsoluteVsRelative(t *testing.T) {
	dir := t.TempDir()
	path := writeRawLines(t, dir, "session.jsonl", []string{
		rawHookLine("⚖️ fathom guard — internal/store/db.go: rule"),
		editLine("tu1", fmt.Sprintf("%s/internal/store/db.go", dir)), // absolute path in the edit
	})
	session := SessionResult{RawPath: path}
	got := RuleServedBeforeEdit(session, []string{"internal/store/db.go"}) // repo-relative governed path
	if got == nil || !*got {
		t.Fatalf("got %v, want true (sameFile matches the absolute edit path to the relative governed path)", got)
	}
}
