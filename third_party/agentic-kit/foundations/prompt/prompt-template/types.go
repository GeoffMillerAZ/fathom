// Package prompttemplate renders versioned prompt templates and produces the
// deterministic reproduction hashes that AI provenance records require.
//
// Purpose: foundations/provenance/ai-provenance.AIAuthor carries PromptHash,
// PromptTemplateVersion, and ParamsHash, but nothing else in the tree can
// produce them. This package closes that gap: Render executes a Go
// text/template against a params map and returns the rendered prompt text
// together with all three provenance values.
//
// Composition: CUE owns the Template shape (see schema.cue); Go owns
// behaviour. The package imports exactly one sibling foundation —
// ai-provenance — and only from provenance.go, keeping the coupling one-way
// and optional: ai-provenance knows nothing about this package, and consumers
// who skip provenance never call into it. There is deliberately no template
// registry; callers own storage and lookup of their templates.
//
// Concurrency contract: safe for unrestricted concurrent use. All exported
// types are plain immutable values and all functions are pure — no shared
// state, no clocks, no I/O.
package prompttemplate

import (
	"fmt"
	"text/template"
)

// Template is a named, versioned Go text/template source for a prompt.
// CUE mirror: #Template in schema.cue.
type Template struct {
	// Name identifies the template (e.g. "summarize-incident"). It is used
	// as the text/template name and appears in error messages.
	Name string

	// Version is the semver-style version of the template text. It flows
	// through Render into Rendered.TemplateVersion and from there into
	// aiprovenance.AIAuthor.PromptTemplateVersion.
	Version string

	// Text is the Go text/template source. Rendering enforces
	// missingkey=error, so every map key the template references must be
	// present in the params passed to Render.
	Text string
}

// Validate returns an error if the Template violates any invariant:
//   - Name, Version, and Text must be non-empty.
//   - Text must parse as a Go text/template.
//
// Validate checks parseability only; missing-parameter detection happens at
// Render time, where the params are known (rendering enforces
// missingkey=error).
func (t Template) Validate() error {
	if t.Name == "" {
		return fmt.Errorf("prompttemplate: Name must not be empty")
	}
	if t.Version == "" {
		return fmt.Errorf("prompttemplate: Version must not be empty")
	}
	if t.Text == "" {
		return fmt.Errorf("prompttemplate: Text must not be empty")
	}
	if _, err := template.New(t.Name).Parse(t.Text); err != nil {
		return fmt.Errorf("prompttemplate: Text of template %q does not parse: %w", t.Name, err)
	}
	return nil
}

// Rendered is the deterministic result of rendering a Template against a
// params map. Its three hash/version fields map one-to-one onto the
// prompt-derived fields of aiprovenance.AIAuthor (see ApplyTo in
// provenance.go).
type Rendered struct {
	// Text is the fully-rendered prompt sent to the model.
	Text string

	// PromptHash is the lowercase hex SHA-256 of Text.
	// Maps to aiprovenance.AIAuthor.PromptHash.
	PromptHash string

	// TemplateVersion is the Version of the Template that produced Text.
	// Maps to aiprovenance.AIAuthor.PromptTemplateVersion.
	TemplateVersion string

	// ParamsHash is the lowercase hex SHA-256 of the canonical JSON
	// encoding of the params map (see Render for the canonical-form
	// guarantees). Maps to aiprovenance.AIAuthor.ParamsHash.
	ParamsHash string
}

// TextHash returns the lowercase hex SHA-256 of Text — the template's
// content-addressed identity. Downstream digest gates key on it so a prompt
// edit is automatically a new compile identity (a hand-bumped Version string
// silently kept 27/32 stale sections in the motivating failure). It hashes
// the TEMPLATE, not a render: Rendered.PromptHash varies per params; TextHash
// varies only when the prompt text itself changes.
func (t Template) TextHash() string {
	return sha256Hex([]byte(t.Text))
}
