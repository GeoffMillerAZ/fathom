// Package fathomdocs embeds the documents the Observatory console renders
// on its Learn pages (vision 35 §3, OC-48): the use-case guides
// (use-cases/*.md, indexed by use-cases/README.md), the glossary
// (intent/domain.md's Vocabulary) and the laws (intent/principles.md). The
// binary carries the guides of its own build, so the console never reads a
// repository checkout to explain the tool it is — and a guide always
// describes the verbs of the binary serving it.
//
// dist/agents/skills is the precedent: a Go file beside the content it
// ships, nothing else.
package fathomdocs

import "embed"

// FS holds the embedded documents at their repository-relative paths under
// docs/ ("use-cases/fleet-health.md", "intent/domain.md").
//
//go:embed use-cases/*.md intent/domain.md intent/principles.md
var FS embed.FS
