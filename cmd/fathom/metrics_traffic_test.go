package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
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

func TestCmdMetricsTraffic_MissingCallLog_HonestZeros(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "does-not-exist.jsonl")
	var out, errBuf bytes.Buffer
	code := cmdMetricsTraffic([]string{"--call-log", logPath, "--json"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errBuf.String())
	}
	var payload map[string]any
	json.Unmarshal(out.Bytes(), &payload)
	if payload["total_calls"].(float64) != 0 {
		t.Fatalf("total_calls = %v, want 0", payload["total_calls"])
	}
	if payload["note"] == "" {
		t.Error("note is empty, want an honest empty-window note")
	}
}
