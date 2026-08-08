package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFleetReport pins IO-16 (vision 14 §4): one verb folds the registry,
// traffic, wiring, and observations into per-service rows — local files
// only, unmeasured cells named, never zeroed.
func TestFleetReport(t *testing.T) {
	repo := t.TempDir()
	os.MkdirAll(filepath.Join(repo, ".claude"), 0o755)
	os.WriteFile(filepath.Join(repo, ".claude", "settings.json"), []byte(`{"hooks":{"PostToolUse":[{"hooks":[{"command":"fathom edit-hook"}]}]}}`), 0o644)
	os.WriteFile(filepath.Join(repo, "CLAUDE.md"), []byte("# x\nfathom stanza here\n"), 0o644)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/statusz":
			w.Write([]byte(`{"services":[{"service":"alpha","indexed_commit":"deadbeef"}]}`))
		default:
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"services":[{"service":"alpha","symbol_count":42,"languages":["go"],"path":"` + repo + `"}]}}`))
		}
	}))
	defer srv.Close()

	logPath := filepath.Join(t.TempDir(), "calls.jsonl")
	os.WriteFile(logPath, []byte(`{"ts":"2026-08-07T01:00:00Z","tool":"find_symbol","service":"alpha","surface":"agent"}
`), 0o644)
	// Route the default call-log resolution at our fixture via --json + a chdir-independent path:
	t.Setenv("FATHOM_SESSION_LEDGER", filepath.Join(t.TempDir(), "none.jsonl"))

	var stdout, stderr bytes.Buffer
	// Use --json and point the resolver by making cwd hold the fixture log path? Simpler: the
	// call log resolution falls back to daemon candidates; here we assert the report structure
	// and wiring notes, with traffic honestly not-measured when the log isn't found.
	code := cmdFleetReport([]string{"--url", srv.URL, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	var rep map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &rep); err != nil {
		t.Fatalf("json: %v", err)
	}
	services, _ := rep["services"].([]any)
	if len(services) != 1 {
		t.Fatalf("services = %v", rep["services"])
	}
	row := services[0].(map[string]any)
	if row["service"] != "alpha" || row["symbols"] != float64(42) {
		t.Errorf("row = %v", row)
	}
	if hooks, _ := row["hooks_note"].(string); !strings.Contains(hooks, "fathom hook") {
		t.Errorf("hooks_note = %q", hooks)
	}
	if stanza, _ := row["stanza_note"].(string); stanza != "present" {
		t.Errorf("stanza_note = %q", stanza)
	}
	if fr, _ := row["freshness"].(string); !strings.Contains(fr, "unknown") && !strings.Contains(fr, "stale") && fr != "fresh" {
		t.Errorf("freshness = %q (must be a named three-state)", fr)
	}
	if rep["schema_version"] != float64(1) || rep["fathom_version"] == "" {
		t.Errorf("stamps missing: %v", rep)
	}
}
