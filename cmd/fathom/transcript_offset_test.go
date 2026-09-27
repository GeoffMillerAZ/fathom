package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// RT-132 (design §11.3): the Stop hook reads the transcript from a
// per-session offset stamp — each turn folds only the bytes appended since
// the last one — and the summary equals a whole-file read's; a replaced or
// shrunken transcript restarts from 0.
func TestStopHookReadsTranscriptIncrementally(t *testing.T) {
	t.Setenv("FATHOM_SESSION_STAMPS", t.TempDir())
	path := filepath.Join(t.TempDir(), "transcript.jsonl")
	turn := func(i int) string {
		return fmt.Sprintf(`{"type":"assistant","timestamp":"2026-09-27T07:%02d:00Z","message":{"usage":{"input_tokens":10,"output_tokens":5},"content":[{"type":"tool_use","name":"Read","id":"u%d","input":{"file_path":"/r/a.go"}}]}}`+"\n", i, i) +
			fmt.Sprintf(`{"type":"user","timestamp":"2026-09-27T07:%02d:30Z","message":{"content":[{"type":"tool_result","tool_use_id":"u%d","content":"%s"}]}}`+"\n", i, i, strings.Repeat("x", 40))
	}
	var whole strings.Builder
	var lastOffset int64
	for i := 0; i < 6; i++ {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		chunk := turn(i)
		f.WriteString(chunk)
		f.Close()
		whole.WriteString(chunk)

		got := readTranscriptFromOffset("sess-1", path)
		want := summarizeTranscriptBytes([]byte(whole.String()))
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("turn %d: incremental summary differs from a whole-file read:\n got %+v\nwant %+v", i, got, want)
		}
		b, err := os.ReadFile(transcriptStampPath("sess-1"))
		if err != nil {
			t.Fatalf("turn %d: no offset stamp: %v", i, err)
		}
		var st transcriptOffsetStamp
		if err := json.Unmarshal(b, &st); err != nil {
			t.Fatal(err)
		}
		// Each Stop read only this turn's bytes: the offset advanced by them.
		if st.Offset-lastOffset != int64(len(chunk)) {
			t.Fatalf("turn %d: offset advanced %d, want %d (the bytes appended)", i, st.Offset-lastOffset, len(chunk))
		}
		lastOffset = st.Offset
	}
	if got := readTranscriptFromOffset("sess-1", path); got.RepeatedReadFiles != 1 || *got.TurnCount != 6 {
		t.Fatalf("the fold carries pairing state across Stops: %+v", got)
	}

	// An unterminated tail counts in this summary but is re-read next time.
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	partial := strings.TrimSuffix(turn(6), "\n")
	f.WriteString(partial)
	f.Close()
	whole.WriteString(partial)
	if got, want := readTranscriptFromOffset("sess-1", path), summarizeTranscriptBytes([]byte(whole.String())); !reflect.DeepEqual(got, want) {
		t.Fatalf("with a partial tail:\n got %+v\nwant %+v", got, want)
	}

	// A replaced (shorter) transcript restarts from 0.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(turn(0)), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, want := readTranscriptFromOffset("sess-1", path), summarizeTranscriptBytes([]byte(turn(0))); !reflect.DeepEqual(got, want) {
		t.Fatalf("a replaced transcript is folded from 0:\n got %+v\nwant %+v", got, want)
	}
	// Another session has its own stamp.
	if transcriptStampPath("sess-2") == transcriptStampPath("sess-1") {
		t.Fatal("stamps are per session")
	}
}
