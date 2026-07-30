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

// seedCompileRunsStore creates a fresh on-disk SQLite store at
// t.TempDir()/s.sqlite, records every run, and closes it — MountReadOnly
// (the read path every metrics verb uses) requires a real on-disk file.
func seedCompileRunsStore(t *testing.T, runs []core.CompileRun) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "s.sqlite")
	st, err := store.NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	ctx := context.Background()
	for _, r := range runs {
		if err := st.RecordCompileRun(ctx, r); err != nil {
			t.Fatalf("RecordCompileRun: %v", err)
		}
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	return path
}

func TestCmdMetricsSpend_JSON_GroupByLayer(t *testing.T) {
	storePath := seedCompileRunsStore(t, []core.CompileRun{
		{RunID: "r1", ServiceID: "svc", Layer: "obligations", CostUSD: 0.10, ClaimsEmitted: 5, RanAt: time.Now().UTC()},
		{RunID: "r2", ServiceID: "svc", Layer: "obligations", CostUSD: 0.20, ClaimsEmitted: 5, RanAt: time.Now().UTC()},
	})
	var out, errBuf bytes.Buffer
	code := cmdMetricsSpend([]string{"--store", storePath, "--service", "svc", "--json"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errBuf.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v\noutput: %s", err, out.String())
	}
	rows, _ := payload["rows"].([]any)
	if len(rows) != 1 {
		t.Fatalf("rows = %+v, want 1", rows)
	}
	row := rows[0].(map[string]any)
	if row["group"] != "obligations" || row["runs"].(float64) != 2 {
		t.Fatalf("row = %+v", row)
	}
}

func TestCmdMetricsSpend_SinceFlag_FiltersWindow(t *testing.T) {
	storePath := seedCompileRunsStore(t, []core.CompileRun{
		{RunID: "old", ServiceID: "svc", Layer: "obligations", CostUSD: 0.10, ClaimsEmitted: 1, RanAt: time.Now().UTC().Add(-30 * 24 * time.Hour)},
		{RunID: "new", ServiceID: "svc", Layer: "obligations", CostUSD: 0.20, ClaimsEmitted: 1, RanAt: time.Now().UTC()},
	})
	var out, errBuf bytes.Buffer
	code := cmdMetricsSpend([]string{"--store", storePath, "--service", "svc", "--since", "24h", "--json"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errBuf.String())
	}
	var payload map[string]any
	json.Unmarshal(out.Bytes(), &payload)
	if payload["total_runs"].(float64) != 1 {
		t.Fatalf("total_runs = %v, want 1 (only the new run)", payload["total_runs"])
	}
}

func TestCmdMetricsSpend_FromTo_FiltersWindow(t *testing.T) {
	fixed := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	storePath := seedCompileRunsStore(t, []core.CompileRun{
		{RunID: "in", ServiceID: "svc", Layer: "obligations", CostUSD: 0.10, ClaimsEmitted: 1, RanAt: fixed},
		{RunID: "out", ServiceID: "svc", Layer: "obligations", CostUSD: 0.20, ClaimsEmitted: 1, RanAt: fixed.Add(48 * time.Hour)},
	})
	var out, errBuf bytes.Buffer
	code := cmdMetricsSpend([]string{
		"--store", storePath, "--service", "svc",
		"--from", "2026-07-30T00:00:00Z", "--to", "2026-07-30T23:59:59Z", "--json",
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errBuf.String())
	}
	var payload map[string]any
	json.Unmarshal(out.Bytes(), &payload)
	if payload["total_runs"].(float64) != 1 {
		t.Fatalf("total_runs = %v, want 1", payload["total_runs"])
	}
}

func TestCmdMetricsSpend_ModuleFilter(t *testing.T) {
	storePath := seedCompileRunsStore(t, []core.CompileRun{
		{RunID: "r1", ServiceID: "svc", Layer: "obligations", CostUSD: 0.10, ClaimsEmitted: 1, RanAt: time.Now().UTC()},
		{RunID: "r2", ServiceID: "svc", Layer: "findings", CostUSD: 0.20, ClaimsEmitted: 1, RanAt: time.Now().UTC()},
	})
	var out, errBuf bytes.Buffer
	code := cmdMetricsSpend([]string{"--store", storePath, "--service", "svc", "--module", "findings", "--json"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errBuf.String())
	}
	var payload map[string]any
	json.Unmarshal(out.Bytes(), &payload)
	rows, _ := payload["rows"].([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["group"] != "findings" {
		t.Fatalf("rows = %+v, want only findings", rows)
	}
}

func TestCmdMetricsSpend_MissingStore_ErrorExit(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := cmdMetricsSpend([]string{"--service", "svc"}, &out, &errBuf)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (missing --store)", code)
	}
}

func TestCmdMetricsSpend_MissingService_ErrorExit(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := cmdMetricsSpend([]string{"--store", "x.sqlite"}, &out, &errBuf)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (missing --service)", code)
	}
}

// TestCmdMetricsSpend_BudgetLookup_LayerNotFoundUnderRoot_HonestNote: no
// layer pack under --root for the run's layer name — BudgetNote must name
// that, never a fabricated ceiling.
func TestCmdMetricsSpend_BudgetLookup_LayerNotFoundUnderRoot_HonestNote(t *testing.T) {
	storePath := seedCompileRunsStore(t, []core.CompileRun{
		{RunID: "r1", ServiceID: "svc", Layer: "no-such-layer-anywhere", CostUSD: 0.10, ClaimsEmitted: 1, RanAt: time.Now().UTC()},
	})
	dir := t.TempDir()
	var out, errBuf bytes.Buffer
	code := cmdMetricsSpend([]string{"--store", storePath, "--service", "svc", "--root", dir, "--json"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errBuf.String())
	}
	var payload map[string]any
	json.Unmarshal(out.Bytes(), &payload)
	rows, _ := payload["rows"].([]any)
	if len(rows) != 1 {
		t.Fatalf("rows = %+v", rows)
	}
	row := rows[0].(map[string]any)
	if row["budget_max_usd"] != nil {
		t.Errorf("budget_max_usd = %v, want nil", row["budget_max_usd"])
	}
	if row["budget_note"] == "" || row["budget_note"] == nil {
		t.Error("budget_note is empty, want an honest not-found note")
	}
}
