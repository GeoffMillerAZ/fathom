package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFoldTranscriptCostIncremental (PE-13, docs/design/progress-and-economics.md
// §5): the cost fields outcome economics joins to a proven row — tokens in
// and out, cache reads and writes, and tool calls — folded incrementally
// (one Stop-hook-shaped append at a time, RT-132's offset stamp) equal a
// whole-file fold's, and ToolCalls counts every tool name (Edit, Bash,
// mcp__fathom__*), not only the raw-discovery trio the exploration ledger
// already tracked.
func TestFoldTranscriptCostIncremental(t *testing.T) {
	t.Setenv("FATHOM_SESSION_STAMPS", t.TempDir())
	path := filepath.Join(t.TempDir(), "transcript.jsonl")
	turn := func(i int, tool string) string {
		return fmt.Sprintf(`{"type":"assistant","timestamp":"2026-09-28T07:%02d:00Z","message":{"usage":{"input_tokens":11,"output_tokens":7,"cache_read_input_tokens":3,"cache_creation_input_tokens":2},"content":[{"type":"tool_use","name":"%s","id":"u%d"}]}}`+"\n", i, tool, i) +
			fmt.Sprintf(`{"type":"user","timestamp":"2026-09-28T07:%02d:30Z","message":{"content":[{"type":"tool_result","tool_use_id":"u%d","content":"ok"}]}}`+"\n", i, i)
	}
	tools := []string{"Read", "Edit", "Bash", "mcp__fathom__search", "Grep"}
	var whole strings.Builder
	var lastCost transcriptCost
	for i, tool := range tools {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		chunk := turn(i, tool)
		if _, err := f.WriteString(chunk); err != nil {
			t.Fatal(err)
		}
		f.Close()
		whole.WriteString(chunk)

		got := foldTranscriptCost("sess-cost", path)
		want := transcriptCostFromSummary(summarizeTranscriptBytes([]byte(whole.String())))
		if got != want {
			t.Fatalf("turn %d (%s): incremental cost differs from whole-file:\n got %+v\nwant %+v", i, tool, got, want)
		}
		lastCost = got
	}
	if wantCalls := len(tools); lastCost.ToolCalls != wantCalls {
		t.Fatalf("ToolCalls = %d, want %d (all tool names, including Edit and Bash)", lastCost.ToolCalls, wantCalls)
	}
	wantIn, wantOut := 11*len(tools), 7*len(tools)
	wantCacheRead, wantCacheCreate := 3*len(tools), 2*len(tools)
	if lastCost.TokensIn != wantIn || lastCost.TokensOut != wantOut ||
		lastCost.TokensCacheRead != wantCacheRead || lastCost.TokensCacheCreate != wantCacheCreate {
		t.Fatalf("token split = in:%d out:%d cacheRead:%d cacheCreate:%d, want in:%d out:%d cacheRead:%d cacheCreate:%d",
			lastCost.TokensIn, lastCost.TokensOut, lastCost.TokensCacheRead, lastCost.TokensCacheCreate,
			wantIn, wantOut, wantCacheRead, wantCacheCreate)
	}
	if !lastCost.Available {
		t.Fatal("Available should be true once usage data was seen")
	}

	// A transcript with no usage data at all reports cost fields as
	// unavailable — never a fabricated zero indistinguishable from a real
	// zero-cost session (law 12).
	empty := filepath.Join(t.TempDir(), "empty.jsonl")
	if err := os.WriteFile(empty, []byte(`{"type":"assistant","timestamp":"2026-09-28T08:00:00Z"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := foldTranscriptCost("sess-empty", empty); got.Available {
		t.Fatalf("no usage data in transcript: want Available=false, got %+v", got)
	}
}
