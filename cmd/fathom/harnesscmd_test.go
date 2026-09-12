package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/harness"
)

// TestCmdHarnessList pins HP-8: `fathom harness list` names every
// registry id, and --json is the exported registry the CUE schema vets.
func TestCmdHarnessList(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"harness", "list"}, &out, &errb); code != 0 {
		t.Fatalf("exit = %d: %s", code, errb.String())
	}
	for _, id := range harness.IDs() {
		if !strings.Contains(out.String(), id) {
			t.Errorf("list lacks %s:\n%s", id, out.String())
		}
	}
	if !strings.Contains(out.String(), "unverified") {
		t.Errorf("list must show the unverified descriptors as such:\n%s", out.String())
	}

	out.Reset()
	if code := run([]string{"harness", "list", "--json"}, &out, &errb); code != 0 {
		t.Fatalf("exit = %d: %s", code, errb.String())
	}
	var descs []map[string]any
	if err := json.Unmarshal(out.Bytes(), &descs); err != nil {
		t.Fatalf("--json not a JSON array: %v", err)
	}
	if len(descs) != len(harness.IDs()) {
		t.Errorf("--json has %d descriptors, want %d", len(descs), len(harness.IDs()))
	}
	if descs[0]["id"] != harness.ClaudeCode {
		t.Errorf("registry order: first id = %v, want claude-code", descs[0]["id"])
	}
}

func TestCmdHarnessCapabilities(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"harness", "capabilities", "--harness", "codex"}, &out, &errb); code != 0 {
		t.Fatalf("exit = %d: %s", code, errb.String())
	}
	rows := 0
	cd, _ := harness.Lookup(harness.Codex)
	wantStatus := "verified"
	if cd.Unverified() {
		wantStatus = "unverified on this machine"
	}
	for _, line := range strings.Split(out.String(), "\n") {
		if !strings.HasPrefix(line, "| `") {
			continue
		}
		rows++
		// The status word is computed from verified_against, never typed:
		// codex read "unverified on this machine" before HP-34's live
		// session and "verified" after it. What the test pins is that the
		// column is never blank and never disagrees with the descriptor.
		if !strings.Contains(line, wantStatus) {
			t.Errorf("codex row lacks the honesty status %q: %s", wantStatus, line)
		}
	}
	if rows != len(harness.Capabilities) {
		t.Errorf("codex table has %d rows, want %d", rows, len(harness.Capabilities))
	}

	out.Reset()
	if code := run([]string{"harness", "capabilities", "--json"}, &out, &errb); code != 0 {
		t.Fatalf("exit = %d: %s", code, errb.String())
	}
	var entries []struct {
		ID   string `json:"id"`
		Rows []struct {
			Capability string `json:"capability"`
			Vehicle    string `json:"vehicle"`
			Status     string `json:"status"`
		} `json:"capabilities"`
	}
	if err := json.Unmarshal(out.Bytes(), &entries); err != nil {
		t.Fatalf("--json: %v", err)
	}
	if len(entries) != len(harness.IDs()) {
		t.Fatalf("--json entries = %d, want %d", len(entries), len(harness.IDs()))
	}
	for _, e := range entries {
		if len(e.Rows) != len(harness.Capabilities) {
			t.Errorf("%s: %d rows, want %d", e.ID, len(e.Rows), len(harness.Capabilities))
		}
	}

	out.Reset()
	errb.Reset()
	if code := run([]string{"harness", "capabilities", "--harness", "bogus"}, &out, &errb); code != 1 || !strings.Contains(errb.String(), "unknown harness") {
		t.Errorf("unknown id: exit %d stderr %q", code, errb.String())
	}
	if code := run([]string{"harness"}, &out, &errb); code != 1 {
		t.Errorf("no subcommand: exit %d", code)
	}
	if code := run([]string{"harness", "bogus"}, &out, &errb); code != 1 {
		t.Errorf("bad subcommand: exit %d", code)
	}
}
