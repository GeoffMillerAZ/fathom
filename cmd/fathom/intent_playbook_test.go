package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const playbookFixture = "testdata/intentplaybook"

func TestBoldLeadIDsWrappedAndBare(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(playbookFixture, "docs/intent/scope.md"))
	if err != nil {
		t.Fatal(err)
	}
	got := boldLeadIDs(sectionOf(string(body), "Current goals"))
	want := []string{
		"**G1** — A single-line goal",
		"**G2** — A wrapped goal title that runs onto a second line and a third line (ADR-1)",
		"**G3** — A title with no trailing period",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}

func TestRenderPlaybookIntentGolden(t *testing.T) {
	got := renderPlaybookIntent(playbookFixture)
	wantB, err := os.ReadFile(filepath.Join(playbookFixture, "want.md"))
	if err != nil {
		t.Fatal(err)
	}
	if got != string(wantB) {
		t.Fatalf("render differs from testdata/intentplaybook/want.md:\n%s", got)
	}
	if n := strings.Count(got, "\n"); n > playbookMaxLines {
		t.Fatalf("body is %d lines, cap %d", n, playbookMaxLines)
	}
	if strings.Contains(got, "Q2") {
		t.Fatal("answered question rendered")
	}
}

func TestRenderPlaybookIntentCapsLongLists(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "docs/intent")
	os.MkdirAll(dir, 0o755)
	var q, g strings.Builder
	g.WriteString("## Current goals\n\n")
	for i := 1; i <= 80; i++ {
		id := strconv.Itoa(i)
		q.WriteString("**Q" + id + ". Open?** Blocks.\n\n")
		g.WriteString("**G" + id + ". A goal.** Done when.\n\n")
	}
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# R\n\nhello\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "questions.md"), []byte(q.String()), 0o644)
	os.WriteFile(filepath.Join(dir, "scope.md"), []byte("# S\n\n"+g.String()), 0o644)
	got := renderPlaybookIntent(root)
	if n := strings.Count(got, "\n"); n != playbookMaxLines {
		t.Fatalf("body is %d lines, want exactly the cap %d", n, playbookMaxLines)
	}
	if !strings.Contains(got, "...and ") {
		t.Fatal("truncation not announced")
	}
}

func TestIntentRenderCheck(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "docs/intent")
	os.MkdirAll(dir, 0o755)
	for _, f := range []string{"README.md", "scope.md", "invariants.md", "questions.md"} {
		b, _ := os.ReadFile(filepath.Join(playbookFixture, "docs/intent", f))
		os.WriteFile(filepath.Join(dir, f), b, 0o644)
	}
	run := func(args ...string) (int, string, string) {
		var o, e bytes.Buffer
		c := cmdIntentRender(append([]string{"--root", root, "--playbook"}, args...), &o, &e)
		return c, o.String(), e.String()
	}
	if c, _, e := run("--check"); c != 1 || !strings.Contains(e, "INTENT.md") {
		t.Fatalf("missing file: code %d %q", c, e)
	}
	if _, err := os.Stat(filepath.Join(root, "INTENT.md")); err == nil {
		t.Fatal("--check wrote the file")
	}
	if c, _, e := run(); c != 0 {
		t.Fatalf("render: %d %s", c, e)
	}
	if c, _, e := run("--check"); c != 0 {
		t.Fatalf("current: %d %s", c, e)
	}
	first, _ := os.ReadFile(filepath.Join(root, "INTENT.md"))
	run()
	if again, _ := os.ReadFile(filepath.Join(root, "INTENT.md")); !bytes.Equal(first, again) {
		t.Fatal("re-render is not idempotent")
	}
	// Corpus drifts: stale, names the file and a line, does not write.
	q := filepath.Join(dir, "questions.md")
	b, _ := os.ReadFile(q)
	os.WriteFile(q, append(b, []byte("\n**Q9. New one?** x.\n")...), 0o644)
	before, _ := os.ReadFile(filepath.Join(root, "INTENT.md"))
	c, _, e := run("--check")
	if c != 1 || !strings.Contains(e, "INTENT.md: stale at line") {
		t.Fatalf("stale: %d %q", c, e)
	}
	after, _ := os.ReadFile(filepath.Join(root, "INTENT.md"))
	if !bytes.Equal(before, after) {
		t.Fatal("--check wrote")
	}
	// Markers removed.
	os.WriteFile(filepath.Join(root, "INTENT.md"), []byte("# hand written\n"), 0o644)
	if c, _, e := run("--check"); c != 1 || !strings.Contains(e, "markers missing") {
		t.Fatalf("unmarked: %d %q", c, e)
	}
}
