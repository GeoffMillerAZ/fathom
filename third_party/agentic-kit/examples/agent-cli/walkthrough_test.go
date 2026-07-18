package main

// TestFirstAgentWalkthrough is the golden check for A6 (docs/plans/
// handoff-h8-public-candidacy.md): it proves docs/first-agent.md's advertised
// mock-mode output is not just prose but a live, checkable claim. It reads
// the doc, extracts the fenced block that follows the "You should see output
// like:" anchor (Option A's expected output), runs the real CLI through the
// same in-process runCLI helper main_test.go already uses, and diffs the two.
// A future doc/behavior drift — either side changing without the other —
// fails this test loudly instead of silently going stale.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// firstAgentDocPath is docs/first-agent.md relative to this package's
// directory (examples/agent-cli/).
const firstAgentDocPath = "../../docs/first-agent.md"

// firstFencedBlockAfter returns the contents of the first ``` ... ``` fenced
// block that appears after the first occurrence of anchor in doc. It fails
// the test (via t.Fatalf) if the anchor or a following fence pair is not
// found, rather than returning a zero value that could make a broken doc
// look like a passing comparison.
func firstFencedBlockAfter(t *testing.T, doc, anchor string) string {
	t.Helper()

	anchorIdx := strings.Index(doc, anchor)
	if anchorIdx < 0 {
		t.Fatalf("docs/first-agent.md: anchor %q not found", anchor)
	}
	rest := doc[anchorIdx+len(anchor):]

	openIdx := strings.Index(rest, "```")
	if openIdx < 0 {
		t.Fatalf("docs/first-agent.md: no fenced block opens after anchor %q", anchor)
	}
	afterOpen := rest[openIdx+len("```"):]
	// Skip the rest of the opening fence line itself (in case it carries a
	// language tag like ```sh); the block's content starts on the next line.
	if nl := strings.Index(afterOpen, "\n"); nl >= 0 {
		afterOpen = afterOpen[nl+1:]
	} else {
		t.Fatalf("docs/first-agent.md: opening fence after anchor %q has no closing newline", anchor)
	}

	closeIdx := strings.Index(afterOpen, "```")
	if closeIdx < 0 {
		t.Fatalf("docs/first-agent.md: fenced block after anchor %q is never closed", anchor)
	}
	return afterOpen[:closeIdx]
}

func TestFirstAgentWalkthrough(t *testing.T) {
	docBytes, err := os.ReadFile(filepath.Join("..", "..", "docs", "first-agent.md"))
	if err != nil {
		t.Fatalf("read %s: %v", firstAgentDocPath, err)
	}

	want := firstFencedBlockAfter(t, string(docBytes), "You should see output like:")

	code, stdout, stderr := runCLI(t, "-mode", "mock")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr:\n%s", code, stderr)
	}

	// Trim trailing whitespace/newlines consistently on both sides: Markdown
	// fencing and the CLI's own trailing "\n" are formatting details, not
	// part of the claim. Everything else must match byte for byte, so real
	// drift (a changed turn count, token count, or final answer) still fails
	// loudly.
	gotTrimmed := strings.TrimRight(stdout, "\n")
	wantTrimmed := strings.TrimRight(want, "\n")

	if gotTrimmed != wantTrimmed {
		t.Errorf("docs/first-agent.md's advertised mock-mode output has drifted from the real CLI output.\n"+
			"docs/first-agent.md says:\n%s\n\nactual `go run . -mode mock` output:\n%s",
			wantTrimmed, gotTrimmed)
	}
}
