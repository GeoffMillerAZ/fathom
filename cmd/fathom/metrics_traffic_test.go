package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCmdMetricsTraffic_JSON_GroupBySurface(t *testing.T) {
	now := time.Now().UTC()
	logPath := filepath.Join(t.TempDir(), "calls.jsonl")
	writeCallLogFixture(t, logPath, []adoptionCallEntry{
		{TS: now.Add(-1 * time.Minute).Format(time.RFC3339Nano), Tool: "find_symbol", Service: "svc", Surface: "agent", LatencyMS: 100},
		{TS: now.Add(-30 * time.Second).Format(time.RFC3339Nano), Tool: "find_symbol", Service: "svc", Surface: "agent", LatencyMS: 200},
	})
	var out, errBuf bytes.Buffer
	code := cmdMetricsTraffic([]string{"--call-log", logPath, "--since", "1h", "--json"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errBuf.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload["total_calls"].(float64) != 2 {
		t.Fatalf("total_calls = %v, want 2", payload["total_calls"])
	}
}

func TestCmdMetricsTraffic_GroupByHour(t *testing.T) {
	now := time.Now().UTC()
	logPath := filepath.Join(t.TempDir(), "calls.jsonl")
	writeCallLogFixture(t, logPath, []adoptionCallEntry{
		{TS: now.Add(-3 * time.Hour).Format(time.RFC3339Nano), Tool: "find_symbol", Service: "svc", Surface: "agent", LatencyMS: 100},
		{TS: now.Add(-10 * time.Minute).Format(time.RFC3339Nano), Tool: "find_symbol", Service: "svc", Surface: "agent", LatencyMS: 100},
	})
	var out, errBuf bytes.Buffer
	code := cmdMetricsTraffic([]string{"--call-log", logPath, "--since", "6h", "--by", "hour", "--json"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errBuf.String())
	}
	var payload map[string]any
	json.Unmarshal(out.Bytes(), &payload)
	rows, _ := payload["rows"].([]any)
	if len(rows) != 2 {
		t.Fatalf("rows = %+v, want 2 hour buckets", rows)
	}
}

func TestCmdMetricsTraffic_ServiceFilter(t *testing.T) {
	now := time.Now().UTC()
	logPath := filepath.Join(t.TempDir(), "calls.jsonl")
	writeCallLogFixture(t, logPath, []adoptionCallEntry{
		{TS: now.Add(-time.Minute).Format(time.RFC3339Nano), Tool: "find_symbol", Service: "svc-a", Surface: "agent", LatencyMS: 100},
		{TS: now.Add(-30 * time.Second).Format(time.RFC3339Nano), Tool: "find_symbol", Service: "svc-b", Surface: "agent", LatencyMS: 100},
	})
	var out, errBuf bytes.Buffer
	code := cmdMetricsTraffic([]string{"--call-log", logPath, "--service", "svc-a", "--since", "1h", "--json"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errBuf.String())
	}
	var payload map[string]any
	json.Unmarshal(out.Bytes(), &payload)
	if payload["total_calls"].(float64) != 1 {
		t.Fatalf("total_calls = %v, want 1 (svc-a only)", payload["total_calls"])
	}
}

func TestCmdMetricsTraffic_MissingCallLog_AbsentNotZero(t *testing.T) {
	// No call log is not an idle daemon: the report keeps its window and
	// grouping, says the log is absent and why, and carries no count at
	// all (cmd/fathom/AGENTS.md rule 6 — JSON carries no zero for it).
	logPath := filepath.Join(t.TempDir(), "does-not-exist.jsonl")
	var out, errBuf bytes.Buffer
	code := cmdMetricsTraffic([]string{"--call-log", logPath, "--json"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errBuf.String())
	}
	var payload map[string]any
	json.Unmarshal(out.Bytes(), &payload)
	if _, ok := payload["total_calls"]; ok {
		t.Fatalf("total_calls = %v, want no count for a log that does not exist", payload["total_calls"])
	}
	if payload["state"] != "absent" || !strings.Contains(payload["reason"].(string), logPath) || payload["group_by"] != "surface" {
		t.Errorf("payload = %v, want state absent with the path as its reason", payload)
	}
	// A log that exists and cannot be read is an error naming the failure.
	dir := t.TempDir()
	out.Reset()
	if code := cmdMetricsTraffic([]string{"--call-log", dir, "--json"}, &out, &errBuf); code != 0 {
		t.Fatalf("exit = %d", code)
	}
	payload = nil
	json.Unmarshal(out.Bytes(), &payload)
	if payload["state"] != "error" || payload["total_calls"] != nil {
		t.Errorf("unreadable log payload = %v, want state error and no counts", payload)
	}
	out.Reset()
	cmdMetricsTraffic([]string{"--call-log", logPath}, &out, &errBuf)
	if !strings.Contains(out.String(), "not measured") {
		t.Errorf("text = %q, want the absence named", out.String())
	}
}
