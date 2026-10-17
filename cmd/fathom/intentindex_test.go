package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCmdIntentIndex_CheckNamesRowHeadingDrift(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "docs/intent/decisions"), 0o755)
	write := func(adr, idx string) {
		os.WriteFile(filepath.Join(root, "ADR.md"), []byte(adr), 0o644)
		os.WriteFile(filepath.Join(root, "docs/intent/decisions/README.md"), []byte(idx), 0o644)
	}
	run := func() (int, string, string) {
		var o, e bytes.Buffer
		c := cmdIntentIndex([]string{"--root", root, "--check"}, &o, &e)
		return c, o.String(), e.String()
	}
	write("## ADR-001 — a\n\n## ADR-002: b (2026)\n", "| ADR-001 | x |\n| ADR-002 | y |\n")
	if c, _, e := run(); c != 0 {
		t.Fatalf("parity: %d %q", c, e)
	}
	write("## ADR-001 — a\n\n## ADR-002: b\n\n## ADR-003: c\n", "| ADR-001 | x |\n| ADR-002 | y |\n| ADR-009 | z |\n")
	c, _, e := run()
	if c != 1 || !strings.Contains(e, "ADR-003: heading in ADR.md has no row") || !strings.Contains(e, "ADR-009: row in docs/intent/decisions/README.md has no heading") {
		t.Fatalf("drift: %d %q", c, e)
	}
	write("## ADR-001 — a\n", "| ADR-001 | x |\n| ADR-001 | x |\n")
	if c, _, e := run(); c != 1 || !strings.Contains(e, "ADR-001: 2 rows") {
		t.Fatalf("dup: %d %q", c, e)
	}
	var o, er bytes.Buffer
	if cmdIntentIndex(nil, &o, &er) != 1 {
		t.Fatal("bare invocation must fail")
	}
}

func TestCmdIntentIndex_RepoInParity(t *testing.T) {
	var o, e bytes.Buffer
	if c := cmdIntentIndex([]string{"--root", filepath.Join("..", ".."), "--check"}, &o, &e); c != 0 {
		t.Fatalf("this repo drifts: %s", e.String())
	}
}
