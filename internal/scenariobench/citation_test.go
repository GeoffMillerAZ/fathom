package scenariobench

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestResolveCitationLine_LineForm pins the pre-existing "path#Lnn"
// behavior: the citation's own number, unchanged, no filesystem read
// needed (path need not even exist).
func TestResolveCitationLine_LineForm(t *testing.T) {
	path, line, err := ResolveCitationLine("/does/not/exist", "ADR.md#L823")
	if err != nil {
		t.Fatalf("ResolveCitationLine: %v", err)
	}
	if path != "ADR.md" || line != 823 {
		t.Errorf("got (%q, %d), want (\"ADR.md\", 823)", path, line)
	}
}

// TestResolveCitationLine_HeadingForm is item 3's failing-first case: a
// heading-anchored citation ("ADR.md#ADR-025") resolves to wherever the
// "## ADR-025 …" heading CURRENTLY sits — proving the whole point of the
// feature by inserting new content above the heading between the two
// resolutions and asserting the line number tracks the move.
func TestResolveCitationLine_HeadingForm(t *testing.T) {
	dir := t.TempDir()
	adr := filepath.Join(dir, "ADR.md")
	original := "# ADR log\n\n## ADR-001 — First\n\nbody\n\n## ADR-025 — DuckDB backend deleted\n\nbody\nmore body\n"
	if err := os.WriteFile(adr, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	path, line, err := ResolveCitationLine(dir, "ADR.md#ADR-025")
	if err != nil {
		t.Fatalf("ResolveCitationLine: %v", err)
	}
	if path != "ADR.md" {
		t.Errorf("path = %q, want ADR.md", path)
	}
	wantLine := strings.Count(original[:strings.Index(original, "## ADR-025")], "\n") + 1
	if line != wantLine {
		t.Errorf("line = %d, want %d (the heading's own line)", line, wantLine)
	}

	// Insert two new ADR records ABOVE ADR-025 — exactly the "every ADR
	// insertion breaks the corpus" scenario a hardcoded #L823 citation
	// cannot survive.
	grown := "# ADR log\n\n## ADR-001 — First\n\nbody\n\n## ADR-023 — Inserted later\n\nnew body\nnew body 2\n\n## ADR-024 — Also inserted\n\nmore\n\n## ADR-025 — DuckDB backend deleted\n\nbody\nmore body\n"
	if err := os.WriteFile(adr, []byte(grown), 0o644); err != nil {
		t.Fatal(err)
	}

	_, grownLine, err := ResolveCitationLine(dir, "ADR.md#ADR-025")
	if err != nil {
		t.Fatalf("ResolveCitationLine after growth: %v", err)
	}
	if grownLine == wantLine {
		t.Fatalf("test fixture did not actually move the heading — grownLine %d == original wantLine %d", grownLine, wantLine)
	}
	wantGrownLine := strings.Count(grown[:strings.Index(grown, "## ADR-025")], "\n") + 1
	if grownLine != wantGrownLine {
		t.Errorf("after ADR.md grew, line = %d, want %d (the heading's NEW line, tracked automatically)", grownLine, wantGrownLine)
	}
}

func TestResolveCitationLine_HeadingNotFound(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ADR.md"), []byte("## ADR-001 — Only one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ResolveCitationLine(dir, "ADR.md#ADR-999"); err == nil {
		t.Fatal("want an error for a heading that does not exist")
	}
}

func TestResolveCitationLine_MalformedSource(t *testing.T) {
	if _, _, err := ResolveCitationLine(".", "ADR.md"); err == nil {
		t.Fatal("want an error for a source with no #anchor at all")
	}
}
