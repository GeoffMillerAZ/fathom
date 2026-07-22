package connector

import (
	"bytes"
	"regexp"
)

// generatedHeaderRe matches the Go ecosystem's "generated file" convention
// (https://go.dev/s/generatedcode): a line containing "Code generated ...
// DO NOT EDIT" (case-insensitive, whitespace-tolerant). protoc-gen-go,
// mockgen, and stringer (ADR-021 §2 / TI-5's named examples) all emit this
// exact convention, so one regex covers all three — there is no separate
// per-tool marker to maintain.
var generatedHeaderRe = regexp.MustCompile(`(?i)code generated .*\bdo not edit\b`)

// generatedScanLines is the number of leading lines inspected for the
// generated-file header. The convention requires the marker to appear before
// the first non-comment/blank line, and in practice it is always the very
// first line; 3 gives room for a leading blank line or shebang without
// scanning the whole file.
const generatedScanLines = 3

// IsGenerated reports whether content's first few lines carry the
// "Code generated ... DO NOT EDIT" marker (ADR-021 §2 / TI-5), regardless of
// the comment syntax used to introduce it ("//", "#", "/* */", …) — the
// regex matches the marker text itself, not any particular comment prefix.
//
// Generated files are still extracted normally (unlike binary content, which
// IsBinary excludes outright): IsGenerated only tags the result so cloc and
// review tooling can separate "code you write" from "code a tool wrote for
// you" without re-deriving the signal.
func IsGenerated(content []byte) bool {
	if len(content) == 0 {
		return false
	}
	lines := bytes.SplitN(content, []byte("\n"), generatedScanLines+1)
	limit := len(lines)
	if limit > generatedScanLines {
		limit = generatedScanLines
	}
	for i := 0; i < limit; i++ {
		if generatedHeaderRe.Match(lines[i]) {
			return true
		}
	}
	return false
}
