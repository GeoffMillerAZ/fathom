package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type adoptionEraFixtureRow struct {
	Version string `json:"version"`
	Schema  int    `json:"schema"`
	Calls   int    `json:"calls"`
	FirstTS string `json:"first_ts"`
	LastTS  string `json:"last_ts"`
}

// Reject daemon requests before dialing; these fixtures use only local logs.
const adoptionEraTestURL = ":"

func TestInstrumentAdoptionEras(t *testing.T) {
	isolateVerifyLedger(t)
	url := adoptionEraTestURL
	root := t.TempDir()
	logPath := filepath.Join(root, "calls.jsonl")
	// File order deliberately differs from timestamp order. The console row
	// and malformed line must not enter either the report or era totals.
	fixture := `{"ts":"2026-07-23T11:00:00Z","tool":"brief","service":"service-a","schema_version":3,"fathom_version":"v0.2.0"}
{"ts":"2026-07-23T10:00:00.5Z","tool":"search","service":"service-a","schema_version":2,"fathom_version":"v0.1.0"}
{"ts":"2026-07-23T09:00:00Z","tool":"search","service":"service-a"}
{"ts":"2026-07-23T10:00:00Z","tool":"search","service":"service-b","schema_version":2,"fathom_version":"v0.1.0"}
{"ts":"2026-07-23T12:00:00Z","tool":"brief","service":"service-b","schema_version":3,"fathom_version":"v0.2.0"}
{"ts":"2026-07-23T09:30:00Z","tool":"search"}
{"ts":"2026-07-23T13:00:00Z","tool":"search","origin":"console","schema_version":3,"fathom_version":"v0.2.0"}
broken
`
	if err := os.WriteFile(logPath, []byte(fixture), 0o600); err != nil {
		t.Fatal(err)
	}
	want := []adoptionEraFixtureRow{
		{"unknown (predates version stamps)", 0, 2, "2026-07-23T09:00:00Z", "2026-07-23T09:30:00Z"},
		{"v0.1.0", 2, 2, "2026-07-23T10:00:00Z", "2026-07-23T10:00:00.5Z"},
		{"v0.2.0", 3, 2, "2026-07-23T11:00:00Z", "2026-07-23T12:00:00Z"},
	}
	for _, scoped := range []bool{false, true} {
		args := []string{"instrument", "--adoption", "--call-log", logPath, "--root", root, "--url", url, "--json"}
		if scoped {
			args = append(args, "--service", "service-a")
		}
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != 0 {
			t.Fatalf("exit=%d: %s", code, &stderr)
		}
		var report struct {
			TotalCalls int                     `json:"total_calls"`
			Eras       []adoptionEraFixtureRow `json:"eras"`
		}
		if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
			t.Fatal(err)
		}
		expected := want
		if scoped {
			expected = []adoptionEraFixtureRow{
				{want[0].Version, 0, 1, want[0].FirstTS, want[0].FirstTS},
				{want[1].Version, 2, 1, want[1].LastTS, want[1].LastTS},
				{want[2].Version, 3, 1, want[2].FirstTS, want[2].FirstTS},
			}
		}
		if !reflect.DeepEqual(report.Eras, expected) {
			t.Errorf("scoped=%v eras=%+v, want %+v", scoped, report.Eras, expected)
		}
		total := 0
		for _, era := range report.Eras {
			total += era.Calls
		}
		if total != report.TotalCalls || total != len(expected)*expected[0].Calls {
			t.Errorf("scoped=%v era total=%d, report total=%d", scoped, total, report.TotalCalls)
		}
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"instrument", "--adoption", "--call-log", logPath, "--root", root, "--url", url}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit=%d: %s", code, &stderr)
	}
	for _, want := range []string{"\neras:\n", "unknown (predates version stamps)", "v0.1.0", "v0.2.0", "schema=2", "schema=3", "calls=2", "first_ts=2026-07-23T10:00:00Z", "last_ts=2026-07-23T10:00:00.5Z"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("text missing %q:\n%s", want, &stdout)
		}
	}
}

func TestInstrumentAdoptionErasSortAndSchema(t *testing.T) {
	isolateVerifyLedger(t)
	url := adoptionEraTestURL
	root := t.TempDir()
	logPath := filepath.Join(root, "calls.jsonl")
	fixture := `{"ts":"2026-07-23T10:00:00Z","tool":"search","schema_version":3,"fathom_version":"v0.2.0"}
{"ts":"2026-07-23T10:00:00Z","tool":"search","schema_version":3,"fathom_version":"v0.1.0"}
{"ts":"2026-07-23T10:00:00Z","tool":"search","schema_version":2,"fathom_version":"v0.1.0"}
{"ts":"2026-07-23T11:00:00Z","tool":"search","fathom_version":"v0.1.0"}
{"ts":"2026-07-23T12:00:00Z","tool":"search","schema_version":2}
`
	if err := os.WriteFile(logPath, []byte(fixture), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"instrument", "--adoption", "--call-log", logPath, "--root", root, "--url", url, "--json"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit=%d: %s", code, &stderr)
	}
	var report struct {
		Eras []adoptionEraFixtureRow `json:"eras"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	want := []adoptionEraFixtureRow{
		{"v0.1.0", 2, 1, "2026-07-23T10:00:00Z", "2026-07-23T10:00:00Z"},
		{"v0.1.0", 3, 1, "2026-07-23T10:00:00Z", "2026-07-23T10:00:00Z"},
		{"v0.2.0", 3, 1, "2026-07-23T10:00:00Z", "2026-07-23T10:00:00Z"},
		{"v0.1.0", 0, 1, "2026-07-23T11:00:00Z", "2026-07-23T11:00:00Z"},
		{"unknown", 2, 1, "2026-07-23T12:00:00Z", "2026-07-23T12:00:00Z"},
	}
	if !reflect.DeepEqual(report.Eras, want) {
		t.Errorf("eras=%+v, want %+v", report.Eras, want)
	}
}

func TestInstrumentAdoptionErasEmpty(t *testing.T) {
	isolateVerifyLedger(t)
	url := adoptionEraTestURL
	root := t.TempDir()
	logPath := filepath.Join(root, "calls.jsonl")
	if err := os.WriteFile(logPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, jsonOut := range []bool{false, true} {
		args := []string{"instrument", "--adoption", "--call-log", logPath, "--root", root, "--url", url}
		if jsonOut {
			args = append(args, "--json")
		}
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != 0 {
			t.Fatalf("exit=%d: %s", code, &stderr)
		}
		if strings.Contains(stdout.String(), "eras") || stderr.Len() != 0 {
			t.Errorf("empty log: stdout=%s stderr=%s", &stdout, &stderr)
		}
	}
}
