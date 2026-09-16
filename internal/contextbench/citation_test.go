package contextbench

import "testing"

// TestResolveHeadingLine_TracksGrowth is item 3's failing-first case for
// contextbench's own resolver: a heading's line number is recomputed
// fresh from the CURRENT body on every call, so it survives new sections
// inserted above it — the "every ADR insertion breaks the corpus" defect
// a hardcoded truth_at_head line/needle pair cannot survive on its own.
func TestResolveHeadingLine_TracksGrowth(t *testing.T) {
	before := "# ADR log\n\n## ADR-001 — First\n\nbody\n\n## ADR-025 — DuckDB backend deleted\n\nbody\n"
	line, err := resolveHeadingLine(before, "ADR-025")
	if err != nil {
		t.Fatalf("resolveHeadingLine: %v", err)
	}
	if line != 7 {
		t.Fatalf("line = %d, want 7", line)
	}

	after := "# ADR log\n\n## ADR-001 — First\n\nbody\n\n## ADR-023 — Inserted\n\nnew\n\n## ADR-024 — Also inserted\n\nnew\n\n## ADR-025 — DuckDB backend deleted\n\nbody\n"
	grownLine, err := resolveHeadingLine(after, "ADR-025")
	if err != nil {
		t.Fatalf("resolveHeadingLine after growth: %v", err)
	}
	if grownLine == line {
		t.Fatalf("fixture did not actually move the heading")
	}
	if grownLine != 15 {
		t.Fatalf("grownLine = %d, want 15 (the heading's new line)", grownLine)
	}
}

func TestResolveHeadingLine_NotFound(t *testing.T) {
	if _, err := resolveHeadingLine("## ADR-001 — Only one\n", "ADR-999"); err == nil {
		t.Fatal("want an error for a heading that does not exist")
	}
}
