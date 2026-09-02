package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestInterventionsDefaultsStorePath pins IT-8: `fathom metrics
// interventions` must default --store to the repo's resolved store path
// (the same [daemon].store-or-literal-default resolution resolveCallLogPath
// already uses for --call-log) instead of silently skipping claim_events
// when no --store flag is given. An explicit --store still wins.
func TestInterventionsDefaultsStorePath(t *testing.T) {
	_, projectDir := withIsolatedConfigHome(t)

	_ = projectDir
	// PC-15 / I31: the default is resolved (repo workspace/ or ~/.fathom),
	// never the bare cwd — withIsolatedConfigHome chdirs into a non-git
	// temp dir with HOME isolated, so this lands under that HOME.
	storePath := defaultStorePath()
	if err := os.MkdirAll(filepath.Dir(storePath), 0o755); err != nil {
		t.Fatal(err)
	}
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
	// No --store flag given — must still find and query the default store.
	code := cmdMetricsInterventions([]string{"--call-log", filepath.Join(t.TempDir(), "none.jsonl"), "--since", "1h", "--json"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errBuf.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	rows, _ := payload["rows"].([]any)
	row2 := rows[1].(map[string]any)
	if row2["count"] == nil || row2["count"].(float64) != 1 {
		t.Fatalf("row2 count = %v, want 1 (claim_events must be queried from the default-resolved store)", row2["count"])
	}
}
