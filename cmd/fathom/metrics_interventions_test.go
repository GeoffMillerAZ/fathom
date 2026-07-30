package main

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestCmdMetricsInterventions_JSON_AllFiveRows(t *testing.T) {
	now := time.Now().UTC()
	callLogPath := filepath.Join(t.TempDir(), "calls.jsonl")
	writeCallLogFixture(t, callLogPath, []adoptionCallEntry{
		{TS: now.Add(-time.Minute).Format(time.RFC3339Nano), Tool: "verify_plan", VerdictOutcome: map[string]int{"claimed": 5, "verified": 3, "failing": 2}},
	})
	beatsPath := writeBeatsFixture(t, nil)

	var out, errBuf bytes.Buffer
	code := cmdMetricsInterventions([]string{"--call-log", callLogPath, "--hooks-log", beatsPath, "--since", "1h", "--json"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errBuf.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	rows, _ := payload["rows"].([]any)
	if len(rows) != 5 {
		t.Fatalf("rows = %+v, want 5", rows)
	}
	row1 := rows[0].(map[string]any)
	if row1["count"].(float64) != 2 {
		t.Fatalf("row1 count = %v, want 2", row1["count"])
	}
}

func TestCmdMetricsInterventions_NoStoreFlag_ReviewRowDegradesHonestly(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := cmdMetricsInterventions([]string{"--call-log", filepath.Join(t.TempDir(), "none.jsonl"), "--json"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errBuf.String())
	}
	var payload map[string]any
	json.Unmarshal(out.Bytes(), &payload)
	rows, _ := payload["rows"].([]any)
	row2 := rows[1].(map[string]any)
	if row2["count"] != nil {
		t.Fatalf("row2 count = %v, want nil (no --store given)", row2["count"])
	}
	if row2["note"] == "" || row2["note"] == nil {
		t.Error("row2 note is empty, want an honest not-queried note")
	}
}

func TestCmdMetricsInterventions_WithStore_ReviewRowFromClaimEvents(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "s.sqlite")
	st, err := store.NewSQLiteStore(storePath)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	now := time.Now().UTC()
	if err := st.AppendClaimEvent(context.Background(), core.ClaimEvent{
		ClaimID: "c1", Verdict: "superseded", Author: "review", At: now.Add(-time.Minute),
	}); err != nil {
		t.Fatalf("AppendClaimEvent: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	var out, errBuf bytes.Buffer
	code := cmdMetricsInterventions([]string{"--store", storePath, "--call-log", filepath.Join(t.TempDir(), "none.jsonl"), "--since", "1h", "--json"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errBuf.String())
	}
	var payload map[string]any
	json.Unmarshal(out.Bytes(), &payload)
	rows, _ := payload["rows"].([]any)
	row2 := rows[1].(map[string]any)
	if row2["count"] == nil || row2["count"].(float64) != 1 {
		t.Fatalf("row2 count = %v, want 1", row2["count"])
	}
}
