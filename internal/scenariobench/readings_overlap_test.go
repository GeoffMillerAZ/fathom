package scenariobench

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

type overlapRead struct {
	Name, File, Result string
}

// writeOverlapTranscript writes one assistant tool_use + user tool_result
// pair per read, as a `claude -p --output-format stream-json` transcript
// would carry them. Read's input key is file_path; every other tool
// (read_source, by name convention "…read_source") uses file.
func writeOverlapTranscript(t *testing.T, dir, name string, reads []overlapRead) string {
	t.Helper()
	var out []byte
	for i, r := range reads {
		id := fmt.Sprintf("tu%d", i)
		inputKey := "file"
		if r.Name == "Read" {
			inputKey = "file_path"
		}
		input, err := json.Marshal(map[string]string{inputKey: r.File})
		if err != nil {
			t.Fatal(err)
		}
		assistant := map[string]any{
			"type": "assistant",
			"message": map[string]any{
				"content": []map[string]any{
					{"type": "tool_use", "id": id, "name": r.Name, "input": json.RawMessage(input)},
				},
			},
		}
		result, err := json.Marshal(r.Result)
		if err != nil {
			t.Fatal(err)
		}
		user := map[string]any{
			"type": "user",
			"message": map[string]any{
				"content": []map[string]any{
					{"type": "tool_result", "tool_use_id": id, "content": json.RawMessage(result)},
				},
			},
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

func TestSessionReadBytes_ReadAndReadSource(t *testing.T) {
	dir := t.TempDir()
	readResult := "0123456789"
	sourceEnvelope := `{"file":"bar.go","lines":[{"line":1,"text":"package bar"}]}`
	path := writeOverlapTranscript(t, dir, "session.jsonl", []overlapRead{
		{Name: "Read", File: "/repo/foo.go", Result: readResult},
		{Name: "mcp__fathom__read_source", File: "bar.go", Result: sourceEnvelope},
		{Name: "Grep", File: "ignored.go", Result: "should not be counted"},
	})

	got, err := SessionReadBytes(path)
	if err != nil {
		t.Fatalf("SessionReadBytes: %v", err)
	}
	if got["/repo/foo.go"] != int64(len(readResult)) {
		t.Errorf("foo.go bytes = %d, want %d", got["/repo/foo.go"], len(readResult))
	}
	if got["bar.go"] != int64(len(sourceEnvelope)) {
		t.Errorf("bar.go bytes = %d, want %d", got["bar.go"], len(sourceEnvelope))
	}
	if _, ok := got["ignored.go"]; ok {
		t.Errorf("Grep must not be counted as a read")
	}
	if len(got) != 2 {
		t.Errorf("got %d files, want 2: %v", len(got), got)
	}
}

func TestSessionReadBytes_EmptyRawPath(t *testing.T) {
	got, err := SessionReadBytes("")
	if err != nil || got != nil {
		t.Fatalf("got %v, %v; want nil, nil", got, err)
	}
}

func TestReexplorationOverlap_ByteMode(t *testing.T) {
	dir := t.TempDir()
	plannerPath := writeOverlapTranscript(t, dir, "planner.jsonl", []overlapRead{
		{Name: "Read", File: "a.go", Result: string(make([]byte, 100))},
		{Name: "Read", File: "b.go", Result: string(make([]byte, 50))},
	})
	worker1Path := writeOverlapTranscript(t, dir, "worker1.jsonl", []overlapRead{
		{Name: "Read", File: "a.go", Result: string(make([]byte, 100))},
		{Name: "Read", File: "c.go", Result: string(make([]byte, 30))},
	})
	worker2Path := writeOverlapTranscript(t, dir, "worker2.jsonl", []overlapRead{
		{Name: "Read", File: "b.go", Result: string(make([]byte, 50))},
		{Name: "Read", File: "d.go", Result: string(make([]byte, 20))},
	})

	sessions := []SessionResult{
		{Role: "planner", RawPath: plannerPath},
		{Role: "worker", RawPath: worker1Path},
		{Role: "worker", RawPath: worker2Path},
	}
	got := ReexplorationOverlap(sessions)
	if got.Workers != 2 {
		t.Fatalf("workers = %d, want 2", got.Workers)
	}
	if got.WorkerBytes != 200 {
		t.Fatalf("worker bytes = %d, want 200", got.WorkerBytes)
	}
	if got.ReexploreBytes != 150 {
		t.Fatalf("reexplore bytes = %d, want 150", got.ReexploreBytes)
	}
	if got.Ratio != 0.75 {
		t.Fatalf("ratio = %v, want 0.75", got.Ratio)
	}
	if got.NotMeasurable != "" {
		t.Fatalf("unexpected not_measurable: %s", got.NotMeasurable)
	}
}

func TestReexplorationOverlap_FileCountFallback(t *testing.T) {
	sessions := []SessionResult{
		{Role: "planner", FilesRead: []string{"a.go", "b.go"}},
		{Role: "worker", FilesRead: []string{"a.go", "c.go"}},
		{Role: "worker", FilesRead: []string{"b.go", "d.go"}},
	}
	got := ReexplorationOverlap(sessions)
	if got.WorkerBytes != 4 {
		t.Fatalf("worker file count = %d, want 4", got.WorkerBytes)
	}
	if got.ReexploreBytes != 2 {
		t.Fatalf("reexplore file count = %d, want 2", got.ReexploreBytes)
	}
	if got.Ratio != 0.5 {
		t.Fatalf("ratio = %v, want 0.5", got.Ratio)
	}
	if got.NotMeasurable == "" {
		t.Fatalf("expected a not_measurable note explaining the file-count fallback")
	}
}

func TestReexplorationOverlap_NoSessions(t *testing.T) {
	got := ReexplorationOverlap([]SessionResult{{Role: "single"}})
	if got.NotMeasurable != "no planner/worker sessions recorded" {
		t.Fatalf("not_measurable = %q", got.NotMeasurable)
	}
}
