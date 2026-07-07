// Package extractor defines the Extractor interface and per-language implementations
// that produce core.SymbolNode slices from raw source bytes.
package extractor

import "github.com/geoffmilleraz/fathom/internal/core"

// Extractor parses a single source file and returns the set of top-level symbols
// it defines. service is the logical service ID (used in call-edge keys), relPath
// is the repo-relative file path (stored in SymbolNode.FilePath), and src holds
// the raw file bytes.
type Extractor interface {
	Extract(service, relPath string, src []byte) ([]core.SymbolNode, error)
}
