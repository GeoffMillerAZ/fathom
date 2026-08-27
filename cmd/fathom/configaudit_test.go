package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConcernFixture(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".fathom"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A router-thinness concern with a 2-line budget, against a 4-line file.
	err := os.WriteFile(filepath.Join(root, ".fathom", "concerns.cue"), []byte(`concerns: {
	router_thin: {
		want:  "the root CLAUDE.md stays a router"
		check: {kind: "max_lines", file: "CLAUDE.md", lines: 2}
	}
}
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("1\n2\n3\n4\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Zero concerns is a NAMED absence and a success — a repo that never
// registered contracts is normal, not broken (law 3, the capability
// registry's discipline).
func TestConfigAuditNamesAbsence(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	var out, errB bytes.Buffer
	code := cmdConfigAudit([]string{"--repo", root}, &out, &errB)
	if code != 0 {
		t.Fatalf("exit %d, stderr %s", code, errB.String())
	}
	if !strings.Contains(out.String(), "no concerns registered") {
		t.Fatalf("absence must be named, got %q", out.String())
	}
	if !strings.Contains(out.String(), "concerns.cue") {
		t.Fatalf("output must teach where concerns live, got %q", out.String())
	}
}

func TestConfigAuditViolationExitsOne(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	writeConcernFixture(t, root)
	var out, errB bytes.Buffer
	code := cmdConfigAudit([]string{"--repo", root}, &out, &errB)
	if code != 1 {
		t.Fatalf("violation must exit 1, got %d (out %s)", code, out.String())
	}
	for _, want := range []string{"router_thin", "4 lines (max 2)", "the root CLAUDE.md stays a router"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output must carry %q, got %q", want, out.String())
		}
	}
}

func TestDoctorConcernsRow(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	clean := t.TempDir()
	row := checkConcerns(clean)
	if row.Status != doctorPass || !strings.Contains(row.Reason, "no concerns registered") {
		t.Fatalf("zero concerns must pass naming the absence, got %+v", row)
	}

	violated := t.TempDir()
	writeConcernFixture(t, violated)
	row = checkConcerns(violated)
	if row.Status != doctorFail {
		t.Fatalf("a violated concern is the red state, got %+v", row)
	}
	if !strings.Contains(row.Reason, "router_thin") {
		t.Fatalf("reason must name the concern, got %q", row.Reason)
	}
	if !strings.Contains(row.Remedy, "fathom config audit") {
		t.Fatalf("remedy must point at the verb, got %q", row.Remedy)
	}
}

// The attention line exists only while violations exist — and it reports;
// it never edits (vision 22 §3b).
func TestConcernAttentionLine(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	clean := t.TempDir()
	if line := concernAttentionLine(clean); line != "" {
		t.Fatalf("clean repo must add nothing, got %q", line)
	}

	violated := t.TempDir()
	writeConcernFixture(t, violated)
	line := concernAttentionLine(violated)
	if !strings.Contains(line, "fathom doctor") || !strings.Contains(line, "config 1") {
		t.Fatalf("attention line must count and point at the verb, got %q", line)
	}
}
