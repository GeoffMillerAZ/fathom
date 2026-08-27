package adr_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/adr"
)

// TestParseADRSectionsAndTrailers (EP-1) parses the single-file decision log
// shape — `## ADR-NNN` sections with `**Status.**` and `**Revisit when.**`
// trailers — against a fixture carrying every wrinkle this repo's own
// ADR.md has: two heading separators, a default status, an explicit
// supersession, a bulleted condition, and two trailers on one record.
func TestParseADRSectionsAndTrailers(t *testing.T) {
	fixture := strings.Join([]string{
		"# Architecture Decision Records",
		"",
		"Status: `accepted` unless noted.",
		"",
		"## ADR-001 — Three-layer architecture",
		"",
		"**Decision.** Separate three layers.",
		"",
		"## ADR-003 — Storage: DuckDB",
		"",
		"**Status.** Superseded by ADR-020 (SQLite default) and ADR-025.",
		"",
		"**Decision.** Persist in DuckDB.",
		"",
		"## ADR-014: Path-scoped tool surfaces (2026-07-16)",
		"",
		"**Decision.** Scope by path.",
		"",
		"**Revisit when.** (1) The group surfaces' 30-day log runs.",
		"(2) A multi-tenant shape re-arms strictSurfaces.",
		"",
		"**Amendment 2 (2026-08-26).** Deleted on law 4.",
		"",
		"**Revisit when.** A hosted deployment appears.",
		"",
		"## ADR-026: The evidence plane (2026-08-26)",
		"",
		"**Revisit when.**",
		"- Any kill clock fires.",
		"- The kit refuses annotations.",
		"",
	}, "\n")

	secs := adr.ParseSections([]byte(fixture))
	if len(secs) != 4 {
		t.Fatalf("parsed %d sections, want 4: %+v", len(secs), adr.SectionIDs(secs))
	}

	byID := map[string]adr.Section{}
	for _, s := range secs {
		byID[s.ID] = s
	}

	if got := byID["ADR-001"]; got.Status != adr.StatusAccepted {
		t.Errorf("a section with no Status line means the file default (accepted), got %q", got.Status)
	} else if got.Title != "Three-layer architecture" {
		t.Errorf("em-dash heading title = %q", got.Title)
	} else if len(got.Revisit) != 0 {
		t.Errorf("ADR-001 has no trailer, got %v", got.Revisit)
	}

	if got := byID["ADR-003"]; got.Status != adr.StatusSuperseded {
		t.Errorf("ADR-003 status = %q, want superseded", got.Status)
	}

	if got := byID["ADR-014"]; got.Title != "Path-scoped tool surfaces (2026-07-16)" {
		t.Errorf("colon heading title = %q", got.Title)
	} else if len(got.Revisit) != 2 {
		t.Errorf("ADR-014 carries two trailers, got %d: %v", len(got.Revisit), got.Revisit)
	} else if !strings.Contains(got.Revisit[0], "multi-tenant") {
		t.Errorf("a wrapped trailer must keep its continuation line: %q", got.Revisit[0])
	}

	if got := byID["ADR-026"]; len(got.Revisit) != 1 {
		t.Fatalf("ADR-026 trailer count = %d", len(got.Revisit))
	} else if !strings.Contains(got.Revisit[0], "kill clock") || !strings.Contains(got.Revisit[0], "annotations") {
		t.Errorf("a bulleted trailer must survive whole: %q", got.Revisit[0])
	}

	if byID["ADR-014"].Line == 0 {
		t.Error("sections must carry their heading line for file:line evidence")
	}
}

// TestParseSectionsAgainstThisRepo runs the parser over the real ADR.md —
// the corpus ADR-026 actually reads. A parser that works only on a fixture
// is the "healthy empty" failure the repo keeps catching.
func TestParseSectionsAgainstThisRepo(t *testing.T) {
	root := repoRoot(t)
	body, err := os.ReadFile(filepath.Join(root, "ADR.md"))
	if err != nil {
		t.Skipf("ADR.md unreadable: %v", err)
	}
	secs := adr.ParseSections(body)
	if len(secs) < 20 {
		t.Fatalf("parsed %d sections from ADR.md — the heading shape changed", len(secs))
	}
	withTrailer := 0
	for _, s := range secs {
		if s.ID == "" {
			t.Errorf("section at line %d has no id", s.Line)
		}
		if len(s.Revisit) > 0 {
			withTrailer++
		}
	}
	if withTrailer == 0 {
		t.Error("no Revisit-when trailers found in ADR.md — the intent corpus would read empty")
	}
	t.Logf("ADR.md: %d sections, %d carrying a Revisit-when trailer", len(secs), withTrailer)
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Skip("repo root not found")
	return ""
}
