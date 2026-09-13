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

// TestSessionReadBytes_GrepAndCatReads is RC-11's fixture proof (vision 28
// §2.4, F6): a worker discovering through Grep and a literal-path Bash
// `cat` is no longer invisible to SessionReadBytes. The committed fixture
// (testdata/scenariobench/streams/grep-and-cat-reads.jsonl) carries one
// Grep call matching a.go and b.go, one `cat a.go`, one PIPED `cat a.go |
// wc -l` (must be ignored — not a literal-path read), and an Edit of a.go
// (must never be counted as a read).
func TestSessionReadBytes_GrepAndCatReads(t *testing.T) {
	got, err := SessionReadBytes(filepath.Join("..", "..", "testdata", "scenariobench", "streams", "grep-and-cat-reads.jsonl"))
	if err != nil {
		t.Fatalf("SessionReadBytes: %v", err)
	}
	// a.go: 8 bytes from the Grep line ("a.go:1:x") + 12 bytes from
	// `cat a.go`'s result ("package foo\n") = 20.
	if got["a.go"] != 20 {
		t.Errorf("a.go bytes = %d, want 20 (8 grep + 12 cat)", got["a.go"])
	}
	// b.go: 8 bytes from the Grep line ("b.go:1:y") only.
	if got["b.go"] != 8 {
		t.Errorf("b.go bytes = %d, want 8 (grep only)", got["b.go"])
	}
	if len(got) != 2 {
		t.Fatalf("got %d files, want 2 (a.go, b.go): %v", len(got), got)
	}
}

func TestBashReadFile(t *testing.T) {
	cases := []struct {
		command string
		want    string
	}{
		{"cat a.go", "a.go"},
		{"cat -n a.go", "a.go"},
		{"head -50 internal/foo.go", "internal/foo.go"},
		{"tail -n 20 internal/foo.go", "internal/foo.go"},
		{"less internal/foo.go", "internal/foo.go"},
		{`sed -n '1,5p' internal/foo.go`, "internal/foo.go"},
		{`awk '{print $1}' internal/foo.go`, "internal/foo.go"},
		{"cat a.go | wc -l", ""},          // pipe: not a literal single-file read
		{"cat a.go > out.txt", ""},        // redirection
		{"cat $FILE", ""},                 // variable, not a literal path
		{"cat *.go", ""},                  // glob, not a literal path
		{"sed -n internal/foo.go", ""},    // sed with no script argument
		{"grep TODO internal/foo.go", ""}, // not a recognized read command
		{"cat", ""},                       // no path at all
		{"cat -n", ""},                    // trailing token is a flag, not a path
	}
	for _, c := range cases {
		if got := bashReadFile(c.command); got != c.want {
			t.Errorf("bashReadFile(%q) = %q, want %q", c.command, got, c.want)
		}
	}
}

func TestGrepFileBytes(t *testing.T) {
	out := grepFileBytes("a.go:1:x\nb.go:1:y\n")
	if out["a.go"] != 8 || out["b.go"] != 8 {
		t.Fatalf("grepFileBytes content-mode = %v, want a.go=8 b.go=8", out)
	}

	out = grepFileBytes("a.go\nb.go\n")
	if out["a.go"] != 4 || out["b.go"] != 4 {
		t.Fatalf("grepFileBytes files_with_matches mode = %v, want a.go=4 b.go=4", out)
	}

	if out := grepFileBytes("no matches found\n"); len(out) != 0 {
		t.Fatalf("grepFileBytes on prose = %v, want empty", out)
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
