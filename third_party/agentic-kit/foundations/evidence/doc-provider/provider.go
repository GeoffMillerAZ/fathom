// Package docprovider defines the DocProvider interface and two in-memory
// implementations: MarkdownProvider and ReadmeProvider.
//
// Neither implementation touches the filesystem or network — content is
// supplied by the caller, making both suitable for deterministic tests and
// offline scenarios.
//
// The package is intentionally org-agnostic: it knows nothing about specific
// documentation portals or project structures.
//
// Boundary note: docprovider is a docs-content PORT — an interface for
// fetching documentation text by reference — not part of the evidence spine.
// It produces no Spans, carries no provenance, and grades nothing; none of
// the evidence-span/evidence-grade/ai-provenance types appear here. It lives
// under foundations/evidence because documentation content is a common raw
// input to evidence ingestion, but ingestion code must still encode what it
// extracts as evidence-span Spans before anything downstream treats it as
// evidence. See foundations/evidence/README.md for the canonical pipeline.
//
// Concurrency contract: MarkdownProvider and ReadmeProvider are immutable
// after construction and safe for concurrent use; Fetch holds no mutable
// state and reads only the fixed content map supplied at construction time.
package docprovider

import (
	"fmt"
	"strings"
)

// Doc is a fetched document.
type Doc struct {
	// ID is the stable identifier used to request this document.
	ID string

	// Title is a human-readable title extracted from the document. May be
	// empty if the content has no discernible title.
	Title string

	// Body is the full text of the document.
	Body string
}

// DocProvider is the interface implemented by any component that can fetch a
// document by reference string.
type DocProvider interface {
	// Fetch retrieves the document identified by ref. It returns an error when
	// no document is registered for that ref.
	Fetch(ref string) (Doc, error)
}

// MarkdownProvider is an in-memory DocProvider that serves Markdown documents.
// Content is registered via Register before any Fetch call.
//
// Title extraction: the first line that starts with "# " is used as the title.
type MarkdownProvider struct {
	docs map[string]Doc
}

// Register adds or replaces the document at ref with the given raw Markdown
// content. The title is extracted from the first H1 heading found in content.
func (p *MarkdownProvider) Register(ref, content string) {
	if p.docs == nil {
		p.docs = make(map[string]Doc)
	}
	p.docs[ref] = Doc{
		ID:    ref,
		Title: extractMarkdownTitle(content),
		Body:  content,
	}
}

// Fetch returns the document registered under ref, or an error if not found.
func (p *MarkdownProvider) Fetch(ref string) (Doc, error) {
	if p.docs == nil {
		return Doc{}, fmt.Errorf("docprovider: no document registered for ref %q", ref)
	}
	doc, ok := p.docs[ref]
	if !ok {
		return Doc{}, fmt.Errorf("docprovider: no document registered for ref %q", ref)
	}
	return doc, nil
}

// extractMarkdownTitle scans content for the first "# " heading and returns
// the heading text. Returns empty string if none found.
func extractMarkdownTitle(content string) string {
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}
	return ""
}

// ReadmeProvider is an in-memory DocProvider specialised for README files.
// It behaves identically to MarkdownProvider but signals to callers that the
// content was sourced from a README artifact.
type ReadmeProvider struct {
	inner MarkdownProvider
}

// Register adds or replaces the README document at ref with the given content.
func (p *ReadmeProvider) Register(ref, content string) {
	p.inner.Register(ref, content)
}

// Fetch returns the README document registered under ref, or an error if not
// found.
func (p *ReadmeProvider) Fetch(ref string) (Doc, error) {
	return p.inner.Fetch(ref)
}
