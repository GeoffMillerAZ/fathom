package semantic

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

const sampleDoc = `Intro line before any heading.

# Gate

Everything must stay green:

## Rules

- sort outputs
- inject clocks

` + "```" + `
# not a heading (fenced)
` + "```" + `

## Rules

Duplicate heading text.
`

func TestSplitMarkdownSections(t *testing.T) {
	secs := SplitMarkdownSections("docs/gate.md", []byte(sampleDoc))
	if len(secs) != 4 {
		t.Fatalf("got %d sections, want 4: %+v", len(secs), subjects(secs))
	}
	wantSubjects := []string{
		"docs/gate.md#preamble",
		"docs/gate.md#gate",
		"docs/gate.md#rules",
		"docs/gate.md#rules-2",
	}
	for i, w := range wantSubjects {
		if secs[i].Subject != w {
			t.Errorf("section %d subject = %q, want %q", i, secs[i].Subject, w)
		}
	}

	// Line ranges are document coordinates: HashLines over a section's range
	// must hash exactly the section text (the compiler/sweep contract).
	for _, s := range secs {
		h1, ok := core.HashLines([]byte(sampleDoc), s.StartLine, s.EndLine)
		if !ok {
			t.Fatalf("section %s range %d-%d does not resolve", s.Subject, s.StartLine, s.EndLine)
		}
		h2, _ := core.HashLines([]byte(s.Text), 1, s.EndLine-s.StartLine+1)
		if h1 != h2 {
			t.Errorf("section %s: document-range hash != section-text hash", s.Subject)
		}
	}

	// Determinism.
	again := SplitMarkdownSections("docs/gate.md", []byte(sampleDoc))
	for i := range secs {
		if secs[i] != again[i] {
			t.Fatalf("non-deterministic split at %d", i)
		}
	}

	// The fenced pseudo-heading must not have split a section.
	for _, s := range secs {
		if s.Subject == "docs/gate.md#not-a-heading-fenced" {
			t.Error("fenced heading was treated as a section boundary")
		}
	}
}

func subjects(secs []Section) []string {
	out := make([]string, len(secs))
	for i, s := range secs {
		out[i] = s.Subject
	}
	return out
}
