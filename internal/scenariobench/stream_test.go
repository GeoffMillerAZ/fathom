package scenariobench

import (
	"path/filepath"
	"testing"
)

const streamsDir = "../../testdata/scenariobench/streams"

func TestParseSessionStream_SingleSession(t *testing.T) {
	r, err := ParseSessionStream(filepath.Join(streamsDir, "single-session.jsonl"))
	if err != nil {
		t.Fatalf("ParseSessionStream: %v", err)
	}
	if r.Turns != 4 {
		t.Errorf("Turns = %d, want 4", r.Turns)
	}
	if r.ToolSearchTurns != 1 {
		t.Errorf("ToolSearchTurns = %d, want 1", r.ToolSearchTurns)
	}
	if r.FirstEditTurn != 3 {
		t.Errorf("FirstEditTurn = %d, want 3", r.FirstEditTurn)
	}
	if r.TokensToFirstEdit != 27 {
		t.Errorf("TokensToFirstEdit = %d, want 27", r.TokensToFirstEdit)
	}
	if len(r.FilesRead) != 1 || r.FilesRead[0] != "/repo/foo.go" {
		t.Errorf("FilesRead = %v, want [/repo/foo.go]", r.FilesRead)
	}
	if len(r.FilesEdited) != 1 || r.FilesEdited[0] != "/repo/foo.go" {
		t.Errorf("FilesEdited = %v, want [/repo/foo.go]", r.FilesEdited)
	}
	wantBytesRead := int64(len("package foo\n\nfunc Foo() {}\n"))
	if r.BytesRead != wantBytesRead {
		t.Errorf("BytesRead = %d, want %d", r.BytesRead, wantBytesRead)
	}
	if r.ToolCalls["ToolSearch"] != 1 || r.ToolCalls["Read"] != 1 || r.ToolCalls["Edit"] != 1 {
		t.Errorf("ToolCalls = %v, want ToolSearch=1 Read=1 Edit=1", r.ToolCalls)
	}
	if len(r.BoxChecks) != 0 {
		t.Errorf("BoxChecks = %v, want none (not a markdown file)", r.BoxChecks)
	}
	if r.TotalTokens != 39 {
		t.Errorf("TotalTokens = %d, want 39 (26 in + 13 out)", r.TotalTokens)
	}
	if r.CostUSD != 0.0123 {
		t.Errorf("CostUSD = %v, want 0.0123", r.CostUSD)
	}
	if r.Error != "" {
		t.Errorf("Error = %q, want empty", r.Error)
	}
}

func TestParseSessionStream_WorkerBoxCheck(t *testing.T) {
	r, err := ParseSessionStream(filepath.Join(streamsDir, "worker.jsonl"))
	if err != nil {
		t.Fatalf("ParseSessionStream: %v", err)
	}
	if len(r.BoxChecks) != 1 {
		t.Fatalf("BoxChecks = %v, want exactly 1", r.BoxChecks)
	}
	bc := r.BoxChecks[0]
	if bc.RowID != "row-1" {
		t.Errorf("BoxChecks[0].RowID = %q, want row-1", bc.RowID)
	}
	if bc.File != "/repo/NOTES.md" {
		t.Errorf("BoxChecks[0].File = %q, want /repo/NOTES.md", bc.File)
	}
	if bc.Turn != 2 {
		t.Errorf("BoxChecks[0].Turn = %d, want 2", bc.Turn)
	}
}

func TestParseSessionStream_PlannerWriteNoBoxCheck(t *testing.T) {
	// The planner's Write creates rows that are UNCHECKED ("- [ ]"); a
	// freshly-written checklist with no checked rows must not be
	// mis-detected as a box check.
	r, err := ParseSessionStream(filepath.Join(streamsDir, "planner.jsonl"))
	if err != nil {
		t.Fatalf("ParseSessionStream: %v", err)
	}
	if len(r.BoxChecks) != 0 {
		t.Errorf("BoxChecks = %v, want none (rows are unchecked)", r.BoxChecks)
	}
	if len(r.FilesEdited) != 1 || r.FilesEdited[0] != "/repo/NOTES.md" {
		t.Errorf("FilesEdited = %v, want [/repo/NOTES.md]", r.FilesEdited)
	}
}

func TestParseSessionStream_MissingFile(t *testing.T) {
	if _, err := ParseSessionStream(filepath.Join(streamsDir, "does-not-exist.jsonl")); err == nil {
		t.Fatal("expected an error for a missing transcript")
	}
}

func TestParseSessionStream_NoResultEvent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "no-result.jsonl")
	writeTestFile(t, path, `{"type":"assistant","message":{"content":[{"type":"text","text":"hi"}],"usage":{"input_tokens":1,"output_tokens":1}}}`+"\n")
	r, err := ParseSessionStream(path)
	if err != nil {
		t.Fatalf("ParseSessionStream: %v", err)
	}
	if r.Error == "" {
		t.Error("expected Error to be set when no terminal result event is present")
	}
}
