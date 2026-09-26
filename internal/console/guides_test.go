package console

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func readDoc(t *testing.T, rel string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "docs", rel))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestGuides_ParseTheShippedDocuments holds the guide projections to the
// documents fathom ships: the use-case index lists every guide file once,
// in the README's order and with its description; the glossary is the
// intent corpus's vocabulary with each term's source; the laws are laws
// 1–15 in order, each with its principle; and the rendered glossary and
// laws give every section a stable anchor.
func TestGuides_ParseTheShippedDocuments(t *testing.T) {
	entries := ParseGuideIndex(readDoc(t, "use-cases/README.md"))
	files, _ := filepath.Glob(filepath.Join("..", "..", "docs", "use-cases", "*.md"))
	if len(entries) == 0 || len(entries) != len(files)-1 {
		t.Fatalf("the README indexes %d guides; %d guide files ship (README excluded)", len(entries), len(files)-1)
	}
	seen := map[string]bool{}
	for _, e := range entries {
		if seen[e.Slug] || !ValidGuideSlug(e.Slug) || e.Description == "" || strings.Contains(e.Description, "**") || e.Kind != GuideUseCase {
			t.Errorf("index entry %+v", e)
		}
		seen[e.Slug] = true
		if _, err := os.Stat(filepath.Join("..", "..", e.Source)); err != nil {
			t.Errorf("%s: %v", e.Source, err)
		}
	}
	if entries[0].Slug != "coding-agents" {
		t.Errorf("the README's order must hold: first is %q", entries[0].Slug)
	}
	if got := GuideTitle(readDoc(t, "use-cases/fleet-health.md")); got == "" || strings.HasPrefix(got, "#") {
		t.Errorf("fleet-health title = %q", got)
	}

	terms := ParseGlossary(readDoc(t, "intent/domain.md"))
	if len(terms) < 20 {
		t.Fatalf("glossary has %d terms", len(terms))
	}
	for _, term := range terms {
		if term.Term == "" || term.Definition == "" || strings.Contains(term.Definition, "Source:") {
			t.Errorf("term %+v", term)
		}
	}
	if terms[0].Term != "observation" || terms[0].Source == "" {
		t.Errorf("first term = %+v", terms[0])
	}

	laws := ParseLaws(readDoc(t, "intent/principles.md"))
	if len(laws) != 15 {
		t.Fatalf("laws = %d, want 15", len(laws))
	}
	for i, l := range laws {
		if l.Number != i+1 || l.Principle != "P"+strconv.Itoa(i+1) || l.Title == "" || l.Text == "" {
			t.Errorf("law %d = %+v", i+1, l)
		}
	}
	if laws[2].Title != "Never silent" || !strings.Contains(laws[2].Text, "absent, never as zero") {
		t.Errorf("law 3 = %+v", laws[2])
	}

	gmd := string(GlossaryMarkdown(terms))
	lmd := string(LawsMarkdown(laws))
	if !strings.HasPrefix(gmd, "# Glossary\n") || !strings.Contains(gmd, "## observation {#term-1}") {
		t.Errorf("glossary markdown:\n%.300s", gmd)
	}
	if !strings.HasPrefix(lmd, "# The laws\n") || !strings.Contains(lmd, "## Law 13 — ") || !strings.Contains(lmd, "{#law-13}") {
		t.Errorf("laws markdown:\n%.300s", lmd)
	}
	if PlainInline("**[a `b`](x.md)** and _c_") != "a b and c" {
		t.Errorf("PlainInline = %q", PlainInline("**[a `b`](x.md)** and _c_"))
	}
}
