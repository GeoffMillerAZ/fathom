package extractor_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

const mdFixture = `# Introduction

This is the intro paragraph.

## Installation

Run the install command.

### Advanced Setup

For advanced users.

## Usage

Basic usage examples.
`

func TestMarkdownExtractor_Basic(t *testing.T) {
	x := extractor.NewMarkdownExtractor()
	nodes, err := x.Extract("svc", "README.md", []byte(mdFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	headings := []string{"Introduction", "Installation", "Advanced Setup", "Usage"}
	for _, h := range headings {
		n, ok := byName[h]
		if !ok {
			t.Errorf("expected heading %q", h)
			continue
		}
		if n.Kind != core.KindDocChunk {
			t.Errorf("heading %q Kind: got %q, want KindDocChunk", h, n.Kind)
		}
		if n.Tier != core.TierDocClaimed {
			t.Errorf("heading %q Tier: got %q, want TierDocClaimed", h, n.Tier)
		}
	}

	if len(nodes) != 4 {
		t.Errorf("expected 4 nodes, got %d", len(nodes))
	}
}

func TestMarkdownExtractor_Signature(t *testing.T) {
	x := extractor.NewMarkdownExtractor()
	nodes, err := x.Extract("svc", "README.md", []byte(mdFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	cases := []struct {
		name string
		sig  string
	}{
		{"Introduction", "h1"},
		{"Installation", "h2"},
		{"Advanced Setup", "h3"},
		{"Usage", "h2"},
	}
	for _, tc := range cases {
		n, ok := byName[tc.name]
		if !ok {
			t.Errorf("heading %q not found", tc.name)
			continue
		}
		if n.Signature != tc.sig {
			t.Errorf("heading %q Signature: got %q, want %q", tc.name, n.Signature, tc.sig)
		}
	}
}

func TestMarkdownExtractor_LineNumbers(t *testing.T) {
	x := extractor.NewMarkdownExtractor()
	nodes, err := x.Extract("svc", "README.md", []byte(mdFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	// "# Introduction" is on line 1.
	if n, ok := byName["Introduction"]; ok {
		if n.Line != 1 {
			t.Errorf("Introduction line: got %d, want 1", n.Line)
		}
	}
}

// TestMarkdownExtractor_EndLine proves T5-5 (docs/plans/trust-and-
// tokenomics.md): each heading's EndLine is the real line before its next
// sibling heading (or EOF for the last one) — a section EXTENT, not just a
// start line, so a downstream caller can pin the section's real lines
// instead of guessing them by arithmetic off the heading (the T08 class).
// mdFixture's real line numbers (1-based, counting the raw string literal):
//
//	1  # Introduction
//	2  (blank)
//	3  This is the intro paragraph.
//	4  (blank)
//	5  ## Installation
//	6  (blank)
//	7  Run the install command.
//	8  (blank)
//	9  ### Advanced Setup
//	10 (blank)
//	11 For advanced users.
//	12 (blank)
//	13 ## Usage
//	14 (blank)
//	15 Basic usage examples.
func TestMarkdownExtractor_EndLine(t *testing.T) {
	x := extractor.NewMarkdownExtractor()
	nodes, err := x.Extract("svc", "README.md", []byte(mdFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	cases := []struct {
		name        string
		wantLine    int
		wantEndLine int
	}{
		{"Introduction", 1, 4},
		{"Installation", 5, 8},
		{"Advanced Setup", 9, 12},
		{"Usage", 13, 15}, // last heading: EndLine is EOF (total lines), not the next heading
	}
	for _, tc := range cases {
		n, ok := byName[tc.name]
		if !ok {
			t.Errorf("heading %q not found", tc.name)
			continue
		}
		if n.Line != tc.wantLine {
			t.Errorf("heading %q Line: got %d, want %d", tc.name, n.Line, tc.wantLine)
		}
		if n.EndLine != tc.wantEndLine {
			t.Errorf("heading %q EndLine: got %d, want %d", tc.name, n.EndLine, tc.wantEndLine)
		}
	}
}

// TestMarkdownExtractor_CodeFenceAwareness asserts that a "#" inside a fenced
// code block is NOT extracted as a heading — the key correctness win over the
// former regex-based approach.
const mdFencedFixture = `# Real Heading

This doc has a code block:

` + "```" + `bash
# This is a shell comment, not a heading
## Also not a heading
echo hello
` + "```" + `

## Another Real Heading

Done.
`

func TestMarkdownExtractor_CodeFenceAwareness(t *testing.T) {
	x := extractor.NewMarkdownExtractor()
	nodes, err := x.Extract("svc", "FENCED.md", []byte(mdFencedFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	// Only the two real headings should be extracted.
	if _, ok := byName["This is a shell comment, not a heading"]; ok {
		t.Error("extracted fenced code comment as heading — code-fence awareness broken")
	}
	if _, ok := byName["Also not a heading"]; ok {
		t.Error("extracted fenced code comment as heading — code-fence awareness broken")
	}

	if _, ok := byName["Real Heading"]; !ok {
		t.Error("expected heading 'Real Heading'")
	}
	if _, ok := byName["Another Real Heading"]; !ok {
		t.Error("expected heading 'Another Real Heading'")
	}

	if len(nodes) != 2 {
		t.Errorf("expected 2 nodes (only real headings), got %d", len(nodes))
		for _, n := range nodes {
			t.Logf("  node: %q", n.Name)
		}
	}
}

func TestMarkdownExtractor_Determinism(t *testing.T) {
	x := extractor.NewMarkdownExtractor()
	n1, err := x.Extract("svc", "README.md", []byte(mdFixture))
	if err != nil {
		t.Fatalf("first Extract: %v", err)
	}
	n2, err := x.Extract("svc", "README.md", []byte(mdFixture))
	if err != nil {
		t.Fatalf("second Extract: %v", err)
	}
	if len(n1) != len(n2) {
		t.Fatalf("non-deterministic length: %d vs %d", len(n1), len(n2))
	}
	for i := range n1 {
		if n1[i].Name != n2[i].Name || n1[i].Line != n2[i].Line {
			t.Errorf("non-deterministic at index %d: %+v vs %+v", i, n1[i], n2[i])
		}
	}
}
