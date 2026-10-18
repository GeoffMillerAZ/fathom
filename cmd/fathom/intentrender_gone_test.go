package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestIntentRenderPlaybook_VerbRemoved pins IV-10 (ADR-074 decision 1): after
// its deprecation release the verb answers exactly as any unknown intent
// subcommand, and the retired INTENT.md stays gone.
func TestIntentRenderPlaybook_VerbRemoved(t *testing.T) {
	run := func(sub string) (int, string, string) {
		var o, e bytes.Buffer
		c := cmdIntent([]string{sub, "--playbook"}, &o, &e)
		return c, o.String(), e.String()
	}
	rc, ro, re := run("render")
	bc, bo, be := run("bogus")
	if rc != 1 || ro != "" {
		t.Fatalf("render: exit %d stdout %q", rc, ro)
	}
	if bc != rc || bo != ro {
		t.Fatalf("render differs from unknown subcommand: %d/%q vs %d/%q", rc, ro, bc, bo)
	}
	if !strings.Contains(re, "unknown intent subcommand \"render\"") {
		t.Fatalf("stderr %q", re)
	}
	if strings.Replace(re, "\"render\"", "\"bogus\"", 1) != be {
		t.Fatalf("stderr differs: %q vs %q", re, be)
	}
	if strings.Contains(be, "render") {
		t.Fatalf("usage list still names render: %q", be)
	}
	if _, err := os.Stat(filepath.Join("..", "..", "INTENT.md")); err == nil {
		t.Fatal("INTENT.md still exists")
	}
}
