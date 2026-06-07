package extractor_test

import (
	"testing"

	"github.com/geoffmilleraz/sextant/internal/core"
	"github.com/geoffmilleraz/sextant/internal/extractor"
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
}
