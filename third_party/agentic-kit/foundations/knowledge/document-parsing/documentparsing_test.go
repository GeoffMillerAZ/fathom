package documentparsing_test

import (
	"testing"

	dp "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/knowledge/document-parsing"
)

// TestParseEmptyDoc verifies that an empty document returns a single Segment
// with empty Heading, empty Text, and Order 0.
func TestParseEmptyDoc(t *testing.T) {
	got := dp.Parse("")
	if len(got) != 1 {
		t.Fatalf("expected 1 segment for empty doc, got %d", len(got))
	}
	s := got[0]
	if s.Heading != "" || s.Text != "" || s.Order != 0 {
		t.Errorf("unexpected segment for empty doc: %+v", s)
	}
}

// TestParseWhitespaceOnlyDoc verifies that a whitespace-only document returns
// a single Segment with empty Heading, empty Text, and Order 0.
func TestParseWhitespaceOnlyDoc(t *testing.T) {
	got := dp.Parse("   \n\t\n  ")
	if len(got) != 1 {
		t.Fatalf("expected 1 segment for whitespace doc, got %d", len(got))
	}
	if got[0].Heading != "" || got[0].Text != "" || got[0].Order != 0 {
		t.Errorf("unexpected segment: %+v", got[0])
	}
}

// TestParsePreambleOnly verifies that plain text with no headings produces
// one Segment with an empty Heading.
func TestParsePreambleOnly(t *testing.T) {
	doc := "Hello world.\nThis is a document."
	got := dp.Parse(doc)
	if len(got) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(got))
	}
	if got[0].Heading != "" {
		t.Errorf("expected empty Heading for preamble, got %q", got[0].Heading)
	}
	if got[0].Text != "Hello world.\nThis is a document." {
		t.Errorf("unexpected Text: %q", got[0].Text)
	}
	if got[0].Order != 0 {
		t.Errorf("expected Order 0, got %d", got[0].Order)
	}
}

// TestParseSingleHeading verifies that a document with one heading and body
// produces exactly two Segments: a preamble and a content segment.
func TestParseSingleHeading(t *testing.T) {
	doc := "Preamble text.\n# Section One\nBody of section one."
	got := dp.Parse(doc)
	if len(got) != 2 {
		t.Fatalf("expected 2 segments, got %d: %v", len(got), got)
	}

	if got[0].Heading != "" {
		t.Errorf("segment 0 Heading: want %q, got %q", "", got[0].Heading)
	}
	if got[0].Text != "Preamble text." {
		t.Errorf("segment 0 Text: want %q, got %q", "Preamble text.", got[0].Text)
	}
	if got[0].Order != 0 {
		t.Errorf("segment 0 Order: want 0, got %d", got[0].Order)
	}

	if got[1].Heading != "Section One" {
		t.Errorf("segment 1 Heading: want %q, got %q", "Section One", got[1].Heading)
	}
	if got[1].Text != "Body of section one." {
		t.Errorf("segment 1 Text: want %q, got %q", "Body of section one.", got[1].Text)
	}
	if got[1].Order != 1 {
		t.Errorf("segment 1 Order: want 1, got %d", got[1].Order)
	}
}

// TestParseMultipleHeadings verifies multi-heading documents are split correctly.
func TestParseMultipleHeadings(t *testing.T) {
	doc := "# Alpha\nContent alpha.\n## Beta\nContent beta.\n### Gamma\nContent gamma."
	got := dp.Parse(doc)
	// No preamble: first chunk is empty preamble + Alpha, Beta, Gamma
	// Actually preamble is empty ("" before first "#"), then Alpha, Beta, Gamma
	if len(got) != 4 {
		t.Fatalf("expected 4 segments (preamble + 3 headings), got %d: %v", len(got), got)
	}

	if got[0].Heading != "" || got[0].Text != "" {
		t.Errorf("preamble segment unexpected: %+v", got[0])
	}
	if got[1].Heading != "Alpha" || got[1].Text != "Content alpha." {
		t.Errorf("segment 1 unexpected: %+v", got[1])
	}
	if got[2].Heading != "Beta" || got[2].Text != "Content beta." {
		t.Errorf("segment 2 unexpected: %+v", got[2])
	}
	if got[3].Heading != "Gamma" || got[3].Text != "Content gamma." {
		t.Errorf("segment 3 unexpected: %+v", got[3])
	}
}

// TestParseOrderIsMonotonic verifies Order values are 0, 1, 2, … in sequence.
func TestParseOrderIsMonotonic(t *testing.T) {
	doc := "# A\n# B\n# C\n# D"
	got := dp.Parse(doc)
	for i, s := range got {
		if s.Order != i {
			t.Errorf("segment %d: expected Order %d, got %d", i, i, s.Order)
		}
	}
}

// TestParseDeterministic verifies that calling Parse twice on the same input
// produces identical results.
func TestParseDeterministic(t *testing.T) {
	doc := "Intro.\n# Sec 1\nBody 1.\n# Sec 2\nBody 2."
	a := dp.Parse(doc)
	b := dp.Parse(doc)

	if len(a) != len(b) {
		t.Fatalf("non-deterministic: first call %d segments, second %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("segment %d differs: %+v vs %+v", i, a[i], b[i])
		}
	}
}

// TestParseHeadingWithNoBody verifies that a heading with no following text
// produces a Segment with empty Text.
func TestParseHeadingWithNoBody(t *testing.T) {
	doc := "# Title\n# Immediate Next"
	got := dp.Parse(doc)
	// preamble (empty) + Title (empty body) + Immediate Next (empty body)
	if len(got) != 3 {
		t.Fatalf("expected 3 segments, got %d: %v", len(got), got)
	}
	if got[1].Heading != "Title" || got[1].Text != "" {
		t.Errorf("segment 1: want Heading=%q Text=%q, got %+v", "Title", "", got[1])
	}
}

// TestParseHashOnlyLineIsNotHeading verifies that a line of only '#' characters
// (with no space separator) is treated as body text, not a heading.
func TestParseHashOnlyLineIsNotHeading(t *testing.T) {
	doc := "##\nsome body"
	got := dp.Parse(doc)
	// No valid heading: the whole document is a single preamble segment.
	if len(got) != 1 {
		t.Fatalf("expected 1 segment (no heading), got %d: %v", len(got), got)
	}
	if got[0].Heading != "" {
		t.Errorf("expected empty heading, got %q", got[0].Heading)
	}
}

// TestParseTextIsStripped verifies that leading/trailing whitespace is stripped
// from the Text of each segment.
func TestParseTextIsStripped(t *testing.T) {
	doc := "# Section\n\n  body text  \n\n"
	got := dp.Parse(doc)
	// preamble (empty) + Section
	if len(got) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(got))
	}
	if got[1].Text != "body text" {
		t.Errorf("expected stripped text %q, got %q", "body text", got[1].Text)
	}
}
