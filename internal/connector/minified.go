package connector

import (
	"bytes"
	"regexp"
)

// The generated/minified gate (LB-F10, docs/design/language-breadth.md §5,
// ADR-048 §7): a file whose head carries a generator marker, or whose first
// 64 KiB averages more than 500 characters a line, is fingerprinted and
// counted as skipped_generated and never handed to a dialect engine.
const (
	// minifiedScanBytes bounds the average-line-length sample.
	minifiedScanBytes = 64 << 10
	// minifiedAvgLine is the average line length a file must EXCEED to be
	// treated as minified.
	minifiedAvgLine = 500
	// generatedMarkerScanBytes bounds the marker scan: generators put the
	// marker in the header.
	generatedMarkerScanBytes = 2 << 10
)

// atGeneratedRe is the `@generated` marker (Meta/Relay/GraphQL-codegen
// style), a whole token.
var atGeneratedRe = regexp.MustCompile(`@generated\b`)

// IsGeneratedOrMinified reports whether content is generated or minified
// (LB-F10): the IsGenerated convention; a `@generated` or `Code generated …
// DO NOT EDIT` marker on a comment line in the first 2 KiB; or an average
// line length over 500 in the first 64 KiB. A marker must sit on a line
// that starts with one of the language's comment tokens (or, for a JSDoc
// block, "*"), so a string literal or prose quoting it is not a marker; a
// language without comments (json) is judged by line length alone.
func IsGeneratedOrMinified(path string, content []byte) bool {
	if len(content) == 0 {
		return false
	}
	if IsGenerated(path, content) || hasGeneratedMarkerHead(path, content) {
		return true
	}
	return isMinified(content)
}

func hasGeneratedMarkerHead(path string, content []byte) bool {
	tokens, known := langCommentTokens[LanguageForPath(path)]
	if !known {
		tokens = unclassifiedCommentTokens
	}
	if len(tokens) == 0 {
		return false
	}
	for _, t := range tokens {
		if t == "//" || t == "/*" {
			// C-family block comments: a JSDoc/Javadoc continuation line.
			tokens = append(append([]string(nil), tokens...), "/*", "*")
			break
		}
	}
	head := content
	if len(head) > generatedMarkerScanBytes {
		head = head[:generatedMarkerScanBytes]
	}
	for _, line := range bytes.Split(head, []byte("\n")) {
		line = bytes.TrimLeft(line, " \t")
		if !hasAnyPrefix(line, tokens) {
			continue
		}
		if atGeneratedRe.Match(line) || generatedMarkerRe.Match(line) {
			return true
		}
	}
	return false
}

// isMinified: the first 64 KiB's average line length exceeds 500. A
// trailing newline does not open a line.
func isMinified(content []byte) bool {
	head := content
	if len(head) > minifiedScanBytes {
		head = head[:minifiedScanBytes]
	}
	lines := bytes.Count(head, []byte("\n"))
	if head[len(head)-1] != '\n' {
		lines++
	}
	return len(head) > minifiedAvgLine*lines
}
