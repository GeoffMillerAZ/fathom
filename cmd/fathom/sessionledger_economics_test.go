package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeTranscript writes lines as a Claude Code transcript and returns its path.
func writeTranscript(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "transcript.jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// ledgerRowMap marshals e the way appendSessionLedger does and decodes it
// back as a generic object, so a test reads the row a reader would.
func ledgerRowMap(t *testing.T, e sessionLedgerEntry) map[string]any {
	t.Helper()
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	var row map[string]any
	if err := json.Unmarshal(b, &row); err != nil {
		t.Fatal(err)
	}
	return row
}

// PE-13w: the Stop hook's ledger row carries the whole-session tool-call
// count from the same fold that fills its token and cache fields, and the
// field is absent (never a zero) when no transcript was parsed (law 12).
func TestSessionLedgerEntryCarriesToolCalls(t *testing.T) {
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	path := writeTranscript(t,
		`{"type":"user","timestamp":"2026-09-29T11:00:00Z"}`,
		`{"type":"assistant","timestamp":"2026-09-29T11:00:05Z","message":{"usage":{"input_tokens":10,"output_tokens":5,"cache_read_input_tokens":100,"cache_creation_input_tokens":20},"content":[{"type":"tool_use","id":"a","name":"Read","input":{"file_path":"x.go"}},{"type":"tool_use","id":"b","name":"Bash"}]}}`,
		`{"type":"assistant","timestamp":"2026-09-29T11:00:09Z","message":{"content":[{"type":"text"},{"type":"tool_use","id":"c","name":"Edit"}]}}`,
	)
	e := buildSessionLedgerEntry("sess-tc", "fathom", transcriptSummaryFrom(path), nil, now)
	if e.ToolCalls == nil || *e.ToolCalls != 3 {
		t.Fatalf("ToolCalls = %v, want 3 (every tool_use block, any name)", e.ToolCalls)
	}
	row := ledgerRowMap(t, e)
	if row["tool_calls"] != float64(3) {
		t.Errorf("row tool_calls = %v, want 3", row["tool_calls"])
	}
	if row["tokens_in"] != float64(10) || row["tokens_cache_read"] != float64(100) {
		t.Errorf("the token and cache fields ride the same fold: %v", row)
	}

	// A parsed transcript with no tool call is a real zero, recorded.
	quiet := writeTranscript(t, `{"type":"assistant","timestamp":"2026-09-29T11:00:05Z","message":{"content":[{"type":"text"}]}}`)
	z := buildSessionLedgerEntry("sess-quiet", "fathom", transcriptSummaryFrom(quiet), nil, now)
	if z.ToolCalls == nil || *z.ToolCalls != 0 {
		t.Errorf("a parsed transcript with no tool_use records 0, got %v", z.ToolCalls)
	}

	// No transcript parsed: the field is absent, never a fabricated zero.
	for name, ts := range map[string]transcriptSummary{
		"no path":    transcriptSummaryFrom(""),
		"unreadable": transcriptSummaryFrom(filepath.Join(t.TempDir(), "missing.jsonl")),
	} {
		e := buildSessionLedgerEntry("sess-none", "fathom", ts, nil, now)
		if e.ToolCalls != nil {
			t.Errorf("%s: ToolCalls = %d, want nil", name, *e.ToolCalls)
		}
		if _, present := ledgerRowMap(t, e)["tool_calls"]; present {
			t.Errorf("%s: tool_calls must be absent when no transcript was parsed", name)
		}
	}

	// The product path: the Stop hook's own append carries it.
	ledger := withTempSessionLedger(t)
	t.Setenv("FATHOM_ARM_STATE", filepath.Join(t.TempDir(), "arm.json"))
	t.Setenv("FATHOM_SESSION_STAMPS", t.TempDir())
	recordSessionLedgerEntry("sess-hook", path, t.TempDir(), now, nil)
	data, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got["tool_calls"] != float64(3) {
		t.Errorf("the Stop hook's row tool_calls = %v, want 3", got["tool_calls"])
	}
}
