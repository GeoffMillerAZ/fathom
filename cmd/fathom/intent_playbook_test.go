package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIntentRenderPlaybook_DeprecatedAndINTENTmdGone(t *testing.T) {
	root := t.TempDir()
	var o, e bytes.Buffer
	for _, args := range [][]string{{"--root", root, "--playbook"}, {"--root", root, "--playbook", "--check"}, {"--playbook", "--dry-run"}} {
		o.Reset()
		e.Reset()
		if c := cmdIntentRender(args, &o, &e); c != 0 {
			t.Fatalf("%v: exit %d", args, c)
		}
		if !strings.Contains(e.String(), "ADR-074") || !strings.Contains(e.String(), "deprecated") {
			t.Fatalf("%v: stderr %q lacks the notice", args, e.String())
		}
		if o.Len() != 0 {
			t.Fatalf("%v: stdout %q", args, o.String())
		}
	}
	if ents, _ := os.ReadDir(root); len(ents) != 0 {
		t.Fatalf("wrote %d entries", len(ents))
	}
	// This repository no longer carries the file.
	if _, err := os.Stat(filepath.Join("..", "..", "INTENT.md")); err == nil {
		t.Fatal("INTENT.md still exists")
	}
}
