package docprovider_test

import (
	"strings"
	"testing"

	dp "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/doc-provider"
)

const markdownDoc = `# Payments Service

This document describes the payments service architecture.

## Overview

The payments service handles all financial transactions.
`

// TestMarkdownProviderFetch verifies a registered document is retrievable.
func TestMarkdownProviderFetch(t *testing.T) {
	var p dp.MarkdownProvider
	p.Register("payments/README.md", markdownDoc)

	doc, err := p.Fetch("payments/README.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.ID != "payments/README.md" {
		t.Errorf("ID: got %q, want %q", doc.ID, "payments/README.md")
	}
	if doc.Title != "Payments Service" {
		t.Errorf("Title: got %q, want %q", doc.Title, "Payments Service")
	}
	if !strings.Contains(doc.Body, "financial transactions") {
		t.Errorf("Body does not contain expected content: %q", doc.Body)
	}
}

// TestMarkdownProviderFetchMissing verifies an error is returned for unknown refs.
func TestMarkdownProviderFetchMissing(t *testing.T) {
	var p dp.MarkdownProvider
	_, err := p.Fetch("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown ref, got nil")
	}
}

// TestMarkdownProviderNoTitle verifies Title is empty when no H1 heading exists.
func TestMarkdownProviderNoTitle(t *testing.T) {
	var p dp.MarkdownProvider
	p.Register("plain.md", "No headings here.\nJust plain text.")

	doc, err := p.Fetch("plain.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.Title != "" {
		t.Errorf("expected empty title, got %q", doc.Title)
	}
}

// TestMarkdownProviderOverwrite verifies later Register calls overwrite.
func TestMarkdownProviderOverwrite(t *testing.T) {
	var p dp.MarkdownProvider
	p.Register("doc.md", "# Old Title\nOld body.")
	p.Register("doc.md", "# New Title\nNew body.")

	doc, err := p.Fetch("doc.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.Title != "New Title" {
		t.Errorf("expected 'New Title' after overwrite, got %q", doc.Title)
	}
}

// TestReadmeProviderFetch verifies ReadmeProvider delegates correctly.
func TestReadmeProviderFetch(t *testing.T) {
	var p dp.ReadmeProvider
	p.Register("repo/README.md", "# Core Repo\nCore service README.")

	doc, err := p.Fetch("repo/README.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.Title != "Core Repo" {
		t.Errorf("Title: got %q, want %q", doc.Title, "Core Repo")
	}
}

// TestReadmeProviderFetchMissing verifies ReadmeProvider returns error for unknown refs.
func TestReadmeProviderFetchMissing(t *testing.T) {
	var p dp.ReadmeProvider
	_, err := p.Fetch("nope")
	if err == nil {
		t.Fatal("expected error for unknown ref, got nil")
	}
}

// TestDocProviderInterface verifies both implementations satisfy the interface.
func TestDocProviderInterface(t *testing.T) {
	var mp dp.MarkdownProvider
	mp.Register("md.md", "# Markdown\nBody.")
	var rp dp.ReadmeProvider
	rp.Register("readme.md", "# Readme\nBody.")

	providers := []dp.DocProvider{&mp, &rp}
	refs := []string{"md.md", "readme.md"}

	for i, provider := range providers {
		doc, err := provider.Fetch(refs[i])
		if err != nil {
			t.Errorf("provider[%d].Fetch: unexpected error: %v", i, err)
		}
		if doc.ID != refs[i] {
			t.Errorf("provider[%d]: ID got %q, want %q", i, doc.ID, refs[i])
		}
	}
}

// TestZeroValueMarkdownProvider verifies zero value is safe to call Fetch on.
func TestZeroValueMarkdownProvider(t *testing.T) {
	var p dp.MarkdownProvider
	_, err := p.Fetch("anything")
	if err == nil {
		t.Fatal("zero-value MarkdownProvider: expected error, got nil")
	}
}
