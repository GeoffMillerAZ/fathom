// Package extractor defines the Extractor interface and per-language implementations
// that produce core.SymbolNode slices from raw source bytes.
package extractor

import (
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// Extractor parses a single source file and returns the set of top-level symbols
// it defines. service is the logical service ID (used in call-edge keys), relPath
// is the repo-relative file path (stored in SymbolNode.FilePath), and src holds
// the raw file bytes.
type Extractor interface {
	Extract(service, relPath string, src []byte) ([]core.SymbolNode, error)
}

// ReportingExtractor is an Extractor that reads best effort and swallows a
// failure to read the file (malformed input yields no nodes and no error).
// ExtractReport returns exactly what Extract returns and, when it could not
// fully read src, a one-line reason. Only the language library's door calls
// it, so the product's behavior and node output do not change. A doc-engine
// extractor says the same through DocResult.Incomplete.
type ReportingExtractor interface {
	Extractor
	ExtractReport(service, relPath string, src []byte) ([]core.SymbolNode, string, error)
}

// incompleteReason is the one-line reason an extractor gives for a file it
// could not fully read: what failed and the decoder's first line of why, with
// no stack and a bounded length.
func incompleteReason(what string, err error) string {
	msg := err.Error()
	if i := strings.IndexByte(msg, '\n'); i >= 0 {
		msg = msg[:i]
	}
	if len(msg) > 160 {
		msg = msg[:160]
	}
	return what + ": " + strings.TrimSpace(msg)
}
