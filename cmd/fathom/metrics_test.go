package main

import (
	"bytes"
	"path/filepath"
	"testing"
)

// TestCmdMetrics_DispatchesEachVerb (Q-23): each verb name reaches its own
// handler rather than falling through to the unknown-verb error — driven
// with just enough flags for each verb to exit 0 against empty/absent
// inputs (every verb degrades honestly on empty input, proven by its own
// dedicated test file).
func TestCmdMetrics_DispatchesEachVerb(t *testing.T) {
	missingLog := filepath.Join(t.TempDir(), "none.jsonl")
	cases := []struct {
		verb string
		args []string
	}{
		{"traffic", []string{"--call-log", missingLog}},
		{"channels", []string{"--hooks-log", missingLog}},
		{"interventions", []string{"--call-log", missingLog}},
		{"series", []string{"spend_total_usd", "--call-log", missingLog, "--root", t.TempDir()}},
		{"rollup", []string{"--root", t.TempDir(), "--call-log", missingLog}},
		{"rotate", []string{"--call-log", missingLog}},
	}
	for _, c := range cases {
		t.Run(c.verb, func(t *testing.T) {
			var out, errBuf bytes.Buffer
			code := cmdMetrics(append([]string{c.verb}, c.args...), &out, &errBuf)
			if code != 0 {
				t.Fatalf("verb %q exit = %d, stderr: %s", c.verb, code, errBuf.String())
			}
		})
	}
}

// spend is dispatched separately: it requires --store/--service (a real
// error, not a dispatch failure) — this asserts it reaches cmdMetricsSpend
// (the "--store is required" message), not the unknown-verb error.
func TestCmdMetrics_DispatchesSpend(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := cmdMetrics([]string{"spend"}, &out, &errBuf)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (missing --store/--service)", code)
	}
	if !bytes.Contains(errBuf.Bytes(), []byte("--store")) {
		t.Errorf("stderr = %q, want the --store-required message (proves dispatch reached cmdMetricsSpend)", errBuf.String())
	}
}

func TestCmdMetrics_UnknownVerb_ErrorExit(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := cmdMetrics([]string{"not-a-real-verb"}, &out, &errBuf)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
}

func TestCmdMetrics_NoArgs_ErrorExit(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := cmdMetrics(nil, &out, &errBuf)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
}

// TestRun_MetricsDispatch_ThroughMain proves main.go's own dispatch table
// wires "metrics" through to cmdMetrics (not just that cmdMetrics itself
// works in isolation).
func TestRun_MetricsDispatch_ThroughMain(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run([]string{"metrics", "channels", "--hooks-log", filepath.Join(t.TempDir(), "none.jsonl")}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errBuf.String())
	}
}
